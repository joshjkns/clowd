# Clowd for VS Code

Language support for [Clowd](https://github.com/joshjkns/clowd) `.clowd`
files:

- **Syntax highlighting** from a TextMate grammar.
- **Diagnostics** as you type: the same errors, warnings and per-cloud
  notes as `clowd check`.
- **Hovers** that show what a resource becomes on each target cloud
  (`AWS: ECS Fargate · GCP: Cloud Run`), and docs for keywords, traits,
  properties and raw provider arguments.
- **Completion** of kinds, traits, properties, verbs, resource names,
  clouds and `on aws` and `on gcp` provider arguments.
- **Formatting** with `clowd fmt` (Format Document, or format on save).

Everything except highlighting comes from the language server built into
the `clowd` binary, `clowd lsp`, so the extension needs `clowd` installed:

```sh
go install github.com/joshjkns/clowd/cmd/clowd@latest
```

If `clowd` isn't on your `PATH`, set **Clowd: Path** (`clowd.path`) to the
binary. After installing a new `clowd`, run **Clowd: Restart Language
Server**.

## Developing

```sh
npm install
npm test             # tokenizes sample source with VS Code's grammar engine
npx vsce package     # builds clowd-<version>.vsix
code --install-extension clowd-0.1.0.vsix
```

To try changes without packaging, open this folder in VS Code and press
F5 to start an Extension Development Host. Set `clowd.path` to a build
from your checkout (`go build -o bin/clowd ./cmd/clowd`) to try server
changes too.
