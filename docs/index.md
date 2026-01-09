# Gigamon Provider

The Gigamon provider allows users to configure and maintain Gigamon FM Cloud configurations. This allows the users to configure and manage the Monitoring Domain, Gigamon Fabric and Policies.

## Example Usage

Terraform 1.14 and later:

```hcl
terraform {
  required_providers {
    gigamon = {
      source = "local/gigamon/gigamon"
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


## Authentication and Configuration

Gigamon Provider should be configured with api_token, that has the appropriate roles to enable the operation required by the TF input scripts.

## Argument Reference

In addition to generic provider arguments, like `alias` and `version`, the following are supported in the provider block

* `fm_address` - (Mandatory) FM DNS name or IP address 
* `api_token` - (Mandtory) API token which will be user to authenticate and authorize the api calls to FM


