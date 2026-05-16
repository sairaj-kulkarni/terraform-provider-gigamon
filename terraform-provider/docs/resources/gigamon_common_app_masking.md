
# Resource: `gigamon_app_masking`

The **Masking application** runs inside a **Monitoring Session** and creates a masking application instance in Fabric Manager.

- `gigamon_app_masking` represents a **masking application instance** attached to one Monitoring Session.
- It supports protocol-aware masking with:
    - a required `alias`
    - a required `monitoring_session_id`
    - a protocol selector
    - offset-based masking fields for most protocols
    - SIP-specific masking through `content_type`
    - a computed typed `id`
- The app is created, updated, and deleted through Monitoring Session `"application"` operations.

This resource supports two semantic modes:

- **Non-SIP masking**
    - uses `protocol`, `offset`, `length`, and `pattern`
- **SIP masking**
    - uses `protocol = "sip"` and `content_type`
    - does **not** allow `length` or `pattern`

Provider-side validation enforces these combinations during plan and create/update.

## Example Usage

### Minimal masking application for non-SIP traffic

```hcl
resource "gigamon_app_masking" "masking" {
  alias                 = "masking-main"
  monitoring_session_id = gigamon_monitoring_session.ms.id

  protocol = "tcp"
  offset   = 64
  length   = 8
  pattern  = "0xFF"
}
```

### Masking with IPv4-relative offset

```hcl
resource "gigamon_app_masking" "masking_ipv4" {
  alias                 = "mask-ipv4"
  monitoring_session_id = gigamon_monitoring_session.ms.id

  protocol = "ipv4"
  offset   = 32
  length   = 4
  pattern  = "0x00"
}
```

### SIP masking

```hcl
resource "gigamon_app_masking" "masking_sip" {
  alias                 = "mask-sip"
  monitoring_session_id = gigamon_monitoring_session.ms.id

  protocol     = "sip"
  content_type = "header"
}
```

### Linking a map to masking, then masking to another object

```hcl
resource "gigamon_app_masking" "masking" {
  alias                 = "masking-main"
  monitoring_session_id = gigamon_monitoring_session.ms.id

  protocol = "tcp"
  offset   = 64
  length   = 8
  pattern  = "0xAA"
}

resource "gigamon_link" "map_to_masking" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  source_id             = gigamon_map_traffic.map.id
  source_aep_id         = 2
  dest_id               = gigamon_app_masking.masking.id
}

resource "gigamon_link" "masking_to_tunnel" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  source_id             = gigamon_app_masking.masking.id
  dest_id               = gigamon_tunnel_out_gre.out.id
}
```

In `gigamon_link`, `source_aep_id` is required only when the **source** is a **map** or a **load balancing app**. It is not used when the source is a masking app.

## Argument Reference

### Required

- **`alias`** (String)  
  Name for this masking application.

- **`monitoring_session_id`** (String)  
  Monitoring Session on which this app is deployed.  
  Changing this forces a new `gigamon_app_masking` resource to be created.

### Optional

- **`protocol`** (String)  
  Protocol relative to which masking is applied.  
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
    - `"sip"`

- **`offset`** (Number)  
  Offset at which masking starts, relative to the selected protocol.  
  Optional, computed, default: `64`.

- **`length`** (Number)  
  Number of bytes to mask from `offset`.  
  Required for all **non-SIP** protocols.  
  Not valid for `protocol = "sip"`.  
  Minimum: `1`.

- **`pattern`** (String)  
  One-byte hex value written as the masking pattern.  
  Required for all **non-SIP** protocols.  
  Not valid for `protocol = "sip"`.

  Must match a one-byte hex format such as:

    - `0x08`
    - `0xFF`


- **`content_type`** (String)  
  Required when `protocol = "sip"`.  
  Not valid for non-SIP protocols.

## Attributes Reference

In addition to the arguments above, `gigamon_app_masking` exports:

- **`id`** (String)  
  Typed ID of this app instance for later use.

This typed ID is what you typically use in resources like `gigamon_link`.

## Validation Rules

The provider applies semantic validation beyond basic schema validation.

### SIP mode

When `protocol = "sip"`:

- `content_type` must be specified
- `length` must not be specified
- `pattern` must not be specified

### Non-SIP mode

When `protocol != "sip"`:

- `length` must be specified
- `pattern` must be specified
- `content_type` must not be specified

This validation runs during:

- plan modification
- create
- update

So invalid combinations fail early, before or during apply.

## FM Mapping

The provider maps Terraform data to an FM application payload shaped like:

```json
{
  "alias": "<alias>",
  "name": "masking",
  "protocol": "<protocol>",
  "offset": 64,
  "length": 8,
  "pattern": "0xFF",
  "contentType": "",
  "id": "<raw-fm-id-on-update>"
}
```

For SIP mode, the important payload field is `contentType` instead of `length` and `pattern`.

Key behavior:

- FM application `Name` is fixed as **`"masking"`**.
- On create, FM returns a raw application UUID.
- The provider wraps that UUID into a **typed application ID** and stores it in Terraform state.

## Behavior and Lifecycle

### Monitoring Session scope

- `gigamon_app_masking` belongs to exactly **one** Monitoring Session.
- The provider manages it through Monitoring Session update operations with:
    - `EntityType = "application"`
    - `Operation = "create" | "update" | "delete"`

### Create

On **Create**, the provider:

1. Reads the Terraform plan into `MaskingModel`.
2. Validates the parameter combination:
    - SIP requires `content_type`
    - non-SIP requires `length` and `pattern`
3. Builds the FM payload with:
    - `Alias = alias`
    - `Name = "masking"`
    - `Protocol = protocol`
    - `Offset = offset`
    - `Length = length`
    - `Pattern = pattern`
    - `ContentType = content_type`
4. Calls Monitoring Session update with an `"application"` `"create"` operation.
5. Receives the FM UUID for the created app.
6. Wraps that UUID into a typed app ID and stores it as `id` in Terraform state.

### Read

On **Read**, the provider:

1. Reads prior Terraform state.
2. Converts the typed `id` back to the raw FM UUID.
3. Fetches the app from the Monitoring Session using app name `"masking"`.
4. If FM reports object not found, the resource is removed from state.
5. Overlays FM-owned values into state:
    - `alias`
    - `protocol`
    - `offset`
    - if protocol is `sip`, `content_type`
    - otherwise, `length` and `pattern`

This protocol-sensitive overlay avoids incorrectly mixing SIP and non-SIP fields in state.

### Update

On **Update**, the provider:

1. Reads the desired plan.
2. Validates the parameter combination again.
3. Builds the FM payload with `Name = "masking"`.
4. Converts the typed state ID into the raw FM UUID.
5. Calls Monitoring Session update with an `"application"` `"update"` operation.
6. Writes the updated plan back to state after overlaying FM-owned fields.

### Delete

On **Delete**, the provider:

1. Reads existing state.
2. Converts the typed `id` to raw FM UUID.
3. Calls Monitoring Session update with an `"application"` `"delete"` operation.
4. Sends a minimal delete payload with:
    - `Id = <raw uuid>`
    - `Name = "Application"`

## Protocol Semantics

The key masking behavior is driven by `protocol`.

### `protocol = "none"`

Offset is applied starting from the first byte of the packet.

### Other non-SIP protocols

For values like `ipv4`, `tcp`, `udp`, `https`, `ssh`, `gtp`, and related variants:

- masking begins relative to the selected protocol header
- `offset`, `length`, and `pattern` define what gets rewritten

### `protocol = "sip"`

SIP masking uses a different semantic path:

- `content_type` determines what SIP-related content is masked
- `length` and `pattern` are disallowed

## Linking and Topology Notes

Because `gigamon_link` accepts application typed IDs as endpoints, `gigamon_app_masking.id` can participate in Monitoring Session topology just like other app resources.

Typical patterns include:

- map → masking
- masking → tunnel
- masking → application
- application → masking

Important `gigamon_link` behavior:

- `source_aep_id` is required when the link source is:
    - a map, or
    - a load balancing app
- `source_aep_id` is not valid for masking as source

So when masking is the source or destination, you normally only provide:

- `monitoring_session_id`
- `source_id`
- `dest_id`

## Import

Import is not supported.

## Summary

Using `gigamon_app_masking`, you can:

- create a masking application instance in a Monitoring Session
- apply byte-pattern masking for non-SIP protocols
- apply SIP-specific masking using `content_type`
- link it into a Monitoring Session topology using `gigamon_link`
- manage its full lifecycle through Terraform with a stable typed app ID
