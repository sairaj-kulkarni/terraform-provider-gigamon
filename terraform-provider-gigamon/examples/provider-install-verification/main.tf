# Copyright (c) HashiCorp, Inc.

terraform {
  required_providers {
    gigamon = {
      source = "local/gigamon/gigamon"
    }
  }
}

provider "gigamon" {
  fm_address = "10.114.202.170"
  skip_verify = true
  api_token = "eyJhbGciOiJIUzI1NiJ9.eyJ0b2tlbklkIjoiNjEwOTAxNDUyMzY1NTMzMiIsInN1YiI6ImphbmEtdG9rZW4iLCJpYXQiOjE3NjE4MzkzNDgsImV4cCI6MTc2NDQzMTM0N30.K-rhKyAJb4i-deW2Wx02mOpC1hJVmoLjK2oU7RJPsv0"
  # api_token = "asdasdasda"
}

#resource "gigamon_esxi_image" "vseries-6-12-00" {
  #file_name = "/home/jana/gigamon-gigavue-vseries-node-6.12.00-533040_amd64.ova"
  #timeout = 280
#}

resource "gigamon_esxi_monitoring_domain" "my-md" {
  alias = "jana-md"
  mtu = 4650
  dual_stack_prefer_ipv6 = true
}

resource "gigamon_esxi_connection" "my-conn" {
  alias = "jana-conn"
  monitoring_domain_id = gigamon_esxi_monitoring_domain.my-md.id
  vcenter_address = "10.115.202.13"
  username = "administrator@vsphere.local"
  password = "Gigamon123!"
}

