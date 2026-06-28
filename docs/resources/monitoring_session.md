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
page_title: "Monitoring Session"
subcategory: "Monitoring Session"
description: "Manage monitoring sessions in Gigamon FM."
---

## Resource: `gigamon_monitoring_session`

A Monitoring Session is a set of rules in Fabric Manager that controls visibility of cloud traffic.
It defines which workloads are in scope, what direction of traffic (ingress, egress, or both) is captured, and how that traffic is processed before being delivered to tools.


## Example Usage

Monitoring Session using Third Party Orchestration Monitoring Domain and Connection

1  Minimal Monitoring Session

```hcl
resource "gigamon_monitoring_session" "terraform_ms" {
  alias                = "example-ms"
  monitoring_domain_id = gigamon_third_party_orchestration_monitoring_domain.terraform_md.id
  connection_id        = gigamon_third_party_orchestration_connection.terraform_conn.id
  tapping_method       = gigamon_third_party_orchestration_connection.terraform_conn.tapping_method

  description = "Example monitoring session"
}
```


2  Monitoring Session with UCT‑V Traffic Acquisition (Mirroring + Precryption) and Secure Tunnels

```hcl
resource "gigamon_monitoring_session" "terraform_ms" {
  alias                = "example-ms-ta-both"
  monitoring_domain_id = gigamon_third_party_orchestration_monitoring_domain.terraform_md.id
  connection_id        = gigamon_third_party_orchestration_connection.terraform_conn.id
  tapping_method       = gigamon_third_party_orchestration_connection.terraform_conn.tapping_method

  description = "UCT-V MS with Mirroring, Precryption and Secure Tunnels"

  traffic_acquisition = {
    mirroring = {
      secure_tunnels_enabled = true
    }

    precryption = {
      secure_tunnels_enabled = true
    }
  }
}
```


3 Monitoring Session with UCT‑V Mirroring and Prefiltering

```hcl
resource "gigamon_monitoring_session" "terraform_ms" {
  alias                = "example-ms-mirroring-filter"
  monitoring_domain_id = gigamon_third_party_orchestration_monitoring_domain.terraform_md.id
  connection_id        = gigamon_third_party_orchestration_connection.terraform_conn.id
  tapping_method       = gigamon_third_party_orchestration_connection.terraform_conn.tapping_method

  description = "UCT-V MS with Mirroring and pre-filtering policy"

  traffic_acquisition = {
    mirroring = {
      secure_tunnels_enabled = false

      uctv_filtering_policy = {
        rules = [
          {
            rule_name  = "TCP"
            action     = "pass"
            direction  = "bidi"
            priority   = 1
            filters = [
              { name = "proto", relation = "EQUAL_TO", value = "TCP" }
            ]
          },
          {
            rule_name  = "UDP"
            action     = "pass"
            direction  = "bidi"
            priority   = 2
            filters = [
              { name = "proto", relation = "EQUAL_TO", value = "UDP" }
            ]
          }
        ]
      }
    }
  }
}
```


## Argument Reference (User‑provided)

### Required
* `alias (String)`                - User-provided alias or name for this Monitoring Session.
* `monitoring_domain_id (String)` - Monitoring Domain ID associated with this monitoring session. Changing this forces a new Monitoring Session to be created.
* `connection_id (String)`        - Connection ID associated with this monitoring session. Changing this forces a new Monitoring Session to be created.
* `tapping_method (String)`       - Tapping method for the session. Possible values: `uctv`, `customerOrchestratedSource`, `platform`. Typically matches the connection tapping method.

### Optional
* `description (String)`          - Description for the monitoring session (must be non-empty if set).
* `distribute_traffic (Bool)`     - If true, enables distributed traffic behavior. Default: false.
* `traffic_acquisition (Object)`  - Optional UCT‑V traffic acquisition configuration.

#### Traffic Acquisition Rules:
* Supported only when `tapping_method = uctv`.
* If specified, at least one of `mirroring` or `precryption` must be configured. Both may be configured together.

#### traffic_acquisition.mirroring
* `secure_tunnels_enabled (Bool)`  - If true, enables Secure Tunnels for mirroring. Default: false.
* `uctv_filtering_policy (Object)` - UCT‑V filtering policy applied to mirroring traffic.

#### traffic_acquisition.precryption
* `secure_tunnels_enabled (Bool)`  - If true, enables Secure Tunnels for precryption. Default: false.


## Attributes Reference (Read-only)

In addition to the arguments above, this resource exposes the following computed attributes:

* `id (String)`                   - The unique identifier of this Monitoring Session. Stored in Terraform as `monitoringSession::<platform>::<uuid>`.
* `deployed (Bool)`               - Indicates whether the Monitoring Session is deployed.
* `deployment_status (String)`    - Deployment status of the Monitoring Session.
* `deploy_validation_error_msg (String)` - Deployment validation error message returned by Fabric Manager (`deployValidationErrorMsg`). Populated from FM during create, read/refresh, update, and import.

## Deployment Behavior and Drift

Terraform manages the configuration of a monitoring session but does not track transient Deploy/Undeploy actions performed from the GigaVUE-FM UI. Manual Undeploy/Deploy of a monitoring session in the FM UI is **not** treated as drift. After a manual undeploy, `terraform plan` will still report **no changes**, and `terraform apply` will not redeploy the session unless there is a configuration change to the `gigamon_monitoring_session` resource.

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
