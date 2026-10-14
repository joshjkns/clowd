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

