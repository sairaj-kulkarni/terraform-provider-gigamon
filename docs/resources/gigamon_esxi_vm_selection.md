## Resource: `gigamon_esxi_vm_selection`

> **This resource is only applicable for VMware ESXi monitoring sessions.**

- The `gigamon_esxi_vm_selection` restricts a traffic map to capture traffic only from specific VMs, identified by their MAC addresses. Internally this sets the `macFilterList` on the underlying traffic map in GigaVUE-FM.

- The `gigamon_traffic_map` resource itself does **not** expose `mac_addresses` as a Terraform attribute. When the traffic map's rules are updated, the provider automatically reads and re-applies the current `macFilterList` from FM so that VM selection is never accidentally cleared.

---

## Example Usage

```hcl
resource "gigamon_esxi_vm_selection" "vm_sel" {
  monitoring_session_id = gigamon_monitoring_session.my_ms.id
  trafficmap_id         = gigamon_traffic_map.my_map.id

  mac_addresses = [
    "00:50:56:AA:BB:CC",
    "00:50:56:DD:EE:FF",
  ]
}
```

Full example with a traffic map:

```hcl
resource "gigamon_monitoring_session" "my_ms" {
  alias                = "esxi-ms"
  connection_id        = gigamon_esxi_connection.my_conn.id
  monitoring_domain_id = gigamon_esxi_monitoring_domain.my_md.id
  tapping_method       = "platform"
}

resource "gigamon_traffic_map" "my_map" {
  name                  = "demo-map"
  monitoring_session_id = gigamon_monitoring_session.my_ms.id

  rule_sets {
    rule_set_id = "1"
    priority    = 1
    aep_id      = 2

    pass_rules {
      rule_id = 1
      ip_version {
        ip_version = "v4"
      }
    }
  }
}

resource "gigamon_esxi_vm_selection" "vm_sel" {
  monitoring_session_id = gigamon_monitoring_session.my_ms.id
  trafficmap_id         = gigamon_traffic_map.my_map.id

  mac_addresses = [
    "00:50:56:AA:BB:CC",
    "00:50:56:b6:63:4d",
    "00:50:56:b6:42:a9",
  ]
}
```

---

## Argument Reference

* `monitoring_session_id` (String, **Required**) – ID of the ESXi monitoring session that owns the target traffic map.
* `trafficmap_id` (String, **Required**) – ID of the traffic map on which to apply VM selection. Use `gigamon_traffic_map.<name>.id`. Referencing this attribute creates an implicit dependency so the traffic map is always created before this resource.
* `mac_addresses` (List of String, **Required**) – List of VM MAC addresses to select on this traffic map. At least one entry is required. Each MAC must be in the format `00:11:22:33:44:55`. Invalid MAC addresses are rejected during Terraform plan/apply.

---

## Attribute Reference

* `id` (String, Computed) – Internally derived typed ID tied to the traffic map:
  `map::esxiVmwareSelection::<trafficmap_uuid>`.

---

## Behavior

* **Create / Update** – Fetches the current traffic map from FM, replaces its `macFilterList`
  with the requested MAC addresses, and pushes the updated map back to FM.

* **Read** – Rebuilds `mac_addresses` from the map's current `macFilterList` in FM.
  If the traffic map no longer exists, this resource is removed from state.

* **Delete** – Clears the `macFilterList` on the underlying traffic map (sets it to empty) and
  removes this resource from state. The traffic map itself is **not** deleted.

---

## Import

Import is **not supported** for `gigamon_esxi_vm_selection`.
