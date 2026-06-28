---
page_title: "Raw Endpoint"
subcategory: "Tunnels and Raw Endpoints"
description: "Manage raw endpoints in Gigamon FM."
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

## Resource: `gigamon_raw_endpoint`

A **Raw Endpoint (REP)** represents a **raw traffic endpoint** inside a **Monitoring Session**.  
It is typically used as a low-level sink or source for traffic within the session and can be:

- Terminated directly on V-Series interfaces via `gigamon_endpoint_iface_mapping`
- Connected to maps, applications, or tunnels via `gigamon_link` (when raw endpoints are supported as endpoints)

All raw endpoints must belong to a single **Monitoring Session**.

---

## Example Usage

### Single raw endpoint in a Monitoring Session

```hcl
resource "gigamon_monitoring_session" "ms" {
  alias       = "demo-ms"
  description = "Monitoring session for demo"
  # ...
}

resource "gigamon_raw_endpoint" "rep_main" {
  monitoring_session_id = gigamon_monitoring_session.ms.id

  alias       = "rep-main"
  description = "Primary raw endpoint for this session"
}
```

### Multiple raw endpoints using `for_each`

Create a set of raw endpoints from a variable:

```hcl
variable "raw_endpoints" {
  type        = set(string)
  description = "Aliases for raw endpoints to create in the monitoring session"
  default     = ["rep-collector-a", "rep-collector-b"]
}

resource "gigamon_monitoring_session" "ms" {
  alias       = "demo-ms"
  description = "Monitoring session with multiple REPs"
  # ...
}

resource "gigamon_raw_endpoint" "rep" {
  for_each = var.raw_endpoints

  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = each.value
  description           = "Raw endpoint ${each.value}"
}
```

You can then reference specific raw endpoints as:

```hcl
output "rep_ids" {
  value = {
    for k, v in gigamon_raw_endpoint.rep : k => v.id
  }
}
```

### Linking a traffic map to a raw endpoint

This example shows a map sending traffic directly to a raw endpoint via `gigamon_link`:

```hcl
resource "gigamon_monitoring_session" "ms" {
  alias = "demo-ms"
  # ...
}

resource "gigamon_trafficmap" "flow_map" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "flow-map-1"
  # rules, conditions, etc.
}

resource "gigamon_raw_endpoint" "rep_sink" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "rep-sink"
  description           = "Sink raw endpoint for map output"
}

resource "gigamon_link" "map_to_rep" {
  monitoring_session_id = gigamon_monitoring_session.ms.id

  # Source: traffic map
  source_id    = gigamon_trafficmap.flow_map.id
  source_aep_id = 1

  # Destination: raw endpoint in the same Monitoring Session
  dest_id = gigamon_raw_endpoint.rep_sink.id
}
```

### Preparing for interface mappings (with `gigamon_endpoint_iface_mapping`)

Raw endpoints are often used as **endpoints for V-Series interfaces**.  
You can create one raw endpoint per interface and later map them using `gigamon_endpoint_iface_mapping`:

```hcl
locals {
  vseries_interfaces = [
    "ens162",
    "ens193",
  ]
}

resource "gigamon_raw_endpoint" "rep_per_iface" {
  for_each = toset(local.vseries_interfaces)

  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "rep-${each.key}"
  description           = "REP bound to interface ${each.key}"
}
```

The mapping from interfaces to these raw endpoints will be configured via `gigamon_endpoint_iface_mapping` (documented separately).

---

## Argument Reference

### Required

- `monitoring_session_id` (String)  
    - Monitoring Session in which this raw endpoint is configured.  
    - Must be set from `gigamon_monitoring_session.<name>.id` (a **typed Monitoring Session ID**, e.g. `monitoringSession::vmwareEsxi::<uuid>`).  
    - Changing this value **forces a new raw endpoint** to be created (resource replacement).
- `alias` (String)  
    - Human-readable **alias of the raw endpoint** as shown in Fabric Manager.  
    - Must be a non-empty string (validated by the provider).  
    - Should be meaningful and stable, as it is how the endpoint appears in the FM UI and in operator workflows.  
    - Changing this value performs an **in-place update** of the raw endpoint in FM (no recreation).

### Optional

- `description` (String)  
    - Optional free‑form **description** of the raw endpoint.  
    - Can be used to capture operational context (e.g. downstream tool, tenant name, purpose).  
    - May be empty or omitted; provider treats empty / missing description as `null`.  
    - Changing this value performs an **in‑place update** of the raw endpoint.

---

## Attributes Reference

In addition to the arguments above, this resource exports the following read‑only attribute:

- `id` (String)  
    - **Typed raw endpoint ID** for this resource.  
    - Shape: `rawEndpoint::raw::<uuid>`.  
    - Backed by a plain UUID from Fabric Manager, wrapped into a typed ID by the provider.  
    - This value is used as:
        - `endpoint_id` in `gigamon_endpoint_iface_mapping.mapping.endpoint_id`
        - A valid endpoint ID for `gigamon_link.source_id` / `gigamon_link.dest_id` when links support raw endpoints  
    - Users **never construct or parse** this value manually; it is always consumed from Terraform state or outputs.

---

## Behavior and Lifecycle

### Creation

- On `apply`, the provider:
  - Builds an FM payload containing `alias` and optional `description`.  
  - Calls the Monitoring Session **batch update API**:  
    - `entityType = "raw"`  
    - `operation = "create"`  
  - FM returns a newly allocated **raw endpoint UUID**.  
  - The provider then **reads back** the Monitoring Session’s raw endpoints, finds the created raw endpoint, and:
    - Stores its alias / description in state
    - Wraps the raw UUID into a typed ID and sets `id`

### Read / Refresh

- During `terraform refresh` or `plan`:
  - The provider fetches the Monitoring Session and enumerates all raw endpoints.  
  - It locates the matching raw endpoint (by its underlying UUID) and updates:
    - `alias` (always refreshed from FM)
    - `description` (set to `null` in state if missing/empty in FM)
    - `id` (kept as a **typed ID**; underlying UUID is re-derived when needed)  
- If FM reports that the raw endpoint **no longer exists**:
    - The provider **removes the resource from state** (idempotent read semantics).

### Updates

- When you change **`alias`** or **`description`**:
  - The provider unwraps the typed `id` to the raw UUID.  
  - Uses the Monitoring Session batch update API with:
    - `entityType = "raw"`
    - `operation = "update"`
    - `raw = { "id": "<uuid>", "alias": "...", "description": "..." }`  
  - After a successful update, the provider **reads back** the raw endpoint and refreshes state.

- When you change **`monitoring_session_id`**:
    - Terraform plans a **delete + recreate**:
            - Old raw endpoint is removed from the old Monitoring Session.
            - A new raw endpoint is created in the new Monitoring Session.

### Deletion

- On `terraform destroy` or when the resource is removed from configuration:
  - The provider unwraps the typed `id` to the raw UUID.  
  - Calls the Monitoring Session batch update API with:
    - `entityType = "raw"`
    - `operation = "delete"`
    - `raw = { "id": "<uuid>" }`  
  - If FM responds that the raw endpoint or Monitoring Session is already gone:
    - The provider treats the delete as **success** (idempotent behavior).  
  - Any other FM errors are reported in diagnostics, and the resource **remains in state** (the destroy is aborted). The user must resolve the FM error before Terraform can remove the resource from state.

---

## Usage Notes and Best Practices

- **One Monitoring Session per raw endpoint:**  
  Each `gigamon_raw_endpoint` always belongs to exactly one Monitoring Session via `monitoring_session_id`. It cannot be moved; to “move” it, create in the new session and delete the old one.

- **Naming conventions:**  
  Use consistent `alias` patterns that are meaningful.

- **Downstream integrations:**  
  Raw endpoints are typically:
    - **Targets** for V-Series interfaces via `gigamon_endpoint_iface_mapping`
    - **End nodes** in link chains defined by `gigamon_link`  
    Plan these together so aliases and counts match what you expect on the canvas.

- **Drift behavior:**  
    - If an operator deletes a raw endpoint in FM, the next `plan` / `apply` will show it as **planned for creation** again (since state no longer matches FM).  
    - If an operator changes alias/description in FM directly, Terraform will detect the difference and update state on the next refresh; subsequent applies will push the configuration back to what’s defined in HCL.

---

## Import

Import is **not currently supported** for `gigamon_raw_endpoint`.  
You must manage raw endpoints exclusively via Terraform for them to appear in state.