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
// letters, digits and underscores, up to 63 of them. It always ends in
// `_db`: RDS rejects reserved words (`db` itself for Postgres, hundreds
// of keywords for MySQL), and no keyword ends that way, so the suffix
// avoids them all without a list to keep up to date. Services read the
// name from <NAME>_NAME, so its exact form doesn't matter to them.
func dbName(name string) string {
	n := strings.ReplaceAll(id(name), "-", "_")
	if n == "db" || !strings.HasSuffix(n, "_db") {
		n += "_db"
	}
	if len(n) > maxDBName {
		n = strings.TrimRight(n[:maxDBName-len("_db")], "_") + "_db"
	}
	return n
}

// maxDBName is the longest database name both engines accept.
const maxDBName = 63

// database lowers d to an RDS instance in the private subnets, reachable
// only from the services that connect to it.
//
// `clowd destroy` deletes it without a final snapshot: destroy means
// destroy, and tofu asks first. A `protected` database can't be deleted
// and keeps a final snapshot if protection is later removed (D15).
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
	if d.StorageGB != 0 {
		// Storage autoscaling stays on, with room to double.
		body["allocated_storage"] = d.StorageGB
		body["max_allocated_storage"] = min(max(100, 2*d.StorageGB), maxStorageGB)
	}
	if d.Has("backups") {
		body["backup_retention_period"] = d.BackupDays
	}
	if d.HighlyAvailable {
		body["multi_az"] = true
	}
	if d.Protected {
		body["deletion_protection"] = true
		body["skip_final_snapshot"] = false
		body["final_snapshot_identifier"] = l.physical(255, n, "final")
	}
	l.cfg.Resource("aws_db_instance", n, body)
}

// maxStorageGB is the largest RDS disk.
const maxStorageGB = 65536

// bucket lowers b to an S3 bucket. Bucket names are global, so it gets a
// generated suffix; services find it through <NAME>_BUCKET. A public
// bucket serves its objects to anyone through a bucket policy; ACLs stay
// off either way.
//
// Like databases, buckets are emptied by `clowd destroy` (force_destroy),
// unless they are `protected`.
func (l *lowerer) bucket(b *ir.Bucket) {
	n := id(b.Name)
	bucket := l.cfg.Resource("aws_s3_bucket", n, tfjson.Body{
		// bucket_prefix allows 37 characters; the dash keeps the suffix apart.
		"bucket_prefix": l.physical(36, n) + "-",
		"force_destroy": !b.Protected,
	})
	var versioning string
	if b.Versioned {
		versioning = l.cfg.Resource("aws_s3_bucket_versioning", n, tfjson.Body{
			"bucket":                   ref(bucket, "id"),
			"versioning_configuration": tfjson.Body{"status": "Enabled"},
		})
	}
	if b.ExpireDays != 0 {
		rule := tfjson.Body{
			"id":         "expire",
			"status":     "Enabled",
			"filter":     tfjson.Body{},
			"expiration": tfjson.Body{"days": b.ExpireDays},
		}
		lifecycle := tfjson.Body{"bucket": ref(bucket, "id"), "rule": []tfjson.Body{rule}}
		if b.Versioned {
			// Expiring an object only hides it behind a delete marker in a
			// versioned bucket; expire the old versions too, or they are
			// kept (and paid for) forever.
			rule["noncurrent_version_expiration"] = tfjson.Body{"noncurrent_days": b.ExpireDays}
			lifecycle["depends_on"] = []string{versioning}
		}
		l.cfg.Resource("aws_s3_bucket_lifecycle_configuration", n, lifecycle)
	}
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
// <NAME>_QUEUE_URL. With `retry`, messages that keep failing move to a
// second queue, <name>-dead, which keeps them as long as SQS allows so
// they can be inspected.
func (l *lowerer) queue(q *ir.Queue) {
	n := id(q.Name)
	name := func(parts ...string) tfjson.Body {
		body := tfjson.Body{"name": l.physical(80, parts...)}
		if q.FIFO {
			body["name"] = l.physical(80-len(".fifo"), parts...) + ".fifo"
			body["fifo_queue"] = true
			body["content_based_deduplication"] = true
		}
		return body
	}
	body := name(n)
	if q.Timeout != 0 {
		body["visibility_timeout_seconds"] = q.Timeout
	}
	if q.Retention != 0 {
		body["message_retention_seconds"] = q.Retention
	}
	if q.Retries != 0 {
		dead := name(n, "dead")
		dead["message_retention_seconds"] = maxSQSRetention
		addr := l.cfg.Resource("aws_sqs_queue", n+"_dead", dead)
		body["redrive_policy"] = jsonExpr(map[string]any{
			"deadLetterTargetArn": ref(addr, "arn"),
			// SQS counts receives: the first try and each retry.
			"maxReceiveCount": q.Retries + 1,
		})
	}
	l.cfg.Resource("aws_sqs_queue", n, body)
}

// maxSQSRetention is the longest SQS keeps a message: 14 days.
const maxSQSRetention = 14 * 24 * 60 * 60
