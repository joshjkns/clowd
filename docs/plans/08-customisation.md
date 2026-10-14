# Customisation without clowd.toml (proposal)

Status: proposal. Nothing here is in the spec yet. If it is accepted, the
spec (01-language.md), the CLI plan (04-cli.md), versioning (06-versioning.md)
and the decision log change in the same PR as the first implementation step.

## Problem

Two gaps showed up once deploys worked:

1. **Too little customisation per resource.** A database is
   `postgres 16, small` and nothing else. There's no way to set storage,
   backups, high availability or deletion protection without an
   `on aws ... sets` override, which is per-cloud, raw and stringly typed.
2. **Settings live in TOML.** Changing the network (private services behind
   NAT, a CIDR, an existing VPC) is only possible in `clowd.toml`. So are
   the project name, edition, runtime and state backend. That is a second
   syntax, with its own parser (D12), for things users think of as part of
   their infrastructure.

The goal is that **a `.clowd` file is the whole project**. No TOML, and
every common customisation is written in Clowd.

## The shape of the answer

Each setting goes to exactly one of four places, chosen by what it means:

| Layer | Where | For | Example |
|---|---|---|---|
| 1. Portable | properties and traits on a resource | intent that every cloud can express | `storage 50gb`, `is highly available` |
| 2. Cloud-specific | `on <cloud>` block **inside** a resource | one cloud's knobs, raw provider names | `on aws` → `platform_version LATEST` |
| 3. Program-wide | `project` and `network` declarations | things that aren't one resource | `network is private` |
| 4. Machine | CLI flags and environment variables | how *this machine* runs Clowd | `CLOWD_RUNTIME=terraform` |

The rule for choosing: if two clouds can both do it, it's layer 1, gets a
plain-English name, and goes in the capability matrix. Layer 2 is only for
what really belongs to one cloud. Layer 1 should keep growing so that
layer 2 stays small.

The whole thing rests on **one grammar change**: a property line may have an
indented block under it. That gives nested `on` blocks, nested provider
blocks and, later, environments, all from the same rule.

## What it looks like

```
use aws in eu-west-2
use gcp in europe-west2

project shop
  tag team payments

network is private
  cidr 10.20.0.0/16

database db is postgres 16, medium, highly available, protected
  storage 100gb
  backups 14 days

bucket uploads is private, versioned
  expire after 30d

queue jobs
  retry 5 times
  timeout 60s

service api
  image ghcr.io/me/api:latest
  port 8080
  scale 2 to 10
  health check /healthz
  on aws
    platform_version LATEST
    enable_execute_command true

load balancer web is public
  listen on 443 with tls for api.example.com
  send / to api

api connects to db
api reads and writes uploads
api sends to jobs
```

Today, about half of this needs `clowd.toml` or `sets` overrides. Under
this proposal none of it does.

## Layer 1: more portable properties

These are the first additions. Each one maps cleanly to AWS and GCP. When
one cloud has narrower limits, that goes in the capability matrix as ⚠ or ✗,
not in the syntax.

### `service`

| Property | Form | AWS | GCP |
|---|---|---|---|
| `health check` | `health check <path>` | target group `health_check.path` | startup + liveness `http_get.path` |
| `command` | `command <word...>`, quote words with spaces | container `command` | container `args` |

### `database` (gains properties; today it has traits only)

| Feature | Form | AWS | GCP |
|---|---|---|---|
| trait `highly available` | | `multi_az` | `availability_type = REGIONAL` |
| trait `protected` | | `deletion_protection`, final snapshot kept | `deletion_protection` |
| `storage` | `storage <n>gb` | `allocated_storage` | `disk_size` |
| `backups` | `backups <n> days`, `backups off` | `backup_retention_period` | daily backups, `retained_backups = n` |

### `bucket`

| Feature | Form | AWS | GCP |
|---|---|---|---|
| trait `versioned` | | `aws_s3_bucket_versioning` | `versioning.enabled` |
| trait `protected` | | no `force_destroy` | no `force_destroy` |
| `expire` | `expire after <duration>` | lifecycle expiration | `lifecycle_rule` age |

### `queue`

| Property | Form | AWS | GCP |
|---|---|---|---|
| `retry` | `retry <n> times`, then a dead-letter queue | redrive `maxReceiveCount` | `dead_letter_policy` (⚠ GCP: 5–100) |
| `timeout` | `timeout <duration>`: how long a consumer has | `visibility_timeout_seconds` | `ack_deadline_seconds` (⚠ 10s–600s) |
| `keep` | `keep messages for <duration>` | `message_retention_seconds` (≤ 14d) | `message_retention_duration` (≤ 31d) |

`protected` is the opt-in from D15. It replaces the advice to write
`sets deletion_protection to true` per cloud.

Durations gain `d` (days), since retention and expiry are counted in days.

## Layer 2: `on <cloud>` inside the resource

Today:

```
on aws
  database db sets backup_retention_period to 14
```

Proposed:

```
database db is postgres 16
  on aws
    backup_retention_period 14
```

- The override sits next to the thing it changes, which is where a reader
  looks for it. The `sets ... to` wording and repeating the kind and name
  both go away.
- Keys are still raw provider argument names (snake_case), so they still
  look visibly different from portable code (the reason behind D4).
- D13 still applies: each kind has one primary resource per cloud, keys are
  allowlisted, and wiring arguments can't be set. `clowd check` still
  reports them in the Overrides section, one row per resource.
- **Nested blocks become possible**, because a key can have its own block.
  That closes the gap D13 left open:

  ```
  service api
    on aws
      capacity_provider_strategy
        capacity_provider FARGATE_SPOT
        weight 1
  ```

  A list value is written with commas: `security_groups sg-1, sg-2`. The
  allowlist covers nested blocks the same way it covers top-level
  arguments.
- A top-level `on <cloud>` block stays, but only for program-wide settings
  of that cloud (layer 3). `<kind> <name> sets <key> to <value>` becomes an
  error that suggests moving the line into the resource. Edition 2026 hasn't
  shipped, so nobody's files break.

## Layer 3: `project` and `network`

Both use the existing Declare shape (`<kind> [name] [is traits]` plus
properties). That means no new statement shape, and the parser, formatter
and LSP handle them like any other declaration. Each appears at most once
per program.

### `project`

```
project shop
  edition 2026
  tag team payments
  tag cost-centre 42
```

| Item | Replaces | Default |
|---|---|---|
| name | `name` in clowd.toml | the directory's name |
| `edition` | `edition` in clowd.toml (D7) | latest |
| `tag <key> <value>`, repeatable | — (new) | none; AWS tags and GCP labels on every resource |
| `state` | `[state] backend` | local; reserved until remote state exists |

The whole declaration is optional. A file without one deploys exactly as it
does today.

### `network`

```
network is private
  cidr 10.20.0.0/16
```

| Item | Replaces | Default |
|---|---|---|
| trait `public` / `private` | `[aws.network] services` | `public`: services get public IPs and no NAT (D14) |
| `cidr` | `[aws.network] cidr` | `10.0.0.0/16` |

`network` takes no name, because a program has one network. Using an
existing VPC is a per-cloud fact (its IDs only mean something on AWS), so
it goes in the top-level `on aws`:

```
on aws
  network
    vpc vpc-0123
    public_subnets subnet-a, subnet-b
    private_subnets subnet-c, subnet-d
```

This reverses part of D14 ("deploy settings, not infrastructure design").
Whether services can reach the internet without NAT, and which address
range they use, *is* infrastructure design. A user reading the `.clowd`
file should be able to see it.

## Layer 4: machine settings

| Setting | Was | Now |
|---|---|---|
| runtime | `runtime` in clowd.toml | `--runtime tofu\|terraform` or `CLOWD_RUNTIME`; default `tofu`, else `terraform` |

The runtime is a property of the developer's machine, not of the
infrastructure. Two people on a team can use different ones against the
same state, which is the point of D1.

## What happens to the project files

| File | Before | After |
|---|---|---|
| `clowd.toml` | optional, user-written | **removed**. A `clowd.toml` that exists is an error naming where each key moved. |
| `clowd.lock` | CLI-written | unchanged, next to the `.clowd` file |
| `.clowd/` | build output | unchanged |

The project directory is the directory of the `.clowd` file. `clowd init`
writes only `app.clowd` (with a commented `project` block) and the
`.gitignore` line. The TOML-subset parser from D12 is still needed for
`clowd.lock`, unless the lock moves to JSON. JSON is simpler, and nobody
edits the lock by hand, so this proposal moves it.

## Later, on the same grammar

These aren't part of this proposal. They're listed to show that the
sub-block rule extends to them without new syntax.

- **Environments.** An `in <env>` block inside a resource, chosen with
  `--env`:

  ```
  service api
    scale 1
    in prod
      scale 2 to 10
  ```

  Before, this was planned as `clowd.toml` profiles (01-language.md,
  "Not in v1").
- **Shared defaults.** `every service` with properties that apply to every
  service unless that service sets them.
- **Variables and secrets.** These still need their own design.

## Pipeline impact

| Package | Change |
|---|---|
| `syntax/parser`, `ast` | `Property` gains `Children []*Property`. Bodies are parsed recursively with the same error recovery. `project`/`network` kinds; `network` has no name. |
| `format` | print nested blocks at 2 spaces per level |
| `check` | per-kind specs for the new properties and traits; `on` blocks validated per key path |
| `ir` | new fields on each resource; `Program.Project`, `Program.Network`; `Override` gets a key path and a nested value |
| `capability` | entries for every new feature (GCP retry and timeout limits as ⚠) |
| `lower/aws` | read network and tags from the IR instead of `Options`; nested overrides merged into JSON blocks |
| `project` | load no TOML; find the root from the file; JSON lock |
| `cli` | `--runtime` and `CLOWD_RUNTIME`; `init` writes no TOML |

Lowering still sees only the IR. Network and tags now come from the
program, not from `lower/aws.Options`, which removes the one path from
config to lowering that bypassed the IR.

## Order of work

Each step is one PR that builds and passes tests.

1. **Sub-blocks in the grammar.** Parser, AST, formatter, goldens. No new
   meaning yet: `check` rejects children wherever they aren't allowed.
2. **`on <cloud>` inside resources.** Move overrides; error on the old form;
   nested provider blocks with allowlists. Update `examples/app.clowd`.
3. **`project` and `network`; remove clowd.toml.** Move network and name
   into the IR; JSON lock; `--runtime`. Record decisions D16–D18 (below).
4. **Portable properties**, one kind per PR, each with capability entries
   and AWS lowering.

Steps 1–3 should land before `feat/cli-deploy` merges, so the CLI doesn't
ship with TOML that is removed immediately afterwards.

## Decisions this would record

- **D16:** No `clowd.toml`. Project and network settings are written in
  Clowd. This supersedes D7 (edition in TOML) and D12 (TOML-subset parser),
  and the "deploy settings" part of D14.
- **D17:** Overrides live inside the resource they change, and may set
  nested blocks. This supersedes the form in the spec and extends D13.
- **D18:** The four-layer rule: portable first, per-cloud nested, program-wide
  declarations, machine settings as flags.

## Open questions

- `network is private` versus a per-service trait. Per-service is more
  granular, but NAT is paid for per network, and `service api is private`
  would read as "not exposed", which every service already is.
- Should `project` live in its own file once multi-file programs exist
  (like `go.mod`)? Probably yes. It would still be Clowd syntax.
- `backups` on GCP counts retained daily backups, not days. That matches
  `backups <n> days` only while backups are daily.
