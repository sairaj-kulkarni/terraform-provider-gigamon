
## Resource: `gigamon_app_amx`

The **AMX application** (Application Metadata Exporter, internally `ogw`) runs inside a **Cloud Monitoring Session** and is responsible for:

- **Ingesting** telemetry/metadata (AMI, mobility, Netflow/IPFIX).
- **Exporting** JSON records to **HTTP/HTTPS endpoints** and/or **Kafka topics**.
- Optionally performing **metadata enrichment** from cloud and workload sources (AWS, Azure, VMware, AKS, generic sources).

It always belongs to a **single Monitoring Session** and can be linked to maps/applications/endpoints via `gigamon_link` just like other `app_*` resources.

- `gigamon_app_amx` represents the **AMX/OGW application instance** inside one Monitoring Session.
- You can configure:
    - **one or more ingestors** (at least one is required for AMX to receive any data),
    - one **exporter** (containing any number of HTTP/Kafka exports),
    - optional **mobility**, **workload**, and **other** enrichment profiles.

  > **Netflow-only limitation:**  
  > If **all** defined ingestors are `type = "netflow"`, **no enrichment** (`mobility_enrichment`, `workload_enrichment`, `other_enrichment`) is allowed. This is enforced by provider-side validation.

  > **Ingestor port uniqueness:**  
  > When more than one `ingestor` block is configured, every `port` value **must be unique**. The provider rejects duplicates at plan time.

  > **Enrichment combinations:**  
  > - `workload_enrichment` and `mobility_enrichment` are **mutually exclusive** — only one of them may be present.  
  > - Inside `workload_enrichment`, **exactly one platform** (`aws`, `azure`, `vmware_vcenter`, or `aks`) may be configured when the block is present, and only a single block of that platform is allowed.  
  > - `workload_enrichment` (with its single platform) or `mobility_enrichment` may coexist with any number of `other_enrichment` blocks.

### Prerequisites

- **V Series form factor for AMX**  
  The AMX application must run on a **V Series node with the large form factor**, with sufficient CPU, memory, and disk as per the AMX validated design. Running AMX on smaller V Series form factors is not supported.

- **Monitoring Domain type**  
  `gigamon_app_amx` is supported only on Monitoring Sessions whose Monitoring Domain has **Traffic Acquisition Method = Customer Orchestrated Source**.  
  AMX is **not** supported on UCT-V / Traffic Mirroring / Platform Tapping domains.
---

## Example Usage

### Minimal AMX application with AMI ingestor and HTTP export

```hcl
resource "gigamon_app_amx" "amx" {
  alias                 = "amx-ogw"
  monitoring_session_id = gigamon_monitoring_session.ms.id

  # Ingest AMI records on UDP/4739
  ingestor {
    name     = "ami_ingestor"
    port     = 4739
    type     = "ami"       # one of: ami, mobility, netflow
  }

  exporter {
    debug = false

    # Export enriched AMI to HTTP endpoint
    http_export {
      name      = "grafana"
      endpoint  = "https://grafana.example/api/amx"
      data_type = "ami_enriched" # one of: ami, mobility, ami_enriched, netflow

      headers = [
        "Authorization: Bearer ${var.grafana_token}",
      ]

      labels = {
        env  = "prod"
        sink = "grafana"
      }

      # Other fields (compress, intervals, retries...) use FM defaults.
    }
  }
}
```

### AMX with AMI + mobility ingestors, HTTP and Kafka exports

```hcl
resource "gigamon_app_amx" "amx_full" {
  alias                 = "amx-full"
  monitoring_session_id = gigamon_monitoring_session.ms.id

  # AMI ingestor
  ingestor {
    name     = "ami_ingestor"
    port     = 4739
    type     = "ami"
  }

  # Mobility ingestor (mapped to GTPC/GTPC_HIER in FM)
  ingestor {
    name     = "mobility_ingestor"
    port     = 2123
    type     = "mobility"
  }

  exporter {
    debug = true

    # HTTP export to SaaS tool
    http_export {
      name      = "saas_sink"
      endpoint  = "https://api.example.com/amx"
      data_type = "ami_enriched"

      headers = [
        "Authorization: Bearer ${var.saas_token}",
        "X-Tenant: ${var.tenant_id}",
      ]

      compress               = true
      flush_interval_seconds = 30
      max_records_per_batch  = 5000

      labels = {
        env     = "staging"
        product = "amx"
      }
    }

    # Kafka export to analytics platform
    kafka_export {
      name   = "kafka_main"
      topic  = "amx-ami"
      brokers = [
        "kafka1:9092",
        "kafka2:9092",
      ]

      data_type = "ami"

      producer_configs = [
        "acks=all",
        "compression.type=gzip",
      ]

      labels = {
        env  = "staging"
        role = "primary"
      }
    }
  }
}
```

### AMX with AWS workload enrichment

```hcl
resource "gigamon_app_amx" "amx_enriched" {
  alias                 = "amx-enriched"
  monitoring_session_id = gigamon_monitoring_session.ms.id

  # Require at least one non-netflow ingestor when enrichment is present
  ingestor {
    name     = "ami_ingestor"
    port     = 4739
    type     = "ami"
  }

  exporter {
    http_export {
      name      = "enriched_sink"
      endpoint  = "https://enriched.example/api"
      data_type = "ami_enriched"
    }
  }

  # Optional, at most one workload_enrichment block
  workload_enrichment {
    # AWS workload enrichment, a single profile
    aws {
      name       = "aws-account-1"
      attributes = ["instance_id", "vpc_id", "tags"]
      settings = {
        aws_refresh_interval = "300"
      }

      # At least one source per profile
      source {
        name = "primary-account"

        # One or more key/value settings (typically credentials or config refs)
        setting {
          key   = "aws_access_key_id"
          value = var.aws_access_key_id
        }

        setting {
          key   = "aws_secret_access_key"
          value = var.aws_secret_access_key
        }

        setting {
          key   = "aws_region"
          value = "us-west-2"
        }
      }
    }
  }

  # Additional arbitrary enrichments
  other_enrichment {
    name       = "geoip"
    attributes = ["country", "city"]
    settings   = ["geoip_db=/etc/geoip/db.mmdb"]
  }

  other_enrichment {
    name       = "device_inventory"
    attributes = ["device_type", "vendor"]
    settings   = []
  }
}
```

### Configuration using dictionaries and dynamic blocks

You can keep the root configuration minimal and drive AMX profiles from locals using `for_each` + `dynamic` blocks.

#### Example: ingestors from a map

```hcl
locals {
  amx_ingestors = {
    ami = {
      port     = 4739
      type     = "ami"
    }
    mobility = {
      port     = 2123
      type     = "mobility"
    }
  }
}

resource "gigamon_app_amx" "amx" {
  alias                 = "amx-ogw"
  monitoring_session_id = gigamon_monitoring_session.ms.id

  dynamic "ingestor" {
    for_each = local.amx_ingestors
    content {
      name     = each.key
      port     = each.value.port
      type     = each.value.type
    }
  }

  exporter {
    debug = false

    # See next examples for http_export / kafka_export blocks
  }
}
```

#### Example: HTTP and Kafka exports from maps

```hcl
locals {
  amx_http_exports = {
    grafana = {
      endpoint  = "https://grafana.example/api/amx"
      data_type = "ami_enriched"
      headers   = ["Authorization: Bearer ${var.grafana_token}"]
      labels    = { env = "prod", sink = "grafana" }
    }
    s3 = {
      endpoint  = "https://s3.example/bucket/path"
      data_type = "ami"
      labels    = { env = "prod", sink = "s3" }
    }
  }

  amx_kafka_exports = {
    main = {
      topic   = "amx-ami"
      brokers = ["kafka1:9092", "kafka2:9092"]
      labels  = { env = "prod" }
    }
  }
}

resource "gigamon_app_amx" "amx" {
  alias                 = "amx-ogw"
  monitoring_session_id = gigamon_monitoring_session.ms.id

  ingestor {
    name = "ami_ingestor"
    port = 4739
    type = "ami"
  }

  exporter {
    dynamic "http_export" {
      for_each = local.amx_http_exports
      content {
        name      = each.key
        endpoint  = each.value.endpoint
        data_type = lookup(each.value, "data_type", "ami")
        headers   = lookup(each.value, "headers", [])
        labels    = lookup(each.value, "labels", {})

        # Other tunables can be set as needed or left to defaults.
      }
    }

    dynamic "kafka_export" {
      for_each = local.amx_kafka_exports
      content {
        name    = each.key
        topic   = each.value.topic
        brokers = each.value.brokers
        labels  = lookup(each.value, "labels", {})
      }
    }
  }
}
```

#### Example: workload enrichment AWS driven by maps

```hcl
locals {
  amx_workload_aws = {
    name = "account1"
    attributes = ["account_id", "region"]
    settings = {
      aws_refresh_interval = "300"
    }
    sources = {
      primary = {
        settings = [
          { key = "aws_access_key_id", value = var.aws_access_key_id },
          { key = "aws_secret_access_key", value = var.aws_secret_access_key },
        ]
      }
    }
  }
}

resource "gigamon_app_amx" "amx" {
  alias = "amx-aws-workload"
  monitoring_session_id = gigamon_monitoring_session.ms.id

  ingestor {
    name = "ami_ingestor"
    port = 4739
    type = "ami"
  }

  exporter {
    http_export {
      name = "aws_sink"
      endpoint = "https://example/api/ami"
    }
  }

  workload_enrichment {
    aws {
      name       = local.amx_workload_aws.name
      attributes = lookup(local.amx_workload_aws, "attributes", [])
      settings   = lookup(local.amx_workload_aws, "settings", {})

      dynamic "source" {
        for_each = local.amx_workload_aws.sources
        content {
          name = source.key

          dynamic "setting" {
            for_each = source.value.settings
            content {
              key   = setting.value.key
              value = setting.value.value
            }
          }
        }
      }
    }
  }
}
```

---

## Argument Reference

### Required

- **`monitoring_session_id`** (String)  
  Monitoring Session in which this AMX application is configured.  
  Typically set from `gigamon_monitoring_session.<name>.id`.  
  Changing this forces a new `gigamon_app_amx` to be created.  
  **The associated Monitoring Domain must use Traffic Acquisition Method = `Customer Orchestrated Source`; AMX is not supported on UCT-V / Traffic Mirroring / Platform Tapping domains.**

- **`alias`** (String)  
  Alias / name for this AMX application instance, unique within the Monitoring Session.  
  Must be non-empty.

- **`exporter`** (Block)  
  Single block that contains HTTP and/or Kafka exports (see below).  
  At least one of:
    - `exporter.http_export { ... }`
    - `exporter.kafka_export { ... }`  
  must be present. This is enforced by a config validator.

### Optional

- **`ingestor`** (Block, multiple)  
  One or more blocks defining AMX ingestors.  
  At least one ingestor must be configured; an AMX instance without ingestors will not receive or export any data.

- **`mobility_enrichment`** (Block, 0–1)  
  Optional mobility enrichment configuration.

- **`workload_enrichment`** (Block, 0–1)  
  Optional workload enrichment configuration grouping AWS, Azure, VMware vCenter, and AKS.

- **`other_enrichment`** (Block, multiple)  
  Optional generic enrichment profiles.

> **Netflow-only constraint:**  
> If all `ingestor` blocks use `type = "netflow"`, **no** enrichment blocks may be configured.  
> The provider validates this and fails the plan/apply if violated.

---

## Block Reference

### `ingestor` block

```hcl
ingestor {
  name     = string (optional)
  port     = number (int32, 1–65535, required)
  type     = string (required: "ami", "mobility", "netflow")
}
```

- **`name`** (String)  
  Optional label for this ingestor.

- **`port`** (Number)  
  UDP/TCP port on which AMX listens for this ingestor.  
  Range: **1–65535**.

- **`type`** (String)  
  Ingestor type:
    - `"ami"` – AMI (Application Metadata Interface).
    - `"mobility"` – mapped to FM `gtpc` / `gtpc_hier` internally.
    - `"netflow"` – Netflow/IPFIX.

**Internal mapping:**

- `type = "mobility"` (TF) → FM ingestor type `"gtpc"` (or related mobility GTPC types).
- `type = "ami"` and `type = "netflow"` are mapped as-is.
- `data_type = "mobility"` (TF) → FM `Type = "gtpc"`.
- FM `Type = "gtpc"` or `"gtpc_hier"` → `data_type = "mobility"` in Terraform state.

---

### `exporter` block

```hcl
exporter {
  debug = bool (optional, default false)

  http_export  { ... }
  kafka_export { ... }
}
```

- **`debug`** (Boolean)  
  Enable/disable AMX exporter debug logging.  
  Optional, computed; default is `false`.

At least one of **`http_export`** or **`kafka_export`** must be present.

---

### `exporter.http_export` block

```hcl
http_export {
  name                   = string (required)
  enabled                = bool   (optional, default true)
  data_type              = string (optional, default "ami", one of: "ami", "mobility", "ami_enriched", "netflow")
  endpoint               = string (required)
  secure_endpoint        = bool   (optional)
  headers                = [string] (optional)
  secure_keys            = [string] (optional)
  bind_ip_address        = string  (optional)
  # format             = read-only (always "json"; not user-configurable)
  compress               = bool    (optional, default true)
  flush_interval_seconds = number  (optional, default 30, 10–1800)
  parallel_workers       = number  (optional, default 4)
  max_retries            = number  (optional, default 4, minimum 4)
  max_records_per_batch  = number  (optional, default 5000)
  self_heal_window_seconds = number (optional, default 0)
  upload_timeout_seconds   = number (optional, default 10)
  labels                 = map(string) (optional)
}
```

- **`name`** (String, required)  
  Unique alias for this HTTP export within the exporter.

- **`enabled`** (Boolean)  
  Whether this HTTP export is active.  
  Optional; default **true**.

- **`data_type`** (String)  
  Type of data exported: **AMI, Mobility Control, AMI Enriched, or NetFlow/IPFIX.**
    - `"ami"`
    - `"mobility"`
    - `"ami_enriched"`
    - `"netflow"`  
  Optional; default **"ami"**.

- **`endpoint`** (String, required)  
  Target HTTP/HTTPS endpoint URL.

- **`secure_endpoint`** (Boolean)  
  Whether the configured endpoint should be treated as a secure endpoint by AMX. Maps to FM `maskEndpointApiKey`: when set to `true`, FM masks the endpoint API key in responses and treats the endpoint as sensitive. Optional; default **false**.

- **`headers`** (List of String)  
  HTTP headers to send, e.g. `["Authorization: Bearer ..."]`.  
  Optional; persisted as write-only/sensitive‑like: FM does **not** return them, but the provider attempts to preserve them in state across reads.

- **`secure_keys`** (List of String)  
  Names of headers/fields that should be treated as secure keys on AMX side.  
  Optional.

- **`bind_ip_address`** (String)  
  Local source IP address AMX should bind for outgoing connections.  
  Optional.

- **`format`** (String, **read-only**)  
  Payload format. Not user-configurable: the provider always sends `"json"` to FM and exposes it back as a computed value. Setting it in configuration is not supported.

- **`compress`** (Boolean)  
  Compress uploads (gzip). Optional; default **true**.

- **`flush_interval_seconds`** (Number)  
  Upload interval; range: **10–1800**. Optional; default **30**.

- **`parallel_workers`** (Number)  
  Number of parallel upload workers. Optional; default **4**.

- **`max_retries`** (Number)  
  Retry attempts per batch. Optional; default **4**, minimum **4**. Values less than 4 are rejected by provider validation.

- **`max_records_per_batch`** (Number)  
  Maximum records per HTTP batch. Optional; default **5000**.

- **`self_heal_window_seconds`** (Number)  
  Self-heal window; Optional; default **0**.

- **`upload_timeout_seconds`** (Number)  
  HTTP client upload timeout. Optional; default **10**.

- **`labels`** (Map of String)  
  Static labels added to all records from this HTTP export.

---

### `exporter.kafka_export` block

```hcl
kafka_export {
  name                   = string (required)
  topic                  = string (required)
  enabled                = bool   (optional, default true)
  brokers                = [string] (required)
  bind_ip_address        = string   (optional)
  data_type              = string   (optional, default "ami", one of: "ami", "mobility", "ami_enriched", "netflow")
  # format             = read-only (always "json"; not user-configurable)
  flush_interval_seconds = number   (optional, default 30)
  parallel_workers       = number   (optional, default 4)
  max_retries            = number   (optional, default 4, minimum 4)
  max_records_per_batch  = number   (optional, default 5000)
  self_heal_window_seconds = number (optional, default 0)
  labels                 = map(string)  (optional)
  producer_configs       = [string]     (optional)
}
```

- **`name`** (String, required)  
  Unique alias for this Kafka export.

- **`topic`** (String, required)  
  Kafka topic to which AMX sends records.

- **`enabled`** (Boolean)  
  Whether this Kafka export is active. Optional; default **true**.

- **`brokers`** (List of String, required)  
  List of Kafka brokers, e.g. `["kafka1:9092", "kafka2:9092"]`.  
  At least one is required.

- **`bind_ip_address`** (String)  
  Local source IP for outgoing connections. Optional.

- **`data_type`** (String)  
  Type of data exported: **AMI, Mobility Control, AMI Enriched, or NetFlow/IPFIX.**
    - `"ami"`
    - `"mobility"`
    - `"ami_enriched"`
    - `"netflow"`
  Optional; default `"ami"`.

- **`format`** (String, **read-only**)  
  Payload format. Not user-configurable: the provider always sends `"json"` to FM and exposes it back as a computed value. Setting it in configuration is not supported.

- **`flush_interval_seconds`**, **`parallel_workers`**,  
  **`max_records_per_batch`**, **`self_heal_window_seconds`**  
  Same semantics as in HTTP export, but applied to Kafka producers.

- **`max_retries`** (Number)
  Retry attempts per batch. Optional; default **4**, minimum **4**. Values less than 4 are rejected by provider validation.

- **`labels`** (Map of String)  
  Static labels for this export.

- **`producer_configs`** (List of String)  
  Additional Kafka `key=value` producer configuration strings.

---

### `mobility_enrichment` block

```hcl
mobility_enrichment {
  name       = string (required)
  enabled    = bool   (optional, default true)
  attributes = [string] (optional)
}
```

- **`name`** (String, required)  
  Identifier for this mobility enrichment profile.

- **`enabled`** (Boolean)  
  Whether this enrichment is applied. Optional; default **true**.

- **`attributes`** (List of String)  
  Mobility attribute names to export (e.g. IMSI, APN, cell ID).

**Constraints:**

- At most **one** `mobility_enrichment` block per AMX resource.
- `mobility_enrichment` and `workload_enrichment` are **mutually exclusive**.

---

### `workload_enrichment` block

Single block grouping workloads across multiple platforms:

```hcl
workload_enrichment {
  aws {
    ...
    source { ... setting { ... } }
  }

  azure {
    ...
    source { ... setting { ... } }
  }

  vmware_vcenter {
    ...
    source { ... setting { ... } }
  }

  aks {
    ...
    source { ... setting { ... } }
  }
}
```

Top-level:

- At most **one** `workload_enrichment` block in the resource.

Each platform block (`aws`, `azure`, `vmware_vcenter`, `aks`) has this shape:

```hcl
aws {
  name       = string (required)
  enabled    = bool   (optional, default true)
  attributes = [string] (optional)
  settings   = map(string) (optional)

  source {
    name = string (required)

    setting {
      secure = bool   (optional, default true)
      file   = string (optional)
      key    = string (required)
      value  = string (optional, sensitive)
    }
  }
}
```

For each platform:

- **`name`** (String, required)  
  Workload enrichment profile name (e.g. account, subscription, cluster profile).

- **`enabled`** (Boolean)  
  Whether this workload enrichment profile is in effect. Optional; default **true**.

- **`attributes`** (List of String)  
  Workload attribute names to export (e.g. instance_id, vpc_id, tags).

- **`settings`** (Map of String)  
  Additional workload settings as key/value pairs. Keys and values are passed through to AMX as `"key=value"` strings; use only under Gigamon guidance.

**`source` sub-block:**

- **`name`** (String, required)  
  Name/label of this source, e.g. account/cluster identifier.

- **`setting`** (multiple):

  - **`secure`** (Boolean)  
    Whether the value is a secret (AMX will encrypt it). Optional; default **true**.

  - **`file`** (String) 
  Optional path to a file.
  > **Important:** This is only meaningful for **AKS** workloads, as part of the `k8s_kubeconfig` setting.

  - **`key`** (String, required)  
    Property key, e.g. `aws_access_key_id`, `azure_client_id`, `k8s_kubeconfig`.

  - **`value`** (String, optional, Sensitive)  
    Plain property value when `file` is not used.

**Constraints:**

- At most **one** `workload_enrichment` block.
- Inside that block, **exactly one platform** (`aws`, `azure`, `vmware_vcenter`, or `aks`) may be present, and **only a single block** of that platform is allowed (no multiple `aws` blocks, no `aws` + `azure`, etc.).
- `workload_enrichment` and `mobility_enrichment` are **mutually exclusive**; both may not be present in the same AMX resource.
- For every workload platform profile present, there must be **at least one `source` block**. If absent, plan/apply fails.
- For `aks` platforms, each `source` must include a valid kubeconfig setting (`key = "k8s_kubeconfig"` with non-empty file and/or value), or plan/apply will fail.

For **AKS** workload profiles:

- Each `aks` profile's `source` **must** include a `setting` with `key = "k8s_kubeconfig"` and non-empty `file` and/or `value` (kubeconfig is required for AKS workload enrichment). The provider validates this and rejects configurations that don't have a valid kubeconfig setting.

---

### `other_enrichment` block

```hcl
other_enrichment {
  name       = string (required)
  enabled    = bool   (optional, default true)
  attributes = [string] (optional)
  settings   = [string] (optional)
}
```

- **`name`** (String, required)  
  Name of this generic enrichment profile.

- **`enabled`** (Boolean)  
  Whether this enrichment is active. Optional; default **true**.

- **`attributes`** (List of String)  
  Generic attribute names to export.

- **`settings`** (List of String)  
  Advanced settings for this 'other' enrichment. Each string is sent as-is to AMX (matches FM UI Settings list); semantics are internal/advanced.

Multiple `other_enrichment` blocks are allowed.

---

## Attributes Reference

In addition to the arguments above, `gigamon_app_amx` exports:

- **`id`** (String)  
  Typed AMX application ID assigned by the provider (wrapping the FM app UUID).  
  Used for linking (for example, from `gigamon_link`) and for update/delete.  
  You never construct this ID manually.

- **Nested block attributes**  
  All fields under `ingestor`, `exporter`, and enrichments are either:
    - persisted from configuration, or
    - refreshed from FM when FM echoes them back (ingestor fields, exporter intervals, etc.).

**Special behavior:**

- For `exporter.http_export.headers` and `secure_keys`, FM does **not** echo these values. The provider **preserves** them by copying from previous state back into new state during `Read`, keyed on export `name`. This avoids constant drift for write-only fields.

- For enrichment blocks, FM may not echo all fine-grained fields. The provider currently treats Terraform config/state as the source of truth and does **not** attempt full round-trip mapping for all enrichment internals. This avoids unnecessary churn when FM omits or normalizes fields.

---

## Behavior and Lifecycle

### Monitoring Session scope

- `gigamon_app_amx` belongs to exactly **one** Monitoring Session (`monitoring_session_id`).
- The provider configures AMX by updating the Monitoring Session object via Fabric Manager (`/monitoringSessions/{id}/update`) with `"application"` operations.

### Type mapping

- The AMX app is represented as FM `FMAmx` with:
    - `Alias = alias`
    - `Name = "ogw"` (fixed)
    - `Ingestor[]`, `Exporter`, and `AttrEnrichment[]` derived from the Terraform model.
- Workload enrichments map to FM `AttrEnrichment.Type` values such as:
    - `"workload_aws"`, `"workload_azure"`, `"workload_vmware_esxi"`, `"workload_k8s"`.
- Mobility enrichment maps to `"mobility"`.
- Other enrichment maps to `"other"`.

**Ingestor type mapping:**

- `type = "mobility"` (TF) → FM ingestor type `"gtpc"` (or related mobility GTPC types).
- `type = "ami"` and `type = "netflow"` are mapped as-is.

**Exporter data_type mapping:**

- `data_type = "mobility"` (TF) → FM `Type = "gtpc"`.
- FM `Type = "gtpc"` or `"gtpc_hier"` → `data_type = "mobility"` in Terraform state.
- `data_type = "ami"`, `"ami_enriched"`, and `"netflow"` are mapped as-is.
- Terraform users only see `mobility` as an option; FM's internal `gtpc`/`gtpc_hier` distinction is not exposed.

### Create

- On **Create**, the provider:
  1. Reads the plan into `AmxModel`.
  2. Runs `validateAmxPlan`:
     - Enforces netflow-only vs enrichment rule.
     - Ensures at most one `mobility_enrichment` and `workload_enrichment`.
     - Ensures each workload platform profile has at least one `source`.
  3. Builds FM payload (`FMAmx`) via `createFMStruct`.
  4. Calls `UpdateMonSess` with an `"application"` `"create"` operation containing the AMX payload.
  5. Receives a raw FM UUID for the created app, wraps it in a **typed ID**, and stores that as `id` in state.

### Read / Drift handling

- On **Read**, the provider:
  1. Reads existing state to get `id` and `monitoring_session_id`.
  2. Converts typed `id` → raw UUID using `UUIDFromTypedID`.
  3. Calls `GetMSAppData` with app name `"ogw"` to fetch current AMX configuration from FM.
  4. If FM reports **ObjectNotFound**, the resource is removed from state (idempotent if deleted out-of-band).
  5. Uses `updateTFStruct` to overlay FM-owned fields into Terraform state:
     - Updates `alias`, `ingestor`, `exporter` (CloudUpload & Kafka).
     - Leaves enrichment blocks largely state‑driven to avoid churn where FM does not echo exact user input.
  6. Specifically restores `headers` and `secure_keys` from old state into new state for matching `http_export.name` entries.

### Update

- On **Update**, the provider:
    1. Reads plan into `AmxModel`.
    2. Validates semantics via `validateAmxPlan` (same checks as Create).
    3. Builds FM payload `FMAmx` from plan.
    4. Converts typed `id` from state to raw UUID and sets `FMAmx.Id`.
    5. Calls `UpdateMonSess` with `"application"` `"update"` for this AMX instance.
    6. Writes plan (with any FM normalization already applied by `createFMStruct`) back to state.

> **Note:** Unlike some other app resources, AMX `Update` currently does **not** perform a second Read to overlay FM-derived defaults; it assumes the FM payload structure is stable and matches the plan.

### Delete

- On **Delete**, the provider:
    1. Reads existing state to get `id` and `monitoring_session_id`.
    2. Converts typed `id` to raw UUID.
    3. Calls `UpdateMonSess` with `"application"` `"delete"` and a minimalist `FMAmx` containing the ID and `Name = "Application"`.
    4. If FM reports object missing, deletion is treated as successful.

---

## Import

Import is **not yet supported** for `gigamon_app_amx`.

---

## Summary of What You Can Achieve with `gigamon_app_amx`

Using this resource you can, in a **composable, loop-friendly way**:

  - Configure **multiple AMX ingestors** (AMI, mobility, Netflow) on arbitrary ports, optionally driven by maps and `dynamic` blocks.
  - Attach **any number of HTTP exports** to different SaaS or custom endpoints with:
      - Per-export headers, secure keys, labels, and tunable batching.
  - Attach **any number of Kafka exports** to different topics/brokers including custom producer configs.
  - Enable:
      - **Mobility enrichment** (0–1 profile),
      - **Workload enrichment** covering AWS/Azure/VMware/AKS with multiple sources each,
      - **Generic “other” enrichments** as a list of lightweight profiles.
  - Enforce and benefit from semantic validation around:
      - Netflow-only vs enrichment support,
      - Required sources per workload profile,
      - Cardinality constraints for enrichment blocks.

The current schema is flexible enough to express an **entire AMX/OGW profile tree** via Terraform and to generate it from dictionaries/lists, while staying close to the FM model and preventing invalid combinations at plan time.
