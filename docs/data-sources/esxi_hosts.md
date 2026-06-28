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
page_title: "ESXi Hosts"
subcategory: "ESXi"
description: "Read ESXi host information from Gigamon FM."
---

## Data Source: gigamon_esxi_hosts

Data source to provide details about the hosts managed in the vCenter associated with this connection

## Example Usage

In this case, we would like to deploy VSeries nodes on the two hosts 10.115.201.45 and 10.115.201.46. First get the detils of the host from the host datastore. This will return details such as the datastore/networks connected to this host along with their MORef. This can then be used to form the map required for the esxi_fabric resource to create the deployment spec for these hosts

```hcl
data "gigamon_esxi_hosts" "my-hosts" {
 connection_id = gigamon_esxi_connection.my-connection.id
 data_center_moref = data.gigamon_esxi_datacenter.my-dc.data_center_moref
 cluster_moref = [
   data.gigamon_esxi_cluster.my-cluster.cluster_moref,
 ]
 hostname = [
  "10.115.201.45",
  "10.115.201.46",
 ]
}
```

Prepare a map variable from the above datasource return, that can be used as the host_vm_spec map in the esxi_fabirc resource.

```hcl
locals {
  hostspec = {
    for host, host_spec in data.gigamon_esxi_hosts.my-hosts.host_details: host_spec.host_moref =>   {
      host_name = host_spec.hostname
      host_moref = host_spec.host_moref
      datastore_cluster_moref = host_spec.datastore_cluster_moref.datastore_qnap2tb
      admin_password = "gigamon123A!"
      name = host_spec.hostname
      management_interface = {
        network_moref = host_spec.network_moref.VM-Network
      }
      tunnel_interface = {
         network_moref = host_spec.network_moref.VM-Network
      }
    }
  }
}
```

Deploy a fabric, on the above two hosts

```hcl
resource "gigamon_esxi_fabric" "my-fabric" {
  name = "my-fabric"
  connection_id = gigamon_esxi_connection.my-conn.id
  datacenter_moref = data.gigamon_esxi_datacenter.my-dc.data_center_moref
  image_id = gigamon_esxi_image.vseries-6-14.id
  host_vm_spec = local.hostspec
}
```

## Argument Refernece

This data soruce supports the following arguments

* `connection_id` - (Required) ID of the connection to use.
* `data_center_moref` - (Required) Datacenter MORef(Managed Object Reference/ID). Returns host details of the hosts belonging to this datacenter
* `cluster_moref` - (Optional) Cluster MORef(Managed Object Reference/ID). If speciefied restricts the returned hosts, to those belonging to this cluster
* `hostname` - (Optional) list of host names. This returns the details of the hosts specified in this list
* `hostname_pattern` - (Optional) Returns the details of the hosts, whose name matches the specified pattern
    * either hostname or hostname_pattern must be specified
    * hostname and hostname_pattern are mutually exclusive, only one of them can be specified

## Attribute Reference

### The attributes are returned as a map, with the name (hostname, or network name) as the key. If the name contains '.' or space ' ', these characters are replaced with a '-' in the corresponding key. Hence when using the attributes, use the names with these characters replaced with '-' as appropriate

This data source exports the following attributes in addition to the arguments above

* `host_details` - Map where the key is the hostname and the value is an object containing the details of that host

### attributed returned for each host

For each host specified in the hostname list (or the hosts matching the hostname pattern), the following details are returned as the value of the 

* `host_moref` - The Host MORef (Managed Object Reference/ID) of this host in the vCenter
* `hostname` - The name of the host in vCenter
* `datastore_moref` - A map, where the key is the datastore name of datastores associated with this host, and the value is the datastore MORef(Managed Object Reference/ID) that is used in vCenter to identify this datastore uniquely. Useful to provide the datastore_moref for the Vseries node spec in esxi_fabric resource
* `datstore_cluster_moref` - A map where the key is the datastore cluster names that are associated with this host, and the value is the corresponding MORef (Managed Object Reference/ID)
* `network_moref` - A map where the key is the networks in vCenter that are associated with this host and can be used as the network interface for the VSeries nodes. The values are the corresponding MORef(Manager Object Referece/ID)
* `distributed_port_group_moref` - A map where the key is the distributed port group associated with this host, which allows this host to be a member of DVS. The corresponding value is the associated MORef(Mangerd Object Reference/ID)
