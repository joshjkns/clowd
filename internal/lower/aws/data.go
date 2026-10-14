package aws

import (
	"strings"

	"github.com/joshjkns/clowd/internal/emit/tfjson"
	"github.com/joshjkns/clowd/internal/ir"
)

// dbUser is the master user of every database. Its password is generated
// and kept in Secrets Manager by RDS, so it never appears in the
// configuration or the state.
const dbUser = "clowd"

// instanceClasses map database sizes to RDS instance classes. Graviton
// classes are the cheapest at each size and run every supported version.
var instanceClasses = map[ir.Size]string{
	ir.Small:  "db.t4g.micro",
	ir.Medium: "db.t4g.medium",
	ir.Large:  "db.m7g.large",
}

func dbPort(e ir.Engine) int {
	if e == ir.MySQL {
		return 3306
	}
	return 5432
}

// dbName is the database created inside the instance; both engines take
// letters, digits and underscores.
func dbName(name string) string { return strings.ReplaceAll(id(name), "-", "_") }

// database lowers d to an RDS instance in the private subnets, reachable
// only from the services that connect to it.
//
// `clowd destroy` deletes it without a final snapshot: destroy means
// destroy, and tofu asks first. `on aws` can set deletion_protection or
// skip_final_snapshot for databases that must survive it.
func (l *lowerer) database(d *ir.Database) {
	n := id(d.Name)
	port := dbPort(d.Engine)
	subnets := l.cfg.Resource("aws_db_subnet_group", n, tfjson.Body{
		"name":       l.physical(255, n),
		"subnet_ids": l.net.private,
	})
	sg := l.securityGroup(n, "clowd database "+d.Name)
	for _, rel := range l.p.Relations {
		if rel.To != d.Name || rel.Verb != ir.ConnectsTo {
			continue
		}
		l.cfg.Resource("aws_vpc_security_group_ingress_rule", n+"_from_"+id(rel.From), tfjson.Body{
			"security_group_id":            ref(sg, "id"),
			"description":                  "from service " + rel.From,
			"referenced_security_group_id": ref("aws_security_group."+id(rel.From), "id"),
			"ip_protocol":                  "tcp",
			"from_port":                    port,
			"to_port":                      port,
		})
	}
	body := tfjson.Body{
		"identifier":                  l.physical(63, n),
		"engine":                      d.Engine.String(),
		"instance_class":              instanceClasses[d.Size],
		"allocated_storage":           20,
		"max_allocated_storage":       100,
		"storage_type":                "gp3",
		"storage_encrypted":           true,
		"db_name":                     dbName(d.Name),
		"username":                    dbUser,
		"manage_master_user_password": true,
		"db_subnet_group_name":        ref(subnets, "name"),
		"vpc_security_group_ids":      []tfjson.Expr{ref(sg, "id")},
		"publicly_accessible":         false,
		"copy_tags_to_snapshot":       true,
		"skip_final_snapshot":         true,
	}
	if d.Version != "" {
		body["engine_version"] = d.Version
	}
	l.cfg.Resource("aws_db_instance", n, body)
}

// bucket lowers b to an S3 bucket. Bucket names are global, so it gets a
// generated suffix; services find it through <NAME>_BUCKET. A public
// bucket serves its objects to anyone through a bucket policy; ACLs stay
// off either way.
//
// Like databases, buckets are emptied by `clowd destroy` (force_destroy),
// which `on aws` can turn off.
func (l *lowerer) bucket(b *ir.Bucket) {
	n := id(b.Name)
	bucket := l.cfg.Resource("aws_s3_bucket", n, tfjson.Body{
		// bucket_prefix allows 37 characters; the dash keeps the suffix apart.
		"bucket_prefix": l.physical(36, n) + "-",
		"force_destroy": true,
	})
	block := l.cfg.Resource("aws_s3_bucket_public_access_block", n, tfjson.Body{
		"bucket":                  ref(bucket, "id"),
		"block_public_acls":       true,
		"ignore_public_acls":      true,
		"block_public_policy":     !b.Public,
		"restrict_public_buckets": !b.Public,
	})
	if b.Public {
		l.cfg.Resource("aws_s3_bucket_policy", n, tfjson.Body{
			"bucket": ref(bucket, "id"),
			"policy": jsonExpr(map[string]any{
				"Version": "2012-10-17",
				"Statement": []any{map[string]any{
					"Sid":       "PublicRead",
					"Effect":    "Allow",
					"Principal": "*",
					"Action":    "s3:GetObject",
					"Resource":  tfjson.Expr("${" + bucket + ".arn}/*"),
				}},
			}),
			// The policy is rejected while the access block forbids it.
			"depends_on": []string{block},
		})
	}
	l.cfg.Outputs = append(l.cfg.Outputs, tfjson.Output{
		Name:        n + "_bucket",
		Description: "The name of bucket " + b.Name,
		Value:       ref(bucket, "bucket"),
	})
}

// queue lowers q to an SQS queue; services find it through
// <NAME>_QUEUE_URL.
func (l *lowerer) queue(q *ir.Queue) {
	n := id(q.Name)
	body := tfjson.Body{"name": l.physical(80, n)}
	if q.FIFO {
		body["name"] = l.physical(80-len(".fifo"), n) + ".fifo"
		body["fifo_queue"] = true
		body["content_based_deduplication"] = true
	}
	l.cfg.Resource("aws_sqs_queue", n, body)
}
