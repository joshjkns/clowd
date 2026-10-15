package capability

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/ir"
)

// Feature is something a program can ask for. The name is what users see
// in notes and in the generated support table.
type Feature string

// Resource kinds. A kind feature is listed first among a resource's uses.
const (
	Service      Feature = "service"
	Database     Feature = "database"
	Bucket       Feature = "bucket"
	LoadBalancer Feature = "load balancer"
	Queue        Feature = "queue"
)

// Features of resources, relations and blocks.
const (
	ServiceAutoscale   Feature = "service autoscaling"
	ServiceScaleToZero Feature = "service scaling to zero"
	ServiceCPU         Feature = "service cpu size"
	ServiceMemory      Feature = "service memory size"
	ServiceEnv         Feature = "service environment variables"
	ServiceRegistry    Feature = "service image from a registry other than Docker Hub"

	Postgres        Feature = "postgres database"
	MySQL           Feature = "mysql database"
	DatabaseVersion Feature = "database version"

	PublicBucket Feature = "public bucket"
	FIFOQueue    Feature = "fifo queue"

	InternalLB    Feature = "internal load balancer"
	LBTLS         Feature = "tls listener"
	LBPathRouting Feature = "path routing"
	LBOddPort     Feature = "public listener on a port other than 80, 8080 or 443"

	ConnectsToDatabase Feature = "service connects to database"
	ConnectsToService  Feature = "service connects to service"
	BucketAccess       Feature = "service reads or writes bucket"
	WritesBucket       Feature = "service writes bucket without reading it"
	ReadsQueue         Feature = "service reads queue"
	SendsToQueue       Feature = "service sends to queue"

	Overrides Feature = "provider overrides"

	ProjectTags    Feature = "project tags"
	PrivateNetwork Feature = "private network"
	NetworkCIDR    Feature = "network address block"

	ServiceHealthCheck Feature = "service health check path"
	ServiceCommand     Feature = "service command"
	DatabaseHA         Feature = "highly available database"
	DatabaseProtected  Feature = "protected database"
	DatabaseStorage    Feature = "database storage size"
	DatabaseBackups    Feature = "database backup retention"
	BucketVersioning   Feature = "versioned bucket"
	BucketProtected    Feature = "protected bucket"
	BucketExpiry       Feature = "bucket object expiry"
	QueueRetry         Feature = "queue retries with a dead-letter queue"
	QueueTimeout       Feature = "queue timeout"
	QueueRetention     Feature = "queue message retention"
)

func (f Feature) isKind() bool {
	switch f {
	case Service, Database, Bucket, LoadBalancer, Queue:
		return true
	}
	return false
}

// use is one feature asked for at a place in the source.
type use struct {
	feature Feature
	pos     diag.Span
	// check, if set, decides the entry instead of the matrix; used for
	// features whose answer depends on a value, such as a version.
	check func(ir.Cloud) Entry
}

func (u use) entry(cloud ir.Cloud) Entry {
	if u.check != nil {
		return u.check(cloud)
	}
	return Lookup(u.feature, cloud)
}

func kindFeature(k ir.Kind) Feature {
	switch k {
	case ir.ServiceKind:
		return Service
	case ir.DatabaseKind:
		return Database
	case ir.BucketKind:
		return Bucket
	case ir.LoadBalancerKind:
		return LoadBalancer
	case ir.QueueKind:
		return Queue
	}
	return ""
}

// resourceFeatures lists what r asks for, kind first.
func resourceFeatures(r ir.Resource) []use {
	h := r.Meta()
	uses := []use{{feature: kindFeature(r.Kind()), pos: h.Pos}}
	add := func(f Feature, key string) { uses = append(uses, use{feature: f, pos: h.At(key)}) }

	switch r := r.(type) {
	case *ir.Service:
		if r.Scale.Min < r.Scale.Max {
			add(ServiceAutoscale, "scale")
		}
		if r.Scale.Min == 0 {
			add(ServiceScaleToZero, "scale")
		}
		if host, ok := registry(r.Image); ok {
			uses = append(uses, use{feature: ServiceRegistry, pos: h.At("image"), check: registryCheck(host)})
		}
		if r.CPU != 0 {
			add(ServiceCPU, "cpu")
		}
		if r.Memory != 0 {
			uses = append(uses, use{feature: ServiceMemory, pos: h.At("memory"), check: memoryCheck(r.Memory)})
		}
		if len(r.Env) > 0 {
			add(ServiceEnv, "env")
		}
		if r.HealthCheck != "" {
			add(ServiceHealthCheck, "health")
		}
		if r.Command != nil {
			add(ServiceCommand, "command")
		}
	case *ir.Database:
		switch r.Engine {
		case ir.Postgres:
			add(Postgres, "engine")
		case ir.MySQL:
			add(MySQL, "engine")
		}
		if r.Version != "" && r.Engine != 0 {
			uses = append(uses, use{feature: DatabaseVersion, pos: h.At("engine"), check: versionCheck(r.Engine, r.Version)})
		}
		if r.HighlyAvailable {
			uses = append(uses, use{feature: DatabaseHA, pos: h.At("availability"), check: haCheck(r)})
		}
		if r.Protected {
			add(DatabaseProtected, "protection")
		}
		if r.StorageGB != 0 {
			add(DatabaseStorage, "storage")
		}
		if r.Has("backups") {
			uses = append(uses, use{feature: DatabaseBackups, pos: h.At("backups"), check: rangeCheck(DatabaseBackups, r.BackupDays, maxBackupDays, "backups are kept for at most %d days")})
		}
	case *ir.Bucket:
		if r.Public {
			add(PublicBucket, "visibility")
		}
		if r.Versioned {
			add(BucketVersioning, "versioning")
		}
		if r.Protected {
			add(BucketProtected, "protection")
		}
		if r.ExpireDays != 0 {
			add(BucketExpiry, "expire")
		}
	case *ir.Queue:
		if r.FIFO {
			add(FIFOQueue, "fifo")
		}
		if r.Retries != 0 {
			// Clouds count deliveries: the first try and the retries.
			uses = append(uses, use{feature: QueueRetry, pos: h.At("retry"), check: limitCheck(QueueRetry, r.Retries+1, queueDeliveries,
				func(lo, hi int) string { return fmt.Sprintf("a message can be retried %d to %d times", lo-1, hi-1) })})
		}
		if r.Timeout != 0 {
			uses = append(uses, use{feature: QueueTimeout, pos: h.At("timeout"), check: limitCheck(QueueTimeout, r.Timeout, queueTimeouts,
				func(lo, hi int) string {
					return fmt.Sprintf("the timeout is between %s and %s", seconds(lo), seconds(hi))
				})})
		}
		if r.Retention != 0 {
			uses = append(uses, use{feature: QueueRetention, pos: h.At("keep"), check: limitCheck(QueueRetention, r.Retention, queueRetention,
				func(lo, hi int) string {
					return fmt.Sprintf("messages are kept for %s to %s", seconds(lo), seconds(hi))
				})})
		}
	case *ir.LoadBalancer:
		if r.Internal {
			add(InternalLB, "visibility")
		}
		if i := slices.IndexFunc(r.Listeners, ir.Listener.TLS); i >= 0 {
			uses = append(uses, use{feature: LBTLS, pos: r.Listeners[i].Pos})
		}
		if !r.Internal {
			for _, l := range r.Listeners {
				if !standardPort(l) {
					uses = append(uses, use{feature: LBOddPort, pos: l.Pos})
					break
				}
			}
		}
		if i := slices.IndexFunc(r.Routes, func(rt ir.Route) bool { return rt.Path != "/" }); i >= 0 {
			uses = append(uses, use{feature: LBPathRouting, pos: r.Routes[i].Pos})
		}
	}
	return uses
}

// standardPort reports whether l uses a port every cloud's public load
// balancer accepts for its protocol.
func standardPort(l ir.Listener) bool {
	if l.TLS() {
		return l.Port == 443
	}
	return l.Port == 80 || l.Port == 8080
}

// relationFeature returns the feature for rel. check has already rejected
// relations that don't apply.
func relationFeature(p *ir.Program, rel ir.Relation) (Feature, bool) {
	to := p.Resource(rel.To)
	if to == nil {
		return "", false
	}
	switch k := to.Kind(); {
	case rel.Verb == ir.ConnectsTo && k == ir.DatabaseKind:
		return ConnectsToDatabase, true
	case rel.Verb == ir.ConnectsTo && k == ir.ServiceKind:
		return ConnectsToService, true
	case rel.Verb == ir.Writes && k == ir.BucketKind:
		return WritesBucket, true
	case k == ir.BucketKind:
		return BucketAccess, true
	case rel.Verb == ir.Reads && k == ir.QueueKind:
		return ReadsQueue, true
	case rel.Verb == ir.SendsTo && k == ir.QueueKind:
		return SendsToQueue, true
	}
	return "", false
}

// programFeatures lists what the project and network declarations ask
// for, each as a row label and its uses; rows without uses are left out.
func programFeatures(p *ir.Program) (rows []struct {
	label string
	pos   diag.Span
	uses  []use
}) {
	add := func(label string, pos diag.Span, uses []use) {
		if len(uses) > 0 {
			rows = append(rows, struct {
				label string
				pos   diag.Span
				uses  []use
			}{label, pos, uses})
		}
	}
	pr := p.Project
	var uses []use
	for _, t := range pr.Tags {
		uses = append(uses, use{feature: ProjectTags, pos: t.Pos, check: tagCheck(t.Value)})
	}
	label := "project"
	if pr.Name != "" {
		label += " " + pr.Name
	}
	add(label, pr.Pos, uses)

	n := p.Network
	uses = nil
	if n.Private {
		uses = append(uses, use{feature: PrivateNetwork, pos: n.At("services")})
	}
	if n.CIDR != "" {
		uses = append(uses, use{feature: NetworkCIDR, pos: n.At("cidr")})
	}
	add("network", n.Pos, uses)
	return rows
}

// gcpLabelValue is what GCP accepts as a label value.
var gcpLabelValue = regexp.MustCompile(`^[a-z0-9_-]{0,63}$`)

// tagCheck returns a check that a cloud accepts value as a tag value.
func tagCheck(value string) func(ir.Cloud) Entry {
	return func(cloud ir.Cloud) Entry {
		if cloud == ir.GCP && !gcpLabelValue.MatchString(value) {
			return Entry{Level: Unsupported, Note: "label values are up to 63 lowercase letters, digits, `_` and `-`"}
		}
		return Lookup(ProjectTags, cloud)
	}
}

// haCheck returns a check that a cloud can keep a standby of d.
func haCheck(d *ir.Database) func(ir.Cloud) Entry {
	return func(cloud ir.Cloud) Entry {
		if cloud == ir.GCP && d.Has("backups") && d.BackupDays == 0 {
			return Entry{Level: Unsupported, Note: "a highly available Cloud SQL database needs backups; remove `backups off`"}
		}
		return Lookup(DatabaseHA, cloud)
	}
}

// rangeCheck returns a check that n is at most a cloud's limit in max;
// clouds without a limit take the matrix entry.
func rangeCheck(f Feature, n int, max map[ir.Cloud]int, note string) func(ir.Cloud) Entry {
	return func(cloud ir.Cloud) Entry {
		if limit, ok := max[cloud]; ok && n > limit {
			return Entry{Level: Unsupported, Note: fmt.Sprintf(note, limit)}
		}
		return Lookup(f, cloud)
	}
}

// limitCheck returns a check that n is within a cloud's [lo, hi] in
// limits; note describes the range.
func limitCheck(f Feature, n int, limits map[ir.Cloud][2]int, note func(lo, hi int) string) func(ir.Cloud) Entry {
	return func(cloud ir.Cloud) Entry {
		if l, ok := limits[cloud]; ok && (n < l[0] || n > l[1]) {
			return Entry{Level: Unsupported, Note: note(l[0], l[1])}
		}
		return Lookup(f, cloud)
	}
}

// seconds writes a number of seconds in the largest unit that divides it.
func seconds(s int) string {
	for _, u := range []struct {
		n    int
		unit string
	}{{86400, "d"}, {3600, "h"}, {60, "m"}} {
		if s >= u.n && s%u.n == 0 {
			return fmt.Sprintf("%d%s", s/u.n, u.unit)
		}
	}
	return fmt.Sprintf("%ds", s)
}

// registry returns the registry host of an image reference, if it names
// one other than Docker Hub: a first part with a dot or a port, or
// localhost, as Docker decides.
func registry(image string) (string, bool) {
	host, _, ok := strings.Cut(image, "/")
	if !ok || !strings.ContainsAny(host, ".:") && host != "localhost" {
		return "", false
	}
	switch host {
	case "docker.io", "index.docker.io", "registry-1.docker.io":
		return "", false
	}
	return host, true
}

// registryCheck returns a check that a cloud pulls images from host.
// Cloud Run pulls only from Artifact Registry (and Docker Hub), so other
// registries go through a proxy repository.
func registryCheck(host string) func(ir.Cloud) Entry {
	return func(cloud ir.Cloud) Entry {
		if cloud == ir.GCP && (host == "gcr.io" || strings.HasSuffix(host, ".gcr.io") || strings.HasSuffix(host, "-docker.pkg.dev")) {
			return ok
		}
		return Lookup(ServiceRegistry, cloud)
	}
}

// memoryCheck returns a check that a cloud's services can have mib of
// memory.
func memoryCheck(mib int) func(ir.Cloud) Entry {
	return func(cloud ir.Cloud) Entry {
		e := Lookup(ServiceMemory, cloud)
		if limit, ok := maxMemory[cloud]; ok && mib > limit {
			return Entry{Level: Unsupported, Note: fmt.Sprintf("services have at most %dgb of memory", limit/1024)}
		}
		return e
	}
}

// versionCheck returns a check that a cloud offers engine at version. A
// version matches a known one exactly or by refining it (`16.4` is a `16`).
func versionCheck(engine ir.Engine, version string) func(ir.Cloud) Entry {
	return func(cloud ir.Cloud) Entry {
		known, ok := versions[cloud][engine]
		if !ok {
			return Entry{Level: NotImplemented}
		}
		for _, k := range known {
			if version == k || strings.HasPrefix(version, k+".") {
				return Entry{Level: Supported}
			}
		}
		return Entry{Level: Unsupported, Note: fmt.Sprintf("%s %s is not offered; versions are %s", engine, version, list(known))}
	}
}

func list(words []string) string {
	if len(words) <= 1 {
		return strings.Join(words, "")
	}
	return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
}
