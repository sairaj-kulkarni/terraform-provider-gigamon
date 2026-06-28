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
page_title: "Third-Party Orchestration Connection"
subcategory: "Third-Party Orchestration"
description: "Manage third-party orchestration connections in Gigamon FM."
---

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
* `tapping_method (String)`       - Tapping method for the connection. Possible values: `uctv`, `customerOrchestratedSource`.


## Attributes Reference (Read-only)

In addition to the arguments above, this resource exposes the following computed attributes:
For Third‑Party Orchestration, platform is refered as anyCloud in FM.

* `id (String)`     - The unique identifier of this Connection. Stored in Terraform as `connection::anyCloud::<uuid>`.
* `status (String)` - Connectivity status of this connection.


## Behavioral Notes

For Third‑Party Orchestration, Fabric Manager does not allow deletion of a Connection while UCT‑Vs or VSeries nodes remain registered. As a result, `terraform destroy` will fail when attempting to delete this resource if any such nodes are still associated.

To complete the destroy operation:
1. Manually unregister all UCT‑Vs and VSeries nodes.
2. Re‑run `terraform destroy`.

After the nodes are successfully unregistered, Terraform will be able to delete this resource.


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
