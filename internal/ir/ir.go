// Package ir is the cloud-neutral resource graph: the contract between the
// language above it and the per-cloud backends below it.
//
// A Program is built by check from a resolved AST. Every value in it has
// been validated and interpreted (`2 to 10` is a Scale, `512mb` is a number
// of MiB) and defaults have been filled in, so capability evaluation and
// lowering never re-read source text. Spans are kept so later stages can
// still point diagnostics at the line that asked for a feature.
//
// The IR does not import the AST: lowering must not depend on syntax.
package ir

import (
	"fmt"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
)

// Program is a checked file.
type Program struct {
	// Targets are the file's `use` lines, in source order.
	Targets []Target
	// Resources are in source order.
	Resources []Resource
	Relations []Relation
	Overrides []Override
	Project   Project
	Network   Network
}

// Project is the `project` declaration. Without one, every field takes
// its default.
type Project struct {
	// Header.Name is the project name, which prefixes every deployed name.
	// Empty means the deploy tool's default, the directory's name.
	Header
	// Edition is the language edition the file is written in; 0 means the
	// latest.
	Edition int
	Tags    []Tag
}

// Tag is a `tag <key> <value>` property: an AWS tag and a GCP label on
// every resource.
type Tag struct {
	Key, Value string
	Pos        diag.Span
}

// Network is where resources are placed: the `network` declaration and
// the per-cloud `network` settings. Without them, each backend creates a
// network with its defaults.
type Network struct {
	// Header.Pos is the declaration, or zero without one; Header.Name is
	// always empty.
	Header
	// Private runs services in private subnets that reach the internet
	// through NAT, instead of giving them public addresses.
	Private bool
	// CIDR is the address block of the network Clowd creates; empty means
	// the backend's default.
	CIDR string
	// Existing are networks to use instead of creating one, at most one
	// per cloud.
	Existing []ExistingNetwork
}

// ExistingNetwork is an `on <cloud>` `network` block naming a network
// that already exists.
type ExistingNetwork struct {
	Cloud Cloud
	// VPC is the network's ID.
	VPC string
	// PublicSubnets hold load balancers (and public services);
	// PrivateSubnets hold databases (and private services).
	PublicSubnets, PrivateSubnets []string
	Pos                           diag.Span
}

// ExistingOn returns the existing network to use on cloud, or nil.
func (n *Network) ExistingOn(cloud Cloud) *ExistingNetwork {
	for i := range n.Existing {
		if n.Existing[i].Cloud == cloud {
			return &n.Existing[i]
		}
	}
	return nil
}

// Resource returns the resource called name (as declared), or nil.
func (p *Program) Resource(name string) Resource {
	for _, r := range p.Resources {
		if r.Meta().Name == name {
			return r
		}
	}
	return nil
}

// Cloud is a deploy target.
type Cloud int

// Known clouds. Azure is known to the language but has no backend yet.
const (
	AWS Cloud = iota + 1
	GCP
	Azure
)

// Clouds lists every known cloud in canonical order.
var Clouds = []Cloud{AWS, GCP, Azure}

func (c Cloud) String() string {
	switch c {
	case AWS:
		return "aws"
	case GCP:
		return "gcp"
	case Azure:
		return "azure"
	}
	return fmt.Sprintf("Cloud(%d)", int(c))
}

// ParseCloud returns the cloud named s, ignoring case.
func ParseCloud(s string) (Cloud, bool) {
	for _, c := range Clouds {
		if strings.EqualFold(s, c.String()) {
			return c, true
		}
	}
	return 0, false
}

// Target is a `use` line.
type Target struct {
	Cloud Cloud
	// Region is empty when the `use` line has no `in` clause.
	Region string
	Pos    diag.Span
}

// Kind is a resource kind.
type Kind int

// Resource kinds.
const (
	ServiceKind Kind = iota + 1
	DatabaseKind
	BucketKind
	LoadBalancerKind
	QueueKind
)

func (k Kind) String() string {
	switch k {
	case ServiceKind:
		return "service"
	case DatabaseKind:
		return "database"
	case BucketKind:
		return "bucket"
	case LoadBalancerKind:
		return "load balancer"
	case QueueKind:
		return "queue"
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// Resource is one of *Service, *Database, *Bucket, *LoadBalancer or *Queue.
type Resource interface {
	Meta() *Header
	Kind() Kind
}

// Header holds what every resource has.
type Header struct {
	// Name is the name as declared.
	Name string
	// Pos is the span of the name in the declaration.
	Pos diag.Span
	// Spans records where each feature was written, keyed by the property
	// or trait group name (`scale`, `engine`, `visibility`, ...). Features
	// that took their default are absent.
	Spans map[string]diag.Span
}

// Meta returns h; embedding Header gives each resource the method.
func (h *Header) Meta() *Header { return h }

// At returns where key was written, or the resource name if it took its
// default.
func (h *Header) At(key string) diag.Span {
	if sp, ok := h.Spans[key]; ok {
		return sp
	}
	return h.Pos
}

// Has reports whether key was written explicitly.
func (h *Header) Has(key string) bool {
	_, ok := h.Spans[key]
	return ok
}

// Service is a long-running container.
type Service struct {
	Header
	Image string
	// Port is the container port; defaults to 8080.
	Port  int
	Scale Scale
	// CPU is in millicores (1000 = one vCPU); 0 means the cloud's default.
	CPU int
	// Memory is in MiB; 0 means the cloud's default.
	Memory int
	Env    []EnvVar
	// HealthCheck is the path load balancers check; empty means `/`,
	// where any answer below 500 counts as healthy.
	HealthCheck string
	// Command replaces the image's command when set.
	Command []string
}

// Scale is an instance count range. Min == Max is a fixed count.
type Scale struct {
	Min, Max int
}

// EnvVar is one `env` property.
type EnvVar struct {
	Name, Value string
	Pos         diag.Span
}

// Engine is a database engine.
type Engine int

// Database engines.
const (
	Postgres Engine = iota + 1
	MySQL
)

func (e Engine) String() string {
	switch e {
	case Postgres:
		return "postgres"
	case MySQL:
		return "mysql"
	}
	return fmt.Sprintf("Engine(%d)", int(e))
}

// Size is a database size class; lowering maps it to an instance type.
type Size int

// Database sizes.
const (
	Small Size = iota + 1
	Medium
	Large
)

func (s Size) String() string {
	switch s {
	case Small:
		return "small"
	case Medium:
		return "medium"
	case Large:
		return "large"
	}
	return fmt.Sprintf("Size(%d)", int(s))
}

// Database is a managed relational database.
type Database struct {
	Header
	Engine Engine
	// Version is as written (`16`, `8.0`); empty means the backend's
	// default for the engine.
	Version string
	Size    Size
	// HighlyAvailable keeps a standby in another zone that takes over on
	// failure.
	HighlyAvailable bool
	// Protected databases can't be deleted, and keep a final snapshot.
	Protected bool
	// StorageGB is the initial disk size; 0 means the backend's default.
	StorageGB int
	// BackupDays is how long daily backups are kept. 0 means the
	// backend's default, unless Has("backups"), when backups are off.
	BackupDays int
}

// Bucket is object storage.
type Bucket struct {
	Header
	Public bool
	// Versioned buckets keep every version of an object.
	Versioned bool
	// Protected buckets can't be deleted while they hold objects.
	Protected bool
	// ExpireDays deletes objects this many days after they are written;
	// 0 keeps them.
	ExpireDays int
}

// Queue is a message queue.
type Queue struct {
	Header
	FIFO bool
	// Retries is how many times a failed message is retried before it
	// moves to a dead-letter queue; 0 means no dead-letter queue.
	Retries int
	// Timeout is how long, in seconds, a consumer has to handle a
	// message before it is delivered again; 0 means the backend's
	// default.
	Timeout int
	// Retention is how long, in seconds, unhandled messages are kept; 0
	// means the backend's default.
	Retention int
}

// LoadBalancer is an HTTP(S) load balancer.
type LoadBalancer struct {
	Header
	Internal  bool
	Listeners []Listener
	Routes    []Route
}

// Listener is a `listen on` property.
type Listener struct {
	Port int
	// Domain is set when the listener terminates TLS.
	Domain string
	Pos    diag.Span
}

// TLS reports whether the listener terminates TLS.
func (l Listener) TLS() bool { return l.Domain != "" }

// Route is a `send <path> to <service>` property. Paths are prefixes.
type Route struct {
	Path string
	// Service is the target's declared name.
	Service string
	Pos     diag.Span
}

func (*Service) Kind() Kind      { return ServiceKind }
func (*Database) Kind() Kind     { return DatabaseKind }
func (*Bucket) Kind() Kind       { return BucketKind }
func (*Queue) Kind() Kind        { return QueueKind }
func (*LoadBalancer) Kind() Kind { return LoadBalancerKind }

// Verb is a relationship verb.
type Verb int

// Relationship verbs.
const (
	ConnectsTo Verb = iota + 1
	Reads
	Writes
	ReadsAndWrites
	SendsTo
)

func (v Verb) String() string {
	switch v {
	case ConnectsTo:
		return "connects to"
	case Reads:
		return "reads"
	case Writes:
		return "writes"
	case ReadsAndWrites:
		return "reads and writes"
	case SendsTo:
		return "sends to"
	}
	return fmt.Sprintf("Verb(%d)", int(v))
}

// Relation is a relationship between two resources, by declared name.
type Relation struct {
	From string
	Verb Verb
	To   string
	Pos  diag.Span
}

// Override is an `on <cloud>` block inside a resource: raw provider
// settings applied only when lowering to Cloud.
type Override struct {
	Cloud Cloud
	// Resource is the resource's declared name.
	Resource string
	Kind     Kind
	Settings []Setting
	// Pos is the `on <cloud>` line.
	Pos diag.Span
}

// Setting is one raw line in an `on` block: a provider argument and its
// value, or a nested block of settings.
type Setting struct {
	// Key is the provider argument, exactly as written.
	Key    string
	KeyPos diag.Span
	// Value is the value words joined by single spaces. List is set
	// instead for a comma list, and Block for a nested block.
	Value string
	List  []string
	Block []Setting
	Pos   diag.Span
}
