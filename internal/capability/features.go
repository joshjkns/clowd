package capability

import (
	"fmt"
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
	ReadsQueue         Feature = "service reads queue"
	SendsToQueue       Feature = "service sends to queue"

	Overrides Feature = "provider overrides"
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
		if r.CPU != 0 {
			add(ServiceCPU, "cpu")
		}
		if r.Memory != 0 {
			uses = append(uses, use{feature: ServiceMemory, pos: h.At("memory"), check: memoryCheck(r.Memory)})
		}
		if len(r.Env) > 0 {
			add(ServiceEnv, "env")
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
	case *ir.Bucket:
		if r.Public {
			add(PublicBucket, "visibility")
		}
	case *ir.Queue:
		if r.FIFO {
			add(FIFOQueue, "fifo")
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
	case k == ir.BucketKind:
		return BucketAccess, true
	case rel.Verb == ir.Reads && k == ir.QueueKind:
		return ReadsQueue, true
	case rel.Verb == ir.SendsTo && k == ir.QueueKind:
		return SendsToQueue, true
	}
	return "", false
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
