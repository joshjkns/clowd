# Roadmap

Each phase ends with something runnable and tested.

## Phase 1 — Syntax ✓
- `internal/diag`, `internal/syntax/{token,lexer,ast,parser}`
- Indent-aware lexer, error-tolerant parser with spans
- Golden tests from `examples/`
- **Done when:** every example parses; broken inputs give useful errors and a
  partial AST.

## Phase 2 — Meaning ✓
- `internal/resolve`, `internal/check`, `internal/ir`
- `internal/capability` with AWS + GCP entries
- `clowd check` with the matrix table and `--json`
- **Done when:** `clowd check examples/app.clowd` prints the matrix.

## Phase 3 — Formatter ✓
- `internal/format`, `clowd fmt` (+ `--check`)
- **Done when:** `fmt` is idempotent on all examples.

## Phase 4 — AWS deploy
- `internal/lower/aws`, `internal/emit/tfjson`, `internal/runner`, `internal/project`
- `clowd init`, `build`, `plan`, `apply`, `destroy`
- **Done when:** `examples/app.clowd` deploys to a real AWS account and is
  reachable through the load balancer.

## Phase 5 — Editor
- `internal/lsp`: diagnostics → hover → completion → formatting
- `editors/vscode`: TextMate grammar + LSP client
- **Done when:** editing in VS Code shows live diagnostics and per-cloud hovers.

## Phase 6 — GCP deploy
- `internal/lower/gcp`, capability entries
- **Done when:** the same example deploys to GCP unchanged.

## Phase 7 — Later
- tree-sitter grammar
- Azure
- Variables, environments, secrets, outputs, imports (see 01-language.md)

Phase 5 can move before phase 4 if an early demo matters more than deploys.
