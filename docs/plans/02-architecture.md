# Architecture

## Pipeline

```
.clowd source
  → lexer      tokens incl. NEWLINE / INDENT / DEDENT, every token has a span
  → parser     AST with spans + error nodes; never stops at the first error
  → resolve    symbol table: names → declarations, case warnings, duplicates
  → check      kinds, traits, properties, relate verbs valid for the kinds,
               building the IR as it goes (D9)
  → ir         cloud-neutral typed graph of resources and relations
  → capability per target cloud: supported / emulated / unsupported / not yet
  → lower      per cloud: IR → concrete provider resources (aws, gcp, ...)
  → emit       OpenTofu/Terraform JSON (.tf.json)
  → runner     tofu init / plan / apply in .clowd/build/<cloud>/
```

Every stage reports problems through one shared diagnostics type
(severity, span, message, optional suggestion). The CLI and the LSP both
consume the same diagnostics.

## Key principles

- **The parser serves the editor first.** It must be error-tolerant and record
  spans, because the LSP runs it on half-typed files. Hand-written recursive
  descent, not a parser generator.
- **One binary.** CLI, compiler and LSP (`clowd lsp`) share all packages.
- **The IR is the contract.** The language can change above it, backends can
  change below it. Lowering never sees the AST.
- **Lowering is per cloud and isolated.** `lower/aws` knows nothing about GCP.
  Adding a cloud = adding a lowering package + capability entries.
- **Readable output.** Emitted `.tf.json` should be understandable by a human,
  so users can eject to plain Terraform/OpenTofu.

## Backend: OpenTofu

Clowd does not build its own engine. OpenTofu provides state, diffing,
dependency ordering, partial-failure recovery, drift, locking and provider
coverage.

- Emit `.tf.json` (simpler to generate correctly than HCL; read by both tools).
- Default runtime is `tofu`; `terraform` should also work from the same output.
- Each target cloud gets its own root module in `.clowd/build/<cloud>/` with its
  own state.
- The state backend is a `project` setting (local by default; S3/GCS later).
- Provider versions are pinned in `clowd.lock`.

## Repo layout

```
cmd/clowd/              CLI entrypoint (main package)
internal/
  diag/                 diagnostics: severity, span, message, suggestion
  term/                 terminal styling: ANSI colour, NO_COLOR, --color
  syntax/
    token/              token kinds and spans
    lexer/              source → tokens (indent-aware)
    ast/                AST node types
    parser/             tokens → AST, error-tolerant
  resolve/              name resolution, symbol table
  compile/              parse → resolve → check in one call, for CLI and LSP
  spell/                edit distance for "did you mean" suggestions
  check/                semantic checks on the resolved AST; builds the IR
  ir/                   cloud-neutral resource graph
  capability/           capability matrix (data) + evaluation per target
  lower/
    aws/                IR → AWS resources
    gcp/                IR → GCP resources
  emit/tfjson/          resources → .tf.json
  format/               canonical formatter (clowd fmt)
  lsp/                  language server
  project/              project directory + clowd.lock loading
  runner/               invokes tofu / terraform
editors/
  vscode/               VS Code extension (TextMate grammar + LSP client)
  tree-sitter-clowd/    tree-sitter grammar (later)
examples/               example .clowd files, also used as end-to-end fixtures
docs/
  plans/                design docs (this folder)
  decisions.md          decision log
```

Tests live next to the package (`*_test.go`) with fixtures in each package's
`testdata/`. Compiler stages use golden files (`input.clowd` → `expected.*`).
