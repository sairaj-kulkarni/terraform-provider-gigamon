## Resource: gigamon_esxi_connection

Manages a Vcenter instance. FM maintains an inventory of the asscoiated Vcenter and manages the Vseries Node fabric that is deployed on this Vcenter.

## Example Usage

```hcl
resource "gigamon_esxi_monitoring_domain" "my-md" {
  alias = "my-esxi-md"
}
```

```hcl
resource "gigamon_esxi_connection" "my-conn" {
  alias = "my-conn"
  monitoring_domain_id = gigamon_esxu_monitoring_domain.my-md.id
  vcenter_address = "production-vcenter.company.com"
  username = "admin@company.com"
  password = "myPassword#"
  maximum_nodes_per_host = 5
  tapping_method = "platform"
}
```

## Argument Refernece

The arguments supported by this provder are

* `alias` - (Required) user provided name for this connection
* `monitoring_domain_id` - (Required) The monitoring domain to which this connection is attached to.
* `vcenter_address` - (Required) The Vcenter IP address / FQDN Name
* `username` - (Required) User name which is used to login and communicate with Vcenter
* `password` - (Required) Password of the above user, for communication with Vcenter
* `resource_allocation`- (Optional) Used to detemine how the traffic is distributed to the Vseries Nodes. Can be one of
    * `TargetVMBased` - In this case, the target VMs of that host are dsitributed to the Vseries node in the same host based on the count of targetVMs in an uniform manner. This can be used when the host has less than 8 VSS or VDS associated with it
    * `SwithcBased` - In this case, the target VMs are distributed to the VSeries nodes based on the switch they are connected to. In case a host has more than 8 VSS or VDS, than we should use this method as a Vseries node can at the most tap from 8 VSS or VDS
    * `none` - This is used when the tapping_method is set to none i.e. for customer orchestrated sources
        * Default is set to "TargetVMBased"
* `tapping_method` - (Optional) Used to determine how the customer traffic is tapped and sent to Vseries Nodes
    * `platform` - In this method, FM uses port mirroring and manages the port mirroring session creation/deletion to capture traffic from workload VMs
    * `none` - In this method, the customer is expected to feed the traffic directly to the VSeries nodes using tunnels and setting up the Monitoring Session on Vseries nodes to use the incoming tunnel specification
        * If none is chosen for tapping_method, the resource_allocation field must also be set to none
* `maximum_nodes_per_host` - (Optional) allows the user to specify the maximum number of Vseries nodes that will be bought up in a single host. The default is 1, and the user can choose between 1 and 10
* `timeout` - (Optional) Specifies the amount of time in seconds, to wait for the connection to move to "connected" state. Default is 60 seconds, can be set to a value between 30 and 36000

## Attribute Reference

This resource exposes the following attributes in addition to the above arguments

* `id` - identifies this vCetner connection instance
* `status` - Status of the connection, will be one of "connected", "connFailed" and "notConnected". A connection which is in connected status indicates that the connection to vCenter was established and the inventory was collected from vCenter. The connection has to move to connected state before the data sources or other resources depending on this connection can be executed.
