<div class="page-header">
  <h1>Gigamon Provider and backend</h1>
  <a href="/logout" class="reset-link">Reset view</a>
</div>

The Gigamon provider allows users to configure and maintain Gigamon FM Cloud configurations. This allows the users to configure and manage the Monitoring Domain, Gigamon Fabric and Policies.

## Gigamon Provider Example Usage

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

## Gigamon Backend

Terraform/Opentofu state files store the current known state of the infrastructure. In production environments, this should be stored in a shared location whcih is shared across all the users of that infrastructre.

Terraform provides many remote backends which act as a shared storage for e.g. s3 bucket in aws, or azure blob storage. It is also possible for customers who are using Terraform/Opentofu to use FM as the backend shared storage for their teams.

This backend is implemented as a http backend, using the terraform internal http backend. An example configuration is shown below

```hcl
terraform {
  backend "http" {
    address = "https://<your fm address>/terraform-state/<your-infra-name>"
    lock_address = "https://<your fm address>/terraform-state/<your-infra-name>/lock"
    unlock_address = "https://<your fm address>/terraform-state/<your-infra-name>/lock"
    skip_cert_verification = true # Do not do this in production
    username = "user1" # user name can be anything and is not used
    password = <your fm api token> # Authentication/Authorization is based on FM API Token
  }
}
```

# Gigamon Backend parameters

* `address` - (Required) provide the URL that Terrform uses when getting or storing state. The URL should be your FM address followed by /terraform-state/<your-infra-name>. All users managing the same infra should use the same url including the your-infra-name. That will ensure that all of them would be using the same backend data/lock
* `lock_address` - (Required) provides the LOCK URL that Terraform uses for calling the LOCK APIto get a LOCK on the state when doing a apply or other operations. This should be the same your-infra-name/lock. All users manging an infra should use the same URL
* `unlock_address` - (Required) Provides the UNLOCK URL that Terraform uses for calling the UNLOCK operation.
* `skip_cert_validation` - (Optional) If set to true, Terraform skips the certificate validationwhen performing the state operation. It is default set to false, i.e. certificate validation will be done. In production environments this should always be set to false
* `username` - (Optional) For Gigamon FM Backend this parameter is not used, but the password parameter is used. Hence the user can provide any string here, as long as it is provided and not empty. The user can also set the environment variable TF_HTTP_USERNAME. Either the environment variable or the username should be specified
* `password` - (Optional) In the password field, the user should provide the FM API token for the user who is running the terraform command. This token is used to authenticate and authorize the user. The token can be provided as the password parameter or as the environment variable TF_HTTP_PASSWORD. It is recommended to use the environmental variables to aovid exposing secrest in the configuraiton files


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


