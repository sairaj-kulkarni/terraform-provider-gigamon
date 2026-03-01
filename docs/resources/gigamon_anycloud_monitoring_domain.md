## Resource: `gigamon_anycloud_monitoring_domain`

A Third‑Party Orchestration (3PO) Monitoring Domain is a logical boundary in Gigamon Fabric Manager that defines which part of your environment is monitored and how traffic is acquired. 
It groups Gigamon‑monitored entities such as V Series nodes, VSeries Proxy, UCT‑Vs, and UCT‑V Controllers.

This resource supports **exactly one** tapping method configuration:

* `uctv` (UCT‑V tapping), or
* `none` (Customer‑Orchestrated Source)


## Example Usage

### UCT‑V tapping method

```hcl
resource "gigamon_anycloud_monitoring_domain" "terraform_md" {
  alias = "example-md"

  # Exactly one of `uctv` or `none` must be set.
  uctv = {
    mtu                    = 1350
    dual_stack_prefer_ipv6 = true
  }
}
``` 

### Customer‑Orchestrated Source, tapping method = none

```hcl
resource "gigamon_anycloud_monitoring_domain" "terraform_md" {
  alias = "example-md"

  # Exactly one of `uctv` or `none` must be set.
  none = {
    uniform_traffic_policy = true
  }
}
``` 

### Notes
Mutual exclusivity: Exactly one of `uctv` or `none` must be configured.

If tapping method is `uctv`:

* `uctv.mtu` defaults to `1450`
* `uctv.dual_stack_prefer_ipv6` defaults to `false`.

If tapping method is `none`:

* `none.uniform_traffic_policy` defaults to `false`.


## Argument Reference (User‑provided)

### Required
* `alias` *(String)*    - User-provided alias or name for this Monitoring Domain.

### Optional, exactly one required
* `uctv` *(Object)* - UCT‑V tapping method configuration. UCT‑Vs are deployed on your VMs to acquire traffic and forward it to V Series nodes.
* `none` *(Object)* - Customer‑Orchestrated Source configuration. You must ensure that mirrored, tunneled, or raw traffic from workloads is directed to the V Series nodes.

### uctv
* `mtu` *(Number)*                  - MTU between UCT‑V and V Series nodes. Valid range: `1280`–`9000`. Defaults to `1450`.
* `dual_stack_prefer_ipv6` *(Bool)* - If `true`, IPv6 tunnels are preferred between UCT‑V and V Series nodes. Defaults to `false`.

### none
* `uniform_traffic_policy` *(Bool)* - If `true`, the same monitoring session configuration is applied to all V Series nodes in the monitoring domain. Defaults to `false`.


## Attributes Reference (Read-only)

In addition to the arguments above, this resource exposes the following computed attributes:

* `id` *(String)*               - The unique identifier of this Monitoring Domain. Stored in Terraform as `monitoringDomain::anyCloud::<uuid>`.
* `connection_id` *(String)*    - The connection ID associated with this monitoring domain. This will be available after you create the Connection resource.
* `platform` *(String)*         - The platform on which this Monitoring Domain was created. For Third‑Party Orchestration, this is anyCloud.
* `tapping_method` *(String)*   - The derived tapping method based on the configured nested block. One of `uctv` or `none`.
* `user_launched` *(Bool)*      - Indicates whether the V Series nodes are launched and managed by the user. True for Third‑Party Orchestration.


## Import

This resource supports Terraform configuration-driven imports using the `import` block. The import `id` is the Monitoring Domain `alias`.

```hcl
import {
  to = gigamon_anycloud_monitoring_domain.terraform_md
  id = "example-md"
}
```

* `to`  - Must match an existing resource address in your configuration.
* `id`  - Monitoring Domain alias in Fabric Manager.
