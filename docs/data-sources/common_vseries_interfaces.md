---
page_title: "V Series Interfaces"
subcategory: "V Series Interfaces"
description: "Read V Series interface information from Gigamon FM."
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

## Data Source: `gigamon_vseries_interfaces`

The `gigamon_vseries_interfaces` data source discovers **all vSeries nodes under a Fabric connection** and exposes normalized **per‑node interface maps**:

- IP ↔ interface name (IPv4 and IPv6)
- Interface name ↔ IPs (IPv4 and IPv6)
- Interface name ↔ MAC
- MAC ↔ interface name

It works across platforms (for example **ThirdPartyOrchestrated**, **VMware ESXi**) by decoding the typed `conn_id` and calling the appropriate FM REST endpoint, then normalizing the response.

---

## Example Usage

### Discover interfaces for a connection

```hcl
data "gigamon_vseries_interfaces" "vsn" {
  # Typed Fabric connection ID (e.g. connection::anyCloud::<uuid>)
  conn_id = gigamon_connection.acme.id
}

output "vseries_nodes" {
  value = data.gigamon_vseries_interfaces.vsn.nodes
}
```

The `nodes` map is keyed by vSeries node ID (FM `nodeId`), for example:

```hcl
# Example lookups in HCL (conceptual)

# All node IDs
locals {
  vseries_node_ids = keys(data.gigamon_vseries_interfaces.vsn.nodes)
}

# Interface name for a given IPv4 on a specific node
# node_id is one of local.vseries_node_ids
output "iface_for_10_0_0_10" {
  value = data.gigamon_vseries_interfaces.vsn.nodes[local.vseries_node_ids[0]]
    .ipv4_to_interface_name["10.0.0.10"]
}
```

### Selecting vSeries nodes by interface name

```hcl
locals {
  # Keep only nodes that have an ens162 interface
  nodes_with_ens162 = {
    for node_id, node in data.gigamon_vseries_interfaces.vsn.nodes :
    node_id => node
    if contains(keys(node.interface_name_to_mac), "ens162")
  }

  vseries_node_ids = keys(local.nodes_with_ens162)
}
```

You can then feed `local.vseries_node_ids` into `gigamon_endpoint_iface_mapping.vseries_node_ids`.

---

## Argument Reference

### Required

- `conn_id` (String)  
    - **Typed Fabric connection ID** whose vSeries nodes you want to inspect.  
    - Must be a valid FM connection typed ID, for example:  
        - `connection::anyCloud::<uuid>`  
        - `connection::vmwareEsxi::<uuid>`  
    - Typically comes from a connection resource in this provider (for example `gigamon_connection.<name>.id`).  
    - The data source decodes this ID to determine which FM vSeries‑node API to call.

---

## Attributes Reference

- `nodes` (Map of Objects) **Computed**  
        Per‑vSeries‑node interface mappings, keyed by vSeries node ID (FM `nodeId`).  
        If `nodeId` is absent in the FM response, the provider falls back to `mgmtIp`, then `name`, as the map key.  
        Each map value is an object with the following attributes:
    - `name` (String)  
        vSeries node name.
    - `mgmt_ip` (String)  
        vSeries node management IP address.
    - `platform` (String)  
        vSeries node platform (for example `anyCloud`, `vmwareEsxi`).
    - `interface_name_to_ipv4` (Map of List[String])  
        - Key: interface name (for example `ens162`).  
        - Value: list of IPv4 addresses configured on that interface for this node.  
        - Only interfaces with at least one IPv4 address are present.
    - `interface_name_to_ipv6` (Map of List[String])  
        - Key: interface name.  
        - Value: list of IPv6 addresses configured on that interface for this node.  
        - Only interfaces with at least one IPv6 address are present.
    - `ipv4_to_interface_name` (Map of String)  
        - Key: IPv4 address.  
        - Value: interface name that owns that address.
    - `ipv6_to_interface_name` (Map of String)  
        - Key: IPv6 address.  
        - Value: interface name that owns that address.
    - `interface_name_to_mac` (Map of String)  
        - Key: interface name.  
        - Value: MAC address of that interface.  
        - Includes IP‑less data interfaces that still have a MAC.
    - `mac_to_interface_name` (Map of String)  
        - Key: MAC address.  
        - Value: interface name with that MAC.

If FM returns **no vSeries nodes** for `conn_id`, `nodes` is still present but is an **empty map**.

---

## Behavior

- The data source:
    - Parses the typed `conn_id` to determine the platform (for example `anyCloud`, `vmwareEsxi`).  
    - Calls the corresponding FM vSeries‑nodes API with `connId=<uuid>` as a query parameter.  
    - Normalizes the FM response into a list of nodes and their interfaces.  
    - Builds the various per‑node maps (IP ↔ interface, interface ↔ MAC, etc.) before returning them to Terraform.

- If `conn_id` cannot be parsed as a typed connection ID, or FM returns an error, the data source fails with diagnostics.

---

# Resource: `gigamon_endpoint_iface_mapping`

The `gigamon_endpoint_iface_mapping` resource configures **vSeries interface ↔ endpoint mappings** for a **Monitoring Session**.

Each resource instance:

- Targets **one Monitoring Session** (via `monitoring_session_id`).
- Applies the same set of **interface ↔ endpoint** mappings to a **set of vSeries nodes** (`vseries_node_ids`).
- Is typically used to bind **Raw Endpoints (REPs)** from `gigamon_raw_endpoint` to specific vSeries interfaces discovered via the `gigamon_vseries_interfaces` data source.

Under the hood it drives FM’s **Monitoring Session endpoint‑iface mapping** API.

---

## Example Usage

### Basic: map one vSeries node's interface to a raw endpoint

```hcl
# Discover vSeries interfaces for a connection
data "gigamon_vseries_interfaces" "vsn" {
  conn_id = gigamon_connection.acme.id
}

# Create a raw endpoint in the same Monitoring Session
resource "gigamon_raw_endpoint" "rep_ens162" {
  monitoring_session_id = gigamon_monitoring_session.ms.id

  alias       = "rep-ens162"
  description = "REP bound to ens162 on vSeries nodes"
}

# Choose one node ID (for demo); in real configs select explicitly
locals {
  primary_node_id = keys(data.gigamon_vseries_interfaces.vsn.nodes)[0]
}

resource "gigamon_endpoint_iface_mapping" "primary_node" {
  monitoring_session_id = gigamon_monitoring_session.ms.id

  # Apply this mapping only to the chosen vSeries node
  vseries_node_ids = [local.primary_node_id]

  mapping {
    iface       = "ens162"
    endpoint_id = gigamon_raw_endpoint.rep_ens162.id
  }
}
```

### Global mapping for all nodes using dynamic blocks

Configure the same **interface ↔ endpoint** mappings on **all vSeries nodes** under a connection.

```hcl
data "gigamon_vseries_interfaces" "vsn" {
  conn_id = gigamon_connection.acme.id
}

# One REP per interface we care about
locals {
  vseries_ifaces = ["ens162", "ens193"]
}

resource "gigamon_raw_endpoint" "rep_per_iface" {
  for_each = toset(local.vseries_ifaces)

  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "rep-${each.key}"
  description           = "REP bound to interface ${each.key}"
}

# Helper map: iface -> endpoint_id
locals {
  iface_to_endpoint = {
    for iface in local.vseries_ifaces :
    iface => gigamon_raw_endpoint.rep_per_iface[iface].id
  }

  # All discovered vSeries node IDs under the connection
  vseries_node_ids = keys(data.gigamon_vseries_interfaces.vsn.nodes)
}

resource "gigamon_endpoint_iface_mapping" "global" {
  monitoring_session_id = gigamon_monitoring_session.ms.id

  # Apply the same mappings to all nodes
  vseries_node_ids = local.vseries_node_ids

  dynamic "mapping" {
    for_each = local.iface_to_endpoint

    content {
      iface       = mapping.key
      endpoint_id = mapping.value
    }
  }
}
```

This pattern is equivalent to a “global” iface mapping for all nodes (similar to the `__RawEndPointIfaceMapping:global` concept in the backend).

---

## Argument Reference

### Required

- `monitoring_session_id` (String)  
    - **Monitoring Session ID** this mapping belongs to.  
    - Must be a **typed Monitoring Session ID** (for example `monitoringSession::vmwareEsxi::<uuid>`), typically from `gigamon_monitoring_session.<name>.id`.  
    - Changing this value **forces a new mapping resource** (resource replacement).
- `vseries_node_ids` (List of String)  
    - List of **vSeries node IDs** this mapping applies to.  
    - Each entry must be a valid FM vSeries node ID (usually sourced from `data.gigamon_vseries_interfaces.vsn.nodes`).  
    - Must contain **at least one** node ID.  
    - Changing the contents of this list triggers an in‑place update of mappings for the Monitoring Session.
- `mapping` (Block; repeatable)  
  Defines one **interface ↔ endpoint** pair. At least one `mapping` block is required.
  Block arguments:
    - `iface` (String)  
        - Interface name on the vSeries node (for example `ens162`, `ens193`).  
        - Must match an interface name that exists on each node in `vseries_node_ids`.  
        - Must be non‑empty.
    - `endpoint_id` (String)  
        - **Typed endpoint ID** to bind to the given interface.  
        - For raw endpoints, this is the ID from `gigamon_raw_endpoint.<name>.id`:
            - Shape: `rawEndpoint::raw::<uuid>`.  
        - The provider unwraps the typed ID and passes the raw UUID to FM.  
        - Must be non‑empty.

---

## Attributes Reference

- `id` (String) – **Computed**  
    - Synthetic, **typed ID** for this mapping:  
        - `endpointIfaceMapping::mapping::<monitoring_session_uuid>`  
    - Derived from `monitoring_session_id`.  
    - Used only internally by the provider; users never construct or parse this ID manually.

---

## Behavior and Lifecycle

### Creation

- On `apply`, the provider:
    1. Validates configuration:
        - `monitoring_session_id` must be a valid typed ID.  
        - `vseries_node_ids` must not be empty.  
        - At least one `mapping` block must be present.  
    2. Builds the FM payload:  

        ```json
        {
        "monitoringSessionId": "<monitoring-session-uuid>",
        "vseriesEndpointIfaceMappings": [
            {
            "vseriesNodeIds": ["<node-id-1>", "<node-id-2>", ...],
            "endpointIfaceMappings": [
                { "iface": "<iface>", "endpointId": "<endpoint-uuid>" },
                ...
            ]
            }
        ]
        }
        ```

        - `monitoringSessionId` is the **raw UUID** from `monitoring_session_id`.  
        - Each `endpointId` is the **raw UUID** derived from the typed `endpoint_id`.

    3. Issues a **POST** to FM:  

        - `POST /api/v1.3/cloud/monitoringSessions/<monitoring-session-uuid>/endpointIfaceMappings`

    4. Computes `id` as `endpointIfaceMapping::mapping::<monitoring_session_uuid>` and stores it in state.

    5. Performs a best‑effort **read‑back** (`GET` same endpoint) and updates `vseries_node_ids` and `mapping` blocks from FM’s view.

### Read / Refresh

- During `terraform refresh` or `plan`:

    1. The provider calls:

        - `GET /api/v1.3/cloud/monitoringSessions/<monitoring-session-uuid>/endpointIfaceMappings`

    2. If FM returns a mapping for the Monitoring Session:
        - The provider updates:
        - `vseries_node_ids` from the `vseriesNodeIds` reported by FM.  
        - `mapping` blocks from FM’s `endpointIfaceMappings`, wrapping each raw `endpointId` back into a **typed raw endpoint ID** where possible.

    3. If FM returns **ObjectNotFound** (for example, the Monitoring Session or its mappings were deleted):
        - The provider **removes the resource from state** (idempotent read).

### Updates

- When any of these change:
    - `vseries_node_ids`  
    - Any `mapping` block (`iface` or `endpoint_id`)

- The provider:

    1. Re‑builds the FM payload from the **planned** configuration (same shape as creation).  
    2. Issues a **POST** to `/endpointIfaceMappings`.  
        - This acts as an **upsert** for the mappings of the selected vSeries nodes.  
    3. Recomputes `id` from `monitoring_session_id`.  
    4. Reads back from FM and updates state, just as during `Create`.

> **Important:** Because the FM API treats mappings at the Monitoring Session level, you should usually have **at most one** `gigamon_endpoint_iface_mapping` per Monitoring Session. Multiple resources targeting the same `monitoring_session_id` will overwrite each other’s mappings via POSTs to the same endpoint.

### Deletion

- On `terraform destroy` or when the resource is removed from configuration:

    1. The provider parses `monitoring_session_id` and derives the raw Monitoring Session UUID.  
    2. Calls **DELETE**:

        - `DELETE /api/v1.3/cloud/monitoringSessions/<monitoring-session-uuid>/endpointIfaceMappings`

    3. Behavior:

        - If FM reports **ObjectNotFound**, the provider treats deletion as **successful** (idempotent).  
        - Any other FM errors are ignored (best‑effort), and Terraform still removes the resource from state.

---

## Usage Notes and Best Practices

- **One mapping resource per Monitoring Session**  
    - To avoid last‑writer‑wins behavior, define a **single** `gigamon_endpoint_iface_mapping` per `monitoring_session_id` and manage all node IDs and mappings within it (using `dynamic "mapping"` blocks as needed).

- **Discover before mapping**  
    - Always use `data "gigamon_vseries_interfaces"` to:
        - Discover valid `vseries_node_ids`.  
        - Inspect available interface names (`interface_name_to_mac` keys).  
    - This reduces the chance of invalid iface names or node IDs.

- **Pairing with raw endpoints**  
    - Recommended pattern:
        - Create one `gigamon_raw_endpoint` per vSeries interface you plan to use as a REP (for example `rep-ens162`, `rep-ens193`).  
        - Feed those IDs into `endpoint_id` mappings.  
    - Use clear naming conventions for raw endpoint aliases:
        - `rep-<tool>-<zone>` (for example `rep-ids-prod`)  
        - `rep-<vseries_iface>` (for example `rep-ens162`) when pairing 1:1 with V-Series interfaces.

- **Validation behavior**  
    - If `vseries_node_ids` is empty, the provider fails plan/apply with a clear error.  
    - If no `mapping` blocks are provided, a **config validator** rejects the configuration with:  
        - “gigamon_endpoint_iface_mapping must have at least one mapping block.”  
    - If any `endpoint_id` cannot be parsed as a typed ID, the provider records an error and skips that mapping entry.

- **Drift and platform behavior**  
    - If mappings are changed directly in FM, a subsequent `plan` may show changes as Terraform re‑applies the configuration defined in HCL.  
    - If vSeries nodes are upgraded, replaced, or re‑registered, interface mappings may need to be re‑applied; use this resource (and the data source) to re‑drive FM configuration declaratively.

---

## Import

Import is **not currently supported** for `gigamon_endpoint_iface_mapping`.  
You should manage endpoint‑iface mappings exclusively via Terraform for them to appear in state.