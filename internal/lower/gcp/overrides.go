package gcp

import (
	"github.com/joshjkns/clowd/internal/emit/tfjson"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/lower/override"
)

var (
	scalars = override.Scalars
	with    = override.With
)

type arg = override.Arg

// overridable lists, per kind, the resource an `on gcp` block sets
// arguments on and the arguments it may set (D13, D16). As on AWS,
// arguments that carry Clowd's own wiring are left out and listed in
// Wiring, so the error can say why.
//
// A load balancer's block sets every backend service it has, one per
// service it sends to, since that is where Google Cloud's per-route
// settings (CDN, Cloud Armor, logging) live.
var overridable = map[ir.Kind]override.Spec{
	ir.ServiceKind: {
		Resource: "google_cloud_run_v2_service",
		Args: with(scalars(
			"default_uri_disabled",
			"description",
			"iap_enabled",
			"launch_stage",
		),
			"template", arg{Block: scalars(
				"encryption_key",
				"execution_environment",
				"max_instance_request_concurrency",
				"session_affinity",
				"timeout",
			)}),
		Wiring:   []string{"deletion_protection", "ingress", "invoker_iam_disabled", "location", "name"},
		Portable: map[string]string{"scaling": "`scale`"},
	},
	ir.DatabaseKind: {
		Resource: "google_sql_database_instance",
		Args: with(scalars(
			"encryption_key_name",
			"maintenance_version",
		),
			"settings", arg{Block: with(with(with(with(scalars(
				"activation_policy",
				"collation",
				"connector_enforcement",
				"disk_autoresize",
				"disk_autoresize_limit",
				"disk_type",
				"edition",
				"pricing_plan",
				"retain_backups_on_delete",
				"tier",
				"time_zone",
			),
				"backup_configuration", arg{Block: scalars("location", "point_in_time_recovery_enabled", "start_time", "transaction_log_retention_days")}),
				"database_flags", arg{Repeat: true, Block: scalars("name", "value")}),
				"insights_config", arg{Block: scalars("query_insights_enabled", "query_plans_per_minute", "query_string_length", "record_application_tags", "record_client_address")}),
				"maintenance_window", arg{Block: scalars("day", "hour", "update_track")})}),
		Wiring: []string{"database_version", "name", "region"},
		Portable: map[string]string{
			"deletion_protection": "the `protected` trait",
		},
	},
	ir.BucketKind: {
		Resource: "google_storage_bucket",
		Args: with(with(with(scalars(
			"default_event_based_hold",
			"enable_object_retention",
			"requester_pays",
			"rpo",
			"storage_class",
		),
			"cors", arg{Repeat: true, Block: with(with(with(scalars("max_age_seconds"),
				"method", arg{List: true}),
				"origin", arg{List: true}),
				"response_header", arg{List: true})}),
			"logging", arg{Block: scalars("log_bucket", "log_object_prefix")}),
			"soft_delete_policy", arg{Block: scalars("retention_duration_seconds")}),
		Wiring: []string{"location", "name", "public_access_prevention", "uniform_bucket_level_access"},
		Portable: map[string]string{
			"force_destroy":  "the `protected` trait",
			"versioning":     "the `versioned` trait",
			"lifecycle_rule": "`expire after <duration>`",
		},
	},
	ir.QueueKind: {
		Resource: "google_pubsub_subscription",
		Args: with(scalars(
			"filter",
			"retain_acked_messages",
		),
			"retry_policy", arg{Block: scalars("maximum_backoff", "minimum_backoff")}),
		Wiring: []string{"name", "topic", "expiration_policy"},
		Portable: map[string]string{
			"ack_deadline_seconds":         "`timeout <duration>`",
			"message_retention_duration":   "`keep messages for <duration>`",
			"dead_letter_policy":           "`retry <n> times`",
			"enable_message_ordering":      "the `fifo` trait",
			"enable_exactly_once_delivery": "the `fifo` trait",
		},
	},
	ir.LoadBalancerKind: {
		Resource: "google_compute_backend_service",
		Args: with(with(with(scalars(
			"description",
			"enable_cdn",
			"security_policy",
		),
			"custom_request_headers", arg{List: true}),
			"custom_response_headers", arg{List: true}),
			"log_config", arg{Block: scalars("enable", "sample_rate")}),
		Wiring: []string{"backend", "load_balancing_scheme", "name", "protocol"},
	},
}

// Overridable returns the resource type an `on gcp` block in a resource
// of kind k sets, and the arguments it accepts, for completion and
// hovers. With a path of nested block names (`settings`) the keys are
// that block's arguments; a path that isn't a block has none.
func Overridable(k ir.Kind, path ...string) (resource string, keys []string) {
	s := overridable[k]
	return s.Resource, s.Keys(path...)
}

// overrides applies the program's `on gcp` blocks.
func (l *lowerer) overrides() {
	for _, o := range l.p.Overrides {
		if o.Cloud != ir.GCP {
			continue
		}
		spec := overridable[o.Kind]
		bodies := l.overridden(o, &spec)
		if len(bodies) == 0 {
			// Settings are checked even when nothing was generated, so
			// `clowd check` reports every mistake.
			bodies = []tfjson.Body{{}}
		}
		for i, body := range bodies {
			diags := override.Apply(body, o.Settings, spec, ir.GCP)
			if i == 0 {
				l.diags.Add(diags...)
			}
		}
	}
}

// overridden returns the bodies an `on gcp` block in o's resource sets:
// one resource, or each backend service of a load balancer.
func (l *lowerer) overridden(o ir.Override, spec *override.Spec) []tfjson.Body {
	if o.Kind != ir.LoadBalancerKind {
		if b := l.cfg.Find(spec.Resource + "." + id(o.Resource)); b != nil {
			return []tfjson.Body{b.Body}
		}
		return nil
	}
	if lb, ok := l.p.Resource(o.Resource).(*ir.LoadBalancer); ok && lb.Internal {
		spec.Resource = internalLB.backend
	}
	var bodies []tfjson.Body
	for _, addr := range l.backends[o.Resource] {
		bodies = append(bodies, l.cfg.Find(addr).Body)
	}
	return bodies
}
