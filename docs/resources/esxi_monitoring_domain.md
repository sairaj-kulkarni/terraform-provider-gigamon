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
page_title: "ESXi Monitoring Domain"
subcategory: "ESXi"
description: "Manage ESXi monitoring domains in Gigamon FM."
---

# gigamon_esxi_monitoring_domain

Manages a **Gigamon ESXi Monitoring Domain** in Fabric Manager. A Monitoring Domain is the top-level logical container for a VMware ESXi deployment — it groups the vCenter connection, the deployed vSeries Nodes, and the workload inventory being monitored.

The typical creation order is:

1. `gigamon_esxi_monitoring_domain` — create the domain
2. `gigamon_esxi_connection` — attach a vCenter connection (references the domain's `id`)
3. `gigamon_esxi_fabric` — deploy vSeries Nodes (references the connection's `id`)

## Example Usage

### Minimal monitoring domain

```hcl
resource "gigamon_esxi_monitoring_domain" "md" {
  alias = "prod-esxi-md"
}
```

### With public IP notifications enabled

Use this when FM is deployed in an environment where it has both private and public management addresses (for example, AWS VPC or OpenStack), and vSeries Nodes must reach FM over the public address.

```hcl
resource "gigamon_esxi_monitoring_domain" "md" {
  alias                          = "prod-esxi-md"
  use_public_ip_for_notifications = true
}
```

### Full ESXi stack — domain, connection, and fabric

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

resource "gigamon_esxi_fabric" "fabric" {
  name             = "prod-fabric"
  connection_id    = gigamon_esxi_connection.vcenter.id
  datacenter_moref = data.gigamon_esxi_datacenter.dc.data_center_moref
  image_id         = gigamon_esxi_image.vseries_image.id

  host_vm_spec = { /* ... */ }
}
```

### Output the domain ID

```hcl
output "monitoring_domain_id" {
  description = "Monitoring domain ID — pass to gigamon_esxi_connection as monitoring_domain_id"
  value       = gigamon_esxi_monitoring_domain.md.id
}
```

## Argument Reference

- `alias` (String, **Required**) – User-provided name for this Monitoring Domain. Changing this forces a new resource.
- `use_public_ip_for_notifications` (Boolean, Optional, default `false`) – When `true`, FM advertises its public management IP address to vSeries Nodes for event notifications. Set this to `true` when FM has both private and public addresses and vSeries Nodes must reach FM over the public interface (for example, FM deployed in an AWS VPC or OpenStack environment).

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `id` (String) – Identifier for this Monitoring Domain. Pass this to `gigamon_esxi_connection` as `monitoring_domain_id`.
- `platform` (String) – Cloud platform for this Monitoring Domain. Always `"vmwareEsxi"` for ESXi domains.
- `connection_id` (String) – ID of the vCenter connection currently associated with this Monitoring Domain. Populated by FM once a `gigamon_esxi_connection` referencing this domain has been successfully created.

## Import

This resource supports import using the Monitoring Domain's typed ID:

```bash
terraform import gigamon_esxi_monitoring_domain.md <monitoring_domain_id>
```

The `<monitoring_domain_id>` is the value of the `id` attribute as stored in Terraform state (a typed ID in the form `monitoringDomain::vmwareEsxi::<uuid>`).

## Related Resources

- `gigamon_esxi_connection` – vCenter connection that references this domain via `monitoring_domain_id`.
- `gigamon_esxi_fabric` – Fabric deployment that references the connection attached to this domain.

