<!--
Copyright (c) 2017-2026 Gigamon, Inc. All rights reserved.

Author: Gigamon Terraform Team (gigamon-terraform-team@gigamon.com)

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, version 3 of the License.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>
-->

# Copilot Instructions

## Repository Overview

This repo contains two independent components:

- **`terraform-provider/`** — A Terraform provider (`terraform-provider-gigamon`) that manages Gigamon Fabric Manager (FM) cloud resources via FM's REST API.
- **`tf_fm_backend/`** — A standalone HTTP server (Go + Gin) that exposes Terraform's [HTTP backend](https://developer.hashicorp.com/terraform/language/backend/http) interface, storing state in FM's MongoDB (`fmdb2`). Runs as a systemd service behind HA Proxy at the `/terraform-state` path.

Both components are part of a Go workspace (`go.work`).

## Build Commands

```bash
# Build and install the provider binary to $GOBIN
go install ./terraform-provider

# Build the backend server
go build ./tf_fm_backend

# Release build (cross-compiles for linux/darwin/windows × amd64/arm64, tags repo)
./release.sh <branch> [true|false]   # second arg enables code coverage instrumentation

# Generate docs (run from repo root)
tfplugindocs
```

The provider version is injected at build time:
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X 'main.version=v<version>'" ./terraform-provider
```

After rebuilding for local testing, delete `.terraform/` and `.terraform.lock.hcl` in the consumer TF project before re-running `terraform init` (the local plugin path has a fixed version, so checksums change every build).

## Module Structure

Each internal package is its own Go module (declared in `go.work`):

| Module path | Purpose |
|---|---|
| `terraform-provider/internal/fmclient` | HTTP client for FM REST API |
| `terraform-provider/internal/provider` | Provider registration, auth config |
| `terraform-provider/internal/commonresources` | Resources shared across all platforms (Monitoring Session, apps, maps, tunnels, etc.) |
| `terraform-provider/internal/commonutils` | TypedID system, MS update helpers |
| `terraform-provider/internal/esxiresources` | VMware ESXi-specific resources |
| `terraform-provider/internal/esxidatasources` | VMware ESXi data sources |
| `terraform-provider/internal/thirdpartyorchestrationresources` | Third-party/anyCloud resources |
| `terraform-provider/internal/thirdpartyorchestrationdatasources` | Third-party/anyCloud data sources |
| `terraform-provider/internal/securetunnelcertsresources` | Secure tunnel cert resources |
| `terraform-provider/internal/securetunnelcertsdatasources` | Secure tunnel cert data sources |

## Key Conventions

### TypedID

Resources store their Terraform `id` as a **TypedID** — a compound string `<module>::<type>::<uuid>`:

```
monitoringDomain::vmwareEsxi::550e8400-e29b-41d4-a716-446655440000
```

Helpers in `commonutils/typedid_utils.go`: `MakeTypedID`, `ParseTypedID`, `UUIDFromTypedID`. Module and type constants live in `typedid_enums.go`. Always use these helpers — never construct or parse TypedIDs with string concatenation.

### Resource Pattern

Every resource follows this two-struct pattern:

- **TF model** (e.g. `EsxiMDModel`) — uses `tfsdk:` struct tags, mirrors the TF schema.
- **FM API struct** (e.g. `EsxiFmMD`) — uses `json:` struct tags, maps to the FM REST API. GET responses and POST/PATCH payloads can differ (e.g., `connections` vs `connIds`).

Resources implement `resource.Resource` + `resource.ResourceWithImportState`. Many also implement `resource.ResourceWithModifyPlan`.

### FM Client

`fmclient.FmClient` serializes all **non-GET** operations via an internal mutex to avoid FM concurrency issues (FM can misbehave when multiple resources are created in the same API call burst). GET requests are not serialized.

Authentication uses an API token. The `FM_API_TOKEN` environment variable takes precedence over the `api_token` provider config attribute.

### tf_fm_backend

Documents are stored in MongoDB collection `terraformBackendState` in database `fmdb2`. A compound unique index on `(doc_type, project)` enforces one state doc and one lock doc per project. The service authorizes requests by calling FM's internal auth service (`http://127.0.0.1:6687/authorize`).
