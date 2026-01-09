Data Source: gigamon_esxi_hosts

## Example Usage


```hcl
data "gigamon_esxi_hosts" "my-hosts" {
 connection_id = gigamon_esxi_connection.my-connection.id
 data_center_moref = data.gigamon_esxi_datacenter.my-dc.data_center_moref
 cluster_moref = [
   data.gigamon_esxi_cluster.my-cluster.cluster_moref,
 ]
}
```


## Argument Refernece

This data soruce supports the following arguments

* `connection_id` - (Mandatory) specifies the connection to use while fetching the details of the hosts on the associated vSpehere
* `data_center_moref` - (Mandatory) Specifies the data center Moref (vSpehere ID) for the datacenter for which we are getting the host details

## Attribute Reference

This data source exports the following attributes in addition to the arguments above

* id - (Mandatory) Specifies the internal ID which is used by FM
