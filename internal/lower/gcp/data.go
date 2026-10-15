package gcp

import (
	"fmt"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/emit/tfjson"
	"github.com/joshjkns/clowd/internal/ir"
)

// dbUser is the user services connect as.
const dbUser = "clowd"

// tiers map database sizes to Cloud SQL machine types, close to the RDS
// classes lower/aws picks: a shared core, then 1 and 2 dedicated vCPUs.
var tiers = map[ir.Size]string{
	ir.Small:  "db-g1-small",
	ir.Medium: "db-custom-1-3840",
	ir.Large:  "db-custom-2-8192",
}

// defaultVersions are used when a database names no version: the newest
// each engine offers in the capability matrix.
var defaultVersions = map[ir.Engine]string{
	ir.Postgres: "17",
	ir.MySQL:    "8.4",
}

func dbPort(e ir.Engine) int {
	if e == ir.MySQL {
		return 3306
	}
	return 5432
}

// dbName is the database created inside the instance, named as on every
// cloud: lowercase, underscores, ending in `_db` so it is never a
// reserved word (see lower/aws).
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

// databaseVersion is Cloud SQL's name for d's engine and version:
// POSTGRES_16, MYSQL_8_0. Cloud SQL chooses the minor version itself.
func (l *lowerer) databaseVersion(d *ir.Database) string {
	v := d.Version
	if v == "" {
		v = defaultVersions[d.Engine]
	}
	if d.Engine == ir.Postgres {
		if major, _, minor := strings.Cut(v, "."); minor {
			l.diags.Add(diag.Infof(d.At("engine"), "on gcp: Cloud SQL chooses the minor version; %s runs the latest postgres %s", d.Name, major))
			v = major
		}
	}
	return strings.ToUpper(d.Engine.String()) + "_" + strings.ReplaceAll(v, ".", "_")
}

// database lowers d to a Cloud SQL instance reachable only on a private
// address in the network.
//
// The password is generated for each run and written only to write-only
// arguments, of the database user and of a Secret Manager secret that
// services read when they start, so it never appears in the
// configuration or the state. The `_wo_version` arguments make it
// written once, when they are created.
//
// Instance names can't be reused for a week after a delete, so the name
// ends in a generated suffix and `clowd destroy` then `apply` works. As
// on AWS, destroy deletes the instance unless it is `protected` (D15),
// which also keeps a final backup.
func (l *lowerer) database(d *ir.Database) {
	n := id(d.Name)
	settings := tfjson.Body{
		"tier": tiers[d.Size],
		// Enterprise Plus, the default for new Postgres versions, has no
		// shared-core machine types.
		"edition":           "ENTERPRISE",
		"availability_type": "ZONAL",
		"ip_configuration": tfjson.Body{
			"ipv4_enabled":    false,
			"private_network": l.net.id,
		},
		"deletion_protection_enabled": d.Protected,
		"user_labels":                 l.labels(),
	}
	if d.StorageGB != 0 {
		settings["disk_size"] = d.StorageGB
	}
	if d.HighlyAvailable {
		settings["availability_type"] = "REGIONAL"
	}
	if d.Has("backups") || d.HighlyAvailable {
		backups := tfjson.Body{"enabled": d.BackupDays != 0 || !d.Has("backups")}
		if d.BackupDays != 0 {
			backups["backup_retention_settings"] = tfjson.Body{"retained_backups": d.BackupDays, "retention_unit": "COUNT"}
		}
		if d.HighlyAvailable {
			// Failover replays the log: binary logging on MySQL,
			// point-in-time recovery on Postgres.
			if d.Engine == ir.MySQL {
				backups["binary_log_enabled"] = true
			} else {
				backups["point_in_time_recovery_enabled"] = true
			}
		}
		settings["backup_configuration"] = backups
	}
	if d.Protected {
		settings["final_backup_config"] = tfjson.Body{"enabled": true, "retention_days": finalBackupDays}
	}
	inst := l.cfg.Resource("google_sql_database_instance", n, tfjson.Body{
		"name":                l.suffixed(maxInstanceName, n, 2),
		"region":              l.loc,
		"database_version":    l.databaseVersion(d),
		"settings":            settings,
		"deletion_protection": d.Protected,
		"depends_on":          []string{l.net.peering},
	})
	l.cfg.Resource("google_sql_database", n, tfjson.Body{
		"name":     dbName(d.Name),
		"instance": ref(inst, "name"),
		// Deleting the instance deletes its databases; dropping one
		// first fails while services are connected.
		"deletion_policy": "ABANDON",
	})

	l.usesRandom = true
	password := l.cfg.EphemeralResource("random_password", n, tfjson.Body{
		"length": 32,
		// Letters and digits only, so the password needs no escaping in
		// connection strings.
		"special": false,
	})
	user := tfjson.Body{
		"name":                dbUser,
		"instance":            ref(inst, "name"),
		"password_wo":         ref(password, "result"),
		"password_wo_version": 1,
	}
	if d.Engine == ir.Postgres {
		// A Postgres role that owns tables can't be dropped; it goes
		// with the instance.
		user["deletion_policy"] = "ABANDON"
	}
	l.cfg.Resource("google_sql_user", n, user)
	secret := l.cfg.Resource("google_secret_manager_secret", n+"_password", tfjson.Body{
		"secret_id":   l.physical(255, n, "password"),
		"replication": tfjson.Body{"auto": tfjson.Body{}},
	})
	l.cfg.Resource("google_secret_manager_secret_version", n+"_password", tfjson.Body{
		"secret":                 ref(secret, "id"),
		"secret_data_wo":         ref(password, "result"),
		"secret_data_wo_version": "1",
	})
}

// maxInstanceName is the longest Cloud SQL instance name.
const maxInstanceName = 98

// finalBackupDays is how long a protected database's final backup is
// kept after the instance is deleted.
const finalBackupDays = 30

// bucket lowers b to a Cloud Storage bucket. Bucket names are global, so
// it gets a generated suffix; services find it through <NAME>_BUCKET.
// Access is uniform (IAM only, no object ACLs), and a private bucket
// can't be made public by accident. A public bucket lets anyone read its
// objects.
//
// Like databases, buckets are emptied by `clowd destroy` (force_destroy),
// unless they are `protected`.
func (l *lowerer) bucket(b *ir.Bucket) {
	n := id(b.Name)
	body := tfjson.Body{
		"name":                        l.suffixed(63, n, 4),
		"location":                    l.upperLoc(),
		"uniform_bucket_level_access": true,
		"public_access_prevention":    "enforced",
		"force_destroy":               !b.Protected,
	}
	if b.Public {
		body["public_access_prevention"] = "inherited"
	}
	if b.Versioned {
		body["versioning"] = tfjson.Body{"enabled": true}
	}
	if b.ExpireDays != 0 {
		rules := []tfjson.Body{{
			"action":    tfjson.Body{"type": "Delete"},
			"condition": tfjson.Body{"age": b.ExpireDays},
		}}
		if b.Versioned {
			// Deleting a live object in a versioned bucket keeps it as an
			// old version; delete those too, or they are kept (and paid
			// for) forever.
			rules = append(rules, tfjson.Body{
				"action":    tfjson.Body{"type": "Delete"},
				"condition": tfjson.Body{"days_since_noncurrent_time": b.ExpireDays, "with_state": "ARCHIVED"},
			})
		}
		body["lifecycle_rule"] = rules
	}
	bucket := l.cfg.Resource("google_storage_bucket", n, body)
	if b.Public {
		l.cfg.Resource("google_storage_bucket_iam_member", n+"_public", tfjson.Body{
			"bucket": ref(bucket, "name"),
			"role":   "roles/storage.objectViewer",
			"member": "allUsers",
		})
	}
	l.cfg.Outputs = append(l.cfg.Outputs, tfjson.Output{
		Name:        n + "_bucket",
		Description: "The name of bucket " + b.Name,
		Value:       ref(bucket, "name"),
	})
}

// upperLoc is the region in the upper case Cloud Storage reports, so
// plans don't show a change from `europe-west2` to `EUROPE-WEST2`.
func (l *lowerer) upperLoc() any {
	if s, ok := l.loc.(string); ok {
		return strings.ToUpper(s)
	}
	return tfjson.Expr("${upper(data.google_client_config.current.region)}")
}

// queue lowers q to a Pub/Sub topic that senders publish to and one
// subscription that readers pull from, so each message is handled once,
// as with SQS. Subscriptions normally expire after a month unused; this
// one never does.
//
// With `retry`, messages that keep failing go to a second topic,
// <name>-dead, whose subscription keeps them as long as Pub/Sub allows so
// they can be inspected.
func (l *lowerer) queue(q *ir.Queue) {
	n := id(q.Name)
	topic := l.cfg.Resource("google_pubsub_topic", n, tfjson.Body{"name": l.physical(255, n)})
	sub := tfjson.Body{
		"name":              l.physical(255, n),
		"topic":             ref(topic, "id"),
		"expiration_policy": tfjson.Body{"ttl": ""},
	}
	if q.Timeout != 0 {
		sub["ack_deadline_seconds"] = q.Timeout
	}
	if q.Retention != 0 {
		sub["message_retention_duration"] = fmt.Sprintf("%ds", q.Retention)
	}
	if q.FIFO {
		sub["enable_message_ordering"] = true
		sub["enable_exactly_once_delivery"] = true
	}
	if q.Retries != 0 {
		dead := l.cfg.Resource("google_pubsub_topic", n+"_dead", tfjson.Body{"name": l.physical(255, n, "dead")})
		l.cfg.Resource("google_pubsub_subscription", n+"_dead", tfjson.Body{
			"name":                       l.physical(255, n, "dead"),
			"topic":                      ref(dead, "id"),
			"message_retention_duration": fmt.Sprintf("%ds", maxRetention),
			"expiration_policy":          tfjson.Body{"ttl": ""},
		})
		sub["dead_letter_policy"] = tfjson.Body{
			"dead_letter_topic": ref(dead, "id"),
			// Pub/Sub counts deliveries: the first try and each retry.
			"max_delivery_attempts": q.Retries + 1,
		}
		// Pub/Sub moves messages as its own service agent, which needs
		// to publish to the dead-letter topic and to acknowledge the
		// message it moved.
		agent := tfjson.Expr("serviceAccount:service-${" + l.projectData() + ".number}@gcp-sa-pubsub.iam.gserviceaccount.com")
		l.cfg.Resource("google_pubsub_topic_iam_member", n+"_dead_letters", tfjson.Body{
			"topic":  ref(dead, "name"),
			"role":   "roles/pubsub.publisher",
			"member": agent,
		})
		l.cfg.Resource("google_pubsub_subscription_iam_member", n+"_dead_letters", tfjson.Body{
			"subscription": ref("google_pubsub_subscription."+n, "name"),
			"role":         "roles/pubsub.subscriber",
			"member":       agent,
		})
	}
	l.cfg.Resource("google_pubsub_subscription", n, sub)
}

// maxRetention is the longest Pub/Sub keeps a message: 31 days.
const maxRetention = 31 * 24 * 60 * 60

// projectData returns the address of a data source describing the
// Google Cloud project, adding it the first time.
func (l *lowerer) projectData() string {
	const addr = "data.google_project.current"
	for _, d := range l.cfg.Data {
		if "data."+d.Addr() == addr {
			return addr
		}
	}
	return l.cfg.DataSource("google_project", "current", tfjson.Body{})
}
