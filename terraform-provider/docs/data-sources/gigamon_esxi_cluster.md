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

## Data Source: gigamon_esxi_cluster

This provides the details of a cluster that is available in the VCenter being managed by FM in this connection

## Example Usage


```hcl
data "gigamon_esxi_cluster" "my-cluster-1" {
 connection_id = gigamon_esxi_connection.my-connection.id
 data_center_moref = data.gigamon_esxi_datacenter.my-dc.data_center_moref
 cluster_name = "my-cluster-1"
}
```


## Argument Refernece

This data soruce supports the following arguments

* `connection_id` - (Required) specifies the connection to use while fetching the details of the cluster on the associated vCenter
* `data_center_moref` - (Required) Specifies the data center Moref (vSpehere ID) for the datacenter for which we are getting the cluster details

## Attribute Reference

This data source exports the following attributes in addition to the arguments above

* `cluster_moref` - Cluster MORef (Managed Object reference/ID) by which this cluster is uniquely identified within that vCenter
