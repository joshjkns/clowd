# Clowd

Describe your cloud infrastructure in plain, structured English. Deploy it to any cloud.

```
use aws in eu-west-2
use gcp in europe-west2

database db is postgres 16, small

service api
  image ghcr.io/me/api:latest
  port 8080

api connects to db
```

```
$ clowd check
               aws   gcp
database db     ✓     ✓
service api     ✓     ✓

Deployable: aws ✓  gcp ✓
```

Clowd compiles to [OpenTofu](https://opentofu.org) JSON, so you get mature state
management and provider support, and can eject to plain Terraform at any time.

> **Status:** early. `clowd check` parses, checks and shows the per-cloud
> capability matrix, and `clowd apply` deploys to AWS (`clowd init` to
> start). GCP is next. See [`docs/plans`](docs/plans/00-overview.md).
