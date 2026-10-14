# CLI

Single binary: `clowd`.

## Commands

| Command | What it does |
|---|---|
| `clowd init` | Create a starter `app.clowd` |
| `clowd check [file]` | Parse, check, and show the per-cloud capability matrix |
| `clowd fmt [file...]` | Format in place (`--check` to only report) |
| `clowd build [--target c]` | Compile to `.clowd/build/<cloud>/main.tf.json` |
| `clowd plan [--target c]` | Build, then `tofu plan` |
| `clowd apply [--target c]` | Build, then `tofu apply` |
| `clowd destroy [--target c]` | `tofu destroy` for a target |
| `clowd lsp` | Run the language server on stdio |
| `clowd version` | Print version and edition |

## Conventions

- With no file argument, commands use the `*.clowd` files in the current
  directory. A project is the directory holding its `.clowd` file; there
  is no config file (D17).
- `--target` is required for `plan` / `apply` / `destroy` when the file has more
  than one `use` line. Deploying to several clouds at once is deliberate, never
  implicit.
- `--json` on `check` and `build` for CI.
- Exit codes: `0` success, `1` check/deploy failure, `2` usage or source errors.
- Human output is coloured on a terminal; see Output below. `--json` never
  is.
- The CLI never shows OpenTofu jargon unless `--verbose` is set; tofu output is
  passed through for `plan` / `apply`.

## Output

Every command with human output takes `--color=auto|always|never`. `auto`
(the default) colours a stream only when it is a terminal, `NO_COLOR` is
unset and `TERM` isn't `dumb`; stdout and stderr are decided separately.
Only the 16 basic colours are used, so the terminal theme decides how they
look. Styling lives in `internal/term` and diagnostics are rendered with
`diag.RenderAllStyled`.

- **Diagnostics** follow rustc: the severity in its colour (error red,
  warning yellow, info blue), the message bold, the gutter and `-->` blue,
  carets in the severity colour, `help:` cyan.
- **Results** end with one summary line: `✓` and green for success, `✗`
  and red (or yellow, when it is something to fix rather than a failure),
  with parts joined by ` · `.
- Paths that scripts may want (`fmt --check`) go to stdout one per line;
  summaries go to stderr.

## `clowd check`

```
clowd check [--target cloud] [--json] [file.clowd ...]
```

- Columns are the file's `use` clouds; `--target aws,azure` (or repeated
  `--target`) checks other clouds instead. Flags may come before or after
  files.
- No file argument means every `*.clowd` in the current directory, each
  checked on its own.
- Diagnostics go to stderr with source snippets; the matrix goes to stdout,
  under a header naming the file and its clouds and regions, in sections
  (Resources, Relationships, Overrides), followed by a legend and a
  summary of which clouds are deployable.
  A file with errors shows only its diagnostics, since its matrix would be
  built from a partial program.
- `--json` prints `{"files": [...]}` with, per file, `path`, `ok`,
  `clouds`, `deployable`, `rows` (section, label, position, a cell per cloud with
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
- The path of every file changed (or, with `--check`, that would change)
  goes to stdout; a summary such as `✓ formatted 1 of 3 files` goes to
  stderr.
- `--check` changes nothing; the exit code is `1` if any file is not
  formatted. Use it in CI.
- A file with syntax errors is left unchanged: its diagnostics go to stderr
  and the exit code is `2`. Other files are still formatted. Check errors
  (unknown properties, missing images, ...) don't block formatting.
- No file argument means every `*.clowd` in the project directory, as for
  `check`.
- For clouds Clowd can deploy to, `check` also runs lowering, so problems
  only the backend can see (an `on aws` key the resource doesn't have, a
  Fargate size that had to be rounded up) are reported before deploying.

## `clowd init`

```
clowd init [dir]
```

Writes a starter `app.clowd` and adds `.clowd/` to `.gitignore`. The
starter declares `project <name>`, with the name taken from the
directory, so renaming the directory later doesn't rename everything
deployed. Refuses to run where there are `.clowd` files already.

## `clowd build`, `plan`, `apply`, `destroy`

```
clowd build   [--target cloud] [--json] [file.clowd]
clowd plan    [--target cloud] [--runtime tool] [file.clowd]
clowd apply   [--target cloud] [--runtime tool] [--yes] [file.clowd]
clowd destroy [--target cloud] [--runtime tool] [--yes] [file.clowd]
```

- A project deploys one program. With no file argument the project
  directory must hold exactly one `.clowd` file.
- `--target` must be a cloud the file `use`s. `build` builds every target
  unless given one; the others need exactly one, and say so when the file
  has several.
- Before lowering, the file is checked and evaluated against the matrix;
  errors exit `2` and an undeployable target exits `1`, as for `check`.
- `build` writes `.clowd/build/<cloud>/main.tf.json` and nothing else, so
  OpenTofu's state in the same directory is untouched.
- `plan`, `apply` and `destroy` run `tofu init` quietly (its output is
  shown on failure or with `--verbose`), then pass the tool's own output
  through. `--yes` skips the confirmation. Ctrl-C goes to OpenTofu, which
  stops cleanly.
- After `init`, the provider version OpenTofu chose is written to
  `clowd.lock`, and every later build pins it. A pin that changed (after
  a `git pull`) replaces the directory's own lock.
- `apply` with TLS listeners runs in two steps the first time: it creates
  the certificates alone, prints the DNS records that validate them, then
  applies the rest, which waits until the records exist. Afterwards it
  prints the outputs (URLs, bucket names) and, for a TLS domain, the CNAME
  to point at the load balancer.
- `destroy` uses the configuration last built for that cloud, which is
  what the state was made from, even if the file has changed since. With
  nothing built it has nothing to do. It removes data too (D15).
- The tool is `--runtime`, else `$CLOWD_RUNTIME`, else `tofu` if it is
  installed, else `terraform`. It is a property of the machine, not of the
  project: two people can use different tools on the same state (D1).
- The project's name prefixes every deployed name. It is the `project`
  declaration's name, else the directory's.
- State is local, in `.clowd/build/<cloud>/`: deleting `.clowd/` loses
  track of what was deployed. Remote state backends come later.

## Project settings

Everything about a project is written in its `.clowd` file (D17): the
`project` and `network` declarations, and an `on aws` `network` block for
an existing VPC (01-language.md). A `clowd.toml` from an earlier version is
an error that says where each of its settings went.

## Project files

| File | Written by | Purpose |
|---|---|---|
| `*.clowd` | `clowd init`, user | infrastructure and project settings |
| `clowd.lock` | CLI | pinned provider versions, as JSON |
| `.clowd/` | CLI | build output and tofu working dirs (gitignored) |
