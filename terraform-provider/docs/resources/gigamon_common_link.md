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

## Resource: `gigamon_link`

A **Link** connects two endpoints (Maps, Applications, Tunnels, or Raw Endpoints) inside a **Monitoring Session**
It represents a directed edge on the Monitoring Session canvas, controlling how traffic flows from a **source** object to a **destination** object.

All endpoints you connect with a `gigamon_link` must belong to the same Monitoring Session.

---

## Example Usage

### Map → Tunnel (simple flow)

```hcl
resource "gigamon_link" "map_to_tunnel" {
  monitoring_session_id = gigamon_monitoring_session.ms.id

  # Source: traffic map in this monitoring session
  source_id     = gigamon_trafficmap.flow_map.id
  source_aep_id = 2  # required for traffic-map sources (AEP ID from the map's rule set)

  # Destination: tunnel out in this monitoring session
  dest_id = gigamon_tunnel_out.l2gre_out.id
}
```

### Map → Application → Tunnel (chained links)

```hcl
# Map → Dedup application
resource "gigamon_link" "map_to_dedup" {
  monitoring_session_id = gigamon_monitoring_session.my_ms.id

  # Source: traffic map
  source_id     = gigamon_trafficmap.my_map.id
  source_aep_id = 2  # required for map sources

  # Destination: dedup application
  dest_id = gigamon_app_dedup.demo_dedup.id
}

# Dedup application → Tunnel
resource "gigamon_link" "dedup_to_tunnel" {
  monitoring_session_id = gigamon_monitoring_session.my_ms.id

  # Source: dedup application
  # For most applications, source_aep_id is NOT required.
  source_id = gigamon_app_dedup.demo_dedup.id

  # Destination: tunnel out
  dest_id = gigamon_tunnel_out.tun_l2gre_out.id
}
```


---

## Argument Reference

### Required

- monitoring_session_id (String)  
      - Monitoring Session in which this link is configured  
      - Set from gigamon_monitoring_session.&lt;name&gt;.id
      - Changing this forces a new link to be created

- source_id (String)  
  ID of the source object for this link
  Must be the ID exported by another resource in the same Monitoring Session, for example:

    - gigamon_trafficmap (traffic map)
    - gigamon_app_* application (dedup, masking, slicing, header stripping, load-balancing, etc.)
    - gigamon_tunnel_out or gigamon_tunnel_in
    - Raw endpoint resource (when available)

    Must be non-empty. Changing this forces a new link to be created

- dest_id (String)  
    - ID of the destination object for this link, in the same Monitoring Session.  
    - Same kinds of objects as source_id (map, application, tunnel, or raw endpoint).  
    
    Must be non-empty. Changing this forces a new link to be created

### Optional

- source_aep_id (Number)  
  AEP ID on the source side of the link

  - Valid range: 1–64 (inclusive)

  - Required when the source is:
    - a traffic map
    - a load-balancing application

  - Must NOT be set when the source is:
    - a non–load-balancing application
    - a tunnel
    - a raw endpoint

  Validation behavior:

  - If omitted when required (map / load-balancing app source), the provider fails plan/apply with a clear error.
  - If set when not allowed, the provider fails plan/apply with a clear error

  Changing this value forces a new link to be created

---

## Attributes Reference

In addition to the arguments above, this resource exports the following read-only attributes:

- id (String)  
  Raw link ID assigned by Fabric Manager (plain UUID, not a typed ID).
  Used by the provider for reads, updates, and deletion.
  Users never construct or parse this value, it is only consumed from state or outputs.

- source_type (String)  
  Type of the source object for this link. Computed from source_id
  Possible values:
    - map
    - application
    - tunnel
    - raw


- dest_type (String)  
  Type of the destination object for this link, computed from dest_id
  Same possible values as source_type

source_type and dest_type are computed only, users never configure them

---

## Behavior and Lifecycle

- Links are treated as create-only connections:
    - Any change to monitoring_session_id, source_id, dest_id, or source_aep_id forces recreation
    - Updates are modeled as “destroy old link, create new link”

- Endpoint type resolution:
    - The provider fetches the Monitoring Session using monitoring_session_id
    - Finds the matching map / application / tunnel / endpoint for source_id and dest_id
    - And fills in source_type and dest_type automatically

- Consistency requirements:
    - source_id and dest_id must refer to objects that actually exist in the same Monitoring Session
    - They must come from other Terraform resources in this provider (for example gigamon_trafficmap.&lt;name&gt;.id)
    - If they are invalid or cannot be matched, the provider fails plan/apply with an error indicating that the link cannot be created

---
## Import

Not yet supported
