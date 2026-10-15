package lsp

import "github.com/joshjkns/clowd/internal/syntax/ast"

// Hover and completion documentation, summarised from 01-language.md.
// docs_test.go checks that every trait and property check accepts has an
// entry, so a new one can't ship undocumented.

var kindDocs = map[ast.Kind]string{
	ast.Service:      "A long-running container. It has no traits; to expose it, route to it from a load balancer with `send <path> to <service>`.",
	ast.Database:     "A managed relational database. Its traits choose the engine and version, size, availability and protection.",
	ast.Bucket:       "Object storage. Private unless it is `public`.",
	ast.LoadBalancer: "An HTTP(S) load balancer that routes paths to services. Also written `lb`.",
	ast.Queue:        "A message queue. Services publish with `sends to` and consume with `reads`.",
	ast.Project:      "The project: its name prefixes everything deployed, so two projects can share an account. Optional; without it, the directory's name is used.",
	ast.Network:      "Where resources are placed. Optional; without it, Clowd creates a network with defaults.",
}

var verbDocs = map[ast.Verb]string{
	ast.ConnectsTo:     "Gives the service a network path to a database or service, and for a database its credentials in `<NAME>_HOST`, `<NAME>_PORT`, `<NAME>_NAME`, `<NAME>_USER` and `<NAME>_PASSWORD`.",
	ast.Reads:          "Gives the service read access to a bucket (`<NAME>_BUCKET`), or makes it consume a queue (`<NAME>_QUEUE_URL`).",
	ast.Writes:         "Gives the service write access to a bucket (`<NAME>_BUCKET`). To publish to a queue, use `sends to`.",
	ast.ReadsAndWrites: "Gives the service read and write access to a bucket (`<NAME>_BUCKET`).",
	ast.SendsTo:        "Lets the service publish to a queue (`<NAME>_QUEUE_URL`).",
}

var traitDocs = map[ast.Kind]map[string]string{
	ast.Database: {
		"postgres":         "PostgreSQL. Optionally followed by a version: a major (`16`) or a minor to pin (`16.4`).",
		"mysql":            "MySQL. Optionally followed by a release line, such as `8.0` or `8.4`.",
		"small":            "The smallest instance size. The default.",
		"medium":           "A medium instance size.",
		"large":            "A large instance size.",
		"highly available": "Keeps a standby in another zone that takes over on failure.",
		"protected":        "Can't be deleted, and keeps a final snapshot. Without it, `clowd destroy` deletes the database.",
	},
	ast.Bucket: {
		"private":   "Only services with a relationship can read or write it. The default.",
		"public":    "Anyone can read its objects.",
		"versioned": "Keeps every version of an object.",
		"protected": "Can't be deleted while it holds objects. Without it, `clowd destroy` empties it.",
	},
	ast.LoadBalancer: {
		"public":   "Reachable from the internet. The default.",
		"internal": "Reachable only from inside the network.",
	},
	ast.Queue: {
		"fifo": "Ordered, exactly-once delivery. Standard (unordered, at-least-once) by default.",
	},
	ast.Network: {
		"public":  "Services get public addresses and need no NAT gateway. The default.",
		"private": "Services run in private subnets and reach the internet through NAT. Databases are always private.",
	},
}

var propDocs = map[ast.Kind]map[string]string{
	ast.Service: {
		"image":   "The container image to run. Required.",
		"port":    "The port the container listens on, 1–65535. Default `8080`.",
		"scale":   "How many instances run: `<n>`, or `<min> to <max>` to autoscale. `0 to <max>` scales to zero when idle. Default `1`.",
		"cpu":     "vCPUs per instance: `0.25`, `0.5`, `1`, `2`, `4` or `8`. Default: the cloud's.",
		"memory":  "Memory per instance, in whole `mb` or `gb` (`1gb` = `1024mb`). Default: the cloud's.",
		"env":     "An environment variable: `env <NAME> <value>`. Repeatable; quote values with spaces.",
		"health":  "The path load balancers check; it must answer 2xx or 3xx. Default `/`, where any answer below 500 counts.",
		"command": "Replaces the image's command. Quote words with spaces.",
	},
	ast.Database: {
		"storage": "The initial disk size, `<n>gb` from 20 to 65536. Grows automatically on AWS.",
		"backups": "How long daily backups are kept: `backups <n> days` (1–365, at most 35 on AWS), or `backups off`.",
	},
	ast.Bucket: {
		"expire": "Deletes objects, and old versions of versioned ones, this many whole days after they are written: `expire after <n>d`.",
	},
	ast.LoadBalancer: {
		"listen": "A port to listen on: `listen on <port>`, or `listen on 443 with tls for <domain>` for HTTPS. Repeatable; at least one.",
		"send":   "Routes a path prefix to a service: `send <path> to <service>`. `/` matches everything. Repeatable; at least one.",
	},
	ast.Queue: {
		"retry":   "`retry <n> times`: after the first try and n retries, a message moves to the dead-letter queue `<name>-dead`.",
		"timeout": "How long a consumer has to handle a message before it is delivered again, 1s–12h.",
		"keep":    "How long unhandled messages are kept, 1m–31d.",
	},
	ast.Project: {
		"edition": "The language edition the file is written in. Default: the latest.",
		"tag":     "`tag <key> <value>`: an AWS tag and a GCP label on every resource. Repeatable.",
		"state":   "Where deploy state is kept. Only `local` (in `.clowd/`) for now.",
	},
	ast.Network: {
		"cidr": "The address block of the network Clowd creates, a /16 to /20. Default `10.0.0.0/16`.",
	},
}

var keywordDocs = map[string]string{
	"use": "`use <cloud> [in <region>]` makes a cloud a deploy target. `clowd check` validates the file against every target.",
	"on":  "`on <cloud>` holds settings for one cloud, applied only when deploying there. Inside a resource, its lines set raw provider arguments on the resource Clowd generates; at the top level, program-wide settings such as an existing `network`.",
	"is":  "Starts the traits, joined by `,` or `and`.",
}
