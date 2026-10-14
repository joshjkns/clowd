package aws

import (
	"slices"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/spell"
)

// overridable lists, per kind, the resource an `on aws` override sets
// arguments on and the arguments it may set. They are the provider's
// top-level arguments that take a single value; nested blocks and lists
// can't be written as `sets <key> to <value>`, and arguments that hold
// Clowd's own wiring (subnets, security groups, roles) are left out
// because changing them would break what Clowd generated.
var overridable = map[ir.Kind]struct {
	resource string
	keys     []string
}{
	ir.ServiceKind: {"aws_ecs_service", []string{
		"availability_zone_rebalancing",
		"deployment_maximum_percent",
		"deployment_minimum_healthy_percent",
		"enable_ecs_managed_tags",
		"enable_execute_command",
		"force_new_deployment",
		"health_check_grace_period_seconds",
		"platform_version",
		"propagate_tags",
		"wait_for_steady_state",
	}},
	ir.DatabaseKind: {"aws_db_instance", []string{
		"allocated_storage",
		"allow_major_version_upgrade",
		"apply_immediately",
		"auto_minor_version_upgrade",
		"backup_retention_period",
		"backup_window",
		"ca_cert_identifier",
		"copy_tags_to_snapshot",
		"delete_automated_backups",
		"deletion_protection",
		"engine_version",
		"final_snapshot_identifier",
		"iam_database_authentication_enabled",
		"instance_class",
		"iops",
		"kms_key_id",
		"maintenance_window",
		"max_allocated_storage",
		"monitoring_interval",
		"multi_az",
		"parameter_group_name",
		"performance_insights_enabled",
		"performance_insights_retention_period",
		"skip_final_snapshot",
		"storage_throughput",
		"storage_type",
	}},
	ir.BucketKind: {"aws_s3_bucket", []string{
		"force_destroy",
		"object_lock_enabled",
	}},
	ir.QueueKind: {"aws_sqs_queue", []string{
		"deduplication_scope",
		"delay_seconds",
		"fifo_throughput_limit",
		"kms_data_key_reuse_period_seconds",
		"kms_master_key_id",
		"max_message_size",
		"message_retention_seconds",
		"receive_wait_time_seconds",
		"sqs_managed_sse_enabled",
		"visibility_timeout_seconds",
	}},
	ir.LoadBalancerKind: {"aws_lb", []string{
		"client_keep_alive",
		"desync_mitigation_mode",
		"drop_invalid_header_fields",
		"enable_deletion_protection",
		"enable_http2",
		"idle_timeout",
		"ip_address_type",
		"preserve_host_header",
		"xff_header_processing_mode",
	}},
}

// nestedBlocks are well-known nested blocks of the overridable resources,
// recognised in errors so the user learns why they can't be set.
var nestedBlocks = map[string][]string{
	"aws_ecs_service": {"capacity_provider_strategy", "deployment_circuit_breaker", "load_balancer",
		"network_configuration", "ordered_placement_strategy", "placement_constraints", "service_connect_configuration"},
	"aws_db_instance": {"blue_green_update", "restore_to_point_in_time", "s3_import"},
	"aws_s3_bucket":   {"lifecycle_rule", "versioning", "website", "cors_rule", "logging"},
	"aws_lb":          {"access_logs", "connection_logs", "subnet_mapping"},
	"aws_sqs_queue":   {"redrive_policy"},
}

// Overridable returns the resource type an `on aws` override of kind k
// sets, and the keys it accepts, for completion and hovers.
func Overridable(k ir.Kind) (resource string, keys []string) {
	o := overridable[k]
	return o.resource, slices.Clone(o.keys)
}

// overrides applies the program's `on aws` settings. Values are written as
// strings; the provider converts "14" and "true" to the argument's type
// and reports a value that doesn't fit at plan time.
func (l *lowerer) overrides() {
	for _, o := range l.p.Overrides {
		if o.Cloud != ir.AWS {
			continue
		}
		spec := overridable[o.Kind]
		key := strings.ToLower(o.Key)
		if !slices.Contains(spec.keys, key) {
			l.diags.Add(unknownKey(o, spec.resource, spec.keys))
			continue
		}
		b := l.cfg.Find(spec.resource + "." + id(o.Resource))
		if b == nil {
			continue // the resource was skipped as undeployable
		}
		b.Body[key] = o.Value
	}
}

func unknownKey(o ir.Override, resource string, keys []string) diag.Diagnostic {
	d := diag.Errorf(o.KeyPos, "%s has no setting `%s` that `on aws` can change", resource, o.Key)
	if s, ok := spell.Closest(o.Key, keys); ok {
		return d.Suggest("did you mean `%s`?", s)
	}
	// The commonest miss is a field of a nested block, such as
	// capacity_provider in capacity_provider_strategy.
	key := strings.ToLower(o.Key)
	for _, b := range nestedBlocks[resource] {
		if key == b || strings.HasPrefix(b, key) {
			return d.Suggest("`%s` is a nested block of %s, which `sets` can't change yet", b, resource)
		}
	}
	return d.Suggest("%s settings that can be overridden: %s", resource, strings.Join(keys, ", "))
}
