# Decision Log

Short records of decisions that shape the project. Add new ones at the bottom;
if a decision is reversed, add a new entry rather than editing the old one.

## D1 — Compile to OpenTofu, don't build an engine
State, diffing, dependency ordering, drift, locking and provider coverage are
years of work. Emitting `.tf.json` gets all of it and lets users eject.
OpenTofu is the default runtime because it is open source (MPL); Terraform reads
the same output.

## D2 — Go
Single-binary distribution, and OpenTofu, its providers and HCL tooling are Go,
so schemas and validation code can be reused.

## D3 — File extension `.clowd`
`.cl` is used by Common Lisp and OpenCL, so editors and GitHub would misidentify it.

## D4 — Abstract nouns, not cloud products
`service`, not `ecs`. The compiler chooses the product per cloud. Provider
specifics go in `on <cloud>` blocks.

## D5 — Structured English, one canonical phrasing
Case-insensitive keywords, no semicolons/braces, indentation for blocks.
English-like languages are easy to read but easy to guess wrong, so each concept
has exactly one phrasing and errors suggest the right one.

## D6 — Hand-written, error-tolerant parser
The LSP runs on half-typed files, so the parser must recover from errors and
record spans. Hand-written recursive descent gives better errors than a generator.

## D7 — Edition in `clowd.toml`, not in each file
Keeps source files clean. Missing config means latest edition. A per-file
override is reserved for later.

## D8 — Contextual keywords; the lexer only knows words
The lexer emits words, strings, commas and layout tokens; the parser decides
what is a keyword from position. Keywords also appear inside values (`to` in
`scale 2 to 10` and `send / to api`, `on` in `listen on 443`), and users will
name resources `web`, `jobs` or worse. A keyword-aware lexer would have to guess.
Contextual keywords also make case-insensitivity, "did you mean" suggestions and
LSP completion on half-typed lines simpler, because the parser sees the words
the user actually wrote. Numbers, ranges and durations are words until `check`
interprets them.

## D9 — Check builds the IR in the same pass
Validating a value means interpreting it: `scale 2 to 10` has to become a
range to know the minimum is not above the maximum, and `listen on 443 with
tls for api.example.com` has to be taken apart to know it is well formed.
Doing that once in `check` and handing the result to `ir` means the IR can
never disagree with what check accepted. The packages stay separate (`check`
imports `ir`; `ir` imports neither the AST nor check), so lowering still
sees only the IR. Invalid pieces are left out or defaulted, so the IR is
always usable by the LSP on broken files.

## D10 — Azure is a known cloud before it has a backend
`use azure` is valid and shows as "not implemented" in the capability
matrix rather than failing as an unknown cloud. That is the honest answer
(the language supports it; Clowd doesn't yet), it matches the example in
03-check.md, and it means adding the backend later changes no user files.

## D11 — The formatter prints the AST and refuses broken files
`clowd fmt` prints from the AST, so its output is canonical by
construction rather than a cleanup of the user's tokens. On a file with
syntax errors the AST is the parser's best guess, and printing a guess
would silently rewrite what the user typed (a misspelt verb would become
the suggested one, a skipped line would vanish), so such files are left
unchanged, like gofmt. Where the language allows two spellings on purpose,
`lb` / `load balancer` and `,` / `and` between traits, the formatter keeps
the user's choice: they are documented alternatives, not synonyms to
correct, and design rule 2 is enforced by errors, not by the formatter.

## D12 — clowd.toml is read by a small TOML-subset parser
The project files need strings, integers, booleans, string arrays and
`[tables]`. A hand-written parser for that subset avoids a dependency, and
reports problems as ordinary diagnostics with spans and "did you mean"
help, like errors in `.clowd` files. Everything else in TOML (floats,
dates, inline tables, dotted keys, multi-line strings) is rejected as
unsupported rather than misread, so a file accepted here means the same
in any TOML parser. If the files ever need more, switch to a library.

## D13 — Overrides set top-level scalar arguments of one resource per kind
`on <cloud>` overrides name a raw provider argument, so they need a
resource to land on and a check that the argument exists. Each kind has
one primary resource per cloud (on AWS: the ECS service, RDS instance, S3
bucket, ALB and SQS queue), and an allowlist of its single-valued
arguments, so `clowd check` can say "no such setting" with a suggestion
instead of failing at plan time. Nested blocks aren't settable by
`sets <key> to <value>`, which is why the old example (`service api sets
capacity_provider to FARGATE_SPOT`) could never have worked: Spot needs a
`capacity_provider_strategy` block and cluster capacity providers. The
example now uses `database db sets backup_retention_period to 14`.
Nested-block overrides can be designed later if users need them.

## D14 — Default AWS network: Clowd-made VPC, services in public subnets
A program with services, databases or load balancers gets its own VPC
with a public and a private subnet in two zones (the minimum for an ALB
and an RDS subnet group). Services run in the public subnets with public
IPs, so they can pull images without a NAT gateway (which costs more per
month than a small service); their security groups admit only their load
balancers. Databases are always private. `[aws.network]` in clowd.toml
can move services behind a NAT gateway (`services = "private"`), change
the CIDR, or use an existing VPC and subnets. These are deploy settings,
not infrastructure design, so they stay out of `.clowd` files.

## D15 — `clowd destroy` removes data too
Buckets are created with `force_destroy` and databases skip the final
snapshot, so `destroy` removes everything `apply` created. Destroy is
explicit and OpenTofu asks for confirmation; anything that must survive
can opt in to protection with an override (`deletion_protection`,
`skip_final_snapshot`, `force_destroy`). The alternative, destroys that
fail halfway on a non-empty bucket or need a unique snapshot name each
time, is worse for the common case of a dev or demo environment.

## D16 — Overrides live inside the resource, and can set nested blocks
`on <cloud>` blocks go inside the declaration they change, with one
`<argument> <value>` per line, instead of top-level
`<kind> <name> sets <key> to <value>` lines. The setting sits where a
reader looks for it, without repeating the kind and name, and the
grammar change behind it (a property may have an indented block) lets
an argument have a block under it. That closes the gap D13 left open:
nested blocks such as `capacity_provider_strategy` can now be set,
checked against the same allowlist as top-level arguments. Keys stay raw
provider names, so they still look different from portable code (D4).
A top-level `on <cloud>` block remains for settings that apply to the
whole program. The old form is an error that shows the new one; no
edition has shipped, so no files break.

## D17 — No clowd.toml: a project is its .clowd file
Project settings are written in Clowd. `project <name>` holds the name,
edition, tags and (later) the state backend; `network` holds whether
services are private and the address block; an existing VPC goes in a
top-level `on aws` `network` block, since its IDs only mean something
on AWS. Whether services reach the internet through NAT, and which
addresses they use, is infrastructure design that a reader of the file
should see, so this reverses the "deploy settings" part of D14. It also
supersedes D7 (the edition moves into the `project` block) and D12 (the
TOML parser is deleted). The tool to run is a property of the machine,
not the project, so it is `--runtime` or `CLOWD_RUNTIME`. clowd.lock is
written by the CLI and never edited, so it is JSON, read with the
standard library. An existing clowd.toml is an error that says where
each setting went, rather than being silently ignored. Without a
`project` declaration the directory's name is still used, but
`clowd init` writes one, so renaming the directory doesn't rename
everything deployed.

## D18 — Portable first: customisation has a plain-English form where clouds agree
Each setting goes in one place, chosen by what it means: a portable
property or trait when every cloud can express it (`storage 50gb`,
`highly available`, `retry 5 times`), an `on <cloud>` block in the
resource when it belongs to one cloud (D16), a `project` or `network`
declaration when it isn't about one resource (D17), and a CLI flag when
it is about the machine. Portable features go in the capability matrix,
which reports a cloud's narrower limits (Pub/Sub's 5–100 delivery
attempts, RDS's 35 days of backups) instead of the syntax narrowing to
the strictest cloud. `on` blocks are for what really is one cloud's, so
the overrides allowlist points at the portable form where there is one
(`versioning` on a bucket → the `versioned` trait). `protected` replaces
the per-cloud overrides D15 suggested for data that must survive
`clowd destroy`.

## D19 — The language server is hand-written and shares check's code
LSP is JSON-RPC with a Content-Length header, and the server needs a
dozen message types, so it is written with the standard library instead
of adding a dependency (CLAUDE.md asks for few). It answers what
`clowd check` would, by calling the same `analyze.File`, and re-checks
the whole file on each change: files are small, and incremental
checking would be a second implementation to keep in step. What
completion offers is read from the checker (`check.VocabularyOf` runs
it in a recording mode) rather than listed again, so the editor and the
compiler can't disagree; hover and completion docs are a table in
`internal/lsp`, with a test that every trait and property has one. The
VS Code extension is plain JavaScript, so it needs no build step, and its
only runtime dependency is `vscode-languageclient`.

## D20 — Highlighting comes from the server; editors/vim is the fallback
`clowd lsp` sends semantic tokens for the whole file, built from the
same parse as `clowd check`, so every LSP editor colours Clowd as the
compiler reads it (`queue connects to db` is a relationship) without a
grammar per editor. The token types are standard ones (`struct` for
kinds, `operator` for verbs, `decorator` for raw provider arguments) so
themes colour them with no setup. Grammars are still needed for colour
before the server starts and in editors without semantic tokens: the
VS Code extension keeps its TextMate grammar, and `editors/vim` ships a
regex syntax file with file-type detection and an `lsp/clowd.lua`, so
Neovim needs only `vim.lsp.enable("clowd")`. A tree-sitter grammar
(phase 7) would replace the syntax file, not the tokens.
