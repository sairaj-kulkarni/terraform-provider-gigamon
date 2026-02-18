Resource: gigamon_esxi_connection

## Example Usage

```hcl
resource "gigamon_esxi_connection" "my-conn" {
 alias = "customerProvidedName'
}
```

## Argument Refernece

The arguments supported by this provder are

* `alias` - (Required) user provided name for this connection
* `monitoring_domain_id` - (Required) The monitoring domain to which this connection is attached to.
* `vcenter_address` - (Required) The Vcenter IP address / FQDN Name
* `username` - (Required) User name which is used to login and communicate with Vcenter
* `password` - (Required) Password of the above user, for communication with Vcenter
* `resource_allocation`- (Optional)
* `tapping_method` - (Optional)
* `maximum_nodes_per_host` - (Optional)
* `timeout` - (Optional)

## Attribute Reference

This resource exposes the following attributes in addition to the above arguments

* `id` - identifies this monitoring domain instance
* `status` - Status of the monitoring solutions
