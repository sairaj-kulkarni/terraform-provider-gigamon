# Copyright (c) HashiCorp, Inc.

terraform {
  required_providers {
    gigamon = {
      source = "local/gigamon/gigamon"
    }
  }
}

provider "gigamon" {
  fm_address = "myfm.gigamon.com"
  # user_name = "jana"
  # password =  "jana123"
  api_token = "asdasdasda"
}

data "gigamon_example" "example" {
}

resource "gigamon_example" "example1" {
}
