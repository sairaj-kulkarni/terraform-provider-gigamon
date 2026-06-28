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
page_title: "Cloud SSL Keys"
subcategory: "Certificates"
description: "Manage Cloud SSL keys in Gigamon FM."
---

## Resource: `gigamon_cloud_ssl_keys`

Cloud SSL Keys represent a private key and certificate pair uploaded to Gigamon Fabric Manager.
These keys are later referenced when configuring secure tunnels.

The private key and certificate files are write‑only inputs, while Fabric Manager exposes read‑only SSL key metadata.
Cloud SSL Keys are typically referenced by the resource, `gigamon_secure_tunnel_certs_apply`.


## Example Usage

### Create Cloud SSL Keys

```hcl
resource "gigamon_cloud_ssl_keys" "ssl_keys" {
  alias             = "SSL_KEYS"
  key_store_alias   = "DEFAULT_CLOUD_SSL_KS"
  private_key_path  = "/path/to/SSL.key"
  certificate_path  = "/path/to/SSL.crt"
}
```

## Argument Reference (User‑provided)

### Required
* `alias (String)`                - User‑defined alias for the Cloud SSL Keys entry in Fabric Manager.

### Optional
* `key_store_alias (String)`      - Key store alias where the SSL keys are stored in Fabric Manager. Default key store alias `DEFAULT_CLOUD_SSL_KS` can be used. This value is required during creation.
* `private_key_path (String)`     - Path to the private key file (.key) to upload. This value is required during creation or replacement, but is not stored in Terraform state.
* `certificate_path (String)`     - Path to the certificate file (.crt) to upload. This value is required during creation or replacement, but is not stored in Terraform state.


## Attributes Reference (Read-only)

In addition to the arguments above, this resource exposes the following computed attributes.
These values represent SSL key metadata returned by Fabric Manager after upload.

* `common_name (String)`   - Common Name (CN) from the certificate.
* `organization (String)`  - Organization (O) from the certificate subject.
* `expiry (String)`        - Certificate expiration date.
* `key_type (String)`      - Key type (for example, rsa).
* `certificate (Bool)`     - Indicates whether a certificate is present.
* `private_key (Bool)`     - Indicates whether a private key is present.
* `installed_on (String)`  - Timestamp when the SSL keys were installed in Fabric Manager.


## Import

This resource supports Terraform configuration-driven imports using the `import` block. The import `id` is the SSL Key `alias`.

```hcl
import {
  to = gigamon_cloud_ssl_keys.ssl_keys
  id = "SSL_KEYS"
}
```

* `to`  - Must match an existing resource address in your configuration.
* `id`  - The SSL Key  alias in Fabric Manager.

### Note
* `private_key_path` and `certificate_path` are not required for import.
* `key_store_alias` is populated automatically from Fabric Manager during read or import.
