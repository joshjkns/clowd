# Editor Tooling

## Language server (`clowd lsp`)

Built into the CLI binary and backed by the same parser and checker as the
compiler. Features, in priority order:

1. **Diagnostics** — the same output as `clowd check`, including per-cloud hints
   inline (`⚠ not supported on azure` under the line).
2. **Hover** — what a declaration compiles to on each target
   ("AWS: ECS Fargate · GCP: Cloud Run"), plus docs for keywords and properties.
3. **Completion** — context-aware: kinds at line start, valid properties inside a
   block, verbs after a name, declared names after a verb, raw provider
   properties inside `on <cloud>` blocks.
4. **Formatting** — via `internal/format`.
5. **Go to definition / find references / rename** for resource names.

## Formatter (`clowd fmt`)

One canonical style, no options:
- lowercase keywords
- 2-space indentation
- one blank line between declarations; relate lines grouped together
- `,` between traits normalised to `, `

Ship it early. It ends style debates and makes golden tests stable.

## Syntax highlighting

| Target | Approach |
|---|---|
| VS Code | TextMate grammar in `editors/vscode`, plus LSP client |
| Neovim, Zed, Helix | tree-sitter grammar in `editors/tree-sitter-clowd` (later) |
| GitHub | linguist entry once the language has users |

The tree-sitter grammar needs a small external scanner (C) for
INDENT/DEDENT, like tree-sitter-python.

Semantic tokens from the LSP can refine highlighting later (e.g. colour
resource names by kind).
