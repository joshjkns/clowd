package aws

import (
	"fmt"
	"strconv"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/emit/tfjson"
	"github.com/joshjkns/clowd/internal/ir"
)

// cpuTarget is the average CPU utilisation, in percent, that autoscaling
// keeps services at.
const cpuTarget = 70

// logRetentionDays is how long service logs are kept.
const logRetentionDays = 30

// service lowers s to an ECS Fargate service and everything it needs.
func (l *lowerer) service(s *ir.Service) {
	n := id(s.Name)
	env, secrets, statements, secretARNs := l.wiring(s)

	logs := l.cfg.Resource("aws_cloudwatch_log_group", n, tfjson.Body{
		"name":              fmt.Sprintf("/clowd/%s/%s", l.opts.Project, n),
		"retention_in_days": logRetentionDays,
	})

	// The execution role is what ECS uses to start the task: pull the
	// image, write logs, read secrets. The task role is what the app runs
	// as, and gets the permissions its relationships need.
	exec := l.cfg.Resource("aws_iam_role", n+"_execution", tfjson.Body{
		"name":               l.physical(64, n, "execution"),
		"assume_role_policy": assumeRole("ecs-tasks.amazonaws.com"),
	})
	execPolicy := l.cfg.Resource("aws_iam_role_policy_attachment", n+"_execution", tfjson.Body{
		"role":       ref(exec, "name"),
		"policy_arn": "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy",
	})
	dependsOn := []string{execPolicy}
	if len(secretARNs) > 0 {
		dependsOn = append(dependsOn, l.cfg.Resource("aws_iam_role_policy", n+"_execution", tfjson.Body{
			"name":   "secrets",
			"role":   ref(exec, "id"),
			"policy": policy(statement([]string{"secretsmanager:GetSecretValue"}, secretARNs...)),
		}))
	}
	task := l.cfg.Resource("aws_iam_role", n+"_task", tfjson.Body{
		"name":               l.physical(64, n, "task"),
		"assume_role_policy": assumeRole("ecs-tasks.amazonaws.com"),
	})
	if len(statements) > 0 {
		l.cfg.Resource("aws_iam_role_policy", n+"_task", tfjson.Body{
			"name":   "relationships",
			"role":   ref(task, "id"),
			"policy": policy(statements...),
		})
	}

	sg := l.securityGroup(n, "clowd service "+s.Name)
	l.cfg.Resource("aws_vpc_security_group_egress_rule", n+"_all", tfjson.Body{
		"security_group_id": ref(sg, "id"),
		"description":       "pull images and reach dependencies",
		"ip_protocol":       "-1",
		"cidr_ipv4":         "0.0.0.0/0",
	})

	cpu, memory := l.fargateSize(s)
	container := map[string]any{
		"name":         n,
		"image":        s.Image,
		"essential":    true,
		"portMappings": []any{map[string]any{"containerPort": s.Port, "protocol": "tcp"}},
		"environment":  env,
		"secrets":      secrets,
		"logConfiguration": map[string]any{
			"logDriver": "awslogs",
			"options": map[string]any{
				"awslogs-group":         ref(logs, "name"),
				"awslogs-region":        tfjson.Ref("data.aws_region.current.region"),
				"awslogs-stream-prefix": n,
			},
		},
	}
	taskDef := l.cfg.Resource("aws_ecs_task_definition", n, tfjson.Body{
		"family":                   l.physical(255, n),
		"requires_compatibilities": []string{"FARGATE"},
		"network_mode":             "awsvpc",
		"cpu":                      strconv.Itoa(cpu),
		"memory":                   strconv.Itoa(memory),
		"execution_role_arn":       ref(exec, "arn"),
		"task_role_arn":            ref(task, "arn"),
		"container_definitions":    jsonExpr([]any{container}),
		"runtime_platform": tfjson.Body{
			"operating_system_family": "LINUX",
			"cpu_architecture":        "X86_64",
		},
	})

	svc := tfjson.Body{
		"name":            n,
		"cluster":         tfjson.Ref("aws_ecs_cluster.main.id"),
		"task_definition": ref(taskDef, "arn"),
		"desired_count":   s.Scale.Min,
		"launch_type":     "FARGATE",
		"network_configuration": tfjson.Body{
			"subnets":          l.serviceSubnets(),
			"security_groups":  []tfjson.Expr{ref(sg, "id")},
			"assign_public_ip": !l.opts.Network.PrivateServices,
		},
		"deployment_circuit_breaker": tfjson.Body{"enable": true, "rollback": true},
		"propagate_tags":             "SERVICE",
	}
	var targets []tfjson.Body
	for _, lb := range l.routingTo(s.Name) {
		l.cfg.Resource("aws_vpc_security_group_ingress_rule", n+"_from_"+id(lb.Name), tfjson.Body{
			"security_group_id":            ref(sg, "id"),
			"description":                  "from load balancer " + lb.Name,
			"referenced_security_group_id": ref("aws_security_group."+id(lb.Name), "id"),
			"ip_protocol":                  "tcp",
			"from_port":                    s.Port,
			"to_port":                      s.Port,
		})
		targets = append(targets, tfjson.Body{
			"target_group_arn": ref(targetGroupAddr(lb.Name, s.Name), "arn"),
			"container_name":   n,
			"container_port":   s.Port,
		})
		// A target group can only be attached once a listener uses it.
		for _, ln := range lb.Listeners {
			dependsOn = append(dependsOn, listenerAddr(lb.Name, ln.Port))
		}
	}
	if len(targets) > 0 {
		svc["load_balancer"] = targets
		svc["health_check_grace_period_seconds"] = 60
	}
	autoscale := s.Scale.Min < s.Scale.Max
	if autoscale {
		// Autoscaling owns the count once the service exists.
		svc["lifecycle"] = tfjson.Body{"ignore_changes": []string{"desired_count"}}
	}
	svc["depends_on"] = dependsOn
	service := l.cfg.Resource("aws_ecs_service", n, svc)

	if autoscale {
		target := l.cfg.Resource("aws_appautoscaling_target", n, tfjson.Body{
			"service_namespace":  "ecs",
			"resource_id":        tfjson.Expr(fmt.Sprintf("service/${aws_ecs_cluster.main.name}/${%s.name}", service)),
			"scalable_dimension": "ecs:service:DesiredCount",
			"min_capacity":       s.Scale.Min,
			"max_capacity":       s.Scale.Max,
		})
		l.cfg.Resource("aws_appautoscaling_policy", n+"_cpu", tfjson.Body{
			"name":               l.physical(255, n, "cpu"),
			"policy_type":        "TargetTrackingScaling",
			"service_namespace":  ref(target, "service_namespace"),
			"resource_id":        ref(target, "resource_id"),
			"scalable_dimension": ref(target, "scalable_dimension"),
			"target_tracking_scaling_policy_configuration": tfjson.Body{
				"target_value": cpuTarget,
				"predefined_metric_specification": tfjson.Body{
					"predefined_metric_type": "ECSServiceAverageCPUUtilization",
				},
			},
		})
	}
}

// securityGroup creates an empty security group for the resource n.
func (l *lowerer) securityGroup(n, description string) string {
	return l.cfg.Resource("aws_security_group", n, tfjson.Body{
		"name":        l.physical(255, n),
		"description": description,
		"vpc_id":      l.net.vpc,
		"tags":        tags(l.physical(255, n)),
	})
}

// routingTo returns the load balancers with a route to the service.
func (l *lowerer) routingTo(service string) []*ir.LoadBalancer {
	var out []*ir.LoadBalancer
	for _, r := range l.p.Resources {
		lb, ok := r.(*ir.LoadBalancer)
		if !ok {
			continue
		}
		for _, rt := range lb.Routes {
			if rt.Service == service {
				out = append(out, lb)
				break
			}
		}
	}
	return out
}

// wiring returns what s's relationships give it: environment variables,
// secrets, task-role policy statements, and the ARNs of the secrets the
// execution role must read. The user's own `env` comes first and wins
// over a generated variable of the same name, with a warning.
func (l *lowerer) wiring(s *ir.Service) (env, secrets []any, statements []any, secretARNs []tfjson.Expr) {
	type variable struct {
		name  string
		value any
		pos   diag.Span
	}
	var generated []variable
	var secretVars []variable
	for _, rel := range l.p.Relations {
		if rel.From != s.Name {
			continue
		}
		prefix := envPrefix(rel.To)
		add := func(suffix string, value any) {
			generated = append(generated, variable{prefix + "_" + suffix, value, rel.Pos})
		}
		switch to := l.p.Resource(rel.To).(type) {
		case *ir.Database:
			if rel.Verb != ir.ConnectsTo {
				continue
			}
			db := "aws_db_instance." + id(to.Name)
			add("HOST", ref(db, "address"))
			add("PORT", strconv.Itoa(dbPort(to.Engine)))
			add("NAME", dbName(to.Name))
			add("USER", dbUser)
			secret := tfjson.Expr(fmt.Sprintf("${%s.master_user_secret[0].secret_arn}", db))
			secretVars = append(secretVars, variable{prefix + "_PASSWORD", tfjson.Expr(string(secret) + ":password::"), rel.Pos})
			secretARNs = append(secretARNs, secret)
		case *ir.Bucket:
			b := "aws_s3_bucket." + id(to.Name)
			add("BUCKET", ref(b, "bucket"))
			var actions []string
			if rel.Verb == ir.Reads || rel.Verb == ir.ReadsAndWrites {
				statements = append(statements, statement([]string{"s3:ListBucket"}, ref(b, "arn")))
				actions = append(actions, "s3:GetObject")
			}
			if rel.Verb == ir.Writes || rel.Verb == ir.ReadsAndWrites {
				actions = append(actions, "s3:PutObject", "s3:DeleteObject", "s3:AbortMultipartUpload")
			}
			statements = append(statements, statement(actions, tfjson.Expr(fmt.Sprintf("${%s.arn}/*", b))))
		case *ir.Queue:
			q := "aws_sqs_queue." + id(to.Name)
			add("QUEUE_URL", ref(q, "url"))
			switch rel.Verb {
			case ir.Reads:
				statements = append(statements, statement([]string{
					"sqs:ReceiveMessage", "sqs:DeleteMessage", "sqs:ChangeMessageVisibility", "sqs:GetQueueAttributes",
				}, ref(q, "arn")))
			case ir.SendsTo:
				statements = append(statements, statement([]string{"sqs:SendMessage", "sqs:GetQueueAttributes"}, ref(q, "arn")))
			}
		}
	}

	env, secrets = []any{}, []any{}
	userSet := map[string]bool{}
	for _, e := range s.Env {
		userSet[e.Name] = true
		env = append(env, map[string]any{"name": e.Name, "value": e.Value})
	}
	clash := func(v variable) bool {
		if !userSet[v.name] {
			return false
		}
		l.diags.Add(diag.Warnf(v.pos, "this sets `%s` on %s, but its `env %s` is used instead", v.name, s.Name, v.name).
			Suggest("rename the `env` variable, or remove it to use the generated value"))
		return true
	}
	for _, v := range generated {
		if !clash(v) {
			env = append(env, map[string]any{"name": v.name, "value": v.value})
		}
	}
	for _, v := range secretVars {
		if !clash(v) {
			secrets = append(secrets, map[string]any{"name": v.name, "valueFrom": v.value})
		}
	}
	return env, secrets, statements, secretARNs
}

func statement(actions []string, resources ...tfjson.Expr) any {
	return map[string]any{"Effect": "Allow", "Action": actions, "Resource": resources}
}

func policy(statements ...any) tfjson.Expr {
	return jsonExpr(map[string]any{"Version": "2012-10-17", "Statement": statements})
}

// jsonExpr is tfjson.JSON for values lowering builds itself, which only
// ever contain types JSON accepts.
func jsonExpr(v any) tfjson.Expr {
	e, err := tfjson.JSON(v)
	if err != nil {
		panic(fmt.Sprintf("lower/aws: built a value that isn't JSON: %v", err))
	}
	return e
}

// fargateSizes are the CPU units (1024 = one vCPU) Fargate offers, each
// with the memory sizes in MiB it allows.
var fargateSizes = []struct {
	cpu    int
	memory []int
}{
	{256, []int{512, 1024, 2048}},
	{512, steps(1024, 4096, 1024)},
	{1024, steps(2048, 8192, 1024)},
	{2048, steps(4096, 16384, 1024)},
	{4096, steps(8192, 30720, 1024)},
	{8192, steps(16384, 61440, 4096)},
	{16384, steps(32768, 122880, 8192)},
}

func steps(from, to, step int) []int {
	var out []int
	for m := from; m <= to; m += step {
		out = append(out, m)
	}
	return out
}

// fargateSize picks the smallest Fargate size with at least the CPU and
// memory s asks for. When that isn't exactly what was asked, it says so,
// since the user pays for the difference.
func (l *lowerer) fargateSize(s *ir.Service) (cpu, memory int) {
	wantCPU := s.CPU * 1024 / 1000 // millicores to CPU units: 250 → 256
	for _, size := range fargateSizes {
		if size.cpu < wantCPU {
			continue
		}
		for _, m := range size.memory {
			if m < s.Memory {
				continue
			}
			cpu, memory = size.cpu, m
			cpuChanged := s.CPU != 0 && cpu != wantCPU
			memChanged := s.Memory != 0 && memory != s.Memory
			switch {
			case cpuChanged && memChanged:
				l.diags.Add(diag.Infof(s.At("cpu"), "on aws: Fargate has no task this size; %s uses %s vCPU and %s", s.Name, vcpus(cpu), mib(memory)))
			case cpuChanged:
				l.diags.Add(diag.Infof(s.At("cpu"), "on aws: Fargate needs at least %s vCPU for %s; %s uses %s vCPU", vcpus(cpu), mib(memory), s.Name, vcpus(cpu)))
			case memChanged:
				l.diags.Add(diag.Infof(s.At("memory"), "on aws: Fargate has no %s vCPU task with %s; %s uses %s", vcpus(cpu), mib(s.Memory), s.Name, mib(memory)))
			}
			return cpu, memory
		}
	}
	// The capability matrix refuses more memory than the largest size
	// holds, so this is unreachable; fall back to the largest size.
	last := fargateSizes[len(fargateSizes)-1]
	return last.cpu, last.memory[len(last.memory)-1]
}

func vcpus(units int) string {
	return strconv.FormatFloat(float64(units)/1024, 'f', -1, 64)
}

func mib(n int) string {
	if n%1024 == 0 {
		return fmt.Sprintf("%dgb", n/1024)
	}
	return fmt.Sprintf("%dmb", n)
}
