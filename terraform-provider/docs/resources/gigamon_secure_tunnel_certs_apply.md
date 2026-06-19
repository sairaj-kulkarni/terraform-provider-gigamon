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

## Resource: `gigamon_secure_tunnel_certs_apply`

The Secure Tunnel Certs Apply resource applies previously uploaded Cloud CA Certificates and Cloud SSL Keys to one or more Monitoring Domains in Gigamon Fabric Manager.

Create and Update operations push certificates to Monitoring Domains. This resource is apply‑only. Read and Delete operations do not perform any remote action.

This resource is typically used together with `gigamon_cloud_ca_cert` and `gigamon_cloud_ssl_keys`.


## Example Usage

### Apply Secure Tunnel Certificates to Monitoring Domains (UCTV → VSN)

```hcl
resource "gigamon_secure_tunnel_certs_apply" "certs_apply" {
  monitoring_domain_ids = [
    gigamon_third_party_orchestration_monitoring_domain.terraform_md.id
  ]

  uctv_ca_cert_alias = gigamon_cloud_ca_cert.ca_cert.alias
  vsn_ssl_key_alias  = gigamon_cloud_ssl_keys.ssl_keys.alias
  key_store_alias    = gigamon_cloud_ssl_keys.ssl_keys.key_store_alias
}
```

### Apply Secure Tunnel Certificates to Monitoring Domains (VSN → VSN)

```hcl
resource "gigamon_secure_tunnel_certs_apply" "certs_apply" {
  monitoring_domain_ids = [
    gigamon_third_party_orchestration_monitoring_domain.terraform_md.id
  ]

  vsn_ssl_key_alias  = gigamon_cloud_ssl_keys.ssl_keys.alias
  key_store_alias    = gigamon_cloud_ssl_keys.ssl_keys.key_store_alias
}
```

## Argument Reference (User‑provided)

### Required
* `monitoring_domain_ids (Set of String)`  - Set of Monitoring Domain IDs to which the Secure Tunnel Certificates will be applied. At least one Monitoring Domain ID must be specified.
* `vsn_ssl_key_alias (String)`             - Alias of the Cloud SSL Keys to apply.
* `key_store_alias (String)`               - Key store alias where the SSL keys are stored in Fabric Manager. This must match the key store alias used when the SSL keys were created.

### Optional
* `uctv_ca_cert_alias (String)`            - Alias of the Cloud CA Certificate to apply. Required only for UCTV → VSN secure tunnel use cases.


## Note
* For UCTV → VSN tunnels, all attributes must be provided.
* For VSN → VSN tunnels, uctv_ca_cert_alias can be omitted.
* Monitoring Sessions that enable Secure Tunnels should be created after this resource is applied on Monitoring Domains, typically using `depends_on`.