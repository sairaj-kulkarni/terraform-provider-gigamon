## Resources: `gigamon_tunnel_out` and `gigamon_tunnel_in`

**Tunnels** are ingress/egress endpoints inside a **Monitoring Session** that carry monitored traffic to or from external peers.  
They are always defined **within a single Monitoring Session**, and can be linked to Maps / Applications / other endpoints via `gigamon_link`.

- `gigamon_tunnel_out` – egress tunnel endpoint (traffic_direction = `out`)
- `gigamon_tunnel_in` – ingress tunnel endpoint (traffic_direction = `in`)

Exactly one tunnel **type block** must be configured per resource (L2GRE, VXLAN, Geneve, ERSPAN, TLS-PCAPNG, UDP – depending on direction).

---

**Tunnel types that appear in the implementation:**

**Supported / exposed in Terraform**

- **`l2gre`**       – L2GRE tunnel (ingress + egress)
- **`vxlan`**       – VXLAN tunnel (ingress + egress)
- **`geneve`**      – Geneve tunnel (**ingress only**)
- **`erspan`**      – ERSPAN tunnel (**ingress only**)
- **`tlspcapng`**   – TLS-PCAPNG tunnel (TLS-based, exposed as `tls_pcapng` block; ingress + egress)
- **`udp`** – UDP tunnel (**egress only**: see constraints below)

**Mentioned but *not supported* in this provider**

- **`udpgre`** – GRE over UDP  
  - Not exposed in the schema (code is commented out).  
  - Requires a **PCAPNG application** on the FM side; that application is **not in scope / not implemented / not a current priority** for this provider.  
  - As a result, **`udpgre` tunnels are not supported**.


---

## Example Usage

### Egress L2GRE tunnel

```hcl
resource "gigamon_tunnel_out" "l2gre_out" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "l2gre-to-tools"

  description = "L2GRE egress tunnel to tools VPC"
  remote_ip   = "10.114.154.4"   # peer IP

  # Optional QoS
  dscp       = 0
  prec       = 0
  flow_label = 0

  l2gre {
    # GRE key (0–4294967295). Defaults to 0 if omitted.
    key = 1234
  }
}
```

### Egress VXLAN tunnel

```hcl
resource "gigamon_tunnel_out" "vxlan_out" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "vxlan-out"

  description = "VXLAN egress tunnel"
  remote_ip   = "192.0.2.10"

  vxlan {
    vni             = 5000        # 1–16777215
    destination_port = 4789       # standard VXLAN UDP port
  }
}
```

### Egress TLS-PCAPNG tunnel (TLS out-tunnel)

```hcl
resource "gigamon_tunnel_out" "tls_pcapng_out" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "tls-pcapng-out"

  description = "TLS PCAPNG egress tunnel"
  remote_ip   = "10.114.154.4"

  tls_pcapng {
    # mTLS is optional; defaults to disabled if not set
    enable_mtls = false

    # Optional TCP tuning and ports
    source_port      = 44300
    destination_port = 1          # required by FM for TLS-PCAPNG out-tunnels
  }
}
```

### Ingress VXLAN tunnel

```hcl
resource "gigamon_tunnel_in" "vxlan_in" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "vxlan-in"

  description = "Ingress VXLAN tunnel from remote site"

  # For many ingress types, remote_ip is computed by Fabric Manager.
  # You usually do not need to set it explicitly.

  vxlan {
    vni             = 5000
    destination_port = 4789
  }
}
```

### Ingress TLS-PCAPNG tunnel (TLS in-tunnel)

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

### Common Arguments (both `gigamon_tunnel_out` and `gigamon_tunnel_in`)

- **monitoring_session_id** (String)  
    - Monitoring Session in which this tunnel endpoint is configured.  
    - Typically set from `gigamon_monitoring_session.<name>.id`.  
    - Changing this forces a new resource to be created.

- **alias** (String)  
    - Alias / name for this tunnel endpoint, unique within the Monitoring Session.  
    - Must be non-empty.

- **type** (String)  
    - **Computed** tunnel type derived from the configured block.  
    - Possible values (depending on direction):  
            - `l2gre`, `vxlan`, `geneve`, `erspan`, `tlspcapng`, `udp`.  
    - Users do **not** set this directly.

- **traffic_direction** (String)  
    - **Computed** traffic direction:
            - `gigamon_tunnel_out`: always `"out"`.
            - `gigamon_tunnel_in`: always `"in"`.  
    - Users do not set this field.

- **description** (String)  
    - Optional free-form description for the tunnel.

---

### Egress-only Arguments (`gigamon_tunnel_out`)

- **remote_ip** (String)  
    - **Required** for egress tunnels.  
    - Remote peer IP address (IPv4 or IPv6 literal).  
    - Validated to be a syntactically correct IP address.  
    - Used to derive `ipVersion` in FM (IPV4/IPV6).

- **mtu** (Number)  
    - Egress tunnel MTU in bytes.  
    - Range: **1280–9600**.  
    - Optional, **Computed with default 1500**.

- **ttl** (Number)  
    - Outer IP TTL for this egress tunnel.  
    - Range: **1–255**.  
    - Optional, **Computed with default 64**.

- **dscp** (Number)  
    - Outer IP DSCP value.  
    - Range: **0–63**.  
    - Optional, **Computed with default 0**.

- **prec** (Number)  
    - Outer IP precedence.  
    - Range: **0–7**.  
    - Optional, **Computed with default 0**.

- **flow_label** (Number)  
    - IPv6 flow label.  
    - Range: **0–1048575**.  
    - Optional, **Computed with default 0**.

---

### Ingress-only Arguments (`gigamon_tunnel_in`)

- **remote_ip** (String)  
    - Optional, **Computed** by Fabric Manager for many ingress types.  
    - Remote peer IP address for the ingress tunnel (if applicable).  
    - Must be a valid IPv4 or IPv6 literal when set.

---

## Tunnel Type Blocks

Exactly **one** of the following blocks must be configured for each tunnel resource.  
This is enforced by the provider using `ExactlyOneOf` validators.

### L2GRE (`l2gre` block)

Supported on **ingress and egress** tunnels.

```hcl
l2gre {
  key = 1234
}
```

- **key** (Number)  
    - L2GRE key.  
    - Egress: defaults to 0 if omitted.  
    - Range: **0–4294967295** (validated).  
    - Optional; if non-zero on FM, read back into state.

---

### VXLAN (`vxlan` block)

Supported on **ingress and egress** tunnels.

```hcl
vxlan {
  vni             = 5000
  destination_port = 4789
}
```

- **vni** (Number)  
    - VXLAN Network Identifier.  
    - Range: **1–16777215**.  
    - Optional in schema, but typically required for a functional VXLAN tunnel.  

- **destination_port** (Number)  
    - Destination UDP port for this VXLAN tunnel.  
    - Range: **1–65535**.  
    - Optional in schema; if set in FM, it is read back into state.

On ingress, `destination_port` is always read from FM; on egress, it is sent only when configured.

---

### Geneve (`geneve` block, ingress only)

```hcl
geneve {
  vni             = 100
  destination_port = 6081
}
```

- **vni** (Number)  
    - Geneve VNI (1–16777215).  

- **destination_port** (Number)  
    - Destination UDP port (1–65535).  

Available only for `gigamon_tunnel_in`.

---

### ERSPAN (`erspan` block, ingress only)

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

### UDP (`udp` block, egress only)

```hcl
udp {
  source_port      = 50000
  destination_port = 50001
}
```

- **source_port** (Number)  
    - Source L4 port for this UDP egress tunnel.  
    - Range: **1–65535**.  

- **destination_port** (Number)  
    - Destination L4 port for this UDP egress tunnel.  
    - Range: **1–65535**.  

Available only for `gigamon_tunnel_out`.

> **Platform constraint:**  
> UDP tunnels are only valid in conjunction with an **AMI application** in the same Monitoring Session, and there must be a **`gigamon_link`** between the AMI application and this UDP tunnel. Without that AMI–UDP pairing and link, a UDP tunnel configuration does not make sense and is not a supported topology.

---

### TLS-PCAPNG (`tls_pcapng` block)

TLS-PCAPNG tunnels model the **TLS encapsulated PCAPNG** stream (TCP-based).  
Exposed as `tlspcapng` in FM, but configured via the `tls_pcapng` block in Terraform.

#### Common TLS-PCAPNG fields (ingress and egress)

```hcl
tls_pcapng {
  enable_mtls     = true
  tls_cipher      = "TLS_AES_128_GCM_SHA256"
  tls_version     = "TLS1.3"
  tls_sack        = "enable"
  tls_syn_retries = 3
  tls_delay_ack   = "enable"

  source_port      = 44300
  destination_port = 1
}
```

- **enable_mtls** (Boolean)  
    - Whether to enable mutual TLS.  
    - Optional, **Computed**; if unset, FM defaults are used.  
    - Provider maps this to FM `mtls = "enable"|"disable"`.

- **tls_cipher** (String)  
    - Cipher suite label for this TLS-PCAPNG tunnel.  
    - Optional, **Computed**; FM typically defaults to `TLS_AES_128_GCM_SHA256` (SHA-256 based cipher).

- **tls_version** (String)  
    - TLS version label (e.g., `TLS1.3`).  
    - Optional, **Computed**; FM typically defaults to `TLS1.3`.

- **tls_sack** (String)  
    - Selective ACK state: `"enable"` or `"disable"`.  
    - Optional, **Computed**.

- **tls_syn_retries** (Number)  
    - SYN retry count.  
    - Range: **1–6**.  
    - Optional, **Computed**; default comes from FM.

- **tls_delay_ack** (String)  
    - Delay ACK state: `"enable"` or `"disable"`.  
    - Optional, **Computed**.

- **source_port** (Number)  
    - Source TCP port for this TLS-PCAPNG tunnel.  
    - Range: **1–65535**.  
    - Optional; egress/ingress may treat this as computed when omitted.

- **destination_port** (Number)  
    - Destination TCP port for this TLS-PCAPNG tunnel.  
    - Range: **1–65535**.  
    - Required by FM for valid TLS-PCAPNG tunnels (typically set to `1` in Gigamon TLS-PCAPNG flows).  

#### Egress TLS-PCAPNG (`gigamon_tunnel_out`)

```hcl
tls_pcapng {
  enable_mtls     = false
  source_port      = 44300
  destination_port = 1
}
```

- **tls_key_alias** is **not allowed** on egress TLS-PCAPNG tunnels.  
    - If set, the provider fails plan/apply with an error:
        - *"tls_key_alias is only valid for ingress TLS-PCAPNG tunnels (gigamon_tunnel_in)"*.

#### Ingress TLS-PCAPNG (`gigamon_tunnel_in`)

```hcl
tls_pcapng {
  enable_mtls   = true
  tls_key_alias = "tls-pcapng-in-key"

  source_port      = 44300
  destination_port = 1
}
```

- **tls_key_alias** (String)  
    - Key alias for this TLS-PCAPNG ingress tunnel.  
    - Required to bind the ingress tunnel to a TLS key in the platform.  
    - Only valid for `gigamon_tunnel_in`; not permitted in egress.

All other TLS fields (cipher, version, ACK behavior, retries) can be left unset to accept platform defaults (e.g., **TLS1.3**, **TLS_AES_128_GCM_SHA256**, SACK/Delay-ACK enabled, SYN retries 3, etc., per FM configuration).

---

## Attributes Reference

### `gigamon_tunnel_out`

In addition to the arguments above, this resource exports:

- **id** (String)  
    - **Typed tunnel ID** assigned by the provider (wrapping the FM tunnel UUID).  
    - Used for linking (e.g., from `gigamon_link`) and for identify/update/delete.  
    - Users never construct this manually; it is always taken from resource state/outputs.

- **type** (String)  
    - Tunnel type inferred from the active block (`l2gre`, `vxlan`, `udp`, `tlspcapng`, …).  
    - Computed; not user-settable.

- **traffic_direction** (String)  
    - Always `"out"`.

Other fields (`remote_ip`, `mtu`, `ttl`, `dscp`, `prec`, `flow_label`, and nested block fields) are reflected from FM on read and may include defaults chosen by FM.

### `gigamon_tunnel_in`

In addition to its arguments, this resource exports:

- **id** (String)  
    - Typed ingress tunnel ID assigned by the provider (wrapping the FM tunnel UUID).  

- **type** (String)  
    - Inferred tunnel type from the configured block.

- **traffic_direction** (String)  
    - Always `"in"`.

- **remote_ip** (String)  
    - Computed from FM for many ingress tunnel types.

Nested block fields (L2GRE/VXLAN/Geneve/ERSPAN/TLS-PCAPNG) are read back from FM, including defaults such as TLS parameters, VNI, ports, etc.

---

## Behavior and Lifecycle

- **Tunnels exist inside a Monitoring Session**  
    - `monitoring_session_id` must reference a valid `gigamon_monitoring_session` in the same provider configuration.
    - The provider updates the Monitoring Session via Fabric Manager APIs (create/update/delete operations on the `tunnels[]` array).

- **Type selection**  
    - Exactly one of the tunnel-type blocks must be present:
            - Egress: `l2gre`, `vxlan`, `tls_pcapng`, `udp`.  
            - Ingress: `l2gre`, `vxlan`, `geneve`, `erspan`, `tls_pcapng`.  
    - If none or more than one block is specified, plan/apply fails.

- **IP version derivation (egress)**  
    - For `gigamon_tunnel_out`, `ipVersion` is derived from `remote_ip`:
            - IPv4 literal → `IPV4`
            - IPv6 literal → `IPV6`

- **TLS-PCAPNG semantics**  
    - Egress: `tls_key_alias` is forbidden; mTLS is optional and defaults to disabled if unspecified.  
    - Ingress: `tls_key_alias` is required for TLS-PCAPNG tunnels; mTLS and other fields may inherit defaults from FM.  

- **Updates**  
    - Both ingress and egress tunnels support in-place updates.  
    - On update, the provider:
            - Builds an FM tunnel object from the planned state.  
            - Issues an `update` operation to the Monitoring Session.  
            - Keeps the existing `id` (typed tunnel ID).  

- **Read / Drift handling**  
    - On each read, the provider fetches the Monitoring Session and locates the tunnel by `id` (and/or alias).  
    - If the Monitoring Session or tunnel no longer exists in FM, the resource is removed from state (idempotent behavior).  
    - Computed fields (including TLS defaults, VNI/ports, QoS fields) are refreshed from FM.

- **Delete**  
    - On delete, the provider sends a `tunnel` delete operation (using the decoded FM UUID and tunnel type).  
    - If FM reports the tunnel as missing, the delete is treated as successful (idempotent).

---

## Import

Import is **not yet supported** for `gigamon_tunnel_out` or `gigamon_tunnel_in`.