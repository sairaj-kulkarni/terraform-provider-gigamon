## Resource: `gigamon_tunnel_out`

A **tunnel out** is an **egress tunnel endpoint** inside a **Monitoring Session** that carries monitored traffic from Gigamon to an external peer (tool VPC, collector, TLS receiver, AMI, etc.).  
It always belongs to a **single Monitoring Session** and can be linked to Maps / Applications / other endpoints via `gigamon_link`.

- `gigamon_tunnel_out` represents the **egress endpoint** (`traffic_direction = "out"`).
- Exactly **one tunnel type block** must be configured per resource.

Supported tunnel types for `gigamon_tunnel_out`:

- `l2gre`
- `vxlan`
- `tlspcapng` (TLS-PCAPNG, configured via `tls_pcapng` block)
- `udp` (with AMI constraint; see below)

> `udpgre` (GRE over UDP) is **not supported**: it requires a PCAPNG application that is not in scope

---

## Example Usage

### L2GRE egress tunnel

```hcl
resource "gigamon_tunnel_out" "l2gre_out" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "l2gre-to-tools"

  description = "L2GRE egress tunnel to tools VPC"
  remote_ip   = "10.114.154.4" # peer IP

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

### VXLAN egress tunnel

```hcl
resource "gigamon_tunnel_out" "vxlan_out" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "vxlan-out"

  description = "VXLAN egress tunnel"
  remote_ip   = "192.0.2.10"

  vxlan {
    vni              = 5000   # 1–16777215
    destination_port = 4789   # standard VXLAN UDP port
  }
}
```

### TLS-PCAPNG egress tunnel

```hcl
resource "gigamon_tunnel_out" "tls_pcapng_out" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "tls-pcapng-out"

  description = "TLS PCAPNG egress tunnel"
  remote_ip   = "10.114.154.4"

  tls_pcapng {
    # mTLS is optional; defaults to disabled if not set
    enable_mtls = false

    # TCP ports for the TLS-PCAPNG stream
    source_port      = 44300
    destination_port = 1      # required by FM for TLS-PCAPNG out-tunnels
  }
}
```

### UDP egress tunnel (with AMI application)

```hcl
resource "gigamon_app_ami" "ami_app" {
  # AMI application configuration (not shown here)
}

resource "gigamon_tunnel_out" "udp_out" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "ami-udp-out"

  description = "UDP egress tunnel for AMI"
  remote_ip   = "198.51.100.10"

  udp {
    source_port      = 50000
    destination_port = 50001
  }
}

resource "gigamon_link" "ami_to_udp" {
  monitoring_session_id = gigamon_monitoring_session.ms.id

  source_id = gigamon_app_ami.ami_app.id
  dest_id   = gigamon_tunnel_out.udp_out.id
}
```

> **Platform constraint:**  
> `udp` tunnels are only meaningful when used together with an **AMI application** in the same Monitoring Session, and there must be a **`gigamon_link`** between the AMI application and the UDP tunnel. A standalone UDP tunnel without this AMI–UDP pairing and link is not a supported topology.

---

## Argument Reference

### Required

- **monitoring_session_id** (String)  
    - Monitoring Session in which this egress tunnel endpoint is configured.  
    - Typically set from `gigamon_monitoring_session.<name>.id`.  
    - Changing this forces a new `gigamon_tunnel_out` to be created.

- **alias** (String)  
    - Alias / name for this egress tunnel endpoint, unique within the Monitoring Session.  
    - Must be non-empty.

- **remote_ip** (String)  
    - Remote peer IP address for this egress tunnel.  
    - Must be a valid IPv4 or IPv6 literal.  
    - Used by the provider to derive `ipVersion` in FM (`IPV4` / `IPV6`).

- **(one type block)**  
    - Exactly one of the following must be configured:
        - `l2gre { ... }`
        - `vxlan { ... }`
        - `tls_pcapng { ... }`
        - `udp { ... }`

### Optional (egress-specific and common)

- **description** (String)  
    - Optional free-form description for this tunnel.

- **mtu** (Number)  
    - Egress tunnel MTU in bytes.  
    - Range: **1280–9600**.  
    - Optional, computed with default **1500**.

- **ttl** (Number)  
    - Outer IP TTL for this egress tunnel.  
    - Range: **1–255**.  
    - Optional, computed with default **64**.

- **dscp** (Number)  
    - Outer IP DSCP value.  
    - Range: **0–63**.  
    - Optional, computed with default **0**.

- **prec** (Number)  
    - Outer IP precedence.  
    - Range: **0–7**.  
    - Optional, computed with default **0**.

- **flow_label** (Number)  
    - IPv6 flow label.  
    - Range: **0–1048575**.  
    - Optional, computed with default **0**.

---

## Tunnel Type Blocks (egress)

Exactly **one** of the following blocks must be set on `gigamon_tunnel_out`.

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
    - Optional in schema but typically required for a functional VXLAN tunnel.

- **destination_port** (Number)  
    - Destination UDP port for this VXLAN tunnel.  
    - Range: **1–65535**.  
    - Optional; if configured, it is sent to FM and read back on subsequent reads.

---

### UDP (`udp`)

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

> **Platform constraint:**  
> Only valid and useful when paired with an **AMI application** in the same Monitoring Session, with a `gigamon_link` between the AMI app and this tunnel.

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

- **source_port** (Number)  
    - Source TCP port for this TLS-PCAPNG tunnel.  
    - Range: **1–65535**.  
    - Optional; may be treated as computed when omitted.

- **destination_port** (Number)  
    - Destination TCP port for this TLS-PCAPNG tunnel.  
    - Range: **1–65535**.  
    - Required by FM for valid TLS-PCAPNG tunnels (commonly set to `1`).

- **Egress constraint:**  
    - **`tls_key_alias` is not allowed** on `gigamon_tunnel_out`. If configured, the provider fails plan/apply with an error:
    - `tls_key_alias is only valid for ingress TLS-PCAPNG tunnels (gigamon_tunnel_in)`  
    - `tls_key_alias` is only valid on the ingress resource (`gigamon_tunnel_in`).

---

### Attributes Reference

In addition to the arguments above, `gigamon_tunnel_out` exports:

- **id** (String)  
    - **Typed tunnel ID** assigned by the provider (wrapping the FM tunnel UUID).  
    - Used for linking (for example, from `gigamon_link`) and for update/delete.  
    - Users never construct this ID manually.

- **type** (String)  
    - Computed tunnel type inferred from the active block.  
    - Possible values: `l2gre`, `vxlan`, `tlspcapng`, `udp`.

- **traffic_direction** (String)  
    - Always `"out"` for this resource.

- **remote_ip**, **mtu**, **ttl**, **dscp**, **prec**, **flow_label**, and nested block fields  
    - Reflected from FM on read and may include FM defaults (for example, TLS defaults, VNI, ports).

---

## Behavior and Lifecycle

- **Monitoring Session scope**  
    - `gigamon_tunnel_out` always belongs to a single Monitoring Session (`monitoring_session_id`).  
    - The provider updates the Monitoring Session’s `tunnels[]` array via Fabric Manager APIs.

- **Type selection**  
    - Exactly one of `l2gre`, `vxlan`, `tls_pcapng`, or `udp` must be present.  
    - If none or more than one is specified, plan/apply fails.

- **IP version derivation**  
    - `ipVersion` is derived from `remote_ip`:
      - IPv4 literal → `IPV4`
      - IPv6 literal → `IPV6`

- **Create / Update**  
    - On create, the provider builds an FM tunnel object from the plan, calls MS update with `"create"`, and stores a typed tunnel ID.  
    - On update, it reuses the existing ID (decoded to a raw UUID) and issues an `"update"` operation.

- **Read / Drift handling**  
    - On read, the provider fetches the Monitoring Session and locates the tunnel by ID (and optionally alias).  
    - If the Monitoring Session or tunnel is missing in FM, the resource is removed from state (idempotent).

- **Delete**  
    - On delete, the provider sends a `"delete"` operation for the tunnel, using the decoded FM UUID and type.  
    - If FM reports the tunnel as not found, deletion is treated as successful.

---

## Import

Import is **not yet supported** for `gigamon_tunnel_out`.