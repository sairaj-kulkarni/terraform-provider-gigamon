---
page_title: "Dedup Application"
subcategory: "Applications"
description: "Manage the Dedup application in Gigamon FM."
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

# Resource: `gigamon_app_dedup`

The **Dedup application** runs inside a **Monitoring Session** and creates a deduplication application instance in Fabric Manager.

- `gigamon_app_dedup` represents a **dedup application instance** attached to one Monitoring Session.
- It is a lightweight app resource with:
    - a required `alias`
    - a required `monitoring_session_id`
    - an optional `description`
    - a computed typed `id`
- The app is created, updated, and deleted through Monitoring Session `"application"` operations.

This resource is distinct from `gigamon_dedup_md_config`, which manages the **global dedup configuration at Monitoring Domain scope**. `gigamon_app_dedup` only manages the **dedup application instance** inside a Monitoring Session.

## Example Usage

### Minimal dedup application

```hcl
resource "gigamon_app_dedup" "dedup" {
  alias                 = "dedup-main"
  monitoring_session_id = gigamon_monitoring_session.ms.id
}
```

### Dedup application with description

```hcl
resource "gigamon_app_dedup" "dedup" {
  alias                 = "dedup-prod"
  monitoring_session_id = gigamon_monitoring_session.ms.id
  description           = "Deduplication app for production monitoring flow"
}
```

### Linking a map to dedup, then dedup to another object

```hcl
resource "gigamon_app_dedup" "dedup" {
  alias                 = "dedup-main"
  monitoring_session_id = gigamon_monitoring_session.ms.id
}

resource "gigamon_link" "map_to_dedup" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  source_id             = gigamon_map_traffic.map.id
  source_aep_id         = 2
  dest_id               = gigamon_app_dedup.dedup.id
}

resource "gigamon_link" "dedup_to_tunnel" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  source_id             = gigamon_app_dedup.dedup.id
  dest_id               = gigamon_tunnel_out_gre.out.id
}
```

In `gigamon_link`, `source_aep_id` is required only when the **source** is a **map** or a **load balancing app**. It is **not used** when the source is a dedup app.

## Argument Reference

### Required

- **`alias`** (String)  
  Name for this dedup application.

- **`monitoring_session_id`** (String)  
  Monitoring Session on which this app is deployed.  
  Changing this forces a new `gigamon_app_dedup` resource to be created.

### Optional

- **`description`** (String)  
  Optional description for this dedup application.

## Attributes Reference

In addition to the arguments above, `gigamon_app_dedup` exports:

- **`id`** (String)  
  Typed ID of this app instance for later use.

This typed ID is what you typically use in resources like `gigamon_link`.

## FM Mapping

The provider maps Terraform data to an FM application payload shaped like:

```json
{
  "alias": "<alias>",
  "name": "dedup",
  "description": "<description>",
  "id": "<raw-fm-id-on-update>"
}
```

Key behavior:

- FM application `Name` is fixed as **`"dedup"`**.
- On create, FM returns a raw application UUID.
- The provider wraps that UUID into a **typed application ID** and stores it in Terraform state.

## Behavior and Lifecycle

### Monitoring Session scope

- `gigamon_app_dedup` belongs to exactly **one** Monitoring Session.
- The provider manages it through Monitoring Session update operations with:
    - `EntityType = "application"`
    - `Operation = "create" | "update" | "delete"`

### Create

On **Create**, the provider:

1. Reads the Terraform plan into `DedupModel`.
2. Builds an FM payload with:
    - `Alias = alias`
    - `Name = "dedup"`
    - `Description = description`
3. Calls Monitoring Session update with an `"application"` `"create"` operation.
4. Receives the FM UUID for the created app.
5. Wraps that UUID into a typed app ID and stores it as `id` in Terraform state.

### Read

On **Read**, the provider:

1. Reads prior Terraform state.
2. Converts the typed `id` back to the raw FM UUID.
3. Fetches the app from the Monitoring Session using app name `"dedup"`.
4. If FM reports object not found, the resource is removed from state.
5. Overlays FM-owned values into state:
    - `alias`
    - `description` when FM returns it

### Update

On **Update**, the provider:

1. Reads the desired plan.
2. Builds the FM payload again with `Name = "dedup"`.
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

If deletion fails because the object is already missing, FM-side idempotent delete handling is expected at the Monitoring Session API layer.

## Schema Notes

`gigamon_app_dedup` is intentionally simple:

- no nested blocks
- no special cross-field validation
- no plan-time semantic checks beyond schema requirements
- no in-place replacement triggers except `monitoring_session_id`

## Relationship to Dedup Monitoring Domain Config

Do not confuse this resource with the Monitoring Domain–level dedup configuration resource.

### `gigamon_app_dedup`

Use this to create the **dedup application instance** inside a Monitoring Session.

### `gigamon_dedup_md_config`

Use that to configure **dedup behavior globally** for the Monitoring Domain, such as:

- action
- timer
- IPv4/IPv6 handling
- TCP sequence handling
- VLAN handling

In practice, the Monitoring Domain config defines **how dedup behaves**, while `gigamon_app_dedup` places a **dedup app instance** into the Monitoring Session workflow.

## Linking and Topology Notes

Because `gigamon_link` accepts application typed IDs as endpoints, `gigamon_app_dedup.id` can participate in Monitoring Session topology just like other app resources.

Typical patterns include:

- map → dedup
- dedup → tunnel
- dedup → application
- application → dedup

Important `gigamon_link` behavior:

- `source_aep_id` is required when the link source is:
    - a map, or
    - a load balancing app
- `source_aep_id` is not valid for dedup as source

So when dedup is the source or destination, you normally only provide:

- `monitoring_session_id`
- `source_id`
- `dest_id`

## Import

Import is not supported

## Summary

Using `gigamon_app_dedup`, you can:

- create a dedup application instance in a Monitoring Session
- optionally add a description
- link it into a Monitoring Session topology using `gigamon_link`
- manage its full lifecycle through Terraform with a stable typed app ID