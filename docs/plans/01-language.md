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
| Declare | `<kind> <name> [is <traits>]` + indented properties | `bucket uploads is private` |
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

```
on aws
  service api sets capacity_provider to FARGATE_SPOT
```

Overrides are applied only when lowering to that cloud. They use raw provider
property names on purpose, so they look visibly different from portable code.
`clowd check` reports them per cloud and verifies the property exists on the
generated resource.

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

## Grammar (EBNF draft)

The lexer emits `WORD`, `STRING`, `COMMA`, `NEWLINE`, `INDENT` and `DEDENT`
tokens (layout is Python-style). Quoted terminals below are words matched
case-insensitively by the parser, not distinct token kinds.

```ebnf
file       = { statement } ;
statement  = use | declare | relate | on_block ;

use        = "use" cloud [ "in" value ] NEWLINE ;

declare    = kind name [ "is" traits ] NEWLINE
             [ INDENT { property } DEDENT ] ;
traits     = trait { ( "," | "and" ) trait } ;
trait      = word { word } ;
property   = key value { value } NEWLINE ;

relate     = name verb name NEWLINE ;
verb       = "connects to" | "reads" | "writes"
           | "reads and writes" | "sends to" ;

on_block   = "on" cloud NEWLINE INDENT { override } DEDENT ;
override   = kind name "sets" raw_key "to" value NEWLINE ;

kind       = "service" | "database" | "bucket"
           | "load balancer" | "lb" | "queue" ;
cloud      = "aws" | "gcp" | "azure" ;
```

## Not in v1 (open questions)

- Variables and inputs (`image` per environment)
- Environments (staging vs prod) — likely `clowd.toml` profiles
- Secrets (`secret db_password from ...`)
- Outputs (print the LB URL after apply)
- Multiple files / imports
- Multiple targets per relate line (`api connects to db and cache`)
