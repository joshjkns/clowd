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

Items 1–4 are implemented in `internal/lsp` (D19):

- **Transport.** JSON-RPC over stdio with the standard library. Full-text
  sync: every change re-runs `analyze.File`, the same call `clowd check`
  makes. Positions are UTF-8 when the client offers it, else UTF-16.
- **Diagnostics** are `clowd check`'s, with the suggestion on a second
  line (`help: ...`). As in the CLI, capability findings appear only
  once the file has no errors.
- **Hover** on a kind, resource name or reference shows each target's
  product, or AWS and GCP when the file has no `use` line, plus the
  matrix's notes for that resource or relationship. The matrix is
  evaluated even on a file with errors, since the IR is always usable
  (D9). Keywords, traits, properties, verbs, clouds, raw provider
  arguments (with a link to the provider docs) and top-level `on`
  settings have docs too.
- **Completion** reads the line's words and the lines it is indented
  under, not the AST, because the line being typed rarely parses. It
  offers: kinds, `use`, `on` and services at line start; clouds after
  `use` and `on`; traits after `is`, `,` or `and`, leaving out groups
  already used; the rest of a verb and the names it can target (by
  `check.RelationTargets`); a kind's properties and `on` in its block;
  value keywords (`scale 2 to`, `listen on 443 with tls for`, ...);
  services after `send <path> to`; `on aws` arguments, nested blocks
  included; and top-level `on aws` settings. Traits and properties come
  from `check.VocabularyOf`, which reads them from the checker, so
  completion can't offer what check rejects.
- **Formatting** returns one whole-file edit, or none when the file is
  already formatted or has syntax errors (D11).

## Formatter (`clowd fmt`)

One canonical style, no options:
- lowercase keywords
- 2-space indentation
- one blank line between declarations; relate lines grouped together
- `,` between traits normalised to `, `

Ship it early. It ends style debates and makes golden tests stable.

The full rules, as implemented in `internal/format`:

- **Words.** Single spaces between words. Keywords are lowercase: statement
  words (`use`, `in`, `is`, `and`, `on`, `sets`, `to`), kinds, verbs, cloud
  names, traits, property keys, and the keyword words inside values
  (`scale 2 to 10`, `send / to api`, `listen on 443 with tls for ...`).
  Names, regions, other values (images, env names and values) and raw
  override keys and values are kept as written. Quoted strings stay quoted.
- **Spellings that are both valid are kept** (D11): `lb` stays `lb`, and
  traits joined by `and` keep `and`; `,` becomes `, `.
- **Blank lines.** Consecutive one-line statements of the same shape (use
  lines, relate lines, declarations without a block) are grouped with no
  blank line between them. Every other pair of statements gets one blank
  line. Blocks contain no blank lines.
- **Comments** stay where they are. A comment at the end of a line keeps
  one space before its `#`. An own-line comment belongs to the block it is
  in (re-indented to two spaces), or else to the statement after it. A
  blank line in the source before a leading comment, or between a comment
  and its statement, is kept (as one), so a file header stays separate and
  a group can be split into commented sections.
- Output uses `\n` line endings, drops a byte order mark and ends with one
  newline.
- **Files with syntax errors are not formatted** (D11).

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
