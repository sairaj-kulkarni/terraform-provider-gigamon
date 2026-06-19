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

## Resource: `gigamon_third_party_orchestration_monitoring_domain`

A Third‑Party Orchestration Monitoring Domain is a logical boundary in Gigamon Fabric Manager that defines which part of your environment is monitored and how traffic is acquired.
It groups Gigamon‑monitored entities such as VSeries nodes, VSeries Proxy, UCT‑Vs, and UCT‑V Controllers.

This resource supports **exactly one** tapping method configuration:

* `uctv` (UCT‑V tapping), or
* `customer_orchestrated_source` (Customer‑Orchestrated Source)


## Example Usage

### UCT‑V tapping method, tapping method = uctv

```hcl
resource "gigamon_third_party_orchestration_monitoring_domain" "terraform_md" {
  alias = "example-md"

  # Exactly one of `uctv` or `customer_orchestrated_source` must be set.
  uctv = {
    mtu                    = 1350
    dual_stack_prefer_ipv6 = true
  }
}
``` 

### Customer‑Orchestrated Source, tapping method = customer_orchestrated_source

```hcl
resource "gigamon_third_party_orchestration_monitoring_domain" "terraform_md" {
  alias = "example-md"

  # Exactly one of `uctv` or `customer_orchestrated_source` must be set.
  customer_orchestrated_source = {
    uniform_traffic_policy = true
  }
}
``` 

### Notes
Mutual exclusivity: Exactly one of `uctv` or `customer_orchestrated_source` must be configured.

If tapping method is `customer_orchestrated_source`:

* `uctv.mtu` defaults to `1450`
* `uctv.dual_stack_prefer_ipv6` defaults to `false`.

If tapping method is `customer_orchestrated_source`:

* `customer_orchestrated_source.uniform_traffic_policy` defaults to `false`.


## Argument Reference (User‑provided)

### Required
* `alias (String)`    - User-provided alias or name for this Monitoring Domain.

### Optional, exactly one required
* `uctv (Object)` - UCT‑V tapping method configuration. UCT‑Vs deployed on your VMs to acquire traffic and forward it to VSeries nodes.
* `customer_orchestrated_source (Object)` - Customer‑Orchestrated Source configuration. You must ensure that mirrored, tunneled, or raw traffic from workloads is directed to the VSeries nodes.

### uctv
* `mtu (Number)`                  - MTU between UCT‑V and VSeries nodes. Valid range: `1280`–`9000`. Defaults to `1450`.
* `dual_stack_prefer_ipv6 (Bool)` - If `true`, IPv6 tunnels are preferred between UCT‑V and VSeries nodes. Defaults to `false`.

### customer_orchestrated_source
* `uniform_traffic_policy (Bool)` - If `true`, the same monitoring session configuration is applied to all VSeries nodes in the monitoring domain. Defaults to `false`.


## Attributes Reference (Read-only)

In addition to the arguments above, this resource exposes the following computed attributes:
For Third‑Party Orchestration, platform is refered as `anyCloud` in FM.

* `id (String)`               - The unique identifier of this Monitoring Domain. Stored in Terraform as `monitoringDomain::anyCloud::<uuid>`.
* `connection_id (String)`    - The connection ID associated with this monitoring domain. This will be available after you create the Connection resource.
* `platform (String)`         - The platform on which this Monitoring Domain was created.
* `tapping_method (String)`   - The derived tapping method based on the configured nested block. One of `uctv` or `customer_orchestrated_source`.
* `user_launched (Bool)`      - Indicates whether the VSeries nodes are launched and managed by the user. True for Third‑Party Orchestration.


## Behavioral Notes

For Third‑Party Orchestration, Fabric Manager does not allow deletion of a Monitoring Domain while UCT‑Vs or VSeries nodes remain registered. As a result, `terraform destroy` will fail when attempting to delete this resource if any such nodes are still associated.

To complete the destroy operation:
1. Manually unregister all UCT‑Vs and VSeries nodes.
2. Re‑run `terraform destroy`.

After the nodes are successfully unregistered, Terraform will be able to delete this resource.


## Import

This resource supports Terraform configuration-driven imports using the `import` block. The import `id` is the Monitoring Domain `alias`.

```hcl
import {
  to = gigamon_third_party_orchestration_monitoring_domain.terraform_md
  id = "example-md"
}
```

* `to`  - Must match an existing resource address in your configuration.
* `id`  - Monitoring Domain alias in Fabric Manager.
