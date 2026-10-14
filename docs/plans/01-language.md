# Language Spec (draft)

Status: draft for edition 2026. Anything not listed here is not in the language yet.

## Design rules

1. **Reads like a sentence.** Every line is a short structured-English statement.
2. **One canonical phrasing.** No synonyms (`connects to` only — never `links to`,
   `uses`, `talks to`). Wrong guesses get a "did you mean" error.
3. **Case-insensitive keywords.** `service`, `Service`, `SERVICE` are all valid.
   Lowercase is canonical; `clowd fmt` lowercases keywords.
4. **Declarative.** A file describes what exists. There is no `create`
   (an optional leading `create` may be accepted later for SQL people).
5. **Newlines end statements, indentation makes blocks.** No `;`, `{}`, or `:`.
6. **No quotes unless needed.** Bare values are fine; quote only values with spaces.

## Lexical structure

- **Comments:** `#` to end of line (must be at line start or after whitespace, so
  values like `a#b` survive).
- **Indentation:** spaces only; tabs are an error. Any consistent width;
  `clowd fmt` uses 2.
- **Names:** `[a-z][a-z0-9_-]*`, case-insensitive. Referring to `api` as `API`
  works but warns. Two resources differing only by case is an error.
- **Bare values:** a run of non-whitespace characters, not ending in `,`
  (`ghcr.io/me/api:latest`, `api.example.com`, `/`, `eu-west-2`).
- **Quoted strings:** `"..."` with `\"` and `\\` escapes.
- **Numbers:** integers. **Ranges:** `2 to 10`. **Durations:** `30s`, `5m`, `1h`.
  These are ordinary words to the lexer; `check` gives them meaning from the
  property they appear in.
- **Keywords are contextual** (see D8). The lexer only produces words, strings,
  commas and layout tokens; the parser recognises keywords by position and
  case-insensitively. Multi-word keywords (`load balancer`, alias `lb`;
  `connects to`; `reads and writes`) are matched as a sequence of words, so
  `to` in `scale 2 to 10` and a resource named `web` or `jobs` are unaffected.

## Statement shapes

There are exactly four.

| Shape | Form | Example |
|---|---|---|
| Use | `use <cloud> [in <region>]` | `use aws in eu-west-2` |
| Declare | `<kind> <name> [is <traits>]` + indented properties | `bucket uploads is private`, `project shop`, `network is private` |
| Relate | `<name> <verb> <name>` | `api connects to db` |
| On | `on <cloud>` + indented overrides | see below |

### Use

Declares a target cloud. Each `use` line is a deploy target; `clowd check`
validates against all of them.

```
use aws in eu-west-2
use gcp in europe-west2
```

### Declare

```
database db is postgres 16, small

service api
  image ghcr.io/me/api:latest
  port 8080
  scale 2 to 10
```

- **Traits** (after `is`) are adjectives, joined by `,` or `and`. A trait may be
  several words (`postgres 16`). Contradictory traits are errors
  (`is public and internal`).
- **Properties** are indented `<key> <value...>` lines. Each kind has a fixed set.

### Relate

```
api connects to db
api reads and writes uploads
api sends to jobs
```

Verbs (fixed list for v1):

| Verb | Meaning |
|---|---|
| `connects to` | network path + credentials + connection env var |
| `reads` | read-only access (bucket, queue) |
| `writes` | write-only access |
| `reads and writes` | read + write access |
| `sends to` | publish to a queue |

One target per relate line in v1.

A line whose second word starts a verb is a relationship. This is checked
before anything else, so a resource may share a name with a statement keyword
(`queue connects to db`, `on reads uploads`). The cost is that a verb cannot
be used as a resource name in a declaration: `service reads` is read as an
unfinished relationship.

Common wrong guesses (`links to`, `uses`, `talks to`, `reads from`,
`writes to`, ...) are errors that suggest the canonical verb, and the line is
still parsed with that verb so the rest of the file checks normally.

### On (escape hatch)

Settings that belong to one cloud go in an `on <cloud>` block inside the
resource they change:

```
database db is postgres 16
  on aws
    backup_retention_period 14

service api
  image ghcr.io/me/api:latest
  on aws
    capacity_provider_strategy
      capacity_provider FARGATE_SPOT
      weight 1
```

They are applied only when lowering to that cloud. Keys are raw provider
argument names on purpose, so they look visibly different from portable
code, and the formatter keeps their case. `clowd check` lists each block in
the Overrides section of the matrix and verifies every key exists on the
generated resource.

- A line is `<argument> <value>`, or an argument with an indented block
  under it for a nested block. A nested block that the provider allows more
  than once may be written more than once; the first replaces what Clowd
  generated and the rest add to it. A nested block that appears once is
  merged into Clowd's, so one field can be changed.
- A list is written with commas: `<argument> a, b`. A comma needs a space
  after it; `a,b` is one value.
- Each kind has one resource the block sets arguments on (on AWS:
  `aws_ecs_service`, `aws_db_instance`, `aws_s3_bucket`, `aws_lb`,
  `aws_sqs_queue`), with an allowlist of its arguments and nested blocks.
  Arguments that carry Clowd's own wiring (subnets, security groups, roles)
  can't be set, and the error says so.
- Values are passed as strings, and the provider converts them to the
  argument's type.
- A resource has at most one `on` block per cloud.

A top-level `on <cloud>` block holds program-wide settings for that cloud
instead (see `network` below). The old form `<kind> <name> sets <key> to
<value>` in a top-level block is an error that shows where the line goes
now.

## Resource kinds (MVP)

| Kind | AWS | GCP | Key properties / traits |
|---|---|---|---|
| `service` | ECS Fargate | Cloud Run | `image`, `port`, `scale`, `cpu`, `memory`, `env` |
| `database` | RDS | Cloud SQL | traits: engine + version, size (`small`/`medium`/`large`) |
| `bucket` | S3 | GCS | traits: `private`/`public` |
| `load balancer` / `lb` | ALB | External HTTPS LB | `listen on <port> [with tls for <domain>]`, `send <path> to <name>` |
| `queue` | SQS | Pub/Sub | traits: `fifo` (where supported) |

The full per-cloud support table lives in the capability matrix (see 03-check.md),
not here.

### Kind reference

Values are interpreted by `check` from the property they appear in. Anything
left out takes the default shown. Keys and keyword values (`to`, `on`,
`with tls for`) are case-insensitive.

**`service`** — no traits; to expose one, route to it from a load balancer.

| Property | Form | Default |
|---|---|---|
| `image` | one value, e.g. `ghcr.io/me/api:latest` | required |
| `port` | `1`–`65535` | `8080` |
| `scale` | `<n>` or `<min> to <max>`; `0 to <max>` scales to zero when idle | `1` |
| `cpu` | vCPUs: `0.25`, `0.5`, `1`, `2`, `4`, `8` | cloud default |
| `memory` | whole `mb` or `gb` (binary: `1gb` = `1024mb`) | cloud default |
| `env` | `env <NAME> <value>`, repeatable; quote values with spaces | — |

**`database`** — traits only, no properties.

| Trait group | Values | Default |
|---|---|---|
| engine | `postgres [<version>]`, `mysql [<version>]` | required |
| size | `small`, `medium`, `large` | `small` |

A missing version means the backend's default for that engine. Versions are
numbers like `16` or `8.0`; which ones a cloud offers is in the capability
matrix. Postgres versions are majors (`16`, or `16.4` to pin a minor). MySQL
versions name a release line (`8.0`, `8.4`), so a bare `mysql 8` is an error.

**`bucket`** — trait `private` (default) or `public`.

**`queue`** — trait `fifo` (ordered, exactly-once); standard by default.

**`load balancer`** — trait `public` (default) or `internal`.

| Property | Form | |
|---|---|---|
| `listen` | `listen on <port> [with tls for <domain>]`, repeatable | at least one |
| `send` | `send <path> to <service>`, repeatable | at least one |

Paths start with `/` and match by prefix (`/` matches everything; no
wildcards). Each port and each path appears once.

### Program-wide declarations

`project` and `network` are written like resources but aren't resources:
nothing refers to them (a service may share the project's name), and each
appears at most once. Both are optional. Without them a file is a complete
project, named after its directory, in a network Clowd creates with
defaults (D17).

**`project <name>`** — the name prefixes everything deployed, so two
projects can share an account: up to 20 lowercase letters, digits and
dashes, starting with a letter. Without it, the directory's name is used.

| Property | Form | Default |
|---|---|---|
| `edition` | `edition 2026` (06-versioning.md) | latest |
| `tag` | `tag <key> <value>`, repeatable; an AWS tag and a GCP label on every resource | — |
| `state` | `state local`; other backends come later | `local` |

Tag keys are up to 63 lowercase letters, digits, `_` and `-`, starting
with a letter, so every cloud accepts them. GCP also restricts label
values, which the capability matrix reports.

**`network`** — takes no name, since a program has one network.

| Feature | Form | Default |
|---|---|---|
| trait `public` / `private` | `network is private` | `public`: services get public addresses and need no NAT (D14) |
| `cidr` | `cidr 10.20.0.0/16`, a /16 to /20 | `10.0.0.0/16` |

`private` runs services in private subnets that reach the internet through
NAT. Databases are always private.

Using a network that already exists is a fact about one cloud, so it goes
in that cloud's top-level `on` block, and replaces `cidr`:

```
on aws
  network
    vpc vpc-0123
    public_subnets subnet-a, subnet-b
    private_subnets subnet-c, subnet-d
```

Each subnet list needs two or more subnets in different availability
zones.

### Which relationships apply

Only services start relationships. Load balancers route with `send`, and
data stores are passive.

| Verb | Target kinds |
|---|---|
| `connects to` | `database`, `service` |
| `reads` | `bucket`, `queue` (consume) |
| `writes` | `bucket` |
| `reads and writes` | `bucket` |
| `sends to` | `queue` |

To publish to a queue use `sends to`, never `writes` (one canonical phrasing);
the checker suggests the right verb for each wrong pairing.

### What a relationship gives a service

Besides network access and permissions, a relationship sets environment
variables in the service, named after the target: `<NAME>` is the target's
name in upper case with `-` as `_` (`db` → `DB`, `user-files` →
`USER_FILES`). They are the same on every cloud.

| Relationship | Variables |
|---|---|
| `connects to` a database | `<NAME>_HOST`, `<NAME>_PORT`, `<NAME>_NAME`, `<NAME>_USER`, `<NAME>_PASSWORD` |
| any verb to a bucket | `<NAME>_BUCKET` (the bucket's name) |
| `reads` / `sends to` a queue | `<NAME>_QUEUE_URL` |

The password is a secret: it is generated and stored by the cloud, never
written in the configuration or state, and given to the service at start.
A service's own `env` of the same name wins, with a warning.

### Clouds

`use` and `on` take `aws`, `gcp` or `azure`. Each cloud appears in at most
one `use` line (one region per cloud for now). An `on` block for a cloud
the file doesn't `use` is a warning, since it has no effect.

## Grammar (EBNF draft)

The lexer emits `WORD`, `STRING`, `COMMA`, `NEWLINE`, `INDENT` and `DEDENT`
tokens (layout is Python-style). Quoted terminals below are words matched
case-insensitively by the parser, not distinct token kinds.

```ebnf
file       = { statement } ;
statement  = use | declare | relate | on_block ;

use        = "use" cloud [ "in" value ] NEWLINE ;

declare    = ( kind name | "network" ) [ "is" traits ] NEWLINE [ block ] ;
traits     = trait { ( "," | "and" ) trait } ;
trait      = word { word } ;
block      = INDENT { property } DEDENT ;
property   = key { value } NEWLINE [ block ]
           | "on" cloud NEWLINE settings ;     (* raw provider settings *)
setting    = key { value | "," } NEWLINE [ settings ] ;
settings   = INDENT { setting } DEDENT ;

relate     = name verb name NEWLINE ;
verb       = "connects to" | "reads" | "writes"
           | "reads and writes" | "sends to" ;

on_block   = "on" cloud NEWLINE settings ;

kind       = "service" | "database" | "bucket"
           | "load balancer" | "lb" | "queue" | "project" ;
cloud      = "aws" | "gcp" | "azure" ;
```

## Not in v1 (open questions)

- Variables and inputs (`image` per environment)
- Environments (staging vs prod) — likely `in <env>` blocks inside resources (08-customisation.md)
- Secrets (`secret db_password from ...`)
- Outputs (print the LB URL after apply)
- Multiple files / imports
- Multiple targets per relate line (`api connects to db and cache`)
