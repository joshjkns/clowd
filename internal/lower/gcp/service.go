package gcp

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/emit/tfjson"
	"github.com/joshjkns/clowd/internal/ir"
)

// service lowers s to a Cloud Run service running as its own service
// account, which its relationships grant access to.
//
// A service a load balancer sends to accepts traffic only from load
// balancers, and lets them call it without credentials, as the target
// groups of an ALB would. Other services accept no requests at all, like
// an ECS service no load balancer reaches.
func (l *lowerer) service(s *ir.Service) {
	n := id(s.Name)
	account := l.cfg.Resource("google_service_account", n, tfjson.Body{
		"account_id":   l.accountID(n),
		"display_name": "clowd service " + s.Name,
	})
	w := l.wiring(s, account)

	container := tfjson.Body{
		"image": l.image(s.Image),
		"ports": tfjson.Body{"container_port": s.Port},
		"env":   w.env,
	}
	if s.Command != nil {
		// Command replaces the image's CMD, which Cloud Run calls args;
		// its `command` is the ENTRYPOINT.
		container["args"] = s.Command
	}
	resources := tfjson.Body{}
	if cpu, memory, ok := l.size(s); ok {
		resources["limits"] = map[string]string{"cpu": cpu, "memory": memory}
	}
	if w.readsQueue {
		// A pull subscriber works between requests, so its CPU must stay
		// allocated (instance-based billing).
		resources["cpu_idle"] = false
		if s.Scale.Min == 0 {
			l.diags.Add(diag.Warnf(s.At("scale"), "on gcp: %s reads a queue but can scale to zero, and Cloud Run starts instances only for requests, so messages would wait", s.Name).
				Suggest("scale it from at least 1, such as `scale 1 to %d`", max(s.Scale.Max, 1)))
		}
	}
	if len(resources) > 0 {
		container["resources"] = resources
	}
	if s.HealthCheck != "" {
		// Serverless backends have no load balancer health checks; Cloud
		// Run checks the path itself, holding traffic until it answers
		// and restarting instances that stop.
		probe := tfjson.Body{"http_get": tfjson.Body{"path": s.HealthCheck}}
		container["startup_probe"] = probe
		container["liveness_probe"] = probe
	}

	template := tfjson.Body{
		"service_account": ref(account, "email"),
		"scaling": tfjson.Body{
			"min_instance_count": s.Scale.Min,
			"max_instance_count": s.Scale.Max,
		},
		"containers": []tfjson.Body{container},
	}
	if w.vpc || l.p.Network.Private {
		// Direct VPC egress: the database's private address is reached
		// through the subnet. A private network sends everything that
		// way, out through Cloud NAT.
		egress := "PRIVATE_RANGES_ONLY"
		if l.p.Network.Private {
			egress = "ALL_TRAFFIC"
		}
		template["vpc_access"] = tfjson.Body{
			"network_interfaces": tfjson.Body{"network": l.net.id, "subnetwork": l.net.subnet},
			"egress":             egress,
		}
	}

	body := tfjson.Body{
		"name":     l.physical(49, n),
		"location": l.loc,
		"ingress":  "INGRESS_TRAFFIC_INTERNAL_ONLY",
		"template": template,
		// `clowd destroy` removes what apply created (D15).
		"deletion_protection": false,
	}
	if l.routed(s.Name) {
		body["ingress"] = "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"
		body["invoker_iam_disabled"] = true
	}
	if len(w.dependsOn) > 0 {
		body["depends_on"] = w.dependsOn
	}
	l.cfg.Resource("google_cloud_run_v2_service", n, body)
}

// image is where Cloud Run pulls ref from. It pulls only from Artifact
// Registry and Docker Hub, so an image on another registry (ghcr.io,
// quay.io, ...) goes through an Artifact Registry remote repository that
// proxies it, one per registry.
func (l *lowerer) image(image string) any {
	host, path, ok := strings.Cut(image, "/")
	if !ok || !isRegistryHost(host) || pullsDirectly(host) {
		return image
	}
	name := slug(host)
	addr := "google_artifact_registry_repository." + name
	if l.cfg.Find(addr) == nil {
		l.cfg.Resource("google_artifact_registry_repository", name, tfjson.Body{
			"repository_id": l.physical(63, name),
			"location":      l.loc,
			"format":        "DOCKER",
			"mode":          "REMOTE_REPOSITORY",
			"description":   "images from " + host + ", for Cloud Run",
			"remote_repository_config": tfjson.Body{
				"docker_repository": tfjson.Body{"custom_repository": tfjson.Body{"uri": "https://" + host}},
			},
		})
	}
	return tfjson.Expr("${" + addr + ".registry_uri}/" + tfjson.Escape(path))
}

// isRegistryHost reports whether the first part of an image reference
// names a registry, as Docker decides: it has a dot or a port, or is
// localhost. Otherwise the image is on Docker Hub (`me/api`).
func isRegistryHost(s string) bool {
	return strings.ContainsAny(s, ".:") || s == "localhost"
}

// pullsDirectly reports whether Cloud Run pulls from the registry host
// itself.
func pullsDirectly(host string) bool {
	switch {
	case host == "docker.io", host == "index.docker.io", host == "registry-1.docker.io":
		return true
	case host == "gcr.io", strings.HasSuffix(host, ".gcr.io"), strings.HasSuffix(host, "-docker.pkg.dev"):
		return true
	}
	return false
}

// accountID is a service account ID for the service n: 6 to 30
// characters.
func (l *lowerer) accountID(n string) string {
	name := l.physical(30, n)
	if len(name) < 6 {
		name = l.physical(30, n, "run")
	}
	return name
}

// routed reports whether a load balancer sends to the service.
func (l *lowerer) routed(service string) bool {
	for _, r := range l.p.Resources {
		if lb, ok := r.(*ir.LoadBalancer); ok {
			for _, rt := range lb.Routes {
				if rt.Service == service {
					return true
				}
			}
		}
	}
	return false
}

// wired is what a service's relationships give it.
type wired struct {
	env []tfjson.Body
	// dependsOn are the grants the service needs before it starts.
	dependsOn []string
	// vpc is set when the service reaches a database's private address.
	vpc bool
	// readsQueue is set when the service pulls from a subscription.
	readsQueue bool
}

// wiring grants account what s's relationships need and returns the
// environment variables they set. The user's own `env` comes first and
// wins over a generated variable of the same name, with a warning.
func (l *lowerer) wiring(s *ir.Service, account string) wired {
	n := id(s.Name)
	type variable struct {
		name  string
		value tfjson.Body
		pos   diag.Span
	}
	var w wired
	var generated []variable
	grant := func(typ, target, role string, key string, value any) string {
		return l.cfg.Resource(typ, n+"_"+id(target), tfjson.Body{
			key:      value,
			"role":   role,
			"member": member(account),
		})
	}
	// A service that both reads and sends to a queue needs the
	// subscription and the topic; the subscription keeps the usual name.
	reads := map[string]bool{}
	for _, rel := range l.p.Relations {
		if rel.From == s.Name && rel.Verb == ir.Reads {
			reads[rel.To] = true
		}
	}
	for _, rel := range l.p.Relations {
		if rel.From != s.Name {
			continue
		}
		prefix := envPrefix(rel.To)
		add := func(suffix string, value any) {
			generated = append(generated, variable{prefix + "_" + suffix, tfjson.Body{"value": value}, rel.Pos})
		}
		switch to := l.p.Resource(rel.To).(type) {
		case *ir.Database:
			if rel.Verb != ir.ConnectsTo {
				continue
			}
			db := "google_sql_database_instance." + id(to.Name)
			add("HOST", ref(db, "private_ip_address"))
			add("PORT", strconv.Itoa(dbPort(to.Engine)))
			add("NAME", dbName(to.Name))
			add("USER", dbUser)
			secret := "google_secret_manager_secret." + id(to.Name) + "_password"
			generated = append(generated, variable{prefix + "_PASSWORD", tfjson.Body{
				"value_source": tfjson.Body{"secret_key_ref": tfjson.Body{"secret": ref(secret, "secret_id"), "version": "latest"}},
			}, rel.Pos})
			access := l.cfg.Resource("google_secret_manager_secret_iam_member", n+"_"+id(to.Name)+"_password", tfjson.Body{
				"secret_id": ref(secret, "id"),
				"role":      "roles/secretmanager.secretAccessor",
				"member":    member(account),
			})
			// The first revision reads the password as it starts.
			w.dependsOn = append(w.dependsOn, access, "google_secret_manager_secret_version."+id(to.Name)+"_password")
			w.vpc = true
		case *ir.Bucket:
			b := "google_storage_bucket." + id(to.Name)
			add("BUCKET", ref(b, "name"))
			// objectUser reads, writes and deletes; there is no role that
			// writes without reading (the capability matrix says so).
			role := "roles/storage.objectUser"
			if rel.Verb == ir.Reads {
				role = "roles/storage.objectViewer"
			}
			grant("google_storage_bucket_iam_member", to.Name, role, "bucket", ref(b, "name"))
		case *ir.Queue:
			topic := "google_pubsub_topic." + id(to.Name)
			sub := "google_pubsub_subscription." + id(to.Name)
			switch rel.Verb {
			case ir.Reads:
				w.readsQueue = true
				add("QUEUE_URL", ref(sub, "id"))
				grant("google_pubsub_subscription_iam_member", to.Name+"_read", "roles/pubsub.subscriber", "subscription", ref(sub, "name"))
			case ir.SendsTo:
				if reads[rel.To] {
					add("TOPIC", ref(topic, "id"))
				} else {
					add("QUEUE_URL", ref(topic, "id"))
				}
				grant("google_pubsub_topic_iam_member", to.Name+"_send", "roles/pubsub.publisher", "topic", ref(topic, "name"))
			}
		}
	}

	userSet := map[string]bool{}
	w.env = []tfjson.Body{}
	for _, e := range s.Env {
		userSet[e.Name] = true
		w.env = append(w.env, tfjson.Body{"name": e.Name, "value": e.Value})
	}
	for _, v := range generated {
		if userSet[v.name] {
			l.diags.Add(diag.Warnf(v.pos, "this sets `%s` on %s, but its `env %s` is used instead", v.name, s.Name, v.name).
				Suggest("rename the `env` variable, or remove it to use the generated value"))
			continue
		}
		v.value["name"] = v.name
		w.env = append(w.env, v.value)
	}
	return w
}

// runSizes are the whole vCPUs Cloud Run offers, each with the most
// memory, in MiB, it allows. Fractional CPUs exist too, but only with one
// request at a time, so smaller sizes round up to one vCPU.
var runSizes = []struct{ cpu, memory int }{
	{1, 4096},
	{2, 8192},
	{4, 16384},
	{6, 24576},
	{8, 32768},
}

// minMemory is the least memory a service has: the minimum of Cloud Run's
// second-generation environment, and its default.
const minMemory = 512

// minMemoryFor is the least memory Cloud Run allows with cpu vCPUs.
func minMemoryFor(cpu int) int {
	switch {
	case cpu >= 6:
		return 4096
	case cpu >= 4:
		return 2048
	}
	return minMemory
}

// size picks Cloud Run limits for s: the smallest size with at least the
// CPU and memory it asks for. ok is false when s asks for neither, so
// Cloud Run's defaults apply. When the size isn't what was asked, it
// says so, since the user pays for the difference.
func (l *lowerer) size(s *ir.Service) (cpu, memory string, ok bool) {
	if s.CPU == 0 && s.Memory == 0 {
		return "", "", false
	}
	wantCPU := (s.CPU + 999) / 1000 // millicores, rounded up to whole vCPUs
	for _, size := range runSizes {
		if size.cpu < wantCPU || size.memory < s.Memory {
			continue
		}
		mem := max(s.Memory, minMemoryFor(size.cpu))
		if s.CPU != 0 && s.CPU < 1000 {
			l.diags.Add(diag.Infof(s.At("cpu"), "on gcp: Cloud Run runs services with less than 1 vCPU one request at a time; %s uses 1 vCPU", s.Name))
		}
		if size.cpu > max(wantCPU, 1) {
			l.diags.Add(diag.Infof(s.At("memory"), "on gcp: Cloud Run needs %d vCPU for %s; %s uses %d vCPU", size.cpu, mib(s.Memory), s.Name, size.cpu))
		}
		if s.Memory != 0 && mem != s.Memory {
			l.diags.Add(diag.Infof(s.At("memory"), "on gcp: Cloud Run needs at least %s with %d vCPU; %s uses %s", mib(mem), size.cpu, s.Name, mib(mem)))
		}
		return strconv.Itoa(size.cpu), fmt.Sprintf("%dMi", mem), true
	}
	// The capability matrix refuses more memory than the largest size
	// holds, so this is unreachable; fall back to the largest size.
	last := runSizes[len(runSizes)-1]
	return strconv.Itoa(last.cpu), fmt.Sprintf("%dMi", last.memory), true
}

func mib(n int) string {
	if n%1024 == 0 {
		return fmt.Sprintf("%dgb", n/1024)
	}
	return fmt.Sprintf("%dmb", n)
}
