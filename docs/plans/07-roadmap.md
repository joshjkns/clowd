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

## Phase 4 — AWS deploy ✓
- `internal/lower/aws`, `internal/emit/tfjson`, `internal/runner`, `internal/project`
- `clowd init`, `build`, `plan`, `apply`, `destroy`
- **Done when:** `examples/app.clowd` deploys to a real AWS account and is
  reachable through the load balancer.
- Verified 2026-10-14 in eu-west-2: the example with a public nginx image
  in place of its placeholder, and a port 80 listener in place of TLS for
  `api.example.com` (a domain nobody can validate). `apply` created all
  of it, the load balancer answered 200, and `destroy` removed it. The
  first attempt found the reserved database name fixed alongside.
  The TLS path (certificate, DNS records, two-step apply) still needs a
  run against a real domain.

## Phase 5 — Editor
- `internal/lsp`: diagnostics → hover → completion → formatting
- `editors/vscode`: TextMate grammar + LSP client
- **Done when:** editing in VS Code shows live diagnostics and per-cloud hovers.
- Built: `clowd lsp` with all four features, tested in-process and over
  its stdio stream; the extension's grammar is tested with VS Code's
  tokenizer and the extension packages with `vsce`. Still to do: the
  check by hand in VS Code itself, then mark the phase done. Go to
  definition, references and rename (05-tooling.md, item 6) come after.
- Also built: semantic tokens from `clowd lsp`, and `editors/vim` for
  Vim and Neovim (D20), checked in a clean Neovim 0.12 with only the
  plugin and `vim.lsp.enable("clowd")`.

## Phase 6 — GCP deploy
- `internal/lower/gcp`, capability entries
- **Done when:** the same example deploys to GCP unchanged.

## Phase 7 — Later
- tree-sitter grammar
- Azure
- Variables, environments, secrets, outputs, imports (see 01-language.md)

Phase 5 can move before phase 4 if an early demo matters more than deploys.
