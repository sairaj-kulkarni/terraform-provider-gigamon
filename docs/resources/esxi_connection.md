---
page_title: "ESXi Connection"
subcategory: "ESXi"
description: "Manage ESXi connections in Gigamon FM."
---

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

# gigamon_esxi_connection

Manages a **vCenter connection** in Gigamon Fabric Manager. FM registers the vCenter, collects its inventory (hosts, datastores, networks), and uses the connection as the anchor for all ESXi fabric deployments and data source lookups associated with that vCenter.

The connection must reach the `connected` status before any data sources or fabric resources that reference it can be used.

## Example Usage

### Minimal connection with defaults

```hcl
resource "gigamon_esxi_monitoring_domain" "md" {
  alias = "prod-esxi-md"
}

resource "gigamon_esxi_connection" "vcenter" {
  alias                = "prod-vcenter"
  monitoring_domain_id = gigamon_esxi_monitoring_domain.md.id
  vcenter_address      = "vcenter.company.com"
  username             = "svc-gigamon@vsphere.local"
  password             = var.vcenter_password
  password_version     = 1
}
```

### Full configuration with all optional fields

```hcl
resource "gigamon_esxi_monitoring_domain" "md" {
  alias = "prod-esxi-md"
}

resource "gigamon_esxi_connection" "vcenter" {
  alias                  = "prod-vcenter"
  monitoring_domain_id   = gigamon_esxi_monitoring_domain.md.id
  vcenter_address        = "vcenter.company.com"
  username               = "svc-gigamon@vsphere.local"
  password               = var.vcenter_password
  password_version       = 1
  tapping_method         = "platform"
  resource_allocation    = "SwitchBased"
  maximum_nodes_per_host = 3
  timeout                = 120
}
```

### Rotating the vCenter password

Because `password` is write-only it is never stored in state. To push a new password to FM, update the `password` value **and** increment `password_version`. Terraform detects the version change, treats the resource as modified, and sends the new password on the next apply.

```hcl
resource "gigamon_esxi_connection" "vcenter" {
  alias                = "prod-vcenter"
  monitoring_domain_id = gigamon_esxi_monitoring_domain.md.id
  vcenter_address      = "vcenter.company.com"
  username             = "svc-gigamon@vsphere.local"
  password             = var.vcenter_password   # updated value
  password_version     = 2                      # increment to trigger the change
}
```

### Output the connection ID for use in other resources

```hcl
output "connection_id" {
  description = "Pass this ID to gigamon_esxi_fabric and data sources"
  value       = gigamon_esxi_connection.vcenter.id
}

output "connection_status" {
  value = gigamon_esxi_connection.vcenter.status
}
```

## Argument Reference

- `alias` (String, **Required**) – User-provided name for this connection.
- `monitoring_domain_id` (String, **Required**) – ID of the `gigamon_esxi_monitoring_domain` this connection belongs to. Changing this forces a new resource.
- `vcenter_address` (String, **Required**) – IP address or FQDN of the vCenter server. Changing this forces a new resource.
- `username` (String, **Required**) – Username FM uses to authenticate with vCenter.
- `password` (String, **Required**, write-only) – Password for the above username. This value is never written to the Terraform state file. To update the password, change this value **and** increment `password_version`.
- `password_version` (Number, **Required**) – Integer used to track password changes. Must be at least `1`. Because `password` is write-only, Terraform cannot detect changes to it directly; incrementing this field signals that a new password should be sent to FM on the next apply.
- `tapping_method` (String, Optional, default `"platform"`) – Method FM uses to capture customer VM traffic. The only supported value is `"platform"`, in which FM manages port-mirroring sessions to capture traffic from workload VMs. Changing this forces a new resource.
- `resource_allocation` (String, Optional, default `"TargetVMBased"`) – Controls how workload VMs are distributed across vSeries Nodes when `tapping_method` is `"platform"`. Must be one of:
  - `"TargetVMBased"` – Workload VMs on a host are distributed to vSeries Nodes on that host proportionally by VM count. Suitable when a host has fewer than 8 VSS or VDS switches.
  - `"SwitchBased"` – Workload VMs are distributed based on the virtual switch they are connected to. Use this when a host has 8 or more VSS or VDS switches, because a single vSeries Node can tap at most 8 switches.
  - `"none"` – No resource allocation policy is applied.
- `maximum_nodes_per_host` (Number, Optional, default `1`) – Maximum number of vSeries Nodes that FM will deploy per ESXi host for this connection. Must be between `1` and `10`. Changing this forces a new resource.
- `timeout` (Number, Optional, default `60`) – Time in seconds to wait for the connection to reach `connected` status after creation. Must be between `30` and `36000`.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `id` (String) – Identifier for this connection. Pass this to `gigamon_esxi_fabric`, `gigamon_esxi_hosts`, and other ESXi data sources as `connection_id`.
- `status` (String) – Current connectivity status reported by FM. Possible values:
  - `"connected"` – FM has successfully connected to vCenter and collected its inventory. The connection is ready for use.
  - `"connFailed"` – FM could not establish a connection to vCenter.
  - `"notConnected"` – The connection has not yet been attempted or has been disconnected.

## Behavior Notes

- FM collects vCenter inventory (hosts, datastores, networks) as part of bringing the connection to `connected` state. The `timeout` argument controls how long the provider waits for this to complete.
- `vcenter_address`, `monitoring_domain_id`, `maximum_nodes_per_host`, and `tapping_method` all force resource replacement when changed, because FM treats these as immutable connection properties.
- `resource_allocation` is an in-place updatable field; changing it does not recreate the resource.

## Related Resources

- `gigamon_esxi_monitoring_domain` – Monitoring domain that `monitoring_domain_id` references.
- `gigamon_esxi_fabric` – ESXi fabric deployment that uses this connection via `connection_id`.
- `gigamon_esxi_hosts` – Data source that discovers host inventory for this connection.
- `gigamon_esxi_datacenter` – Data source that resolves datacenter names for this connection.
- `gigamon_esxi_cluster` – Data source that resolves cluster names for this connection.
