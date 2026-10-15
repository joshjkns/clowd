package aws

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

// overridable lists, per kind, the resource an `on aws` block sets
// arguments on and the arguments it may set (D13, D17). Arguments that
// hold Clowd's own wiring are left out, because changing them would break
// what Clowd generated; they are listed in wiring so the error can say so.
var overridable = map[ir.Kind]override.Spec{
	ir.ServiceKind: {Resource: "aws_ecs_service", Args: with(with(scalars(
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
	),
		"capacity_provider_strategy", arg{Repeat: true, Block: scalars("capacity_provider", "weight", "base")}),
		"deployment_circuit_breaker", arg{Block: scalars("enable", "rollback")})},
	ir.DatabaseKind: {Resource: "aws_db_instance", Args: with(scalars(
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
	),
		"blue_green_update", arg{Block: scalars("enabled")})},
	ir.BucketKind: {Resource: "aws_s3_bucket", Args: scalars(
		"force_destroy",
		"object_lock_enabled",
	)},
	ir.QueueKind: {Resource: "aws_sqs_queue", Args: scalars(
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
	)},
	ir.LoadBalancerKind: {Resource: "aws_lb", Args: with(with(scalars(
		"client_keep_alive",
		"desync_mitigation_mode",
		"drop_invalid_header_fields",
		"enable_deletion_protection",
		"enable_http2",
		"idle_timeout",
		"ip_address_type",
		"preserve_host_header",
		"xff_header_processing_mode",
	),
		"access_logs", arg{Block: scalars("bucket", "enabled", "prefix")}),
		"connection_logs", arg{Block: scalars("bucket", "enabled", "prefix")})},
}

// wiring are the arguments Clowd sets to connect what it generates.
var wiring = map[string][]string{
	"aws_ecs_service": {"cluster", "desired_count", "iam_role", "launch_type", "load_balancer", "name", "network_configuration", "task_definition"},
	"aws_db_instance": {"db_name", "db_subnet_group_name", "engine", "identifier", "manage_master_user_password", "publicly_accessible", "username", "vpc_security_group_ids"},
	"aws_s3_bucket":   {"bucket", "bucket_prefix"},
	"aws_lb":          {"internal", "load_balancer_type", "name", "security_groups", "subnets"},
	"aws_sqs_queue":   {"fifo_queue", "name"},
}

// portable are provider settings that Clowd writes portably, so the error
// can point at the portable form (D18).
var portable = map[string]map[string]string{
	"aws_s3_bucket": {
		"versioning":     "the `versioned` trait",
		"lifecycle_rule": "`expire after <duration>`",
	},
	"aws_sqs_queue": {
		"redrive_policy": "`retry <n> times`",
	},
}

func init() {
	for k, s := range overridable {
		s.Wiring = wiring[s.Resource]
		s.Portable = portable[s.Resource]
		overridable[k] = s
	}
}

// Overridable returns the resource type an `on aws` block in a resource
// of kind k sets, and the arguments it accepts, for completion and
// hovers. With a path of nested block names (`capacity_provider_strategy`)
// the keys are that block's arguments; a path that isn't a block has none.
func Overridable(k ir.Kind, path ...string) (resource string, keys []string) {
	s := overridable[k]
	return s.Resource, s.Keys(path...)
}

// overrides applies the program's `on aws` blocks.
func (l *lowerer) overrides() {
	for _, o := range l.p.Overrides {
		if o.Cloud != ir.AWS {
			continue
		}
		spec := overridable[o.Kind]
		b := l.cfg.Find(spec.Resource + "." + id(o.Resource))
		body := tfjson.Body{}
		if b != nil {
			body = b.Body
		}
		// Settings are checked even when the resource was skipped as
		// undeployable, so `clowd check` reports every mistake.
		l.diags.Add(override.Apply(body, o.Settings, spec, ir.AWS)...)
		if b != nil && o.Kind == ir.ServiceKind {
			l.capacityProviders(body)
		}
	}
}

// capacityProviders makes a service with a capacity_provider_strategy
// deployable: the strategy replaces launch_type, and the cluster must
// offer the providers it names.
func (l *lowerer) capacityProviders(svc tfjson.Body) {
	if _, ok := svc["capacity_provider_strategy"]; !ok {
		return
	}
	delete(svc, "launch_type")
	addr := "aws_ecs_cluster_capacity_providers.main"
	if l.cfg.Find(addr) == nil {
		l.cfg.Resource("aws_ecs_cluster_capacity_providers", "main", tfjson.Body{
			"cluster_name":       tfjson.Ref("aws_ecs_cluster.main.name"),
			"capacity_providers": []string{"FARGATE", "FARGATE_SPOT"},
		})
	}
	deps, _ := svc["depends_on"].([]string)
	svc["depends_on"] = append(deps, addr)
}
