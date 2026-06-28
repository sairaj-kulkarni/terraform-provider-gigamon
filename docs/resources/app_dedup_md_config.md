---
page_title: "Dedup Application Global Configuration"
subcategory: "Applications"
description: "Manage the Dedup application's global configuration at monitoiring domain scope in Gigamon FM."
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

# Resource: `gigamon_dedup_md_config`

The **Dedup MD Config** resource manages the **global dedup configuration** at **Monitoring Domain scope** in Fabric Manager.

- `gigamon_dedup_md_config` represents the dedup parameters applied across **all dedup app instances** in a Monitoring Domain.
- It is a singleton per Monitoring Domain — FM always has a dedup config object, so **Create** simply applies your desired values via a PUT.
- It configures:
    - what action to take on duplicate packets (`action`)
    - how long to wait for duplicates (`timer`)
    - whether to include or ignore IPv6 Traffic Class, IPv4 TOS/DSCP, TCP Sequence Number, and VLAN ID fields
- A computed typed `id` is stored in state for reference.

This resource is distinct from `gigamon_app_dedup`, which manages the **dedup application instance** inside a Monitoring Session. `gigamon_dedup_md_config` defines **how dedup behaves** domain-wide; `gigamon_app_dedup` places a **dedup app instance** into the Monitoring Session topology.

## Example Usage

### Minimal dedup MD config (all defaults)

```hcl
resource "gigamon_dedup_md_config" "dedup_cfg" {
  monitoring_domain_id = gigamon_monitoring_session.ms.monitoring_domain_id
}
```

### Dedup config that counts duplicates instead of dropping them

```hcl
resource "gigamon_dedup_md_config" "dedup_cfg" {
  monitoring_domain_id = gigamon_monitoring_session.ms.monitoring_domain_id

  action = "count"
}
```

### Dedup config ignoring TCP sequence numbers and including VLAN

```hcl
resource "gigamon_dedup_md_config" "dedup_cfg" {
  monitoring_domain_id = gigamon_monitoring_session.ms.monitoring_domain_id

  action       = "drop"
  timer        = 100000
  tcp_sequence = "ignore"
  vlan         = "include"
}
```

### Full dedup MD config

```hcl
resource "gigamon_dedup_md_config" "dedup_cfg" {
  monitoring_domain_id = gigamon_monitoring_session.ms.monitoring_domain_id

  action             = "drop"
  timer              = 50000
  ipv6_traffic_class = "include"
  ipv4_tos_field     = "include"
  tcp_sequence       = "include"
  vlan               = "ignore"
}
```

## Argument Reference

### Required

- **`monitoring_domain_id`** (String)  
  Typed ID of the Monitoring Domain this dedup config belongs to.  
  Changing this forces a new `gigamon_dedup_md_config` resource to be created.

### Optional

- **`action`** (String)  
  Action to take on duplicate packets.  
  Optional, computed, default: `"drop"`.

  Allowed values:

    - `"drop"` — duplicate packets are silently dropped
    - `"count"` — duplicate packets are counted but not dropped

- **`timer`** (Number)  
  Time window in microseconds during which duplicates are detected.  
  Optional, computed, default: `50000`.  
  Allowed range: `10` – `500000`.

- **`ipv6_traffic_class`** (String)  
  Whether to include or ignore the IPv6 Traffic Class field when comparing packets.  
  Optional, computed, default: `"include"`.

  Allowed values: `"include"`, `"ignore"`.

- **`ipv4_tos_field`** (String)  
  Whether to include or ignore the IPv4 TOS/DSCP field when comparing packets.  
  Optional, computed, default: `"include"`.

  Allowed values: `"include"`, `"ignore"`.

- **`tcp_sequence`** (String)  
  Whether to include or ignore the TCP Sequence Number field when comparing packets.  
  Optional, computed, default: `"include"`.

  Allowed values: `"include"`, `"ignore"`.

- **`vlan`** (String)  
  Whether to include or ignore the VLAN ID field in the L2 header when comparing packets.  
  Optional, computed, default: `"ignore"`.

  Allowed values: `"include"`, `"ignore"`.

## Attributes Reference

In addition to the arguments above, `gigamon_dedup_md_config` exports:

- **`id`** (String)  
  Typed ID for this dedup config in the form `app::dedup::<md-uuid>`.

## FM Mapping

The provider maps Terraform data to an FM `vseriesGsParams` payload shaped like:

```json
{
  "gsparamsName": "gsParams",
  "dedup": {
    "action": "drop",
    "timer": 50000,
    "ipTclass": "include",
    "ipTos": "include",
    "tcpSeq": "include",
    "vlan": "ignore"
  }
}
```

The FM API endpoint used is:

- **GET** `/api/v1.3/cloud/vseriesGsParams/<monitoring-domain-id>` — reads the current dedup config
- **PUT** `/api/v1.3/cloud/vseriesGsParams/<monitoring-domain-id>` — applies the desired dedup config

## Behavior and Lifecycle

### Singleton scope

`gigamon_dedup_md_config` is a **singleton** — FM always maintains exactly one dedup config per Monitoring Domain. There is no FM-side create or delete operation; the object always exists.

### Create

On **Create**, the provider:

1. Reads the Terraform plan into `DedupConfigModel`.
2. Builds an FM `GsParams` payload with:
    - `GsParamsName = "gsParams"`
    - `Action`, `Timer`, `IPTClass`, `IPTos`, `TCPSeq`, `Vlan` from the plan
3. Calls `SetGsParams` (PUT `/api/v1.3/cloud/vseriesGsParams/<md-id>`) to apply the config.
4. Derives the raw MD UUID from `monitoring_domain_id`.
5. Wraps that UUID into a typed ID (`app::dedup::<md-uuid>`) and stores it as `id` in state.

### Read

On **Read**, the provider:

1. Reads prior Terraform state.
2. Calls `GetGsParams` (GET `/api/v1.3/cloud/vseriesGsParams/<md-id>`) to fetch current FM values.
3. Overlays FM-owned values into state:
    - `action`
    - `timer`
    - `ipv6_traffic_class`
    - `ipv4_tos_field`
    - `tcp_sequence`
    - `vlan`

### Update

On **Update**, the provider:

1. Reads the desired plan into `DedupConfigModel`.
2. Builds the FM `GsParams` payload from the plan.
3. Calls `SetGsParams` (PUT) to push the updated config to FM.
4. Overlays FM-owned fields back into the plan and saves state.

### Delete

On **Delete**, the provider performs **no operation**. The dedup config is a permanent singleton in FM and cannot be deleted; removing this resource from Terraform only removes it from state.

## Schema Notes

`gigamon_dedup_md_config` is intentionally simple:

- no nested blocks
- no `ModifyPlan`
- no cross-field semantic validation beyond schema requirements
- the only in-place replacement trigger is `monitoring_domain_id`

## Relationship to Dedup App Instance

Do not confuse this resource with the Monitoring Session–level dedup app instance resource.

### `gigamon_dedup_md_config`

Use this to configure **dedup behavior globally** for the Monitoring Domain, such as:

- action (`drop` or `count`)
- timer duration
- IPv4/IPv6 header field handling
- TCP sequence number handling
- VLAN handling

### `gigamon_app_dedup`

Use that to create the **dedup application instance** inside a Monitoring Session and wire it into the Monitoring Session topology using `gigamon_link`.

In practice, set `gigamon_dedup_md_config` first to define how dedup behaves, then deploy `gigamon_app_dedup` instances in your Monitoring Sessions.

## Import

Import is not supported.

## Summary

Using `gigamon_dedup_md_config`, you can:

- configure the global dedup action (`drop` or `count`) for a Monitoring Domain
- tune the duplicate detection timer window
- control which packet header fields (IPv6 Traffic Class, IPv4 TOS, TCP Sequence, VLAN) are used during dedup comparison
- manage the configuration lifecycle through Terraform with a stable typed ID
