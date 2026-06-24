## Resource: gigamon_esxi_monitoring_domain

Monitoring Domain is a logical object which holds all the managed nodes (Vseries, UCT-V,..) and also the inventory of the customer workloads which are being monitored

## Example Usage

```hcl
resource "gigamon_esxi_monitoring_domain" "my-md" {
 alias = "my-md"
}
```

## Argument Refernece

The arguments supported by this provder are

* `alias` - (Required) user provided alias (or handle for this Monitoring Domain)
* `use_public_ip_for_notifications` - (Optional). In cases where FM is launched on platforms where it gets multiple management address and some of them are for private instance communication and some for public (like being launched in AWS VPC/Openstack), than set this to true, to ensure that FM provides the publicly reachable IP address to the VSeries nodes when they try to send events to FM. Default is false

## Attribute Reference

This resource exposes the following attributes in addition to the above arguments

* `id` - identifies this monitoring domain instance
* `connection_id` - The connection ID associated with this monitoring domain. This will be available after the user creates a connection that is associated with this monitoring domain
* `platform` - The cloud provider platform on which this Monitoring Domain has been created
