## Resource: `gigamon_monitoring_session`

A Monitoring Session is a set of rules in Fabric Manager that controls visibility of cloud traffic.  
It defines which workloads are in scope, what direction of traffic (ingress, egress, or both) is captured, and how that traffic is processed before being delivered to tools.


## Example Usage

Monitoring Session using Third Party Orchestration (3PO) Monitoring Domain and Connection

```hcl
resource "gigamon_monitoring_session" "terraform_ms" {
  alias                = "example-ms"
  monitoring_domain_id = gigamon_anycloud_monitoring_domain.terraform_md.id
  connection_id        = gigamon_anycloud_connection.terraform_conn.id
  tapping_method       = gigamon_anycloud_connection.terraform_conn.tapping_method

  # Optional (must be non-empty if set)
  description = "Example monitoring session"
}
```


## Argument Reference (User‑provided)

### Required
* `alias` *(String)*                - User-provided alias or name for this Monitoring Session.
* `monitoring_domain_id` *(String)* - Monitoring Domain ID associated with this monitoring session. Changing this forces a new Monitoring Session to be created.
* `connection_id` *(String)*        - Connection ID associated with this monitoring session. Changing this forces a new Monitoring Session to be created.
* `tapping_method` *(String)*       - Tapping method for the session. Possible values: `uctv`, `none`, `platform`. Typically matches the connection tapping method.

### Optional
* `description` *(String)* - Description for the monitoring session (must be non-empty if set).


## Attributes Reference (Read-only)

In addition to the arguments above, this resource exposes the following computed attributes:

* `id` *(String)*                 - The unique identifier of this Monitoring Session. Stored in Terraform as `monitoringSession::<platform>::<uuid>`.
* `deployed` *(Bool)*             - Indicates whether the Monitoring Session is deployed.
* `deployment_status` *(String)*  - Deployment status of the Monitoring Session.


## Import

This resource supports Terraform configuration-driven imports using the `import` block. The import `id` is the Monitoring Session `alias`.

```hcl
import {
  to = gigamon_monitoring_session.terraform_ms
  id = "example-ms"
}
```

* `to`  - Must match an existing resource address in your configuration.
* `id`  - Monitoring Session alias in Fabric Manager.
