terraform {
 required_providers {
 gigamon = {
 source = "local/gigamon/gigamon"
 }
 }
}


# terraform {
#  required_providers {
#     gigamon = {
#         source = "tf-proj.gigamon.com/gigamon/gigamon"
#         version = "6.14.0"
#     }
#  }
# }

# provider "gigamon" {
# fm_address = "10.114.170.124"
# skip_verify = true
# api_token = "eyJhbGciOiJIUzI1NiJ9.eyJ0b2tlbklkIjoiMzM0MjYyODgzNDMzNjMwOSIsInN1YiI6InRlcnJhZm9ybS10b2tlbiIsImlhdCI6MTc3NDAxNTI2NCwiZXhwIjoxNzgzMDg3MjY0fQ.J3gHLse9Y4t_3qWj4UM5uj-kcbR1oIZALo0iWyfpzs0"
# }

provider "gigamon" {
 fm_address = "10.114.170.57"
 skip_verify = true
 api_token = "eyJhbGciOiJIUzI1NiJ9.eyJ0b2tlbklkIjoiNzYwNTMxOTI2Nzc2MDc4MCIsInN1YiI6InRlcnJhZm9ybS11c2VyIiwiaWF0IjoxNzc5ODU0MjE2LCJleHAiOjE3ODI0NDYyMTZ9.9t4y_dVXDP9yABUzOsxaMgWC3-ccO_8NRJoxU74LQLs"
}

########################################
# 3PO Monitoring Domain + Connection
########################################
data "gigamon_third_party_orchestration_monitoring_domain" "md" {
 alias = "amx-md"
}

data "gigamon_third_party_orchestration_connection" "conn" {
 alias = "amx-conn"
 monitoring_domain_id = data.gigamon_third_party_orchestration_monitoring_domain.md.id
}


########################################
# vSeries interfaces
########################################
data "gigamon_vseries_interfaces" "vsn" {
 connection_id = data.gigamon_third_party_orchestration_connection.conn.id
}

########################################
# Monitoring Session
########################################
resource "gigamon_monitoring_session" "ms" {
 alias = "amx-ms"
 monitoring_domain_id = data.gigamon_third_party_orchestration_monitoring_domain.md.id
 connection_id = data.gigamon_third_party_orchestration_connection.conn.id
 tapping_method = "customerOrchestratedSource"
 description = "REP -> amx -> REP topology"
 distribute_traffic = false
}

########################################
# Raw endpoints
########################################
resource "gigamon_raw_endpoint" "src_rep" {
 monitoring_session_id = gigamon_monitoring_session.ms.id
 alias = "rep-src"
 description = "Source REP"
}

resource "gigamon_raw_endpoint" "dst_rep" {
 monitoring_session_id = gigamon_monitoring_session.ms.id
 alias = "rep-dst"
 description = "Destination REP"
}

########################################
# Minimal AMX app for secure_endpoint repro
########################################
resource "gigamon_app_amx" "amx_min" {
 alias = "amx-secure-endpoint-test"
 monitoring_session_id = gigamon_monitoring_session.ms.id

 ingestor {
 name = "ami-ingestor-1"
 port = 2055
 type = "ami"
 }

 exporter {
 http_export {
 name = "http-secure-test"
 enabled = true
 data_type = "ami"
 endpoint = "https://collector.example.com/ingest?api_key=supersecret"
 secure_endpoint = true
 headers = ["Authorization: Bearer supersecret"]
 secure_keys = ["Authorization"]
 compress = true
 flush_interval_seconds = 30
 parallel_workers = 4
 max_retries = 4
 max_records_per_batch = 5000
 self_heal_window_seconds = 0
 upload_timeout_seconds = 10
 }
 }
}
########################################
# Links: REP -> AMX -> REP
########################################
resource "gigamon_link" "rep_to_amx" {
 monitoring_session_id = gigamon_monitoring_session.ms.id
 source_id = gigamon_raw_endpoint.src_rep.id
 dest_id = gigamon_app_amx.amx_min.id
}

resource "gigamon_link" "amx_to_rep" {
 monitoring_session_id = gigamon_monitoring_session.ms.id
 source_id = gigamon_app_amx.amx_min.id
 dest_id = gigamon_raw_endpoint.dst_rep.id
}


########################################
# Endpoint <-> interface mapping
########################################
locals {
 vsn_node_ids = keys(data.gigamon_vseries_interfaces.vsn.nodes)
 first_node_id = local.vsn_node_ids[0]

 first_node_data_ifaces = [
 for iface_name, ips in data.gigamon_vseries_interfaces.vsn.nodes[local.first_node_id].interface_name_to_ipv4 :
 iface_name if length(ips) > 0
 ]

 iface_for_src = local.first_node_data_ifaces[0]
 iface_for_dst = length(local.first_node_data_ifaces) > 1 ? local.first_node_data_ifaces[1] : local.first_node_data_ifaces[0]
}

resource "gigamon_endpoint_iface_mapping" "map" {
 monitoring_session_id = gigamon_monitoring_session.ms.id

 vseries_node_ids = [
 for id in keys(data.gigamon_vseries_interfaces.vsn.nodes) : id
 ]

 mapping {
 iface = "ens224"
 endpoint_id = gigamon_raw_endpoint.src_rep.id
 }

 mapping {
 iface = "ens192"
 endpoint_id = gigamon_raw_endpoint.dst_rep.id
 }
}