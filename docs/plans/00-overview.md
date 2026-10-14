# Clowd — Overview

Clowd is a small, English-like language for describing cloud infrastructure once
and deploying it to multiple clouds.

```
use aws in eu-west-2
use gcp in europe-west2

database db is postgres 16, small
  on aws
    backup_retention_period 14

bucket uploads is private

service api
  image ghcr.io/me/api:latest
  port 8080
  scale 2 to 10

load balancer web is public
  listen on 443 with tls for api.example.com
  send / to api

api connects to db
api reads and writes uploads
```

## The problem

Terraform works, but it is verbose and cloud-specific. Describing "a service that
talks to a database behind a load balancer" takes hundreds of lines of resources,
security groups, IAM policies and wiring, and you rewrite all of it for each cloud.

## The idea

- **Abstract nouns, not products.** You write `service`, the compiler picks ECS
  Fargate on AWS or Cloud Run on GCP.
- **Relationships are first-class.** `api connects to db` generates the networking,
  permissions and connection env vars. This is where most Terraform boilerplate
  lives, and it is Clowd's main value.
- **Portable by default, with escape hatches.** `on aws` blocks pass raw provider
  settings through when the abstraction is not enough.
- **Know before you deploy.** `clowd check` shows, per cloud, whether the file
  will compile (✓ / ⚠ / ✗).
- **No new engine.** Clowd compiles to OpenTofu/Terraform JSON and drives
  OpenTofu. State, planning, diffing and provider coverage come for free.

## Prior art and what we learn from it

| Project | Lesson |
|---|---|
| Pulumi | Real languages, but not cloud-abstract |
| CDKTF | Compile-to-Terraform works technically; deprecated by HashiCorp |
| Winglang | Closest idea; a new language is a big adoption ask — keep Clowd tiny |
| Nitric | Closest abstraction model; uses TF/Pulumi underneath |
| Score | Small, declarative, portable — a good model for scope |
| Docker Compose → ECS/ACI | Retired; simple models leak onto real clouds — escape hatches are mandatory |

The two things that kill projects like this:

1. **Lowest common denominator.** If only the shared subset is exposed, power users
   leave. Answer: traits with good defaults plus `on <cloud>` escape hatches.
2. **No escape hatch.** People hit a wall within a week. Answer: the `on` block
   exists from v1.

## Plan documents

| Doc | Contents |
|---|---|
| [01-language.md](01-language.md) | Syntax spec, grammar draft, resource kinds |
| [02-architecture.md](02-architecture.md) | Compiler pipeline, OpenTofu backend, repo layout |
| [03-check.md](03-check.md) | `clowd check` and the capability matrix |
| [04-cli.md](04-cli.md) | Commands, flags, exit codes |
| [05-tooling.md](05-tooling.md) | LSP, formatter, syntax highlighting |
| [06-versioning.md](06-versioning.md) | Editions, lockfile |
| [07-roadmap.md](07-roadmap.md) | Phases and milestones |
| [../decisions.md](../decisions.md) | Log of key decisions and why |
