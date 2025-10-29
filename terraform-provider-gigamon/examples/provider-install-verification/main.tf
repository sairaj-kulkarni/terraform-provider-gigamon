# Copyright (c) HashiCorp, Inc.

terraform {
  required_providers {
    gigamon = {
      source = "local/gigamon/gigamon"
    }
  }
}

provider "gigamon" {
  fm_address = "10.114.43.21"
  skip_verify = true
  api_token = "eyJhbGciOiJIUzI1NiJ9.eyJ0b2tlbklkIjoiNjk4NDY0MDY0MTM2NDk0NyIsInN1YiI6ImphbmEtdG9rZW4iLCJpYXQiOjE3NjE2NDI5MzAsImV4cCI6MTc2NDIzNDkzMH0.M4Z-4i2zW5j5iqWrJvVFCI--d4W2u2zdUvo3FnFuRpA"
  # api_token = "asdasdasda"
}

resource "gigamon_esxi_image" "vseries-6-12-00" {
  file_name = "/home/jana/gigamon-gigavue-vseries-node-6.12.00-533040_amd64.ova"
  timeout = 280
}
