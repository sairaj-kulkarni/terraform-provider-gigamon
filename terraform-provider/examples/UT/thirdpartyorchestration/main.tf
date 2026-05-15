
//Provider
terraform {
  required_providers {
    gigamon = {
      source = "local/gigamon/gigamon"
    }
  }
}

//FM Client OpenStack 3PO
provider "gigamon" {
  fm_address = "10.115.35.149"
  skip_verify = true
  api_token = "eyJhbGciOiJIUzI1NiJ9.eyJ0b2tlbklkIjoiMzk3NzY0NzgxMTI0MTI2NiIsInN1YiI6InRlcnJhZm9ybS10b2tlbiIsImlhdCI6MTc3NDAwMTY1OCwiZXhwIjoxNzgzMDczNjU4fQ.WmoJ9CfoTCVB8XgDWcQYtZittiSUkHphfGxhoYA6IiE"
}

/*
//FM Client ESXi 3PO
provider "gigamon" {
  fm_address = "10.114.170.47"
  skip_verify = true
  api_token = "eyJhbGciOiJIUzI1NiJ9.eyJ0b2tlbklkIjoiMzM0MjYyODgzNDMzNjMwOSIsInN1YiI6InRlcnJhZm9ybS10b2tlbiIsImlhdCI6MTc3NDAxNTI2NCwiZXhwIjoxNzgzMDg3MjY0fQ.J3gHLse9Y4t_3qWj4UM5uj-kcbR1oIZALo0iWyfpzs0"
}
*/


// Monitoring Domain
/*
// Resource
resource "gigamon_third_party_orchestration_monitoring_domain" "terraform-md" {
  alias = "terraform-md"
  uctv = {
    mtu = 1350
    dual_stack_prefer_ipv6 = true
  }
  #customer_orchestrated_source = {
    #uniform_traffic_policy = true
  #}
}

resource "gigamon_third_party_orchestration_connection" "terraform-conn" {
  alias = "terraform-conn"
  tapping_method = gigamon_third_party_orchestration_monitoring_domain.terraform-md.tapping_method
  monitoring_domain_id = gigamon_third_party_orchestration_monitoring_domain.terraform-md.id
}
*/

/*
// Import Config
resource "gigamon_third_party_orchestration_monitoring_domain" "terraform-md" {
  alias = "MD_Vijay"
  uctv = {
    mtu = 1350
    dual_stack_prefer_ipv6 = true
  }
}

import {
  to = gigamon_third_party_orchestration_monitoring_domain.terraform-md
  id = "MD_Vijay"
}

resource "gigamon_third_party_orchestration_connection" "terraform-conn" {
  alias = "CONN_Vijay"
  tapping_method = gigamon_third_party_orchestration_monitoring_domain.terraform-md.tapping_method
  monitoring_domain_id = gigamon_third_party_orchestration_monitoring_domain.terraform-md.id
}

import {
  to = gigamon_third_party_orchestration_connection.terraform-conn
  id = "CONN_Vijay"
}
*/


// Data Source ESXi
/*
data "gigamon_third_party_orchestration_monitoring_domain" "terraform-md" {
  alias                = "esxi_md"
}

data "gigamon_third_party_orchestration_connection" "terraform-conn" {
  alias                = "esxi_conn"
  monitoring_domain_id = data.gigamon_third_party_orchestration_monitoring_domain.terraform-md.id
}
*/

// Data Source OpenStack
data "gigamon_third_party_orchestration_monitoring_domain" "terraform-md" {
  alias                = "open_stack_md"
}

data "gigamon_third_party_orchestration_connection" "terraform-conn" {
  alias                = "open_stack_conn"
  monitoring_domain_id = data.gigamon_third_party_orchestration_monitoring_domain.terraform-md.id
}

// Motorting Session
resource "gigamon_monitoring_session" "terraform-ms" {

  depends_on = [
    gigamon_secure_tunnel_certs_apply.certs_apply
  ]

  alias                = "terraform-ms"
  monitoring_domain_id = data.gigamon_third_party_orchestration_monitoring_domain.terraform-md.id
  connection_id        = data.gigamon_third_party_orchestration_connection.terraform-conn.id
  tapping_method       = data.gigamon_third_party_orchestration_connection.terraform-conn.tapping_method
  description          = "Terraform MS"
  distribute_traffic   = false
  fast_mode   = true
  scale_unit   = 3

  traffic_acquisition = {
    mirroring = {
      secure_tunnels_enabled = false

      uctv_filtering_policy = {
        rules = [
          {
            rule_name  = "TCP",
            action     = "pass",
            direction  = "bidi",
            priority   = 1
            filters = [
              {
                name = "proto",
                relation = "EQUAL_TO",
                value = "TCP"
              }
            ]
          },
          {
            rule_name  = "UDP",
            action     = "pass",
            direction  = "bidi",
            priority   = 2
            filters = [
              {
                name = "proto",
                relation = "EQUAL_TO",
                value = "UDP"
              }
            ]
          }
        ]
      }
    }
    precryption = {
      secure_tunnels_enabled = true
    }
  }
}

/*
  lifecycle {
    replace_triggered_by = [
      gigamon_third_party_orchestration_monitoring_domain.terraform-md.uctv.mtu,
      gigamon_third_party_orchestration_monitoring_domain.terraform-md.uctv.dual_stack_prefer_ipv6,
    ]
  }
}
*/

resource "gigamon_traffic_map" "terraform-map" {
  name                  = "terraform-map"
  monitoring_session_id = gigamon_monitoring_session.terraform-ms.id

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
  remote_ip = "40.40.4.6"
  mtu = 1400

  vxlan {
   vni = 2345
   destination_port = 4789
  }
}

resource "gigamon_link" "map_to_dedup" {
  monitoring_session_id = gigamon_monitoring_session.terraform-ms.id
  source_id    = gigamon_traffic_map.terraform-map.id
  source_aep_id = 2
  dest_id = gigamon_app_dedup.terraform-dedup.id
}

resource "gigamon_link" "dedup_to_tunnel" {
  monitoring_session_id = gigamon_monitoring_session.terraform-ms.id
  source_id = gigamon_app_dedup.terraform-dedup.id
  dest_id = gigamon_tunnel_out.terraform-tunnel.id
}

/*
resource "gigamon_link" "map_to_tunnel" {
  monitoring_session_id = gigamon_monitoring_session.terraform-ms.id
  source_id    = gigamon_traffic_map.terraform-map.id
  source_aep_id = 2
  dest_id = gigamon_tunnel_out.terraform-tunnel.id
}
*/

/*
// Secure Tunnel Certificates Configuration
resource "gigamon_cloud_ca_cert" "ca_cert" {
  alias = "UCTV_CA_CERT2"
  certificate_path   = "/home/vgopu/certs2/UCTV.crt"
}

import {
  to = gigamon_cloud_ca_cert.ca_cert
  id = "UCTV_CA_CERT"
}

resource "gigamon_cloud_ssl_keys" "ssl_keys" {
  alias = "VSN_SSK_KEYS2"
  key_store_alias    = "DEFAULT_CLOUD_SSL_KS"
  certificate_path = "/home/vgopu/certs2/VSN.crt"
  private_key_path = "/home/vgopu/certs2/VSN.key"
}

import {
  to = gigamon_cloud_ssl_keys.ssl_keys
  id = "VSN_SSK_KEYS"
}
*/

data "gigamon_cloud_ca_cert" "ca_cert" {
  alias = "UCTV_CA_CERT2"
}

data "gigamon_cloud_ssl_keys" "ssl_keys" {
  alias = "VSN_SSK_KEYS1"

}

resource "gigamon_secure_tunnel_certs_apply" "certs_apply" {
  monitoring_domain_ids = [
    data.gigamon_third_party_orchestration_monitoring_domain.terraform-md.id,
  ]

  uctv_ca_cert_alias = data.gigamon_cloud_ca_cert.ca_cert.alias
  vsn_ssl_key_alias  = data.gigamon_cloud_ssl_keys.ssl_keys.alias
  key_store_alias    = data.gigamon_cloud_ssl_keys.ssl_keys.key_store_alias
}