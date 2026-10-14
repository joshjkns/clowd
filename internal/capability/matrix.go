package capability

import "github.com/joshjkns/clowd/internal/ir"

// The matrix. A cloud missing from a feature's entry has not implemented it
// (see Lookup), so Azure appears nowhere yet.
//
// Keep notes short and about the user's file, not about Clowd's internals:
// they are shown under the line that asked for the feature.

var ok = Entry{Level: Supported}

func emulated(note string) Entry    { return Entry{Level: Emulated, Note: note} }
func unsupported(note string) Entry { return Entry{Level: Unsupported, Note: note} }

// notYet marks a feature the cloud could provide but Clowd doesn't yet.
// Listing it explicitly (rather than leaving it out) records that it is
// known backlog, not an oversight.
var notYet = Entry{Level: NotImplemented}

var matrix = map[Feature]map[ir.Cloud]Entry{
	Service:            {ir.AWS: ok, ir.GCP: ok},
	ServiceAutoscale:   {ir.AWS: ok, ir.GCP: ok},
	ServiceScaleToZero: {ir.AWS: unsupported("ECS Fargate can't start a task for an incoming request, so the minimum must be at least 1"), ir.GCP: ok},
	ServiceCPU:         {ir.AWS: ok, ir.GCP: ok},
	ServiceMemory:      {ir.AWS: ok, ir.GCP: ok},
	ServiceEnv:         {ir.AWS: ok, ir.GCP: ok},

	Database: {ir.AWS: ok, ir.GCP: ok},
	Postgres: {ir.AWS: ok, ir.GCP: ok},
	MySQL:    {ir.AWS: ok, ir.GCP: ok},

	Bucket:       {ir.AWS: ok, ir.GCP: ok},
	PublicBucket: {ir.AWS: ok, ir.GCP: ok},

	Queue:     {ir.AWS: ok, ir.GCP: ok},
	FIFOQueue: {ir.AWS: ok, ir.GCP: emulated("Pub/Sub has no FIFO queues; this uses an ordered, exactly-once subscription, which limits throughput")},

	LoadBalancer:  {ir.AWS: ok, ir.GCP: ok},
	InternalLB:    {ir.AWS: ok, ir.GCP: ok},
	LBTLS:         {ir.AWS: ok, ir.GCP: ok},
	LBPathRouting: {ir.AWS: ok, ir.GCP: ok},
	LBOddPort:     {ir.AWS: ok, ir.GCP: unsupported("public load balancers serve HTTP on 80 or 8080 and HTTPS on 443")},

	ConnectsToDatabase: {ir.AWS: ok, ir.GCP: ok},
	ConnectsToService:  {ir.AWS: notYet, ir.GCP: notYet},
	BucketAccess:       {ir.AWS: ok, ir.GCP: ok},
	ReadsQueue:         {ir.AWS: ok, ir.GCP: emulated("Cloud Run consumes a pull subscription, so CPU stays allocated while instances run")},
	SendsToQueue:       {ir.AWS: ok, ir.GCP: ok},

	Overrides: {ir.AWS: ok, ir.GCP: ok},

	ProjectTags:    {ir.AWS: ok, ir.GCP: ok},
	PrivateNetwork: {ir.AWS: ok, ir.GCP: ok},
	NetworkCIDR:    {ir.AWS: ok, ir.GCP: ok},
}

// products are what each kind becomes, for hovers and docs.
var products = map[ir.Kind]map[ir.Cloud]string{
	ir.ServiceKind:      {ir.AWS: "ECS Fargate", ir.GCP: "Cloud Run"},
	ir.DatabaseKind:     {ir.AWS: "RDS", ir.GCP: "Cloud SQL"},
	ir.BucketKind:       {ir.AWS: "S3", ir.GCP: "Cloud Storage"},
	ir.LoadBalancerKind: {ir.AWS: "Application Load Balancer", ir.GCP: "Application Load Balancer"},
	ir.QueueKind:        {ir.AWS: "SQS", ir.GCP: "Pub/Sub"},
}

// versions are the database major versions each cloud offers.
var versions = map[ir.Cloud]map[ir.Engine][]string{
	ir.AWS: {
		ir.Postgres: {"13", "14", "15", "16", "17"},
		ir.MySQL:    {"8.0", "8.4"},
	},
	ir.GCP: {
		ir.Postgres: {"13", "14", "15", "16", "17"},
		ir.MySQL:    {"8.0", "8.4"},
	},
}

// maxMemory is the most memory, in MiB, a service can have on each cloud.
var maxMemory = map[ir.Cloud]int{
	ir.AWS: 120 * 1024,
	ir.GCP: 32 * 1024,
}
