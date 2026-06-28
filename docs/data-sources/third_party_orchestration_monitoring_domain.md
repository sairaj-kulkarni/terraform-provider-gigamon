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

---
page_title: "Third-Party Orchestration Monitoring Domain"
subcategory: "Third-Party Orchestration"
description: "Read third-party orchestration monitoring domain information from Gigamon FM."
---

## Data Source: `gigamon_third_party_orchestration_monitoring_domain`

Use this data source to look up an existing Third‑Party Orchestration Monitoring Domain that was created outside Terraform (for example, in the Fabric Manager UI or other automation).
This data source is read‑only and is typically used to wire an externally managed Monitoring Domain into Terraform‑managed resources (for example, Third‑Party Orchestration Monitoring Session).


## Example Usage

```hcl
data "gigamon_third_party_orchestration_monitoring_domain" "terraform_md" {
  alias = "example-md"
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
* `alias (String)`    -  Monitoring Domain alias to look up in Fabric Manager. Must match an existing Monitoring Domain in Fabric Manager.


## Attributes Reference (Read-only)

In addition to the arguments above, this data source exposes the following computed attributes:
For Third‑Party Orchestration, platform is refered as `anyCloud` in FM.

* `id (String)`                   - The unique identifier of this Monitoring Domain. Stored in Terraform as `monitoringDomain::anyCloud::<uuid>`
* `connection_id (String)`        - The connection ID associated with this monitoring domain. Stored in Terraform as `connection::anyCloud::<uuid>`
* `platform (String)`             - Platform on which the Monitoring Domain exists.
* `mtu (Number)`                  - MTU between UCT‑V and VSeries nodes.
* `user_launched (Bool)`          - `true` if VSeries nodes are launched/managed by the user (expected: true).
* `dual_stack_prefer_ipv6 (Bool)` - `true` if IPv6 tunnels are preferred between UCT‑V and VSeries nodes.
* `uniform_traffic_policy (Bool)` - `true` if the same monitoring session configuration is applied to all VSeries nodes in the monitoring domain.
