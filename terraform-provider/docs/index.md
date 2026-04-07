<div class="page-header">
  <h1>Gigamon Provider</h1>
  <a href="/logout" class="reset-link">Reset view</a>
</div>

The Gigamon provider allows users to configure and maintain Gigamon FM Cloud configurations. This allows the users to configure and manage the Monitoring Domain, Gigamon Fabric and Policies.

## Example Usage

Terraform 1.14 and later:

```hcl
terraform {
  required_providers {
    gigamon = {
      source = "local/gigamon/gigamon"
      version = ">= 6.14"
    }
  }
}
```

# Configure the gigamon Provider

```hcl
provider "gigamon" {
 fm_address = "fm.example.com"
 api_token = "eyJhbGciOiJIUzI1NiJ9.eyJ0b2tlbklkIjoiMzAwNDMyNDkyOTk4MDMwNiIsInN1YiI6ImphbmEtdGYiLCJpYXQiOjE3NjQ5MjI3NjMsImV4cCI6MTc2NzUxNDc2M30.vfNzlViGU932qoiqTdcxCkk6-HgOpUibk8H9TR4mYnA"
}
```

## Version

Gigamon Terraform provider support is available from FM 6.14 onwards. To use Terraform, we need to ensure that the following version compatibilities are maintained

* FM version >= 6.14
* Terraform Version >= 1.14
* Gigamon Provider Version is same as FM version or upto two releases lower. i.e. if FM version is 6.18, Gigamon provider version can be either 6.18, 6.17 or 6.16

## Authentication and Configuration

Gigamon Provider should be configured with an API token that has the appropriate roles to enable the operation required by the TF input scripts.

You can pass the token in either of the following ways:

* Provider argument: `api_token`
* Environment variable: `FM_API_TOKEN`

### Precedence

If both are specified, `FM_API_TOKEN` is used and takes precedence over `api_token` from the Terraform provider block.

If `FM_API_TOKEN` is not set, the provider uses `api_token` from the Terraform configuration.

## Argument Reference

In addition to generic provider arguments, like `alias` and `version`, the following are supported in the provider block

* `fm_address` - (Required) FM DNS name or IP address 
* `api_token` - (Optional) API token used to authenticate and authorize API calls to FM. If `FM_API_TOKEN` is set, it takes precedence over this value
* `skip_verify` - (Optional) default is false. boolean flag to determine if we want to skip or verify the certificate presented by FM. Default is false, which ensures that the certificate is valdiated. Be careful when you set it to true, and should **not set to true** in a production environment


