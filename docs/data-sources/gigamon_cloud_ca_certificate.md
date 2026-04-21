## Data Source: `gigamon_cloud_ca_cert`

Use this data source to look up an existing Cloud CA Certificate that was uploaded outside Terraform (for example, in the Fabric Manager UI or other automation).
This data source is read‑only and is referenced by the resource `gigamon_secure_tunnel_certs_apply`, in secure tunnel configuration.


## Example Usage

### Look up a Cloud CA Certificate

```hcl
data "gigamon_cloud_ca_cert" "ca_cert" {
  alias = "UCTV_CA_CERT"
}
```

Use the data source in dependent resources, for instance:

```hcl
resource "gigamon_secure_tunnel_certs_apply" "certs_apply" {
  uctv_ca_cert_alias = data.gigamon_cloud_ca_cert.ca_cert.alias
  #...
}
```


## Argument Reference (User‑provided)

### Required
* `alias (String)` - Alias of the Cloud CA Certificate entry in Fabric Manager. Must match an existing CA certificate alias.


## Attributes Reference (Read-only)

In addition to the arguments above, this data source exposes the following computed attributes.

* `date_not_after (String)`  - Certificate expiration date.
* `date_not_before (String)` - Certificate validity start date.
* `algorithm (String)`       - Certificate signing algorithm.
* `version (Number)`         - X.509 certificate version.
* `issuer (String)`          - Certificate issuer.
* `name (String)`            - Certificate subject name.
