# CLAUDE.md

Clowd is an English-like language for describing cloud infrastructure once and
deploying it to multiple clouds. It compiles `.clowd` files to OpenTofu JSON.
Written in Go; module `github.com/joshjkns/clowd`.

## Read first

- `docs/plans/00-overview.md` — what Clowd is and why
- `docs/plans/01-language.md` — the language spec (source of truth for syntax)
- `docs/plans/02-architecture.md` — pipeline and package layout
- `docs/plans/07-roadmap.md` — current phase and what "done" means
- `docs/decisions.md` — decisions already made; don't re-litigate them silently

If a change contradicts a plan, update the plan in the same PR and, for anything
significant, add an entry to `docs/decisions.md`.

## Commands

```sh
go build ./...                 # build everything
go test ./...                  # run all tests
go test ./internal/syntax/...  # run one area
go test ./... -update          # regenerate golden files (review the diff!)
go vet ./...
gofmt -l .                     # must print nothing
go run ./cmd/clowd check examples/app.clowd
go run ./cmd/clowd fmt --check examples/app.clowd
cd site && npm run dev         # the website, on localhost:3000
cd site && npm run build       # static export to site/out
```

## Layout

```
cmd/clowd/          CLI entrypoint
internal/diag/      shared diagnostics (severity, span, message, suggestion)
internal/term/      terminal colour for CLI output (auto, NO_COLOR, --color)
internal/syntax/    token, lexer, ast, parser
internal/resolve/   name resolution
internal/spell/     "did you mean" edit distance
internal/check/     semantic checks; builds the IR
internal/compile/   parse → resolve → check in one call
internal/analyze/   everything `clowd check` reports (compile + matrix + lowering)
internal/ir/        cloud-neutral resource graph
internal/capability/ per-cloud capability matrix
internal/lower/     IR → provider resources, one package per cloud;
                    override/ applies `on <cloud>` blocks for all of them
internal/emit/      .tf.json output
internal/format/    clowd fmt
internal/lsp/       language server
internal/project/   project directory, clowd.lock
internal/runner/    tofu/terraform invocation
editors/            VS Code extension, Vim/Neovim plugin, tree-sitter grammar
examples/           example .clowd files (also end-to-end fixtures)
site/               clowd.jooo.sh: landing page and docs (Next.js, static export)
```

The site is deployed to GitHub Pages by `.github/workflows/pages.yml` on
every push to `main` that touches `site/`. Its docs are written for users
and summarise the plans; when a language or CLI change lands, update the
matching page in `site/src/app/docs/` in the same PR.

## Code conventions

- Standard Go style: `gofmt`, `go vet` clean, idiomatic error handling
  (`fmt.Errorf("...: %w", err)`), no panics outside truly impossible states.
- Everything is in `internal/` until there is a reason to expose an API.
- **Pipeline boundaries are strict:** lowering only sees the IR, never the AST;
  `lower/aws` never imports `lower/gcp`. The CLI and LSP share the same packages.
- **The parser must be error-tolerant.** It never stops at the first error, and
  every AST node carries a span. User-facing problems go through `internal/diag`,
  not Go `error`s. Go errors are for internal/IO failures.
- **Error messages** say what is wrong, point at the span, and suggest a fix when
  one exists ("did you mean `connects to`?").
- Keep dependencies minimal; prefer the standard library. Ask before adding one.
- Comments explain *why*, not *what*. Exported identifiers get doc comments.

## Testing

- Table-driven tests next to the code (`*_test.go`).
- Compiler stages use golden files in the package's `testdata/`:
  `name.clowd` → `name.golden`. Regenerate with `-update` and review the diff.
- Every bug fix gets a test that fails without the fix.
- Files in `examples/` must always pass `clowd check` and `clowd fmt --check`.

## Git

### Conventional Commits

All commits follow [Conventional Commits](https://www.conventionalcommits.org):

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

- **Description:** imperative, lowercase, no trailing period, ≤ 72 chars.
  `feat(parser): recover from missing indent`, not `Added indent recovery.`
- **Body:** explain *why* when it isn't obvious. Wrap at 72.
- **Breaking changes:** `feat(lang)!: rename sends to verb` plus a
  `BREAKING CHANGE:` footer.

Types:

| Type | Use for |
|---|---|
| `feat` | new user-facing capability |
| `fix` | bug fix |
| `refactor` | code change with no behaviour change |
| `perf` | performance improvement |
| `test` | adding or fixing tests only |
| `docs` | docs and plans only |
| `build` | build system, dependencies, go.mod |
| `ci` | CI configuration |
| `chore` | anything else (tooling, housekeeping) |

Scopes (use the area touched; omit if it spans many):
`lexer`, `parser`, `ast`, `resolve`, `check`, `ir`, `capability`, `aws`, `gcp`,
`emit`, `fmt`, `lsp`, `cli`, `project`, `runner`, `vscode`, `tree-sitter`,
`lang` (language spec changes), `examples`, `plans`.

### Workflow

- Never commit directly to `main`. Branch per change:
  `<type>/<short-description>`, e.g. `feat/indent-lexer`, `fix/parser-dedent`.
- Small, focused commits; each should build and pass tests.
- Before committing: `gofmt -l .` prints nothing, `go vet ./...` and
  `go test ./...` pass.
- Open a PR into `main`; squash-merge with a conventional-commit title.
- Don't commit generated build output (`.clowd/`, `bin/`, `dist/`).
