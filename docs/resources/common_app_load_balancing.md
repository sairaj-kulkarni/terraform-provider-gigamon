
# Resource: `gigamon_app_load_balancing`

The **Load Balancing application** runs inside a **Monitoring Session** and creates a load balancing application instance in Fabric Manager.

- `gigamon_app_load_balancing` represents a **load balancing application instance** attached to one Monitoring Session.
- It supports two mutually exclusive operating modes:
    - **stateless**
    - **enhanced**
- The resource contains:
    - a required `alias`
    - a required `monitoring_session_id`
    - an optional `description`
    - exactly one of `stateless` or `enhanced`
    - optional `group` blocks
    - a computed typed `id`

This resource has more provider-side validation than Dedup, Masking, or Slicing because link safety and mode transitions are enforced explicitly.

## Example Usage

### Stateless load balancing

```hcl
resource "gigamon_app_load_balancing" "lb" {
  alias                 = "lb-main"
  monitoring_session_id = gigamon_monitoring_session.ms.id
  description           = "Stateless load balancing for AMI flow"

  stateless {
    hash_fields    = "ipOnly"
    field_location = "outer"
  }

  group {
    aep_id = 2
    weight = 50
  }

  group {
    aep_id = 3
    weight = 50
  }
}
```

### Stateless load balancing using `fiveTuple`

```hcl
resource "gigamon_app_load_balancing" "lb" {
  alias                 = "lb-five-tuple"
  monitoring_session_id = gigamon_monitoring_session.ms.id

  stateless {
    hash_fields    = "fiveTuple"
    field_location = "inner"
  }

  group {
    aep_id = 2
    weight = 60
  }

  group {
    aep_id = 4
    weight = 40
  }
}
```

### Stateless load balancing using `gtpuTeid`

```hcl
resource "gigamon_app_load_balancing" "lb_gtpu" {
  alias                 = "lb-gtpu"
  monitoring_session_id = gigamon_monitoring_session.ms.id

  stateless {
    hash_fields = "gtpuTeid"
  }

  group {
    aep_id = 2
    weight = 50
  }

  group {
    aep_id = 3
    weight = 50
  }
}
```

### Stateless load balancing using `greFlowid`

```hcl
resource "gigamon_app_load_balancing" "lb_gre" {
  alias                 = "lb-gre"
  monitoring_session_id = gigamon_monitoring_session.ms.id

  stateless {
    hash_fields = "greFlowid"
  }
}
```

When `hash_fields = "greFlowid"`:

- `field_location` must not be set
- `group` blocks must not be set
- the provider automatically sends an internal FM group payload

### Enhanced load balancing

```hcl
resource "gigamon_app_load_balancing" "lb_enhanced" {
  alias                 = "lb-enhanced"
  monitoring_session_id = gigamon_monitoring_session.ms.id
  description           = "Enhanced load balancing profile"

  enhanced {
    profile = "FmAuto-StatefulApplication-profile"
  }
}
```

### Linking a map to load balancing, then load balancing to another object

```hcl
resource "gigamon_app_load_balancing" "lb" {
  alias                 = "lb-main"
  monitoring_session_id = gigamon_monitoring_session.ms.id

  stateless {
    hash_fields    = "ipAndPort"
    field_location = "outer"
  }

  group {
    aep_id = 2
    weight = 50
  }

  group {
    aep_id = 3
    weight = 50
  }
}

resource "gigamon_link" "map_to_lb" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  source_id             = gigamon_map_traffic.map.id
  source_aep_id         = 2
  dest_id               = gigamon_app_load_balancing.lb.id
}

resource "gigamon_link" "lb_to_tunnel" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  source_id             = gigamon_app_load_balancing.lb.id
  source_aep_id         = 2
  dest_id               = gigamon_tunnel_out_gre.out.id
}
```

When a load balancing app is the **source** of a `gigamon_link`, `source_aep_id` is required, because the link must identify which LB output group/AEP is being used as the source.

## Argument Reference

### Required

- **`alias`** (String)  
  Name for this load balancing application.

- **`monitoring_session_id`** (String)  
  Monitoring Session on which this app is deployed.  
  Changing this forces a new `gigamon_app_load_balancing` resource to be created.

### Optional

- **`description`** (String)  
  Optional description for this load balancing app.

## Attributes Reference

In addition to the arguments and blocks below, `gigamon_app_load_balancing` exports:

- **`id`** (String)  
  Typed ID of this app instance for later use.

This typed ID is what you typically use in resources like `gigamon_link`.

## Block Reference

Exactly one of `stateless` or `enhanced` must be present.

### `stateless`

```hcl
stateless {
  hash_fields    = string
  field_location = string
}
```

- **`hash_fields`** (String)  
  Required semantically when `stateless` is used.

  Allowed values:

    - `"ipOnly"`
    - `"ipAndPort"`
    - `"fiveTuple"`
    - `"gtpuTeid"`
    - `"greFlowid"`

- **`field_location`** (String)  
  Used for applicable hash modes.  
  Optional/computed in schema, but required semantically for most stateless modes.

  Allowed values:

  - `"inner"`
  - `"outer"`

Special rules:

- for `hash_fields = "greFlowid"`, `field_location` must not be set
- for `hash_fields = "gtpuTeid"`, `field_location` must not be set
- for other stateless hash modes, `field_location` is required

### `enhanced`

```hcl
enhanced {
  profile = string
}
```

- **`profile`** (String)  
  Required semantically when `enhanced` is used.

  Allowed values:

  - `"FmAuto-StatefulApplication-profile"`
  - `"FmAuto-EgressScale-profile"`

### `group`

```hcl
group {
  aep_id = number
  weight = number
}
```

- **`aep_id`** (Number)  
  Application Endpoint ID.  
  Required.  
  Range: `2` to `64`.

- **`weight`** (Number)  
  Weight for this endpoint.  
  Required.  
  Range: `1` to `100`.

`group` blocks are used for stateless load balancing, except the `greFlowid` special case where they are provider-managed and must not be set by the user.

## Validation Rules

The provider applies both schema-level and semantic validation.

### Exactly one mode

A config validator enforces that **exactly one** of the following is present:

- `stateless`
- `enhanced`

You cannot configure both modes in the same resource.

### Stateless validation

When `stateless` is used:

- `hash_fields` must be specified

Additional rules depend on the hash mode:

#### `greFlowid`

- `field_location` must not be specified
- `group` blocks must not be specified

#### `gtpuTeid`

- `field_location` must not be specified

#### Other stateless hashes

For:

- `ipOnly`
- `ipAndPort`
- `fiveTuple`

`field_location` must be specified.

### Enhanced validation

When `enhanced` is used:

- `profile` must be specified and non-empty

### Group validation

When `hash_fields != "greFlowid"` and `group` blocks are used:

- at least **2** groups are required
- at most **63** groups are allowed
- `aep_id` values must be unique
- the sum of all `weight` values must be **<= 100**

Important note: the provider mirrors FM/UI behavior and allows total weight **less than 100**, but rejects totals **greater than 100**.

## FM Mapping

The provider maps Terraform data to an FM application payload shaped like:

```json
{
  "alias": "lb-main",
  "name": "lb",
  "description": "Stateless load balancing for AMI flow",
  "stateless": {
    "hashFields": "ipOnly",
    "fieldLocation": "outer"
  },
  "lbg": [
    {
      "aepId": 2,
      "weight": 50
    },
    {
      "aepId": 3,
      "weight": 50
    }
  ],
  "id": "<raw-fm-id-on-update>"
}
```

For enhanced mode, the FM payload uses:

```json
{
  "enhanced": {
    "profile": "FmAuto-StatefulApplication-profile"
  }
}
```

Key behavior:

- FM application `Name` is fixed as **`"lb"`**.
- On create, FM returns a raw application UUID.
- The provider wraps that UUID into a **typed application ID** and stores it in Terraform state.

### `greFlowid` special FM behavior

For `hash_fields = "greFlowid"`:

- the provider automatically sets `fieldLocation = "outer"` in FM
- the provider automatically sends one internal group:

```json
[
  {
    "aepId": 2,
    "weight": 1
  }
]
```

This internal FM payload is intentionally hidden from normal Terraform configuration, and the provider does not expose `group` back into Terraform state for `greFlowid`.

## Behavior and Lifecycle

### Monitoring Session scope

- `gigamon_app_load_balancing` belongs to exactly **one** Monitoring Session.
- The provider manages it through Monitoring Session update operations with:
  - `EntityType = "application"`
  - `Operation = "create" | "update" | "delete"`

### Create

On **Create**, the provider:

1. Reads the Terraform plan into `LoadBalancingModel`.
2. Validates mode, hash/profile, and group semantics.
3. Builds the FM payload with:
    - `Alias = alias`
    - `Name = "lb"`
    - `Description = description`
    - either `Stateless` or `Enhanced`
    - `Lbg` when applicable
4. Calls Monitoring Session update with an `"application"` `"create"` operation.
5. Receives the FM UUID for the created app.
6. Overlays computed/group behavior into state.
7. Wraps the FM UUID into a typed app ID and stores it as `id`.

### Read

On **Read**, the provider:

1. Reads prior Terraform state.
2. Converts the typed `id` back to the raw FM UUID.
3. Fetches the app from the Monitoring Session using app name `"lb"`.
4. If FM reports object not found, the resource is removed from state.
5. Overlays FM-owned values into state:
    - `alias`
    - `description`
    - `stateless` or `enhanced`
    - `group`, except for `greFlowid`

Special read behavior:

- if the hash mode is `greFlowid`, `field_location` is returned as null in Terraform state
- if the hash mode is `greFlowid`, provider-managed FM groups are not surfaced as Terraform `group` blocks

### Update

On **Update**, the provider:

1. Reads both the desired plan and prior state.
2. Validates mode, hash/profile, and group semantics.
3. Performs additional link-safety validation.
4. Builds the FM payload with `Name = "lb"`.
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

## Link-Aware Update Restrictions

This resource includes extra safety checks that depend on `gigamon_link`.

### Changing to or from `greFlowid`

The provider blocks a transition:

- from non-`greFlowid` to `greFlowid`
- from `greFlowid` to any other stateless hash
- from `greFlowid` to enhanced
- from enhanced to `greFlowid`

if the load balancing app is still used as the **source** of any existing links in the same Monitoring Session.

This is enforced because Terraform cannot safely auto-rewrite all dependent `gigamon_link` resources during such a structural change.

In practice, you must first update or delete the affected `gigamon_link` resources, then change the LB hash mode.

### Removing groups that are still linked

When a stateless load balancing app has groups and one of those groups is referenced by a link as `source_aep_id`, the provider blocks removing that group until the link is updated or deleted.

This protects topologies like:

- load balancing app source with `source_aep_id = 2`
- user removes group `aep_id = 2`

The update is rejected until the dependent link is handled.

## Linking and Topology Notes

Because `gigamon_link` accepts application typed IDs as endpoints, `gigamon_app_load_balancing.id` can participate in Monitoring Session topology just like other app resources.

Typical patterns include:

- map → load balancing
- load balancing → tunnel
- load balancing → application
- application → load balancing

Important `gigamon_link` behavior:

- `source_aep_id` is required when the link source is:
    - a map, or
    - a load balancing app
- `source_aep_id` is not valid for other app types as source

So when load balancing is the source, you must provide:

- `monitoring_session_id`
- `source_id`
- `source_aep_id`
- `dest_id`

## Import

Import support is not supported

## Summary

Using `gigamon_app_load_balancing`, you can:

- create a load balancing application instance in a Monitoring Session
- choose exactly one mode: stateless or enhanced
- configure weighted output groups for stateless load balancing
- use special hash modes like `gtpuTeid` and `greFlowid`
- link it into a Monitoring Session topology using `gigamon_link`
- benefit from provider-side safety checks that prevent invalid group removals and unsafe hash-mode transitions while links still exist
- manage its full lifecycle through Terraform with a stable typed app ID