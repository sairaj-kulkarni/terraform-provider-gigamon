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

---
page_title: "Traffic Map"
subcategory: "Maps"
description: "Manage traffic maps in Gigamon FM."
---

# gigamon_traffic_map

## Resource: `gigamon_traffic_map`

A **traffic map** classifies and forwards monitored traffic inside a Monitoring Session.
Each traffic map contains one or more **rule sets** (`rule_sets`) with **pass** and/or **drop** rules.

Each rule set is bound to an **AEP ID** (`aep_id`), which is the output endpoint consumed by `gigamon_link`
to connect the map to applications, tunnels, or other maps.

**Related map types**
> - `gigamon_inclusion_map` – ATS inclusion map (only `pass_rules` are allowed)
> - `gigamon_exclusion_map` – ATS exclusion map (only `drop_rules` are allowed)

---

## Example Usage

### Simple traffic map with one rule set

```hcl
resource "gigamon_traffic_map" "web_traffic" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  name                  = "web-traffic-map"
  description           = "Select HTTP/HTTPS traffic from the frontend tier"

  rule_sets = [
    {
      rule_set_id = "1"
      priority    = 1   # lower number = higher priority
      aep_id      = 10  # output AEP ID consumed by gigamon_link.source_aep_id

      pass_rules = [
        {
          rule_id = 1

          ipv4_source = {
            address   = "10.0.0.0"
            cidr_mask = "24"
          }

          ipv4_protocol = {
            protocol_min    = 6
            protocol_max    = 6
            protocol_subset = "all"
          }
        }
      ]
    }
  ]
}
```

### Linking a traffic map to an application via `source_aep_id`

```hcl
resource "gigamon_link" "web_to_app" {
  monitoring_session_id = gigamon_monitoring_session.ms.id

  # Source is the traffic map; source_aep_id must match rule_sets.aep_id
  source_id     = gigamon_traffic_map.web_traffic.id
  source_aep_id = 10

  dest_id = gigamon_application.app_ats.id
}
```

- `rule_sets[*].aep_id` selects which logical output inside the map the matched traffic hits.
- `gigamon_link.source_aep_id` tells FM which AEP of the source map to connect to the destination.
- `source_aep_id` is **required** when `source_id` refers to any map or load-balancing app, and **invalid** otherwise.

---

### Multiple rule sets and rules from variables

Use a `for` expression to build `rule_sets` and the nested `pass_rules` / `drop_rules` lists
from structured variables, eliminating repetition when managing multiple application tiers or
traffic classes.

```hcl
# ── Variables ────────────────────────────────────────────────────────────────

variable "traffic_tiers" {
  description = "One rule set per application tier; each tier captures its own subnet ranges"
  type = list(object({
    name       = string          # used to label rules, not sent to FM
    priority   = number          # 1–5, lower = higher priority
    aep_id     = number          # 2–63, output AEP for gigamon_link
    src_cidrs  = list(string)    # one pass_rule per CIDR, e.g. ["10.0.1.0/24", "10.0.2.0/24"]
    dst_cidrs  = list(string)    # one pass_rule per CIDR (or empty list to skip)
    protocols  = list(number)    # one pass_rule per protocol number (or empty list to skip)
  }))
}

# ── Locals ────────────────────────────────────────────────────────────────────

locals {
  rule_sets = [
    for i, tier in var.traffic_tiers : {
      rule_set_id = tostring(i + 1)
      priority    = tier.priority
      aep_id      = tier.aep_id

      # One pass_rule per source CIDR — rules within a rule set are OR-combined
      pass_rules = concat(
        [
          for j, cidr in tier.src_cidrs : {
            rule_id = j + 1
            ipv4_source = {
              address   = cidrhost(cidr, 0)
              cidr_mask = tostring(split("/", cidr)[1])
            }
          }
        ],
        [
          for j, cidr in tier.dst_cidrs : {
            rule_id = length(tier.src_cidrs) + j + 1
            ipv4_destination = {
              address   = cidrhost(cidr, 0)
              cidr_mask = tostring(split("/", cidr)[1])
            }
          }
        ],
        [
          for j, proto in tier.protocols : {
            rule_id = length(tier.src_cidrs) + length(tier.dst_cidrs) + j + 1
            ipv4_protocol = {
              protocol_min = proto
            }
          }
        ],
      )

      # No drop_rules in this example; add a drop_rules list here if needed
      drop_rules = []
    }
  ]
}

# ── Traffic map ───────────────────────────────────────────────────────────────

resource "gigamon_traffic_map" "tiered" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  name                  = "tiered-traffic-map"

  rule_sets = local.rule_sets
}
```

Example `terraform.tfvars`:

```hcl
traffic_tiers = [
  {
    name      = "frontend"
    priority  = 1
    aep_id    = 10
    src_cidrs = ["10.0.1.0/24", "10.0.2.0/24"]
    dst_cidrs = []
    protocols = [6, 17]   # TCP and UDP
  },
  {
    name      = "backend"
    priority  = 2
    aep_id    = 11
    src_cidrs = ["10.0.3.0/24"]
    dst_cidrs = ["10.0.4.0/24"]
    protocols = []
  },
]
```

### Rule sets with both pass and drop rules per tier

```hcl
variable "mixed_tiers" {
  type = list(object({
    priority       = number
    aep_id         = number
    allowed_cidrs  = list(string)   # → pass_rules
    blocked_cidrs  = list(string)   # → drop_rules
  }))
}

resource "gigamon_traffic_map" "mixed" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  name                  = "mixed-pass-drop-map"

  rule_sets = [
    for i, tier in var.mixed_tiers : {
      rule_set_id = tostring(i + 1)
      priority    = tier.priority
      aep_id      = tier.aep_id

      pass_rules = [
        for j, cidr in tier.allowed_cidrs : {
          rule_id = j + 1
          ipv4_source = {
            address   = cidrhost(cidr, 0)
            cidr_mask = tostring(split("/", cidr)[1])
          }
        }
      ]

      drop_rules = [
        for j, cidr in tier.blocked_cidrs : {
          rule_id = j + 1
          ipv4_source = {
            address   = cidrhost(cidr, 0)
            cidr_mask = tostring(split("/", cidr)[1])
          }
        }
      ]
    }
  ]
}
```

---

## Argument Reference

### Top-level arguments

* `monitoring_session_id` (String, **Required**) – ID of the Monitoring Session that owns this map. Typically set from `gigamon_monitoring_session.<name>.id`. Changing this forces a new resource.
* `name` (String, **Required**) – Name of the traffic map, unique within the Monitoring Session.
* `description` (String, Optional) – Free-form description for this traffic map.
* `rule_sets` (List of Objects, **Required**) – One or more rule sets that define how traffic is matched and forwarded. At least **1** and at most **5** rule sets per map.

---

## `rule_sets`

```hcl
rule_sets = [
  {
    rule_set_id = "1"
    priority    = 1
    aep_id      = 10

    pass_rules = [{ ... }]
    drop_rules = [{ ... }]
  }
]
```

* `rule_set_id` (String, **Required**) – Identifier of this rule set within the map. Must be a string `"1"`–`"5"`.
* `priority` (Number, **Required**) – Priority of this rule set. Range: **1–5**.
  Lower value = higher priority. When multiple rule sets match, the one with the lowest value is evaluated first.
* `aep_id` (Number, **Required**) – Output AEP endpoint ID for this rule set. Range: **2–63**.
  This value must be referenced by `gigamon_link.source_aep_id` to connect map output to a destination.
* `pass_rules` (List of Objects, Optional) – Rules for traffic to **forward** to `aep_id`. At least one rule is required when this block is specified. At least one of `pass_rules` or `drop_rules` must be present per rule set.
* `drop_rules` (List of Objects, Optional) – Rules for traffic to **discard**. At least one rule is required when this block is specified. At least one of `pass_rules` or `drop_rules` must be present per rule set.

> **Traffic map**: both `pass_rules` and `drop_rules` are allowed in the same rule set.

> **Inclusion map** (`gigamon_inclusion_map`): only `pass_rules` are permitted.

> **Exclusion map** (`gigamon_exclusion_map`): only `drop_rules` are permitted.

---

## `pass_rules` / `drop_rules`

Each item represents one rule. All rule elements inside a single rule are combined with **AND**.
Multiple rules within `pass_rules` or `drop_rules` are combined with **OR**.

```hcl
pass_rules = [
  {
    rule_id = 1

    ether_type = { ... }
    ipv4_source = { ... }
    ipv4_protocol = { ... }
    # ... other rule element blocks ...
  }
]
```

* `rule_id` (Number, **Required**) – Identifier of this rule within the rule set. Recommended range **1–5**.

Each rule may include zero or more of the following match condition blocks. At least one must be present.

- `ether_type` – Match on EtherType / TPID
- `l2_src_mac` – Match on source MAC address
- `l2_dst_mac` – Match on destination MAC address
- `ip_version` – Match on IP version (v4 or v6)
- `ipv4_source` – Match on IPv4 source address/range
- `ipv4_destination` – Match on IPv4 destination address/range
- `ipv6_source` – Match on IPv6 source address/range
- `ipv6_destination` – Match on IPv6 destination address/range
- `vm_name_source` – Match on source VM name prefix
- `vm_name_destination` – Match on destination VM name prefix
- `vm_tag_source` – Match on source VM tag key/value
- `vm_tag_destination` – Match on destination VM tag key/value
- `ipv4_dscp` – Match on IPv4 DSCP code point
- `ipv6_dscp` – Match on IPv6 DSCP code point
- `ipv4_fragmentation` – Match on IPv4 fragmentation mode
- `ipv4_protocol` – Match on IPv4 protocol number
- `erspan_id` – Match on ERSPAN ID
- `ipv4_ttl` – Match on IPv4 TTL
- `ipv4_tos` – Match on IPv4 TOS byte
- `gre_key` – Match on GRE key

---

## Rule Element Blocks

### `ether_type`

```hcl
ether_type = {
  nested_level_count = 0
  ether_type         = "0x0800"
  # or use a range:
  ether_type_start   = "0x0800"
  ether_type_end     = "0x86DD"
}
```

* `nested_level_count` (Number, Optional, default `0`) – VLAN nesting level; `0` = any level.
* `ether_type` (String, Optional) – Single EtherType hex value with `0x` prefix (e.g. `"0x0800"`, `"0x86DD"`). Mutually exclusive with `ether_type_start`. Exactly one of `ether_type` or `ether_type_start` must be provided.
* `ether_type_start`, `ether_type_end` (String, Optional) – EtherType range; both must be set together. Mutually exclusive with `ether_type`.

---

### `l2_src_mac` / `l2_dst_mac`

```hcl
l2_src_mac = {
  nested_level_count = 0
  mac_address        = "00:11:22:33:44:55"
  mac_address_mask   = "FF:FF:FF:FF:FF:FF"
}
# or as a range:
l2_dst_mac = {
  mac_address_start = "00:11:22:33:44:00"
  mac_address_end   = "00:11:22:33:44:FF"
}
```

* `nested_level_count` (Number, Optional, default `0`) – MAC layer to inspect for MAC-in-MAC; `0` = any.
* `mac_address` (String, Optional) – Single MAC address to match (e.g. `"00:1A:2B:3C:4D:5E"`). Mutually exclusive with `mac_address_start`. Exactly one of `mac_address` or `mac_address_start` must be provided.
* `mac_address_mask` (String, Optional, default `FF:FF:FF:FF:FF:FF`) – Bitmask applied to `mac_address` to define a range. Requires `mac_address`.
* `mac_address_start`, `mac_address_end` (String, Optional) – MAC address range; both must be set together. Mutually exclusive with `mac_address`.

---

### `ip_version`

```hcl
ip_version = {
  nested_level_count = 0
  ip_version         = "v4"  # or "v6"
}
```

* `nested_level_count` (Number, Optional, default `0`)
* `ip_version` (String, **Required**) – `"v4"` or `"v6"`.

---

### `ipv4_source` / `ipv4_destination`

```hcl
ipv4_source = {
  nested_level_count = 0
  address            = "10.0.0.0"
  cidr_mask          = "24"
  # alternatives: address_max or netmask
}
```

* `nested_level_count` (Number, Optional, default `0`, range 0–3) – IPv4 header depth for tunneled traffic.
* `address` (String, **Required**) – IPv4 address (start of range or network address).
* `address_max` (String, Optional) – Range end; mutually exclusive with `cidr_mask` and `netmask`.
* `cidr_mask` (String, Optional) – CIDR prefix length `"1"`–`"32"`; mutually exclusive with `address_max` and `netmask`.
* `netmask` (String, Optional) – Dotted-decimal netmask (must be contiguous); mutually exclusive with `address_max` and `cidr_mask`.

---

### `ipv6_source` / `ipv6_destination`

```hcl
ipv6_source = {
  nested_level_count = 0
  address            = "2001:db8::"
  cidr_mask          = "64"
}
```

* `nested_level_count` (Number, Optional, default `0`, range 0–3)
* `address` (String, **Required**) – IPv6 address or start of range.
* `address_max` (String, Optional) – IPv6 range end; mutually exclusive with `cidr_mask` and `netmask`.
* `cidr_mask` (String, Optional) – `"1"`–`"128"`; mutually exclusive with `address_max` and `netmask`.
* `netmask` (String, Optional) – IPv6 netmask as 8 uppercase hextets.

---

### `vm_name_source` / `vm_name_destination`

```hcl
vm_name_source = {
  vm_name_prefix = "frontend-"
}
```

* `vm_name_prefix` (String, **Required**) – Prefix of the VM name to match (exact prefix, no wildcards).
  For vSphere this is the VM name; for clouds it is the VM name as shown in GigaVUE-FM.

---

### `vm_tag_source` / `vm_tag_destination`

```hcl
vm_tag_source = {
  tag_name  = "environment"
  tag_value = "prod"
}
```

* `tag_name` (String, **Required**) – Tag key (vSphere tag name or cloud tag key).
* `tag_value` (String, **Required**) – Tag value (or vSphere tag category).

---

### `ipv4_dscp` / `ipv6_dscp`

```hcl
ipv4_dscp = {
  nested_level_count = 0
  dscp               = "af11"
}
```

* `nested_level_count` (Number, Optional, default `0`, range `0–3`) – Which IP header to inspect in tunneled/stacked traffic: `0`=any, `1`=outermost, `2`=second, `3`=third.
* `dscp` (String, **Required**) – DSCP code point. Valid values: `af11`, `af12`, `af13`, `af21`, `af22`, `af23`, `af31`, `af32`, `af33`, `af41`, `af42`, `af43`, `ef`.

---

### `ipv4_fragmentation`

```hcl
ipv4_fragmentation = {
  nested_level_count = 0
  mode               = "unfragmented_only"
}
```

* `nested_level_count` (Number, Optional, default `0`, range 0–3)
* `mode` (String, **Required**) – One of:
  * `unfragmented_only`
  * `any_fragment`
  * `non_first_fragments`
  * `first_fragment_only`
  * `first_or_unfragmented`

---

### `ipv4_protocol`

```hcl
ipv4_protocol = {
  nested_level_count = 0
  protocol_min       = 6
  protocol_max       = 6
  protocol_subset    = "all"
}
```

* `nested_level_count` (Number, Optional, default `0`, range 0–3)
* `protocol_min` (Number, **Required**) – Lower bound (inclusive), 0–255.
* `protocol_max` (Number, Optional) – Upper bound (inclusive), 0–255. Must be greater than `protocol_min` if set.
* `protocol_subset` (String, Optional, default `"all"`) – `"all"`, `"even"`, or `"odd"` (requires `protocol_max` for even/odd).

---

### `erspan_id`

```hcl
erspan_id = {
  erspan_id_min    = 1
  erspan_id_max    = 10
  erspan_id_subset = "all"
}
```

* `erspan_id_min` (Number, **Required**) – Lower bound (inclusive), 1–1024.
* `erspan_id_max` (Number, Optional) – Upper bound (inclusive), 1–1024; must be greater than `erspan_id_min`.
* `erspan_id_subset` (String, Optional, default `"all"`) – `"all"`, `"even"`, or `"odd"` (requires `erspan_id_max` for even/odd).

---

### `ipv4_ttl`

```hcl
ipv4_ttl = {
  nested_level_count = 0
  ttl_min            = 64
  ttl_max            = 128
  ttl_subset         = "all"
}
```

* `nested_level_count` (Number, Optional, default `0`, range 0–3)
* `ttl_min` (Number, **Required**) – 0–255.
* `ttl_max` (Number, Optional) – 0–255; must be greater than `ttl_min` if set.
* `ttl_subset` (String, Optional, default `"all"`) – `"all"`, `"even"`, or `"odd"` (requires `ttl_max` for even/odd).

---

### `ipv4_tos`

```hcl
ipv4_tos = {
  nested_level_count = 0
  tos_min            = "0A"
  tos_max            = "1F"
  tos_subset         = "all"
}
```

* `nested_level_count` (Number, Optional, default `0`, range 0–3)
* `tos_min` (String, **Required**) – 1-byte hex value (2 hex digits, e.g. `"0A"`).
* `tos_max` (String, Optional) – 1-byte hex value; must be greater than `tos_min` when set.
* `tos_subset` (String, Optional, default `"all"`) – `"all"`, `"even"`, or `"odd"` (requires `tos_max` for even/odd).

---

### `gre_key`

```hcl
gre_key = {
  gre_key_min    = "0000000A"
  gre_key_max    = "000000FF"
  gre_key_subset = "all"
}
```

* `gre_key_min` (String, **Required**) – Lower bound as 4-byte hex (8 hex digits).
* `gre_key_max` (String, Optional) – Upper bound as 4-byte hex; must be greater than `gre_key_min` if set.
* `gre_key_subset` (String, Optional, default `"all"`) – `"all"`, `"even"`, or `"odd"` (requires `gre_key_max` for even/odd).

---

## Attribute Reference

The following attributes are exported in addition to all arguments above:

* `id` (String) – Typed map ID used for linking and lifecycle operations.
  Format: `map::trafficMap::<uuid>`. Pass this as `source_id` in `gigamon_link`.

---

## ESXi VM Selection

On ESXi, you can restrict a traffic map to capture traffic only from specific VMs by their MAC addresses.
This is configured using the separate `gigamon_esxi_vm_selection` resource.

See `gigamon_esxi_vm_selection` for full documentation.

---

## Validation Notes

- `rule_sets` must contain between **1** and **5** items.
- `rule_set_id` must be a string between `"1"` and `"5"` (exactly one character).
- `priority` must be between **1** and **5**.
- `aep_id` must be between **2** and **63**.
- Each `rule_set` must contain at least one of `pass_rules` or `drop_rules`.
- When `pass_rules` is specified it must contain at least **1** rule.
- When `drop_rules` is specified it must contain at least **1** rule.
- `description`, if set, must be non-empty.

---

## Import

Import is **not supported** for `gigamon_traffic_map`.
