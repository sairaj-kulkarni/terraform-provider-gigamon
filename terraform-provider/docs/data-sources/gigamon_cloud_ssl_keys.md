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

## Data Source: `gigamon_cloud_ssl_keys`

Use this data source to look up existing Cloud SSL Keys (private key and certificate pair) that were uploaded outside Terraform (for example, in the Fabric Manager UI or other automation).
This data source is read‑only and is typically referenced by the resource `gigamon_secure_tunnel_certs_apply`, in secure tunnel configuration.


## Example Usage

### Look up Cloud SSL Keys

```hcl
data "gigamon_cloud_ssl_keys" "ssl_keys" {
  alias = "VSN_SSL_KEYS"
}
```

Use the data source in dependent resources, for instance:

```hcl
resource "gigamon_secure_tunnel_certs_apply" "certs_apply" {
  vsn_ssl_key_alias  = data.gigamon_cloud_ssl_keys.ssl_keys.alias
  #...
}
```


## Argument Reference (User‑provided)

### Required
* `alias (String)` - Alias of the Cloud SSL Keys entry in Fabric Manager. Must match an existing SSL Keys alias.


## Attributes Reference (Read-only)

In addition to the arguments above, this data source exposes the following computed attributes.

* `common_name (String)`   - Common Name (CN) from the certificate.
* `organization (String)`  - Organization (O) from the certificate subject.
* `expiry (String)`        - Certificate expiration date.
* `key_type (String)`      - Key type (for example, rsa).
* `certificate (Bool)`     - Indicates whether a certificate is present.
* `private_key (Bool)`     - Indicates whether a private key is present.
* `installed_on (String)`  - Timestamp when the SSL keys were installed in Fabric Manager.
