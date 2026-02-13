terraform {
  required_providers {
    gigamon = {
      source = "local/gigamon/gigamon"
    }
  }
}

provider "gigamon" {
  fm_address = "10.114.58.41"
  skip_verify = true
  api_token = "eyJhbGciOiJIUzI1NiJ9.eyJ0b2tlbklkIjoiODk2MjU3MTgzMjk2MDE0MiIsInN1YiI6IkFDTUVfdG9rZW4iLCJpYXQiOjE3NzA3OTE2MjksImV4cCI6MTc3MzM4MzYyOX0.CHOW-3dNwge-2Ei9egmV4U3VAQHJCvON1UjxJBQDo3A"
}


// Import Config
/*
resource "gigamon_anycloud_monitoring_domain" "md" {
  alias = "test-md"
}

import {
  to = gigamon_anycloud_monitoring_domain.md
  id = "test-md"
}

data "gigamon_anycloud_monitoring_domain" "md" {
  alias = "test-md"
}

resource "gigamon_anycloud_connection" "conn" {
  alias = "test-conn"
  monitoring_domain_id = data.gigamon_anycloud_monitoring_domain.md.id
}

import {
  to = gigamon_anycloud_connection.conn
  id = "test-conn"
}
*/

data "gigamon_anycloud_monitoring_domain" "terraform-md" {
  alias                           = "terraform-md"
}

data "gigamon_anycloud_connection" "terraform-conn" {
  alias                = "terraform-conn"
  monitoring_domain_id = data.gigamon_anycloud_monitoring_domain.terraform-md.id
}

resource "gigamon_monitoring_session" "terraform-ms" {
  alias                = "terraform-ms"
  connection_id        = data.gigamon_anycloud_connection.terraform-conn.id
  monitoring_domain_id = data.gigamon_anycloud_monitoring_domain.terraform-md.id
  description          = "Terraform MS"
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

resource "gigamon_tunnel_out" "terraform_tun" {
  alias                 = "terraform-tunnel-1"
  monitoring_session_id = gigamon_monitoring_session.terraform-ms.id
  remote_ip = "10.114.154.4"
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
  dest_id = gigamon_tunnel_out.terraform_tun.id
}