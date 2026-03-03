## Resource: `gigamon_third_party_orchestration_connection`

A Third‑Party Orchestration Connection attaches a connection to a Third‑Party Orchestration Monitoring Domain in Gigamon Fabric Manager.
A connection is associated with a single Monitoring Domain.


## Example Usage

### Create a connection for an existing Monitoring Domain

```hcl
resource "gigamon_third_party_orchestration_connection" "terraform_conn" {
  alias                = "example-conn"
  monitoring_domain_id = gigamon_third_party_orchestration_monitoring_domain.terraform_md.id
  tapping_method       = gigamon_third_party_orchestration_monitoring_domain.terraform_md.tapping_method
}
```


## Argument Reference (User‑provided)

### Required
* `alias (String)`                - User-provided alias or name for the Connection.
* `monitoring_domain_id (String)` - Monitoring Domain ID to attach this connection to.
* `tapping_method (String)`       - Tapping method for the connection. Possible values: `uctv`, `none`.


## Attributes Reference (Read-only)

In addition to the arguments above, this resource exposes the following computed attributes:
For Third‑Party Orchestration, platform is refered as anyCloud in FM.

* `id (String)`     - The unique identifier of this Connection. Stored in Terraform as `connection::anyCloud::<uuid>`.
* `status (String)` - Connectivity status of this connection.


## Import

This resource supports Terraform configuration-driven imports using the `import` block. The import `id` is the Connection `alias`.

```hcl
import {
  to = gigamon_third_party_orchestration_connection.terraform_conn
  id = "example-conn"
}
```

* `to`  - Must match an existing resource address in your configuration.
* `id`  - The Connection alias in Fabric Manager.
