---
page_title: "Inclusion Map"
subcategory: "Maps"
description: "Manage inclusion maps in Gigamon FM."
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

# gigamon_inclusion_map

A **Gigamon inclusion map** classifies and forwards monitored traffic inside a Monitoring Session for **Automatic Target Selection (ATS)**.

An inclusion map contains one or more **rule sets** (`rule_sets`). Each rule set contains **pass rules only** and includes an **AEP ID** (`aep_id`) as part of the resource definition. Inclusion maps are **standalone ATS resources** and cannot be linked to or from other resources in Fabric Manager.

Unlike `gigamon_traffic_map`, an inclusion map **does not allow `drop_rules`**. If `drop_rules` is present in any `rule_sets` item, provider validation fails.

## Example Usage

### Simple inclusion map with one rule set

```hcl
resource "gigamon_inclusion_map" "frontend_ats" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  name                  = "frontend-inclusion-map"
  description           = "Include frontend production HTTP/HTTPS traffic for ATS"

  rule_sets = [
    {
      rule_set_id = "1"
      priority    = 1
      aep_id      = 10

      pass_rules = [
        {
          rule_id = 1

          vm_tag_source = {
            tag_name  = "environment"
            tag_value = "prod"
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

### Standalone ATS behavior

`gigamon_inclusion_map` is a standalone ATS resource in Fabric Manager.

### Multiple rule sets and pass rules from variables

Use a `for` expression to build `rule_sets` and nested `pass_rules` from a structured list
variable — useful when different VM tag groups need separate ATS rule sets with independent
match priorities.

```hcl
# ── Variables ────────────────────────────────────────────────────────────────

variable "ats_targets" {
  description = "ATS inclusion rule sets, one per team or workload group"
  type = list(object({
    priority = number        # 1–5
    aep_id   = number        # 2–63
    vm_tags  = list(object({ # one pass_rule per VM tag pair
      tag_name  = string
      tag_value = string
    }))
  }))
}

# ── Inclusion map ─────────────────────────────────────────────────────────────

resource "gigamon_inclusion_map" "ats_multi" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  name                  = "ats-vm-tag-inclusion"

  rule_sets = [
    for i, rs in var.ats_targets : {
      rule_set_id = tostring(i + 1)
      priority    = rs.priority
      aep_id      = rs.aep_id

      # Each VM tag pair becomes a separate pass_rule (OR-combined within the rule set)
      pass_rules = [
        for j, tag in rs.vm_tags : {
          rule_id = j + 1
          vm_tag_source = {
            tag_name  = tag.tag_name
            tag_value = tag.tag_value
          }
        }
      ]
    }
  ]
}
```

Example `terraform.tfvars`:

```hcl
ats_targets = [
  {
    priority = 1
    aep_id   = 10
    vm_tags  = [
      { tag_name = "environment", tag_value = "prod" },
      { tag_name = "environment", tag_value = "staging" },
    ]
  },
  {
    priority = 2
    aep_id   = 11
    vm_tags  = [
      { tag_name = "team", tag_value = "payments" },
      { tag_name = "team", tag_value = "auth" },
    ]
  },
]
```

### Multiple rule sets with subnet-based pass rules

```hcl
variable "ats_subnets" {
  description = "ATS inclusion sets driven by source subnets"
  type = list(object({
    priority = number
    aep_id   = number
    cidrs    = list(string)   # one pass_rule per CIDR
  }))
}

resource "gigamon_inclusion_map" "ats_subnets" {
  monitoring_session_id = gigamon_monitoring_session.ms.id
  name                  = "ats-subnet-inclusion"

  rule_sets = [
    for i, rs in var.ats_subnets : {
      rule_set_id = tostring(i + 1)
      priority    = rs.priority
      aep_id      = rs.aep_id

      pass_rules = [
        for j, cidr in rs.cidrs : {
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

## Argument Reference

### Top-level arguments

- `monitoring_session_id` (String, **Required**) – ID of the Monitoring Session that owns this inclusion map. Typically set from `gigamon_monitoring_session.<name>.id`. Changing this forces a new resource.
- `name` (String, **Required**) – Name of the inclusion map, unique within the Monitoring Session.
- `description` (String, Optional) – Free-form description for this inclusion map. If set, it must be non-empty.
- `rule_sets` (List of Objects, **Required**) – One or more rule sets that define how traffic is matched and forwarded. At least **1** and at most **5** rule sets per map.

## `rule_sets`

```hcl
rule_sets = [
  {
    rule_set_id = "1"
    priority    = 1
    aep_id      = 10

    pass_rules = [{ ... }]
  }
]
```

- `rule_set_id` (String, **Required**) – Identifier of this rule set within the map. Must be a string `"1"`–`"5"`.
- `priority` (Number, **Required**) – Priority of this rule set. Range: **1–5**. Lower value = higher priority.
- `aep_id` (Number, **Required**) – AEP identifier for this rule set. Range: **2–63**. This field is part of the inclusion map definition, but inclusion maps are standalone resources and are not linked by `gigamon_link`.
- `pass_rules` (List of Objects, **Required for inclusion maps**) – Rules for traffic to **forward** to `aep_id`. Must contain at least one rule when present.
- `drop_rules` – **Not supported** on inclusion maps.

> Inclusion maps are used for ATS target selection and support **only `pass_rules`**.
> If `drop_rules` appears in any `rule_sets` item, the provider returns a validation error.

## `pass_rules`

Each `pass_rules` item represents one rule. All rule elements inside a single rule are combined with **AND**. Multiple rules inside `pass_rules` are combined with **OR**.

```hcl
pass_rules = [
  {
    rule_id = 1

    ether_type = { ... }
    ipv4_source = { ... }
    ipv4_protocol = { ... }
  }
]
```

- `rule_id` (Number, **Required**) – Identifier of this rule within the rule set. Recommended range **1–5**.

Each rule may include zero or more of the following match condition blocks. In practice, at least one should be provided to define a meaningful match.

- `ether_type` – Match on EtherType / TPID
- `l2_src_mac` – Match on source MAC address
- `l2_dst_mac` – Match on destination MAC address
- `ip_version` – Match on IP version (`v4` or `v6`)
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

## Rule Element Blocks

### `ether_type`

```hcl
ether_type = {
  nested_level_count = 0
  ether_type         = "0x0800"
  # or use a range:
  # ether_type_start = "0x0800"
  # ether_type_end   = "0x86DD"
}
```

- `nested_level_count` (Number, Optional, default `0`) – VLAN nesting level. `0` means any position.
- `ether_type` (String, Optional) – Single EtherType hex value with `0x` prefix (e.g. `"0x0800"`, `"0x86DD"`). Mutually exclusive with `ether_type_start`. Exactly one of `ether_type` or `ether_type_start` must be provided.
- `ether_type_start`, `ether_type_end` (String, Optional) – EtherType range. Both must be set together. Mutually exclusive with `ether_type`.

### `l2_src_mac` / `l2_dst_mac`

```hcl
l2_src_mac = {
  nested_level_count = 0
  mac_address        = "00:11:22:33:44:55"
  mac_address_mask   = "FF:FF:FF:FF:FF:FF"
}
```

```hcl
l2_dst_mac = {
  mac_address_start = "00:11:22:33:44:00"
  mac_address_end   = "00:11:22:33:44:FF"
}
```

- `nested_level_count` (Number, Optional, default `0`) – MAC layer to inspect for MAC-in-MAC. `0` means any position.
- `mac_address` (String, Optional) – Single MAC address to match (e.g. `"00:1A:2B:3C:4D:5E"`). Mutually exclusive with `mac_address_start`. Exactly one of `mac_address` or `mac_address_start` must be provided.
- `mac_address_mask` (String, Optional, default `FF:FF:FF:FF:FF:FF`) – Bitmask applied to `mac_address` to define a range. Requires `mac_address`.
- `mac_address_start`, `mac_address_end` (String, Optional) – MAC address range. Both must be set together. Mutually exclusive with `mac_address`.

### `ip_version`

```hcl
ip_version = {
  nested_level_count = 0
  ip_version         = "v4"
}
```

- `nested_level_count` (Number, Optional, default `0`)
- `ip_version` (String, **Required**) – `"v4"` or `"v6"`.

### `ipv4_source` / `ipv4_destination`

```hcl
ipv4_source = {
  nested_level_count = 0
  address            = "10.0.0.0"
  cidr_mask          = "24"
}
```

- `nested_level_count` (Number, Optional, default `0`, range `0–3`) – IPv4 header depth for tunneled traffic.
- `address` (String, **Required**) – IPv4 address.
- `address_max` (String, Optional) – Range end. Mutually exclusive with `cidr_mask` and `netmask`.
- `cidr_mask` (String, Optional) – CIDR prefix length `"1"`–`"32"`. Mutually exclusive with `address_max` and `netmask`.
- `netmask` (String, Optional) – Dotted-decimal netmask. Mutually exclusive with `address_max` and `cidr_mask`.

### `ipv6_source` / `ipv6_destination`

```hcl
ipv6_source = {
  nested_level_count = 0
  address            = "2001:db8::"
  cidr_mask          = "64"
}
```

- `nested_level_count` (Number, Optional, default `0`, range `0–3`)
- `address` (String, **Required**) – IPv6 address or start of range.
- `address_max` (String, Optional) – Range end. Mutually exclusive with `cidr_mask` and `netmask`.
- `cidr_mask` (String, Optional) – `"1"`–`"128"`. Mutually exclusive with `address_max` and `netmask`.
- `netmask` (String, Optional) – IPv6 netmask as 8 uppercase hextets.

### `vm_name_source` / `vm_name_destination`

```hcl
vm_name_source = {
  vm_name_prefix = "frontend-"
}
```

- `vm_name_prefix` (String, **Required**) – Prefix of the VM name to match. Wildcards are not supported.

### `vm_tag_source` / `vm_tag_destination`

```hcl
vm_tag_source = {
  tag_name  = "environment"
  tag_value = "prod"
}
```

- `tag_name` (String, **Required**) – Tag key. In vSphere this is the tag name; in cloud environments it is the tag key.
- `tag_value` (String, **Required**) – Tag value. In vSphere this corresponds to the tag category.

### `ipv4_dscp` / `ipv6_dscp`

```hcl
ipv4_dscp = {
  nested_level_count = 0
  dscp               = "af11"
}
```

- `nested_level_count` (Number, Optional, default `0`, range `0–3`)
- `dscp` (String, **Required**) – One of `af11`–`af43` or `ef`.

### `ipv4_fragmentation`

```hcl
ipv4_fragmentation = {
  nested_level_count = 0
  mode               = "unfragmented_only"
}
```

- `nested_level_count` (Number, Optional, default `0`, range `0–3`)
- `mode` (String, **Required**) – One of:
    - `unfragmented_only`
    - `any_fragment`
    - `non_first_fragments`
    - `first_fragment_only`
    - `first_or_unfragmented`

### `ipv4_protocol`

```hcl
ipv4_protocol = {
  nested_level_count = 0
  protocol_min       = 6
  protocol_max       = 6
  protocol_subset    = "all"
}
```

- `nested_level_count` (Number, Optional, default `0`, range `0–3`)
- `protocol_min` (Number, **Required**) – Lower bound, `0–255`.
- `protocol_max` (Number, Optional) – Upper bound, `0–255`. Must be greater than `protocol_min` when set.
- `protocol_subset` (String, Optional, default `"all"`) – `"all"`, `"even"`, or `"odd"`. `even` and `odd` require `protocol_max`.

### `erspan_id`

```hcl
erspan_id = {
  erspan_id_min    = 1
  erspan_id_max    = 10
  erspan_id_subset = "all"
}
```

- `erspan_id_min` (Number, **Required**) – Lower bound, `1–1024`.
- `erspan_id_max` (Number, Optional) – Upper bound, `1–1024`. Must be greater than `erspan_id_min`.
- `erspan_id_subset` (String, Optional, default `"all"`) – `"all"`, `"even"`, or `"odd"`. `even` and `odd` require `erspan_id_max`.

### `ipv4_ttl`

```hcl
ipv4_ttl = {
  nested_level_count = 0
  ttl_min            = 64
  ttl_max            = 128
  ttl_subset         = "all"
}
```

- `nested_level_count` (Number, Optional, default `0`, range `0–3`)
- `ttl_min` (Number, **Required**) – `0–255`.
- `ttl_max` (Number, Optional) – `0–255`. Must be greater than `ttl_min`.
- `ttl_subset` (String, Optional, default `"all"`) – `"all"`, `"even"`, or `"odd"`. `even` and `odd` require `ttl_max`.

### `ipv4_tos`

```hcl
ipv4_tos = {
  nested_level_count = 0
  tos_min            = "0A"
  tos_max            = "1F"
  tos_subset         = "all"
}
```

- `nested_level_count` (Number, Optional, default `0`, range `0–3`)
- `tos_min` (String, **Required**) – 1-byte hex value, exactly 2 hex digits.
- `tos_max` (String, Optional) – 1-byte hex value. Must be greater than `tos_min` when set.
- `tos_subset` (String, Optional, default `"all"`) – `"all"`, `"even"`, or `"odd"`. `even` and `odd` require `tos_max`.

### `gre_key`

```hcl
gre_key = {
  gre_key_min    = "0000000A"
  gre_key_max    = "000000FF"
  gre_key_subset = "all"
}
```

- `gre_key_min` (String, **Required**) – Lower bound as 4-byte hex, exactly 8 hex digits.
- `gre_key_max` (String, Optional) – Upper bound as 4-byte hex. Must be greater than `gre_key_min` when set.
- `gre_key_subset` (String, Optional, default `"all"`) – `"all"`, `"even"`, or `"odd"`. `even` and `odd` require `gre_key_max`.

## Validation Notes

- Inclusion maps use the common map schema but add resource-specific validation.
- `drop_rules` is explicitly rejected on `gigamon_inclusion_map`.
- `rule_sets` must contain between **1** and **5** items.
- `rule_set_id` must be a string between `"1"` and `"5"`.
- `priority` must be between **1** and **5**.
- `aep_id` must be between **2** and **63**.
- Many range-style rule elements enforce:
    - valid format
    - mutually exclusive field combinations
    - both range endpoints when using ranges
    - `max > min` when both are provided

## Attribute Reference

In addition to the arguments above, the following attribute is exported:

- `id` (String) – Typed inclusion map ID used for lifecycle operations.

Format:

```text
map::inclusionMap::<uuid>
```

## Behavior Notes

- `monitoring_session_id` is **ForceNew**.
- On read, the provider retrieves inclusion maps from the Monitoring Session’s `inclusionMaps` collection.
- On create/update/delete, the provider sends Monitoring Session update requests with entity type `inclusionMap`.
- The provider stores a typed Terraform ID, while FM uses the raw UUID internally.

## Import

Import is **not supported** for `gigamon_inclusion_map`.

## Related Resources
- `gigamon_traffic_map` – General traffic classification map supporting both `pass_rules` and `drop_rules`
- `gigamon_exclusion_map` – ATS exclusion map supporting `drop_rules` only
- `gigamon_monitoring_session` – Parent Monitoring Session that owns the map