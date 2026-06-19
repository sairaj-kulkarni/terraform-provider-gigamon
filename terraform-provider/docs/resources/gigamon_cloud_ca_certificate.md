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

## Resource: `gigamon_cloud_ca_cert`

A Cloud CA Certificate represents a trusted Certificate Authority (CA) certificate uploaded to Gigamon Fabric Manager.
This certificate is later referenced when configuring secure tunnels.

The CA certificate itself is write‑only, while Fabric Manager exposes read‑only CA certificate metadata.
Cloud CA Certificates are typically referenced by the resource, `gigamon_secure_tunnel_certs_apply`.


## Example Usage

### Create a Cloud CA Certificate

```hcl
resource "gigamon_cloud_ca_cert" "ca_cert" {
  alias            = "CA_CERT"
  certificate_path = "/path/to/CACert.crt"
}
```

## Argument Reference (User‑provided)

### Required
* `alias (String)`                - User‑defined alias for the Cloud CA Certificate in Fabric Manager.

### Optional
* `certificate_path (String)`     - Path to the CA certificate file (.crt) to upload. This value is required during creation or replacement, but is not stored in Terraform state.


## Attributes Reference (Read-only)

In addition to the arguments above, this resource exposes the following computed attributes.
These values represent certificate metadata returned by Fabric Manager after the CA certificate is uploaded.

* `date_not_after (String)`  - Certificate expiration date.
* `date_not_before (String)` - Certificate validity start date.
* `algorithm (String)`       - Certificate signing algorithm.
* `version (Number)`         - X.509 certificate version.
* `issuer (String)`          - Certificate issuer.
* `name (String)`            - Certificate subject name.


## Import

This resource supports Terraform configuration-driven imports using the `import` block. The import `id` is the CA certificate `alias`.

```hcl
import {
  to = gigamon_cloud_ca_cert.ca_cert
  id = "CA_CERT"
}
```

* `to`  - Must match an existing resource address in your configuration.
* `id`  - The CA certificate alias in Fabric Manager.

### Note
* `certificate_path` is not required for import.
