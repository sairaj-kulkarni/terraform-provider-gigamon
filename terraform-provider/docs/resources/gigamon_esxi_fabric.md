# gigamon_esxi_fabric

Manages a **Gigamon vSeries Node fabric deployment** on the VMware ESXi platform. This resource deploys one or more vSeries Nodes across ESXi hosts within a single vCenter datacenter, all sharing the same image version and form factor.

**Key constraints:**

- All nodes in a deployment share the same image version (controlled by `image_id`) and form factor.
- At most one vSeries Node may be deployed per host within a single deployment. Use multiple deployments to place more than one node on a host.
- When upgrading (`image_id` is changed), `form_factor` and `name_server` values in each node spec may also be updated in-place. Changes to those fields without a corresponding `image_id` change will cause the fabric to be deleted and re-deployed.

## Example Usage

### Minimal deployment with DHCP management interface

```hcl
resource "gigamon_esxi_fabric" "my_fabric" {
  name             = "prod-vseries-fabric"
  connection_id    = gigamon_esxi_connection.my_conn.id
  datacenter_moref = "datacenter-21"
  image_id         = gigamon_esxi_image.vseries_image.id

  host_vm_spec = {
    "host-101" = {
      host_moref   = "host-101"
      host_name    = "esxi-host-01.company.com"
      name         = "vseries-node-01"
      admin_password = "S3cur3P@ssw0rd!"

      datastore_moref = "datastore-55"

      management_interface = {
        network_moref = "network-100"
      }
    }
  }
}
```

### Multi-host deployment with static IP and tunnel interface

```hcl
resource "gigamon_esxi_fabric" "fabric_prod" {
  name             = "prod-vseries-fabric"
  connection_id    = gigamon_esxi_connection.vcenter_conn.id
  datacenter_moref = "datacenter-21"
  image_id         = gigamon_esxi_image.vseries_image.id
  form_factor      = "Medium"
  timeout          = 1200

  host_vm_spec = {
    "host-101" = {
      host_moref      = "host-101"
      host_name       = "esxi-host-01.company.com"
      name            = "vseries-node-01"
      admin_password  = "S3cur3P@ssw0rd!"
      disk_format     = "thin"
      vm_folder       = "/Gigamon/vSeries"
      datastore_moref = "datastore-55"
      name_server     = ["8.8.8.8", "8.8.4.4"]

      management_interface = {
        network_moref           = "network-100"
        address_assignment_mode = "Static"
        ip_address              = "192.168.1.10"
        ip_address_mask         = "255.255.255.0"
        gateway_ip              = "192.168.1.1"
        mtu                     = 1500
      }

      tunnel_interface = {
        network_moref           = "network-200"
        address_assignment_mode = "Static"
        ip_address              = "10.0.0.10"
        ip_address_mask         = "255.255.0.0"
        gateway_ip              = "10.0.0.1"
        mtu                     = 9000
      }
    }

    "host-102" = {
      host_moref      = "host-102"
      host_name       = "esxi-host-02.company.com"
      name            = "vseries-node-02"
      admin_password  = "S3cur3P@ssw0rd!"
      datastore_moref = "datastore-56"

      management_interface = {
        network_moref = "network-100"
      }

      tunnel_interface = {
        network_moref = "network-200"
      }
    }
  }
}
```

### Dynamic host spec from a variable

Use a `for` expression to build `host_vm_spec` from a list of host definitions, avoiding repetition when deploying across many hosts.

```hcl
variable "vseries_hosts" {
  description = "List of ESXi hosts on which to deploy vSeries Nodes"
  type = list(object({
    host_moref      = string
    host_name       = string
    node_name       = string
    admin_password  = string
    datastore_moref = string
    mgmt_network    = string
    mgmt_ip         = optional(string)
    mgmt_mask       = optional(string)
    mgmt_gw         = optional(string)
  }))
}

resource "gigamon_esxi_fabric" "fabric_dynamic" {
  name             = "prod-vseries-fabric"
  connection_id    = gigamon_esxi_connection.vcenter_conn.id
  datacenter_moref = "datacenter-21"
  image_id         = gigamon_esxi_image.vseries_image.id
  form_factor      = "Small"

  host_vm_spec = {
    for h in var.vseries_hosts : h.host_moref => {
      host_moref      = h.host_moref
      host_name       = h.host_name
      name            = h.node_name
      admin_password  = h.admin_password
      datastore_moref = h.datastore_moref

      management_interface = {
        network_moref           = h.mgmt_network
        address_assignment_mode = h.mgmt_ip != null ? "Static" : "DHCP"
        ip_address              = h.mgmt_ip
        ip_address_mask         = h.mgmt_mask
        gateway_ip              = h.mgmt_gw
      }
    }
  }
}
```

Example variable input (e.g. `terraform.tfvars`):

```hcl
vseries_hosts = [
  {
    host_moref      = "host-101"
    host_name       = "esxi-host-01.company.com"
    node_name       = "vseries-node-01"
    admin_password  = "S3cur3P@ssw0rd!"
    datastore_moref = "datastore-55"
    mgmt_network    = "network-100"
    mgmt_ip         = "192.168.1.10"
    mgmt_mask       = "255.255.255.0"
    mgmt_gw         = "192.168.1.1"
  },
  {
    host_moref      = "host-102"
    host_name       = "esxi-host-02.company.com"
    node_name       = "vseries-node-02"
    admin_password  = "S3cur3P@ssw0rd!"
    datastore_moref = "datastore-56"
    mgmt_network    = "network-100"
  },
]
```

### Deploy to all hosts in a datacenter/cluster using data sources

Use `gigamon_esxi_datacenter`, `gigamon_esxi_cluster`, and `gigamon_esxi_hosts` to automatically discover every host in a cluster and deploy a vSeries Node on each one. The management and tunnel interfaces are resolved by network name from the inventory returned by the hosts data source. All nodes use DHCP for address assignment.

> **Note:** The `gigamon_esxi_hosts` data source returns map keys with `.` and ` ` replaced by `-`. The `replace()` calls in `locals` apply the same transformation to the user-supplied network and datastore names so the map lookups always match.

```hcl
# ── Variables ────────────────────────────────────────────────────────────────

variable "connection_id" {
  description = "ID of the gigamon_esxi_connection resource to use"
  type        = string
}

variable "datacenter_name" {
  description = "Name of the vCenter datacenter to deploy vSeries Nodes in"
  type        = string
}

variable "cluster_name" {
  description = "Name of the vCenter cluster whose hosts will each receive a vSeries Node"
  type        = string
}

variable "image_id" {
  description = "Image ID for the vSeries deployment (from gigamon_esxi_image)"
  type        = string
}

variable "datastore_name" {
  description = "Name of the vCenter datastore to use for each vSeries Node VM"
  type        = string
}

variable "management_network_name" {
  description = "Name of the vCenter network to attach as the management interface on all vSeries Nodes"
  type        = string
}

variable "data_network_name" {
  description = "Name of the vCenter network to attach as the tunnel (data) interface on all vSeries Nodes"
  type        = string
}

variable "admin_password" {
  description = "Admin password to set on each deployed vSeries Node"
  type        = string
  sensitive   = true
}

variable "node_name_prefix" {
  description = "Prefix for each vSeries Node VM name; the host key is appended"
  type        = string
  default     = "vseries"
}

# ── Data sources ─────────────────────────────────────────────────────────────

data "gigamon_esxi_datacenter" "dc" {
  connection_id    = var.connection_id
  data_center_name = var.datacenter_name
}

data "gigamon_esxi_cluster" "cluster" {
  connection_id     = var.connection_id
  data_center_moref = data.gigamon_esxi_datacenter.dc.data_center_moref
  cluster_name      = var.cluster_name
}

# Fetch every host in the cluster by matching all hostnames with a wildcard.
data "gigamon_esxi_hosts" "all_hosts" {
  connection_id     = var.connection_id
  data_center_moref = data.gigamon_esxi_datacenter.dc.data_center_moref
  cluster_moref     = [data.gigamon_esxi_cluster.cluster.cluster_moref]
  hostname_pattern  = ".*"
}

# ── Locals ───────────────────────────────────────────────────────────────────

locals {
  # The hosts data source replaces '.' and ' ' with '-' in all map keys.
  # Apply the same normalisation to the user-supplied names so lookups match.
  mgmt_net_key   = replace(replace(var.management_network_name, ".", "-"), " ", "-")
  data_net_key   = replace(replace(var.data_network_name, ".", "-"), " ", "-")
  datastore_key  = replace(replace(var.datastore_name, ".", "-"), " ", "-")

  # Build the host_vm_spec map from the discovered inventory.
  # Keyed by host MORef; one entry per host returned by the data source.
  host_vm_spec = {
    for host_key, host in data.gigamon_esxi_hosts.all_hosts.host_details :
    host.host_moref => {
      host_moref      = host.host_moref
      host_name       = host.hostname
      name            = "${var.node_name_prefix}-${host_key}"
      admin_password  = var.admin_password
      datastore_moref = host.datastore_moref[local.datastore_key]

      # DHCP is the default; no IP fields are required.
      management_interface = {
        network_moref = host.network_moref[local.mgmt_net_key]
      }

      tunnel_interface = {
        network_moref = host.network_moref[local.data_net_key]
      }
    }
  }
}

# ── Fabric deployment ─────────────────────────────────────────────────────────

resource "gigamon_esxi_fabric" "cluster_fabric" {
  name             = "${var.cluster_name}-vseries-fabric"
  connection_id    = var.connection_id
  datacenter_moref = data.gigamon_esxi_datacenter.dc.data_center_moref
  image_id         = var.image_id

  host_vm_spec = local.host_vm_spec
}

# ── Outputs ───────────────────────────────────────────────────────────────────

output "fabric_id" {
  description = "ID of the deployed fabric, for use in monitoring sessions"
  value       = gigamon_esxi_fabric.cluster_fabric.id
}

output "node_management_ips" {
  description = "Management IP assigned to each deployed vSeries Node, keyed by host MORef"
  value = {
    for host_moref, spec in gigamon_esxi_fabric.cluster_fabric.host_vm_spec :
    host_moref => spec.management_interface_ip
  }
}
```

### Reference the deployed fabric from a monitoring session

```hcl
output "fabric_id" {
  value = gigamon_esxi_fabric.fabric_prod.id
}

output "node_management_ips" {
  value = {
    for k, v in gigamon_esxi_fabric.fabric_prod.host_vm_spec :
    k => v.management_interface_ip
  }
}
```

## Argument Reference

### Deployment-level arguments

- `name` (String, **Required**) – User-provided name for this fabric deployment.
- `connection_id` (String, **Required**) – ID of the `gigamon_esxi_connection` resource used to launch this fabric. Changing this forces a new resource.
- `datacenter_moref` (String, **Required**) – vCenter Managed Object Reference (MORef) of the datacenter in which the vSeries Nodes will be deployed. Changing this forces a new resource.
- `image_id` (String, **Required**) – Image ID (from `gigamon_esxi_image`) that determines the vSeries software version to deploy on all nodes.
- `form_factor` (String, Optional, default `"Small"`) – Form factor for all vSeries Nodes in this deployment. Determines the CPU, memory, and disk resources allocated. Must be one of:
  - `"Small"` – 2 vCPU, 4 GB RAM, 8 GB Disk
  - `"Medium"` – 4 vCPU, 8 GB RAM, 8 GB Disk
  - `"Large"` – 8 vCPU, 16 GB RAM, 8 GB Disk
- `timeout` (Number, Optional, default `900`) – Time in seconds to wait for all vSeries Nodes to reach the `ok` state after deployment. Must be between `300` and `36000`.
- `host_vm_spec` (Map of objects, **Required**) – Map of vSeries Node specs keyed by **host MORef**. Each entry defines one vSeries Node to deploy on the corresponding ESXi host. At least one entry is required. See [Node spec arguments](#node-spec-arguments) below.

### Node spec arguments

Each value in `host_vm_spec` is an object with the following fields:

- `host_moref` (String, **Required**) – vCenter MORef of the ESXi host on which this vSeries Node is deployed.
- `host_name` (String, **Required**) – Display name of the ESXi host.
- `name` (String, **Required**) – Name to assign to the vSeries Node VM.
- `admin_password` (String, **Required**, write-only) – Admin password for the vSeries Node. This value is only applied during initial creation; subsequent changes are ignored. The value is never written to the Terraform state file.
- `datastore_moref` (String, Optional) – MORef of the vCenter datastore where the vSeries Node VM files are stored. Exactly one of `datastore_moref` or `datastore_cluster_moref` must be provided.
- `datastore_cluster_moref` (String, Optional) – MORef of the vCenter datastore cluster where the vSeries Node VM files are stored. Exactly one of `datastore_moref` or `datastore_cluster_moref` must be provided.
- `disk_format` (String, Optional, default `"thin"`) – Disk format for the vSeries Node. Must be one of `"thin"`, `"thick"`, or `"eagerZeroedThick"`.
- `vm_folder` (String, Optional, default `"/"`) – Folder path within vCenter where the VM files are placed.
- `name_server` (List of String, Optional) – List of DNS name server addresses to statically assign to this vSeries Node. At least one element required if specified.
- `management_interface` (Object, **Required**) – Management network interface configuration. See [Interface arguments](#interface-arguments) below.
- `tunnel_interface` (Object, Optional) – Tunnel (data/tool) network interface configuration. See [Interface arguments](#interface-arguments) below.

### Interface arguments

Applies to both `management_interface` (required) and `tunnel_interface` (optional):

- `network_moref` (String, **Required**) – vCenter MORef of the network to attach this interface to.
- `address_assignment_mode` (String, Optional, default `"DHCP"`) – Address assignment mode. Must be `"DHCP"` or `"Static"`.
- `mtu` (Number, Optional, default `1500`) – MTU for the network interface. Must be between `1280` and `9000`.
- `ip_address` (String, Optional) – Static IP address for the interface. Required when `address_assignment_mode` is `"Static"`.
- `ip_address_mask` (String, Optional) – Subnet mask in dotted-decimal format (e.g. `"255.255.255.0"`). Required when `address_assignment_mode` is `"Static"`.
- `gateway_ip` (String, Optional) – Default gateway IP address. Used when `address_assignment_mode` is `"Static"`.
- `ipv6_prefix_length` (Number, Optional) – IPv6 prefix length for the interface. Must be between `32` and `126`. Applies to the tunnel interface when using an IPv6 address.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

### Deployment-level attributes

- `id` (String) – Identifier for this fabric deployment, used to reference this resource from other resources.

### Per-node attributes (within each `host_vm_spec` entry)

- `vseries_node_id` (String) – Node ID assigned to this vSeries Node VM by FM. This value changes when the node is upgraded or recreated.
- `cluster_moref` (String) – MORef of the vCenter cluster to which this host belongs, populated by FM.
- `status` (String) – Current status of the vSeries Node VM (e.g. `"launching"`, `"ok"`).
- `version` (String) – Gigamon software version currently running on this vSeries Node.
- `management_interface_ip` (String) – IP address assigned to the management interface of this node.
- `data_interface_ips` (List of String) – List of IP addresses assigned to the data/tunnel interfaces of this node.

## Related Resources

- `gigamon_esxi_connection` – vCenter connection resource that `connection_id` references.
- `gigamon_esxi_image` – ESXi image resource that `image_id` references.
- `gigamon_monitoring_session` – Monitoring session resource that can reference the fabric deployment.
