
# Resource: `gigamon_app_header_stripping`

The **Header Stripping application** runs inside a **Monitoring Session** and creates a header stripping application instance in Fabric Manager.

- `gigamon_app_header_stripping` represents a **header stripping application instance** attached to one Monitoring Session.
- It supports multiple protocol-specific stripping modes through **exactly one** protocol block.
- The resource contains:
    - a `monitoring_session_id`
    - an optional `alias`
    - a computed `protocol`
    - one protocol-selection block
    - a computed typed `id`

This resource is more structured than Dedup, Masking, or Slicing because the selected stripping mode is modeled through nested blocks rather than a plain string field.

## Example Usage

### VXLAN header stripping

```hcl
resource "gigamon_app_header_stripping" "hs_vxlan" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "hs-vxlan"

  vxlan {
    vxlan_id = 100
  }
}
```

### VLAN header stripping

```hcl
resource "gigamon_app_header_stripping" "hs_vlan" {
  monitoring_session_id = gigamon_monitoring_session.ms.id

  vlan {
    vlan_header = "all"
  }
}
```

### Generic header stripping

```hcl
resource "gigamon_app_header_stripping" "hs_generic" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "hs-generic"

  generic {
    ah1              = "eth"
    offset           = "offsetRange"
    offset_range_value = 32
    header_count     = 2
    custom_len       = 64
    ah2              = "vlan"
  }
}
```

### Simple protocol mode

```hcl
resource "gigamon_app_header_stripping" "hs_gtp" {
  monitoring_session_id = gigamon_monitoring_session.ms.id

  gtp {}
}
```

### Linking a map to header stripping, then header stripping to another object

```hcl
resource "gigamon_app_header_stripping" "hs" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  alias                 = "hs-main"

  erspan {
    flow_id = 10
  }
}

resource "gigamon_link" "map_to_hs" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  source_id             = gigamon_map_traffic.map.id
  source_aep_id         = 2
  dest_id               = gigamon_app_header_stripping.hs.id
}

resource "gigamon_link" "hs_to_tunnel" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  source_id             = gigamon_app_header_stripping.hs.id
  dest_id               = gigamon_tunnel_out_gre.out.id
}
```

In `gigamon_link`, `source_aep_id` is required only when the **source** is a **map** or a **load balancing app**. It is not used when the source is a header stripping app.

## Argument Reference

### Required

- **`monitoring_session_id`** (String)  
  Monitoring Session on which this app is deployed.  
  Changing this forces a new `gigamon_app_header_stripping` resource to be created.

### Optional

- **`alias`** (String)  
  Name for this header stripping application.  
  Optional, computed, default: `"headerStrip"`.

## Attributes Reference

In addition to the arguments and blocks below, `gigamon_app_header_stripping` exports:

- **`protocol`** (String)  
  Computed protocol derived from the selected block.

  Possible values:

    - `vxlan`
    - `vlan`
    - `fm6000Ts`
    - `erspan`
    - `generic`
    - `gtp`
    - `isl`
    - `mpls`
    - `mplsPlusVlan`
    - `vntag`
    - `geneve`

- **`id`** (String)  
  Typed ID of this app instance for later use.

This typed ID is what you typically use in resources like `gigamon_link`.

## Block Reference

Exactly one of the following blocks must be present.

### `vxlan`

```hcl
vxlan {
  vxlan_id = number
}
```

- **`vxlan_id`** (Number)  
  24-bit VXLAN ID to strip.  
  Optional, computed, default: `0`.  
  Range: `0` to `16777215`.

Special meaning:

- `0` strips all VXLAN IDs.

### `vlan`

```hcl
vlan {
  vlan_header = string
}
```

- **`vlan_header`** (String)  
  Target VLAN header(s) to remove.  
  Optional, computed, default: `"all"`.

  Allowed values:

  - `"outer"`
  - `"all"`

### `fm6000_ts`

```hcl
fm6000_ts {
  timestamp_format = string
}
```

- **`timestamp_format`** (String)  
  Timestamp format.  
  Optional, computed, default: `"none"`.

  Allowed values:

  - `"none"`

### `erspan`

```hcl
erspan {
  flow_id = number
}
```

- **`flow_id`** (Number)  
  ERSPAN flow ID.  
  Optional, computed, default: `0`.  
  Range: `0` to `1023`.

Special meaning:

- `0` matches all flows.

### `generic`

```hcl
generic {
  ah1                = string
  offset             = string
  offset_range_value = number
  header_count       = number
  custom_len         = number
  ah2                = string
}
```

- **`ah1`** (String)  
  First anchor header.  
  Required semantically when `generic` is used.

  Allowed values:

    - `"none"`
    - `"eth"`
    - `"vlan"`
    - `"mpls"`
    - `"ipv4"`
    - `"ipv6"`

- **`offset`** (String)  
  Offset mode.  
  Required semantically when `generic` is used.

  Allowed values:

    - `"start"`
    - `"end"`
    - `"offsetRange"`


- **`offset_range_value`** (Number)  
  Offset from `ah1` when `offset = "offsetRange"`.  
  Optional, but required semantically in that case.  
  Range: `0` to `1500`.

- **`header_count`** (Number)  
  Number of headers to remove.  
  Optional.  
  Range: `1` to `32`.

- **`custom_len`** (Number)  
  Length in bytes of header to strip.  
  Optional.  
  Range: `1` to `1500`.

- **`ah2`** (String)  
  Second anchor header.  
  Required semantically when `generic` is used.

  Allowed values:

  - `"none"`
  - `"eth"`
  - `"vlan"`
  - `"mpls"`

### Simple protocol blocks

These blocks have no inner attributes. Presence of the block alone selects the stripping mode.

```hcl
gtp {}
isl {}
mpls {}
mpls_plus_vlan {}
vntag {}
geneve {}
```

Their Terraform-to-FM protocol mapping is:

- `gtp {}` → `gtp`
- `isl {}` → `isl`
- `mpls {}` → `mpls`
- `mpls_plus_vlan {}` → `mplsPlusVlan`
- `vntag {}` → `vntag`
- `geneve {}` → `geneve`

## Validation Rules

The provider applies both schema-level and semantic validation.

### Exactly one protocol block

A config validator enforces that **exactly one** of the following blocks is present:

- `vxlan`
- `vlan`
- `fm6000_ts`
- `erspan`
- `generic`
- `gtp`
- `isl`
- `mpls`
- `mpls_plus_vlan`
- `vntag`
- `geneve`

You cannot configure multiple stripping modes in the same resource.

### Generic block validation

When the `generic` block is used:

- `ah1` must be specified
- `ah2` must be specified
- `offset` must be specified

Additional rules:

- if `offset = "offsetRange"`, then `offset_range_value` is required
- if `offset != "offsetRange"`, then `offset_range_value` must not be set

This validation runs during create and update.

## FM Mapping

The provider maps Terraform data to an FM application payload shaped like:

```json
{
  "alias": "headerStrip",
  "name": "headerStrip",
  "protocol": "vxlan",
  "vxlan": {
    "vxlanId": 100
  },
  "id": "<raw-fm-id-on-update>"
}
```

The selected nested block determines both:

- the computed Terraform `protocol`
- the FM nested payload section

Examples:

- `vxlan {}` → `protocol = "vxlan"` and `fm.Vxlan`
- `vlan {}` → `protocol = "vlan"` and `fm.Vlan`
- `fm6000_ts {}` → `protocol = "fm6000Ts"` and `fm.Fm6000Ts`
- `erspan {}` → `protocol = "erspan"` and `fm.Erspan`
- `generic {}` → `protocol = "generic"` and `fm.Generic`
- simple blocks only set `protocol`; they do not create extra nested FM sections

Key behavior:

- FM application `Name` is fixed as **`"headerStrip"`**.
- On create, FM returns a raw application UUID.
- The provider wraps that UUID into a **typed application ID** and stores it in Terraform state.

## Behavior and Lifecycle

### Monitoring Session scope

- `gigamon_app_header_stripping` belongs to exactly **one** Monitoring Session.
- The provider manages it through Monitoring Session update operations with:
    - `EntityType = "application"`
    - `Operation = "create" | "update" | "delete"`

### Create

On **Create**, the provider:

1. Reads the Terraform plan into `HeaderStrippingModel`.
2. Validates any generic-block semantics.
3. Infers the stripping protocol from the selected block.
4. Builds the FM payload with:
    - `Alias = alias`
    - `Name = "headerStrip"`
    - `Protocol = inferred protocol`
    - the matching FM nested block if required
5. Calls Monitoring Session update with an `"application"` `"create"` operation.
6. Receives the FM UUID for the created app.
7. Wraps that UUID into a typed app ID and stores it as `id` in Terraform state.

### Read

On **Read**, the provider:

1. Reads prior Terraform state.
2. Converts the typed `id` back to the raw FM UUID.
3. Fetches the app from the Monitoring Session using app name `"headerStrip"`.
4. If FM reports object not found, the resource is removed from state.
5. Overlays FM-owned values into state:
    - `alias`
    - `protocol`
    - the matching protocol block
6. Clears all other protocol blocks first, so the state reflects only the FM-selected mode.

For generic mode, the provider also applies special null-handling:

- `offset_range_value` remains null when FM omits it
- `header_count` of `0` from FM is mapped to null
- `custom_len` of `0` from FM is mapped to null

This avoids invalid Terraform state for fields whose validators require positive values when actually set.

### Update

On **Update**, the provider:

1. Reads the desired plan.
2. Validates generic-block semantics again.
3. Infers the protocol from the selected block.
4. Builds the FM payload with `Name = "headerStrip"`.
5. Converts the typed state ID into the raw FM UUID.
6. Calls Monitoring Session update with an `"application"` `"update"` operation.
7. Writes the updated plan back to state after overlaying FM-owned fields.

### Delete

On **Delete**, the provider:

1. Reads existing state.
2. Converts the typed `id` to raw FM UUID.
3. Calls Monitoring Session update with an `"application"` `"delete"` operation.
4. Sends a minimal delete payload with:
    - `Id = <raw uuid>`
    - `Name = "Application"`

## Protocol Selection Semantics

This resource does not take a top-level user-settable `protocol` argument. Instead, protocol is inferred from the selected block.

Examples:

- `vxlan {}` means protocol is `vxlan`
- `mpls_plus_vlan {}` means protocol is `mplsPlusVlan`
- `geneve {}` means protocol is `geneve`

This block-driven structure makes invalid mixed-mode configurations easier to reject.

## Linking and Topology Notes

Because `gigamon_link` accepts application typed IDs as endpoints, `gigamon_app_header_stripping.id` can participate in Monitoring Session topology just like other app resources.

Typical patterns include:

- map → header stripping
- header stripping → tunnel
- header stripping → application
- application → header stripping

Important `gigamon_link` behavior:

- `source_aep_id` is required when the link source is:
    - a map, or
    - a load balancing app
- `source_aep_id` is not valid for header stripping as source

So when header stripping is the source or destination, you normally only provide:

- `monitoring_session_id`
- `source_id`
- `dest_id`

## Import

Import support is not supported

## Summary

Using `gigamon_app_header_stripping`, you can:

- create a header stripping application instance in a Monitoring Session
- choose exactly one stripping mode through nested protocol blocks
- use both simple and parameterized stripping modes
- benefit from provider-side validation for generic mode
- link it into a Monitoring Session topology using `gigamon_link`
- manage its full lifecycle through Terraform with a stable typed app ID
