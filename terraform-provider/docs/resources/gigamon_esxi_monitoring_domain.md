Resource: gigamon_esxi_monitoring_domain

## Example Usage

```hcl
resource "gigamon_esxi_monitoring_domain" "my=md" {
 alias = "customerProvidedName'
}
```

## Argument Refernece

The arguments supported by this provder are

* `alias` - (Mandatory) user provided alias (or handle for this Monitoring Domain)

## Attribute Reference

This resource exposes the following attributes in addition to the above arguments

* `id` - identifies this monitoring domain instance
