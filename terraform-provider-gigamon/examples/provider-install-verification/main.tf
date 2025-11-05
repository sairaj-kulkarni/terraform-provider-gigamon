# Copyright (c) HashiCorp, Inc.

terraform {
  required_providers {
    gigamon = {
      source = "local/gigamon/gigamon"
    }
  }
}

provider "gigamon" {
  fm_address = "10.114.202.149"
  skip_verify = true
  api_token = "eyJhbGciOiJIUzI1NiJ9.eyJ0b2tlbklkIjoiNDM0NTIxNjQwMDcwOTk1MyIsInN1YiI6InRlc3QtdG9rZW4iLCJpYXQiOjE3NjIyNjQyMjEsImV4cCI6MTc2NDg1NjIyMX0.-2lWAgc3tL_sv2k1llFC-l_Oqhq9rTZylRj_7FpqhcA"
}

data "gigamon_esxi_inventory" "my-invnetory" {
  connection_id = "042d6cf4-05a9-4dc6-ac11-7c51ecff2fa3"
}

