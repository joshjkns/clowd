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
	ServiceRegistry:    {ir.AWS: ok, ir.GCP: emulated("Cloud Run pulls only from Artifact Registry and Docker Hub, so this image comes through an Artifact Registry proxy, which needs credentials if the image is private")},

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
	WritesBucket:       {ir.AWS: ok, ir.GCP: emulated("Cloud Storage has no role that writes objects without reading them, so the service can read the bucket too")},
	ReadsQueue:         {ir.AWS: ok, ir.GCP: emulated("Cloud Run consumes a pull subscription, so CPU stays allocated while instances run")},
	SendsToQueue:       {ir.AWS: ok, ir.GCP: ok},

	Overrides: {ir.AWS: ok, ir.GCP: ok},

	ProjectTags:    {ir.AWS: ok, ir.GCP: ok},
	PrivateNetwork: {ir.AWS: ok, ir.GCP: ok},
	NetworkCIDR:    {ir.AWS: ok, ir.GCP: ok},

	ServiceHealthCheck: {ir.AWS: ok, ir.GCP: ok},
	ServiceCommand:     {ir.AWS: ok, ir.GCP: ok},
	DatabaseHA:         {ir.AWS: ok, ir.GCP: ok},
	DatabaseProtected:  {ir.AWS: ok, ir.GCP: ok},
	DatabaseStorage:    {ir.AWS: ok, ir.GCP: ok},
	DatabaseBackups:    {ir.AWS: ok, ir.GCP: ok},
	BucketVersioning:   {ir.AWS: ok, ir.GCP: ok},
	BucketProtected:    {ir.AWS: ok, ir.GCP: ok},
	BucketExpiry:       {ir.AWS: ok, ir.GCP: ok},
	QueueRetry:         {ir.AWS: ok, ir.GCP: ok},
	QueueTimeout:       {ir.AWS: ok, ir.GCP: ok},
	QueueRetention:     {ir.AWS: ok, ir.GCP: ok},
}

// maxBackupDays is the longest each cloud keeps automated database
// backups.
var maxBackupDays = map[ir.Cloud]int{
	ir.AWS: 35,
	ir.GCP: 365,
}

// queueDeliveries are the fewest and most deliveries (the first try and
// its retries) before a message is dead-lettered.
var queueDeliveries = map[ir.Cloud][2]int{
	ir.AWS: {1, 1000},
	ir.GCP: {5, 100},
}

// queueTimeouts are the shortest and longest time, in seconds, a consumer
// can be given to handle a message.
var queueTimeouts = map[ir.Cloud][2]int{
	ir.AWS: {0, 12 * 3600},
	ir.GCP: {10, 600},
}

// queueRetention are the shortest and longest time, in seconds, unhandled
// messages can be kept.
var queueRetention = map[ir.Cloud][2]int{
	ir.AWS: {60, 14 * 86400},
	ir.GCP: {600, 31 * 86400},
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
