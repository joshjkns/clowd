# `clowd check` and the Capability Matrix

## Goal

Tell the user, per target cloud, whether their file will compile to valid
infrastructure — before anything is deployed.

```
$ clowd check app.clowd
                 aws   gcp   azure
database db       ✓     ✓     ✓
bucket uploads    ✓     ✓     ✓
service api       ✓     ✓     ⚠  min scale 2 needs a dedicated plan
lb web            ✓     ✓     ✗  path routing with TLS not yet implemented
on aws ...        ✓     –     –

Deployable: aws ✓  gcp ✓  azure ✗
```

Columns are the clouds in the file's `use` lines (or `--target` to check others).

## Support levels

| Level | Symbol | Meaning |
|---|---|---|
| supported | ✓ | Maps directly to the cloud |
| emulated | ⚠ | Works, with a caveat shown to the user |
| not implemented | ✗ | The cloud could do it; Clowd doesn't yet |
| unsupported | ✗ | The cloud cannot do it |

"Not implemented" and "unsupported" are separate on purpose: one is our backlog,
the other is a real limitation. The message says which.

## The matrix

Keyed by `(kind, feature, cloud)`, where a feature is a trait, a property, or a
relate verb. Stored as Go tables in `internal/capability` so it is type-checked,
and used to generate the support table in the docs.

The same data drives:
- `clowd check` output
- LSP diagnostics and hover ("AWS: ECS Fargate · GCP: Cloud Run")
- lowering, which refuses features the matrix marks as unavailable

## What check does not promise

A static check means "this compiles to valid config for that cloud". It cannot
catch quotas, IAM permissions on the deploying account, or region availability.
For that, `clowd plan --target <cloud>` runs a real OpenTofu plan.

## Output modes

- Default: the table above, plus diagnostics with source snippets.
- `--json`: machine-readable, for CI.
- Exit code: `0` if every target is deployable, `1` if any is not, `2` on
  syntax/semantic errors.
