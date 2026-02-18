## Data Source: gigamon_esxi_datacenter

This data source provides details of the datacenter managed by the vCenter associated with this connection

## Example Usage


```hcl
data "gigamon_esxi_datacenter" "my-dc" {
  connection_id = gigamon_esxi_connection.my-connection.id
  data_center_name = "my-datacenter-1"
}
```


## Argument Refernece

This data soruce supports the following arguments

* `connection_id` - (Required) Connection ID to use. This determines the vCenter instance that is being used for the query
* `data_center_moref` - (Required) Name of the datacenter for which we need the details

## Attribute Reference

This data source exports the following attributes in addition to the arguments above

* `data_center_moref` - Provides the MORef(Managed Object Reference/ID) of the datacenter. This is an unique ID by whcih this datacenter is managed within that vCenter
