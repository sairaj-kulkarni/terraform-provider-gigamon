# terraform-provider-gigamon

Terraform provider for **Gigamon Fabric Manager (FM) Cloud**, plus an optional HTTP backend service for storing Terraform state inside FM itself.

Compatible with **FM version 6.14 and later**.

| Path | What it is |
|---|---|
| `main.go`, `internal/`, `docs/`, `examples/`, `tools/` | The provider (`terraform-provider-gigamon`). Manages Gigamon FM Cloud resources through the FM REST API. |
| `tf_fm_backend/` | Optional Go service implementing Terraform's HTTP backend protocol for storing shared Terraform state inside FM. |

The two components live in a single Go workspace (`go.work`) for convenient cross-component development, but they build and run independently.

## Quick start — Provider

You can use the provider in two common ways:

* build it locally from this repository (including your fork), or
* download the published **v6.14.0** release artifact from GitHub Releases.

### Option 1: Build locally from this repository or your fork

```bash
go build -o terraform-provider-gigamon
```

Install for local Terraform use:

```bash
mkdir -p ~/.terraform.d/plugins/local/gigamon/gigamon/6.14.0/linux_amd64
cp terraform-provider-gigamon \
  ~/.terraform.d/plugins/local/gigamon/gigamon/6.14.0/linux_amd64/
```

### Option 2: Download from the GitHub release

Download the **v6.14.0** release ZIP from the GitHub Releases page, extract it, rename the binary to `terraform-provider-gigamon` if needed, and place it in the same local Terraform plugin path shown above.

Release page:

[Gigamon provider v6.14.0 release](https://github.com/gigamon-engg/terraform-provider-gigamon/releases/tag/v6.14.0)

Install path:

```bash
mkdir -p ~/.terraform.d/plugins/local/gigamon/gigamon/6.14.0/linux_amd64
cp terraform-provider-gigamon \
  ~/.terraform.d/plugins/local/gigamon/gigamon/6.14.0/linux_amd64/
```

Whether you build from your fork or download the release artifact, the Terraform usage is the same after the binary is placed in the local plugin directory.

### Terraform configuration

```hcl
terraform {
  required_providers {
    gigamon = {
      source  = "local/gigamon/gigamon"
      version = "6.14.0"
    }
  }
}

provider "gigamon" {
  fm_address  = "<your-fm-host>"
  skip_verify = true
  api_token   = "<your-fm-api-token>" # or set the FM_API_TOKEN env var
}
```

See `examples/` for end-to-end configurations and `docs/` for the full resource and data source reference.

## Quick start — Optional FM State Backend

`tf_fm_backend` is a small Go service that runs on the FM appliance and implements Terraform's HTTP backend protocol, storing state inside FM and allowing teams to share Terraform state through FM-managed infrastructure.

Once deployed on FM, point your Terraform configuration at it:

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

Terraform's HTTP backend supports shared state operations over REST, with state fetched via `GET`, updated via `POST`, and optional locking support through dedicated lock and unlock endpoints.

What you get:

* Team-shared state
* Locking for concurrent operations
* Authorization integrated with FM
* No separate external state store to manage

## Backend configuration notes

Terraform's HTTP backend supports the following relevant configuration fields and environment variables:

* `address` / `TF_HTTP_ADDRESS` — required backend endpoint
* `lock_address` / `TF_HTTP_LOCK_ADDRESS` — optional lock endpoint
* `unlock_address` / `TF_HTTP_UNLOCK_ADDRESS` — optional unlock endpoint
* `username` / `TF_HTTP_USERNAME` — optional username for HTTP basic authentication
* `password` / `TF_HTTP_PASSWORD` — optional password for HTTP basic authentication
* `skip_cert_verification` — optional TLS verification bypass, default `false`

Terraform recommends using environment variables for credentials and other sensitive values instead of hardcoding them in backend configuration.

## Development

```bash
# Build provider and backend
go build .
go build ./tf_fm_backend

# Run all tests
go test ./...
```

Generated documentation under `docs/` is produced by [`tfplugindocs`](https://github.com/hashicorp/terraform-plugin-docs).

## License

See [LICENSE](LICENSE)


---

## Sources

- [Backend Type: http | Terraform | HashiCorp Developer](https://developer.hashicorp.com/terraform/language/backend/http)
- [Gigamon provider v6.14.0 release](https://github.com/gigamon-engg/terraform-provider-gigamon/releases/tag/v6.14.0)
