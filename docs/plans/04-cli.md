# CLI

Single binary: `clowd`.

## Commands

| Command | What it does |
|---|---|
| `clowd init` | Create `clowd.toml` and a starter `app.clowd` |
| `clowd check [file]` | Parse, check, and show the per-cloud capability matrix |
| `clowd fmt [file...]` | Format in place (`--check` to only report) |
| `clowd build [--target c]` | Compile to `.clowd/build/<cloud>/main.tf.json` |
| `clowd plan [--target c]` | Build, then `tofu plan` |
| `clowd apply [--target c]` | Build, then `tofu apply` |
| `clowd destroy [--target c]` | `tofu destroy` for a target |
| `clowd lsp` | Run the language server on stdio |
| `clowd version` | Print version and edition |

## Conventions

- With no file argument, commands use the files found via `clowd.toml`
  (or `*.clowd` in the current directory if there is no config).
- `--target` is required for `plan` / `apply` / `destroy` when the file has more
  than one `use` line. Deploying to several clouds at once is deliberate, never
  implicit.
- `--json` on `check` and `build` for CI.
- Exit codes: `0` success, `1` check/deploy failure, `2` usage or source errors.
- The CLI never shows OpenTofu jargon unless `--verbose` is set; tofu output is
  passed through for `plan` / `apply`.

## `clowd check`

```
clowd check [--target cloud] [--json] [file.clowd ...]
```

- Columns are the file's `use` clouds; `--target aws,azure` (or repeated
  `--target`) checks other clouds instead. Flags may come before or after
  files.
- Until `clowd.toml` exists (phase 4), no file argument means every
  `*.clowd` in the current directory, each checked on its own.
- Diagnostics go to stderr with source snippets; the matrix goes to stdout.
  A file with errors shows only its diagnostics, since its matrix would be
  built from a partial program.
- `--json` prints `{"files": [...]}` with, per file, `path`, `ok`,
  `clouds`, `deployable`, `rows` (label, position, a cell per cloud with
  level and findings) and `diagnostics` (severity, message, suggestion,
  1-based `line`/`column`/`endLine`/`endColumn`), plus `error` for a file
  that could not be read (the other files are still checked). Fields are
  only ever added. Levels are `supported`, `emulated`, `not implemented`,
  `unsupported` and `not applicable`.

## `clowd fmt`

```
clowd fmt [--check] [file.clowd ...]
```

- Rewrites each file in place in the canonical style (05-tooling.md),
  keeping its permissions. Files already formatted are not touched.
- `--check` changes nothing and prints the path of every file that is not
  formatted; the exit code is `1` if there are any. Use it in CI.
- A file with syntax errors is left unchanged: its diagnostics go to stderr
  and the exit code is `2`. Other files are still formatted. Check errors
  (unknown properties, missing images, ...) don't block formatting.
- No file argument means every `*.clowd` in the current directory, as for
  `check`.

## Project files

| File | Written by | Purpose |
|---|---|---|
| `*.clowd` | user | infrastructure |
| `clowd.toml` | `clowd init`, user | edition, state backend, defaults |
| `clowd.lock` | CLI | pinned provider versions |
| `.clowd/` | CLI | build output and tofu working dirs (gitignored) |
