
//Provider
terraform {
  required_providers {
    gigamon = {
      source = "local/gigamon/gigamon"
    }
  }
}

//FM Client
provider "gigamon" {
  fm_address = "10.114.58.41"
  skip_verify = true
  api_token = "eyJhbGciOiJIUzI1NiJ9.eyJ0b2tlbklkIjoiODk2MjU3MTgzMjk2MDE0MiIsInN1YiI6IkFDTUVfdG9rZW4iLCJpYXQiOjE3NzA3OTE2MjksImV4cCI6MTc3MzM4MzYyOX0.CHOW-3dNwge-2Ei9egmV4U3VAQHJCvON1UjxJBQDo3A"
}

/*
provider "gigamon" {
  fm_address = "10.114.170.57"
  skip_verify = true
  api_token = "eyJhbGciOiJIUzI1NiJ9.eyJ0b2tlbklkIjoiOTEyNTM2MTA2NDc3NjM0MiIsInN1YiI6InRlcnJhZm9ybS10b2tlbiIsImlhdCI6MTc2OTc0MDcyMiwiZXhwIjoxNzc4ODEyNzIyfQ.OZ23mXtCxWaI9CS5_z8o1mmz42HcWk3wdaDqgoiakIw"
}


// Monitoring Domain
// Resource
resource "gigamon_anycloud_monitoring_domain" "terraform-md" {
  alias = "testing"
  uctv = {
    mtu = 1350
    dual_stack_prefer_ipv6 = false
  }
}

resource "gigamon_anycloud_connection" "terraform-conn" {
  alias = "testing"
  tapping_method = gigamon_anycloud_monitoring_domain.terraform-md.tapping_method
  monitoring_domain_id = gigamon_anycloud_monitoring_domain.terraform-md.id
}
*/


// Import Config
resource "gigamon_anycloud_monitoring_domain" "terraform-md" {
  alias = "MD_Vijay"
  uctv = {
    #mtu = 1350
    dual_stack_prefer_ipv6 = true
  }
}

import {
  to = gigamon_anycloud_monitoring_domain.terraform-md
  id = "MD_Vijay"
}

resource "gigamon_anycloud_connection" "terraform-conn" {
  alias = "CONN_Vijay"
  tapping_method = gigamon_anycloud_monitoring_domain.terraform-md.tapping_method
  monitoring_domain_id = gigamon_anycloud_monitoring_domain.terraform-md.id
}

import {
  to = gigamon_anycloud_connection.terraform-conn
  id = "CONN_Vijay"
}

/*
// Data Source
data "gigamon_anycloud_monitoring_domain" "terraform-md" {
  alias                           = "MD_Vijay"
}

data "gigamon_anycloud_connection" "terraform-conn" {
  alias                = "CONN_Vijay"
  monitoring_domain_id = data.gigamon_anycloud_monitoring_domain.terraform-md.id
}
*/

// Motorting Session
resource "gigamon_monitoring_session" "terraform-ms" {
  alias                = "terraform-ms"
  connection_id        = gigamon_anycloud_connection.terraform-conn.id
  monitoring_domain_id = gigamon_anycloud_monitoring_domain.terraform-md.id
  description          = "Terraform MS"

  lifecycle {
    replace_triggered_by = [
      gigamon_anycloud_monitoring_domain.terraform-md.uctv.mtu,
      gigamon_anycloud_monitoring_domain.terraform-md.uctv.dual_stack_prefer_ipv6,
    ]
  }
}

resource "gigamon_trafficmap" "terraform-map" {
  name                  = "terraform-map"
  monitoring_session_id = gigamon_monitoring_session.terraform-ms.id
  comment               = "Pass all IPv4 traffic from specific MAC"

  rule_sets = [
    {
      rule_set_id = "1"
      priority    = 1
      aep_id      = 2

      pass_rules = [
        {
          rule_id = 1

          ip_version = {
            ip_version = "v4"
          }
        }
      ]
    }
  ]
}

resource "gigamon_app_dedup" "terraform-dedup" {
  monitoring_session_id = gigamon_monitoring_session.terraform-ms.id
  alias                 = "terraform-dedup"
}

resource "gigamon_tunnel_out" "terraform-tunnel" {
  alias                 = "terraform-tunnel"
  monitoring_session_id = gigamon_monitoring_session.terraform-ms.id
  remote_ip = "10.114.58.46"
  vxlan {
   vni = 1
   destination_port = 1
  }
}

resource "gigamon_link" "map_to_dedup" {
  monitoring_session_id = gigamon_monitoring_session.terraform-ms.id
  source_id    = gigamon_trafficmap.terraform-map.id
  source_aep_id = 2
  dest_id = gigamon_app_dedup.terraform-dedup.id
}

resource "gigamon_link" "dedup_to_tunnel" {
  monitoring_session_id = gigamon_monitoring_session.terraform-ms.id
  source_id = gigamon_app_dedup.terraform-dedup.id
  dest_id = gigamon_tunnel_out.terraform-tunnel.id
}
