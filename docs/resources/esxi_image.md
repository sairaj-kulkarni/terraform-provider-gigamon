---
page_title: "ESXi Image"
subcategory: "ESXi"
description: "Manage ESXi images in Gigamon FM."
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

# gigamon_esxi_image

A **Gigamon ESXi image** uploads a local **vSeries image file** to Fabric Manager for the **VMware / ESXi** platform.

This resource is used to register an image package with FM so it can be used for ESXi-based deployments. The provider uploads the file to FM, then reads back the uploaded image metadata such as **image type**, **version**, and **vendor**.

## Example Usage

### Upload a vSeries image to FM

```hcl
resource "gigamon_esxi_image" "vseries_image" {
  file_name = "/tmp/gigamon-vseries-esxi.ova"
  timeout   = 300
}
```

### Use computed attributes

```hcl
output "esxi_image_details" {
  value = {
    id         = gigamon_esxi_image.vseries_image.id
    image_type = gigamon_esxi_image.vseries_image.image_type
    version    = gigamon_esxi_image.vseries_image.version
    vendor     = gigamon_esxi_image.vseries_image.vendor
  }
}
```

## Argument Reference

### Top-level arguments

- `file_name` (String, **Required**) – Local path to the image file to upload to FM. This file must exist and be readable during planning and apply. Changing this forces a new resource.
- `timeout` (Number, Optional, default `120`) – Timeout in seconds for the image upload operation. Changing this forces a new resource.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `id` (String) – Name of the uploaded image as recognized by FM. This value is used for lifecycle operations.
- `image_type` (String) – Type of the uploaded image, returned by FM.
- `version` (String) – Version of the uploaded image, returned by FM.
- `vendor` (String) – Vendor of the uploaded image, returned by FM.

## Behavior Notes

- The provider uploads the file to FM using the local path specified in `file_name`.
- After upload, the provider reads the ESXi image inventory from FM and matches the uploaded image by its **base file name**.
- The Terraform state stores:
    - `id` as the FM image name
    - `image_type`, `version`, and `vendor` from FM
- `file_name` is a local client-side path. FM does not store this local path; it is only used by Terraform during upload and import.
- Both `file_name` and `timeout` are replacement-only arguments.
- This resource supports **create**, **read**, **delete**, and **import**.
- This resource does **not** support **update**. ESXi images can only be uploaded or deleted.

## Plan-time Validation

During planning, the provider validates that the path given in `file_name` can be opened successfully.

If the file does not exist or cannot be read, planning fails.

## Import

Import is supported.

Because FM does not store the original local file path or the Terraform timeout value, the import format must include all three values:

```text
<imageID>:<file_name>:<timeout>
```

### Example

```bash
terraform import gigamon_esxi_image.vseries_image "gigamon-vseries-esxi.ova:/tmp/gigamon-vseries-esxi.ova:300"
```

Where:

- `<imageID>` is the image identifier / image name known to FM
- `<file_name>` is the local file path you want Terraform to keep in state
- `<timeout>` is the upload timeout in seconds

If the import string is not in this format, import fails.

## Lifecycle Summary

- **Create** – Uploads the file to FM
- **Read** – Fetches image details from FM and refreshes computed fields
- **Update** – Not supported
- **Delete** – Deletes the image from FM
- **Import** – Supported with custom import format

## Related Resources

- `gigamon_esxi_connection` – ESXi / vCenter connection resource used for VMware deployments
- `gigamon_esxi_fabric` – ESXi fabric resource that consumes the uploaded image through `image_id`