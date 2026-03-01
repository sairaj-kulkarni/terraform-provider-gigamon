## Data Source: `gigamon_anycloud_monitoring_domain`

Use this data source to look up an existing Third‑Party Orchestration (3PO) Monitoring Domain that was created outside Terraform (for example, in the Fabric Manager UI or other automation).  
This data source is read‑only and is typically used to wire an externally managed Monitoring Domain into Terraform‑managed resources (for example, AnyCloud Monitoring Sessions).


## Example Usage

```hcl
data "gigamon_anycloud_monitoring_domain" "terraform_md" {
  alias = "example-md"
}
```

Use the data source in dependent resources, for instance:

```hcl
resource "gigamon_monitoring_session" "terraform_ms" {
  alias                = "example-ms"
  monitoring_domain_id = data.gigamon_anycloud_monitoring_domain.terraform_md.id
  connection_id        = data.gigamon_anycloud_connection.terraform_conn.id
  tapping_method       = data.gigamon_anycloud_connection.terraform_conn.tapping_method

  # ...
}
```


## Argument Reference (User‑provided)

### Required
* `alias` *(String)*    -  Monitoring Domain alias to look up in Fabric Manager. Must match an existing Monitoring Domain in Fabric Manager.


## Attributes Reference (Read-only)

In addition to the arguments above, this data source exposes the following computed attributes:

* `id` *(String)*                   - The unique identifier of this Monitoring Domain. Stored in Terraform as `monitoringDomain::anyCloud::<uuid>`
* `connection_id` *(String)*        - The connection ID associated with this monitoring domain. Stored in Terraform as `connection::anyCloud::<uuid>`
* `platform` *(String)*             - Platform on which the Monitoring Domain exists (expected: anyCloud).
* `mtu` *(Number)*                  - MTU between UCT‑V and V Series nodes.
* `user_launched` *(Bool)*          - `true` if V Series nodes are launched/managed by the user (expected: true).
* `dual_stack_prefer_ipv6` *(Bool)* - `true` if IPv6 tunnels are preferred between UCT‑V and V Series nodes.
* `uniform_traffic_policy` *(Bool)* - `true` if the same monitoring session configuration is applied to all V Series nodes in the monitoring domain.
