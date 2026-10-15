package aws

import (
	"maps"
	"slices"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/emit/tfjson"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/spell"
)

// arg is an argument an `on aws` block may set: a single value, or a
// nested block with arguments of its own.
type arg struct {
	// block, when set, makes the argument a nested block.
	block map[string]arg
	// repeat allows the block more than once, as a list of blocks.
	repeat bool
}

func scalars(keys ...string) map[string]arg {
	m := make(map[string]arg, len(keys))
	for _, k := range keys {
		m[k] = arg{}
	}
	return m
}

func with(m map[string]arg, key string, a arg) map[string]arg {
	m[key] = a
	return m
}

// overridable lists, per kind, the resource an `on aws` block sets
// arguments on and the arguments it may set (D13, D17). Arguments that
// hold Clowd's own wiring are left out, because changing them would break
// what Clowd generated; they are listed in wiring so the error can say so.
var overridable = map[ir.Kind]struct {
	resource string
	args     map[string]arg
}{
	ir.ServiceKind: {"aws_ecs_service", with(with(scalars(
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
		"capacity_provider_strategy", arg{repeat: true, block: scalars("capacity_provider", "weight", "base")}),
		"deployment_circuit_breaker", arg{block: scalars("enable", "rollback")})},
	ir.DatabaseKind: {"aws_db_instance", with(scalars(
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
		"blue_green_update", arg{block: scalars("enabled")})},
	ir.BucketKind: {"aws_s3_bucket", scalars(
		"force_destroy",
		"object_lock_enabled",
	)},
	ir.QueueKind: {"aws_sqs_queue", scalars(
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
	ir.LoadBalancerKind: {"aws_lb", with(with(scalars(
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
		"access_logs", arg{block: scalars("bucket", "enabled", "prefix")}),
		"connection_logs", arg{block: scalars("bucket", "enabled", "prefix")})},
}

// wiring are the arguments Clowd sets to connect what it generates.
var wiring = map[string][]string{
	"aws_ecs_service": {"cluster", "desired_count", "iam_role", "launch_type", "load_balancer", "name", "network_configuration", "task_definition"},
	"aws_db_instance": {"db_name", "db_subnet_group_name", "engine", "identifier", "manage_master_user_password", "publicly_accessible", "username", "vpc_security_group_ids"},
	"aws_s3_bucket":   {"bucket", "bucket_prefix"},
	"aws_lb":          {"internal", "load_balancer_type", "name", "security_groups", "subnets"},
	"aws_sqs_queue":   {"fifo_queue", "name"},
}

// Overridable returns the resource type an `on aws` block in a resource
// of kind k sets, and the arguments it accepts, for completion and
// hovers. With a path of nested block names (`capacity_provider_strategy`)
// the keys are that block's arguments; a path that isn't a block has none.
func Overridable(k ir.Kind, path ...string) (resource string, keys []string) {
	o := overridable[k]
	args := o.args
	for _, name := range path {
		args = args[strings.ToLower(name)].block
	}
	return o.resource, slices.Sorted(maps.Keys(args))
}

// overrides applies the program's `on aws` blocks. Values are written as
// strings; the provider converts "14" and "true" to the argument's type
// and reports a value that doesn't fit at plan time.
func (l *lowerer) overrides() {
	for _, o := range l.p.Overrides {
		if o.Cloud != ir.AWS {
			continue
		}
		spec := overridable[o.Kind]
		b := l.cfg.Find(spec.resource + "." + id(o.Resource))
		body := tfjson.Body{}
		if b != nil {
			body = b.Body
		}
		// Settings are checked even when the resource was skipped as
		// undeployable, so `clowd check` reports every mistake.
		l.apply(body, o.Settings, spec.args, spec.resource)
		if b != nil && o.Kind == ir.ServiceKind {
			l.capacityProviders(body)
		}
	}
}

// apply sets settings on body, a block of resource whose arguments are
// args.
func (l *lowerer) apply(body tfjson.Body, settings []ir.Setting, args map[string]arg, resource string) {
	replaced := map[string]bool{}
	for _, s := range settings {
		key := strings.ToLower(s.Key)
		a, ok := args[key]
		if !ok {
			l.diags.Add(unknownKey(s, resource, args))
			continue
		}
		if a.block == nil {
			switch {
			case s.Block != nil:
				l.diags.Add(diag.Errorf(s.KeyPos, "`%s` takes a value, not a block", s.Key).
					Suggest("write `%s <value>` on one line", key))
			case s.List != nil:
				l.diags.Add(diag.Errorf(s.Pos, "`%s` takes one value, not a list", s.Key))
			default:
				body[key] = s.Value
			}
			continue
		}
		if s.Block == nil {
			l.diags.Add(diag.Errorf(s.Pos, "`%s` is a block", s.Key).
				Suggest("indent its settings under it, such as `%s`", firstKey(a.block)))
			continue
		}
		inner := tfjson.Body{}
		if !a.repeat {
			// Merge into what Clowd set, so one setting can change one
			// field of a block such as deployment_circuit_breaker.
			if old, ok := body[key].(tfjson.Body); ok {
				inner = old
			}
		}
		l.apply(inner, s.Block, a.block, resource+" "+key)
		switch {
		case !a.repeat:
			body[key] = inner
		case !replaced[key]:
			// The first block replaces whatever Clowd generated.
			body[key] = []tfjson.Body{inner}
			replaced[key] = true
		default:
			body[key] = append(body[key].([]tfjson.Body), inner)
		}
	}
}

func firstKey(args map[string]arg) string {
	return slices.Sorted(maps.Keys(args))[0] + " <value>"
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

func unknownKey(s ir.Setting, resource string, args map[string]arg) diag.Diagnostic {
	key := strings.ToLower(s.Key)
	top, _, _ := strings.Cut(resource, " ")
	d := diag.Errorf(s.KeyPos, "%s has no setting `%s` that `on aws` can change", resource, s.Key)
	if p, ok := portable[resource][key]; ok {
		return d.Suggest("use %s instead, which works on every cloud", p)
	}
	if slices.Contains(wiring[resource], key) {
		return d.Suggest("Clowd sets `%s` to connect %s to the rest of the program", key, top)
	}
	names := slices.Sorted(maps.Keys(args))
	if g, ok := spell.Closest(s.Key, names); ok {
		return d.Suggest("did you mean `%s`?", g)
	}
	// The commonest miss is a field of a nested block, such as
	// capacity_provider in capacity_provider_strategy.
	for _, name := range names {
		if a := args[name]; a.block != nil {
			if _, ok := a.block[key]; ok {
				return d.Suggest("`%s` goes in a `%s` block", key, name)
			}
		}
	}
	return d.Suggest("%s settings that can be changed: %s", resource, strings.Join(names, ", "))
}
