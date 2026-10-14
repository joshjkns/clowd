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
