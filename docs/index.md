# Gigamon Provider

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

Gigamon Provider should be configured with api_token, that has the appropriate roles to enable the operation required by the TF input scripts.

## Argument Reference

In addition to generic provider arguments, like `alias` and `version`, the following are supported in the provider block

* `fm_address` - (Required) FM DNS name or IP address 
* `api_token` - (Required) API token which will be user to authenticate and authorize the api calls to FM
* `skip_verify` - (Optional) default is false. boolean flag to determine if we want to skip or verify the certificate presented by FM. Default is false, which ensures that the certificate is valdiated. Be careful when you set it to true, and should **not set to true** in a production environment


