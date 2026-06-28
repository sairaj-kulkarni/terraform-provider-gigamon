---
page_title: "Slicing Application"
subcategory: "Applications"
description: "Manage the Slicing application in Gigamon FM."
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

# Resource: `gigamon_app_slicing`

The **Slicing application** runs inside a **Monitoring Session** and creates a slicing application instance in Fabric Manager.

- `gigamon_app_slicing` represents a **slicing application instance** attached to one Monitoring Session.
- It supports packet slicing with:
    - a required `alias`
    - a required `monitoring_session_id`
    - a protocol selector
    - an offset value
    - a computed typed `id`
- The app is created, updated, and deleted through Monitoring Session `"application"` operations.

This is a simple application resource with no nested blocks and no cross-field semantic validation beyond schema rules.

## Example Usage

### Minimal slicing application

```hcl
resource "gigamon_app_slicing" "slicing" {
  alias                 = "slice-main"
  monitoring_session_id = gigamon_monitoring_session.ms.id
}
```

### Slicing with explicit protocol and offset

```hcl
resource "gigamon_app_slicing" "slicing" {
  alias                 = "slice-tcp"
  monitoring_session_id = gigamon_monitoring_session.ms.id

  protocol = "tcp"
  offset   = 96
}
```

### GTP-aware slicing

```hcl
resource "gigamon_app_slicing" "slicing_gtp" {
  alias                 = "slice-gtp"
  monitoring_session_id = gigamon_monitoring_session.ms.id

  protocol = "gtp"
  offset   = 128
}
```

### Linking a map to slicing, then slicing to another object

```hcl
resource "gigamon_app_slicing" "slicing" {
  alias                 = "slice-main"
  monitoring_session_id = gigamon_monitoring_session.ms.id

  protocol = "ipv4"
  offset   = 80
}

resource "gigamon_link" "map_to_slicing" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  source_id             = gigamon_map_traffic.map.id
  source_aep_id         = 2
  dest_id               = gigamon_app_slicing.slicing.id
}

resource "gigamon_link" "slicing_to_tunnel" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  source_id             = gigamon_app_slicing.slicing.id
  dest_id               = gigamon_tunnel_out_gre.out.id
}
```

In `gigamon_link`, `source_aep_id` is required only when the **source** is a **map** or a **load balancing app**. It is not used when the source is a slicing app.

## Argument Reference

### Required

- **`alias`** (String)  
  Name for this slicing application.

- **`monitoring_session_id`** (String)  
  Monitoring Session on which this app is deployed.  
  Changing this forces a new `gigamon_app_slicing` resource to be created.

### Optional

- **`protocol`** (String)  
  Protocol to check and skip before applying the offset.  
  Optional, computed, default: `"none"`.

  Allowed values:

    - `"none"`
    - `"ipv4"`
    - `"ipv6"`
    - `"udp"`
    - `"tcp"`
    - `"ftp-data"`
    - `"https"`
    - `"ssh"`
    - `"gtp"`
    - `"gtp-ipv4"`
    - `"gtp-udp"`
    - `"gtp-tcp"`

- **`offset`** (Number)  
  Offset at which slicing is applied.  
  Optional, computed, default: `64`.

## Attributes Reference

In addition to the arguments above, `gigamon_app_slicing` exports:

- **`id`** (String)  
  Typed ID of this app instance for later use.

This typed ID is what you typically use in resources like `gigamon_link`.

## FM Mapping

The provider maps Terraform data to an FM application payload shaped like:

```json
{
  "alias": "<alias>",
  "name": "slicing",
  "protocol": "<protocol>",
  "offset": 64,
  "id": "<raw-fm-id-on-update>"
}
```

Key behavior:

- FM application `Name` is fixed as **`"slicing"`**.
- On create, FM returns a raw application UUID.
- The provider wraps that UUID into a **typed application ID** and stores it in Terraform state.

## Behavior and Lifecycle

### Monitoring Session scope

- `gigamon_app_slicing` belongs to exactly **one** Monitoring Session.
- The provider manages it through Monitoring Session update operations with:
    - `EntityType = "application"`
    - `Operation = "create" | "update" | "delete"`

### Create

On **Create**, the provider:

1. Reads the Terraform plan into `SlicingModel`.
2. Builds the FM payload with:
    - `Alias = alias`
    - `Name = "slicing"`
    - `Protocol = protocol`
    - `Offset = offset`
3. Calls Monitoring Session update with an `"application"` `"create"` operation.
4. Receives the FM UUID for the created app.
5. Wraps that UUID into a typed app ID and stores it as `id` in Terraform state.

### Read

On **Read**, the provider:

1. Reads prior Terraform state.
2. Converts the typed `id` back to the raw FM UUID.
3. Fetches the app from the Monitoring Session using app name `"slicing"`.
4. If FM reports object not found, the resource is removed from state.
5. Overlays FM-owned values into state:
    - `alias`
    - `protocol`
    - `offset`

### Update

On **Update**, the provider:

1. Reads the desired plan.
2. Builds the FM payload again with `Name = "slicing"`.
3. Converts the typed state ID into the raw FM UUID.
4. Calls Monitoring Session update with an `"application"` `"update"` operation.
5. Writes the updated plan back to state after overlaying FM-owned fields.

### Delete

On **Delete**, the provider:

1. Reads existing state.
2. Converts the typed `id` to raw FM UUID.
3. Calls Monitoring Session update with an `"application"` `"delete"` operation.
4. Sends a minimal delete payload with:
    - `Id = <raw uuid>`
    - `Name = "Application"`

## Protocol Semantics

The slicing app uses `protocol` to decide where offset calculation begins.

### `protocol = "none"`

Offset starts from the first byte of the packet.

### Protocol-aware modes

For values like `ipv4`, `ipv6`, `udp`, `tcp`, `https`, `ssh`, and the `gtp*` variants:

- the selected protocol is checked first
- slicing offset is then applied relative to that protocol context

This allows the same slicing app shape to work for plain packet slicing and protocol-relative slicing.

## Schema Notes

`gigamon_app_slicing` is intentionally simple:

- no nested blocks
- no custom `ModifyPlan`
- no extra semantic validation function
- no in-place replacement triggers except `monitoring_session_id`

It is one of the smaller and cleaner application resources in the provider.

## Linking and Topology Notes

Because `gigamon_link` accepts application typed IDs as endpoints, `gigamon_app_slicing.id` can participate in Monitoring Session topology just like other app resources.

Typical patterns include:

- map → slicing
- slicing → tunnel
- slicing → application
- application → slicing

Important `gigamon_link` behavior:

- `source_aep_id` is required when the link source is:
    - a map, or
    - a load balancing app
- `source_aep_id` is not valid for slicing as source

So when slicing is the source or destination, you normally only provide:

- `monitoring_session_id`
- `source_id`
- `dest_id`

## Import

Import support is not supported
## Summary

Using `gigamon_app_slicing`, you can:

- create a slicing application instance in a Monitoring Session
- apply simple or protocol-relative slicing using `protocol` and `offset`
- link it into a Monitoring Session topology using `gigamon_link`
- manage its full lifecycle through Terraform with a stable typed app ID