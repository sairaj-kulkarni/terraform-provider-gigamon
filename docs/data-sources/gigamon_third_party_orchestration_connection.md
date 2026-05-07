## Data Source: `gigamon_third_party_orchestration_connection`

Use this data source to look up an existing Third‑Party Orchestration Connection that was created outside Terraform (for example, in the Fabric Manager UI or other automation).
This data source is read‑only and is typically used to wire an externally managed Connection into Terraform‑managed resources (for example, Third‑Party Orchestration Monitoring Session).


## Example Usage

### Look up a Connection within a Monitoring Domain

```hcl
data "gigamon_third_party_orchestration_connection" "terraform_conn" {
  alias                = "example-conn"
  monitoring_domain_id = data.gigamon_third_party_orchestration_monitoring_domain.terraform_md.id
}
```

Use the data source in dependent resources, for instance:

```hcl
resource "gigamon_monitoring_session" "terraform_ms" {
  alias                = "example-ms"
  monitoring_domain_id = data.gigamon_third_party_orchestration_monitoring_domain.terraform_md.id
  connection_id        = data.gigamon_third_party_orchestration_connection.terraform_conn.id
  tapping_method       = data.gigamon_third_party_orchestration_connection.terraform_conn.tapping_method

  # ...
}
```

## Argument Reference (User‑provided)

### Required
* `alias (String)`                - Connection alias to look up in Fabric Manager. Must match an existing Connection in Fabric Manager.
* `monitoring_domain_id (String)` - Monitoring Domain ID that this Connection must belong to.


## Attributes Reference (Read-only)

In addition to the arguments above, this data source exposes the following computed attributes.
For Third‑Party Orchestration, platform is refered as `anyCloud` in FM.

* `id (String)`             - The unique identifier of this Connection. Stored in Terraform as `connection::anyCloud::<uuid>`
* `tapping_method (String)` - Tapping method reported by Fabric Manager. Possible values: `uctv`, `customerOrchestratedSource`.
* `status (String)`         - Connectivity status of this connection.
