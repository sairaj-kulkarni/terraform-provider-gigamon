# terraform-provider-gigamon

Terraform provider for **Gigamon Fabric Manager (FM) Cloud**, plus an optional
HTTP backend service for storing Terraform state inside FM itself.

| Path | What it is |
|---|---|
| `main.go`, `internal/`, `docs/`, `examples/`, `tools/` | The provider (`terraform-provider-gigamon`). Manages Gigamon FM cloud resources via the FM REST API. |
| [`tf_fm_backend/`](tf_fm_backend/) | Optional Go service implementing Terraform's [HTTP backend protocol](https://developer.hashicorp.com/terraform/language/backend/http) with state stored in FM's MongoDB. Lets your team share Terraform state without standing up S3 / Azure Blob / Consul / Terraform Cloud. |

The two components live in a single Go workspace (`go.work`) for convenient
cross-component development; they ship and run independently.

## Quick start — Provider

```bash
go build -o terraform-provider-gigamon
```

Install for local Terraform use:

```bash
mkdir -p ~/.terraform.d/plugins/local/gigamon/gigamon/1.0.0/linux_amd64
cp terraform-provider-gigamon \
   ~/.terraform.d/plugins/local/gigamon/gigamon/1.0.0/linux_amd64/
```

Minimal Terraform configuration:

```hcl
terraform {
  required_providers {
    gigamon = {
      source = "local/gigamon/gigamon"
    }
  }
}

provider "gigamon" {
  fm_address  = "<your-fm-host>"
  skip_verify = true
  api_token   = "<your-fm-api-token>"   # or set the FM_API_TOKEN env var
}
```

See [`examples/`](examples/) for end-to-end configurations and [`docs/`](docs/)
for the full resource and data-source reference.

## Quick start — Optional FM State Backend

`tf_fm_backend` is a small Go service that runs on the FM appliance and
implements Terraform's HTTP backend protocol, storing state in FM's MongoDB
and authorizing every request through FM's existing user/RBAC system.

Once deployed on FM (fronted by HA Proxy at `/terraform-state`), point your
Terraform configuration at it:

```hcl
terraform {
  backend "http" {
    address        = "https://<fm-host>/terraform-state/<project>"
    lock_address   = "https://<fm-host>/terraform-state/<project>/lock"
    unlock_address = "https://<fm-host>/terraform-state/<project>/lock"
    lock_method    = "POST"
    unlock_method  = "DELETE"
    username       = "<fm-user>"
    password       = "<fm-password-or-api-token>"
  }
}
```

What you get:

- **Team-shared state** — every engineer running `terraform` against the same
  FM project sees the same state.
- **Locking** — concurrent `terraform apply` runs serialize correctly via the
  backend's lock document.
- **Authorization tied to FM** — only FM users with permission to the project
  may read or write its state. No separate IAM to manage.
- **No external dependencies** — no S3 bucket, no Azure storage account, no
  Consul cluster, no Terraform Cloud subscription. State lives in the FM
  MongoDB you already operate.

Build:

```bash
cd tf_fm_backend
go build -o tf_fm_backend
```

The service is intended to run as a systemd unit on FM. Deployment is handled
by Gigamon FM packaging; this repository is the source.

## Development

```bash
# Build provider and backend
go build .
go build ./tf_fm_backend

# Run all tests
go test ./...
```

Generated docs under `docs/` are produced by
[`tfplugindocs`](https://github.com/hashicorp/terraform-plugin-docs).

## License

See [LICENSE](LICENSE).
