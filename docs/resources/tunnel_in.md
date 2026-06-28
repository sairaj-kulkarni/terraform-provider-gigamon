---
page_title: "Inbound Tunnel"
subcategory: "Tunnels and Raw Endpoints"
description: "Manage inbound tunnels in Gigamon FM."
---

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

## Resource: `gigamon_tunnel_in`

A **tunnel in** is an **ingress tunnel endpoint** inside a **Monitoring Session** that receives monitored traffic from external peers into Gigamon (for example, from remote sites, ERSPAN sources, TLS-PCAPNG senders).  
It always belongs to a **single Monitoring Session** and can be linked to Maps / Applications / other endpoints via `gigamon_link`.

- `gigamon_tunnel_in` represents the **ingress endpoint** (`traffic_direction = "in"`).
- Exactly **one tunnel type block** must be configured per resource.

Supported tunnel types for `gigamon_tunnel_in`:

- `l2gre`
- `vxlan`
- `geneve`
- `erspan`
- `tlspcapng` (TLS-PCAPNG, configured via `tls_pcapng` block)

> `udp` and `udpgre` are **not supported** for ingress tunnels in this provider.  
> `udpgre` (GRE over UDP) and would require a PCAPNG application that is not in scope.

---

## Example Usage

### L2GRE ingress tunnel

```hcl
resource "gigamon_tunnel_in" "l2gre_in" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "l2gre-from-branch"

  description = "L2GRE ingress tunnel from branch site"

  l2gre {
    # GRE key (0–4294967295). Defaults to 0 if omitted.
    key = 1234
  }
}
```

### VXLAN ingress tunnel

```hcl
resource "gigamon_tunnel_in" "vxlan_in" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "vxlan-in"

  description = "Ingress VXLAN tunnel from remote site"

  # For many ingress types, remote_ip is computed by Fabric Manager.
  # You usually do not need to set it explicitly.

  vxlan {
    vni              = 5000
    destination_port = 4789
  }
}
```

### Geneve ingress tunnel

```hcl
resource "gigamon_tunnel_in" "geneve_in" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "geneve-in"

  description = "Ingress Geneve tunnel"

  geneve {
    vni              = 100
    destination_port = 6081
  }
}
```

### ERSPAN ingress tunnel

```hcl
resource "gigamon_tunnel_in" "erspan_in" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "erspan-in"

  description = "Ingress ERSPAN tunnel"

  erspan {
    flow_id = 10
  }
}
```

### TLS-PCAPNG ingress tunnel (TLS in-tunnel)

```hcl
resource "gigamon_tunnel_in" "tls_pcapng_in" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "tls-pcapng-in"

  description = "TLS PCAPNG ingress tunnel"

  tls_pcapng {
    # mTLS enable/disable (optional, defaults from FM)
    enable_mtls = true

    # Required: key alias for this TLS tunnel on ingress
    tls_key_alias = "tls-pcapng-in-key"

    # Other fields are usually left to FM defaults:
    # tls_cipher, tls_version, tls_sack, tls_syn_retries, tls_delay_ack

    source_port      = 44300
    destination_port = 1
  }
}
```

---

## Argument Reference

### Required

- **monitoring_session_id** (String)  
    - Monitoring Session in which this ingress tunnel endpoint is configured.  
    - Typically set from `gigamon_monitoring_session.<name>.id`.  
    - Changing this forces a new `gigamon_tunnel_in` to be created.

- **alias** (String)  
    - Alias / name for this ingress tunnel endpoint, unique within the Monitoring Session.  
    - Must be non-empty.

- **(one type block)**  
    - Exactly one of the following must be configured:
        - `l2gre { ... }`
        - `vxlan { ... }`
        - `geneve { ... }`
        - `erspan { ... }`
        - `tls_pcapng { ... }`

### Optional (ingress-specific and common)

- **description** (String)  
    - Optional free-form description for this tunnel.

- **remote_ip** (String)  
    - Optional, computed by Fabric Manager for many ingress types.  
    - Remote peer IP address for the ingress tunnel (if applicable).  
    - Must be a valid IPv4 or IPv6 literal when set.

---

## Tunnel Type Blocks (ingress)

Exactly **one** of the following blocks must be set on `gigamon_tunnel_in`.

### L2GRE (`l2gre`)

```hcl
l2gre {
  key = 1234
}
```

- **key** (Number)  
    - L2GRE key.  
    - Range: **0–4294967295**.  
    - Optional; defaults to **0** if omitted.  
    - If FM reports a non-zero key, it is read back into state.

---

### VXLAN (`vxlan`)

```hcl
vxlan {
  vni              = 5000
  destination_port = 4789
}
```

- **vni** (Number)  
    - VXLAN Network Identifier.  
    - Range: **1–16777215**.

- **destination_port** (Number)  
    - Destination UDP port for this VXLAN tunnel.  
    - Range: **1–65535**.

On ingress, `destination_port` is always read back from FM when present.

---

### Geneve (`geneve`)

```hcl
geneve {
  vni              = 100
  destination_port = 6081
}
```

- **vni** (Number)  
    - Geneve VNI.  
    - Range: **1–16777215**.

- **destination_port** (Number)  
    - Destination UDP port.  
    - Range: **1–65535**.

Available only for `gigamon_tunnel_in`.

---

### ERSPAN (`erspan`)

```hcl
erspan {
  flow_id = 10
}
```

- **flow_id** (Number)  
    - ERSPAN Flow ID.  
    - Range: **1–1023**.

Available only for `gigamon_tunnel_in`.

---

### TLS-PCAPNG (`tls_pcapng`)

TLS-PCAPNG tunnels model a **TLS-encapsulated PCAPNG** stream over TCP. They appear in FM as `tlspcapng` but are configured via the `tls_pcapng` block in Terraform.

#### Fields

```hcl
tls_pcapng {
  enable_mtls     = true
  tls_cipher      = "TLS_AES_128_GCM_SHA256"
  tls_version     = "TLS1.3"
  tls_sack        = "enable"
  tls_syn_retries = 3
  tls_delay_ack   = "enable"

  tls_key_alias   = "tls-pcapng-in-key"

  source_port      = 44300
  destination_port = 1
}
```

- **enable_mtls** (Boolean)  
    - Enable or disable mutual TLS.  
    - Optional, computed; if unset, FM defaults are used.  
    - Mapped to FM `mtls = "enable" | "disable"`.

- **tls_cipher** (String)  
    - Cipher suite label.  
    - Optional, computed; FM typically defaults to `TLS_AES_128_GCM_SHA256`.

- **tls_version** (String)  
    - TLS version label (for example, `TLS1.3`).  
    - Optional, computed; FM typically defaults to `TLS1.3`.

- **tls_sack** (String)  
    - Selective ACK state: `"enable"` or `"disable"`.  
    - Optional, computed.

- **tls_syn_retries** (Number)  
    - SYN retry count.  
    - Range: **1–6**.  
    - Optional, computed; FM default is used if not set.

- **tls_delay_ack** (String)  
    - Delay ACK state: `"enable"` or `"disable"`.  
    - Optional, computed.

- **tls_key_alias** (String)  
    - Key alias for this TLS-PCAPNG ingress tunnel.  
    - Used to bind the ingress tunnel to a TLS key in the platform.  
    - **Valid only on `gigamon_tunnel_in`**; not allowed on `gigamon_tunnel_out`.

- **source_port** (Number)  
    - Source TCP port for this TLS-PCAPNG tunnel.  
    - Range: **1–65535**.

- **destination_port** (Number)  
    - Destination TCP port for this TLS-PCAPNG tunnel.  
    - Range: **1–65535**.  
    - Required by FM for valid TLS-PCAPNG tunnels (commonly set to `1`).

---

## Attributes Reference

In addition to the arguments above, `gigamon_tunnel_in` exports:

- **id** (String)  
    - **Typed ingress tunnel ID** assigned by the provider (wrapping the FM tunnel UUID).  
    - Used for linking (for example, from `gigamon_link`) and for update/delete.  
    - Users never construct this ID manually.

- **type** (String)  
    - Computed tunnel type inferred from the active block.  
    - Possible values: `l2gre`, `vxlan`, `geneve`, `erspan`, `tlspcapng`.

- **traffic_direction** (String)  
    - Always `"in"` for this resource.

- **remote_ip** (String)  
    - Remote peer IP address for this ingress tunnel (if applicable).  
    - Often **computed** by FM; may be empty in config and filled on read.

- Nested block fields (L2GRE / VXLAN / Geneve / ERSPAN / TLS-PCAPNG)  
    - Reflected from FM on read and may include FM defaults (for example, TLS defaults, VNI, ports).

---

## Behavior and Lifecycle

- **Monitoring Session scope**  
    - `gigamon_tunnel_in` always belongs to a single Monitoring Session (`monitoring_session_id`).  
    - The provider updates the Monitoring Session’s `tunnels[]` array via Fabric Manager APIs.

- **Type selection**  
    - Exactly one of `l2gre`, `vxlan`, `geneve`, `erspan`, or `tls_pcapng` must be present.  
    - If none or more than one block is specified, plan/apply fails.

- **Create / Update**  
    - On create, the provider builds an FM tunnel object from the plan, calls Monitoring Session update with `"create"`, and stores a typed tunnel ID.  
    - On update, it decodes the existing typed ID to a raw UUID and issues an `"update"` operation, keeping the same typed ID in state.

- **Read / Drift handling**  
    - On read, the provider fetches the Monitoring Session and locates the tunnel by ID (and optionally alias).  
    - If the Monitoring Session or tunnel is missing in FM, the resource is removed from state (idempotent behavior).  
    - Computed fields (including TLS defaults, VNI/ports) are refreshed from FM.

- **Delete**  
    - On delete, the provider sends a `"delete"` operation for the tunnel, using the decoded FM UUID and type.  
    - If FM reports the tunnel as not found, deletion is treated as successful.

---

## Import

Import is **not yet supported** for `gigamon_tunnel_in`.
