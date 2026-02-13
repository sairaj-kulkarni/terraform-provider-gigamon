terraform {
  required_providers {
    gigamon = {
      source = "local/gigamon/gigamon"
    }
  }
}

provider "gigamon" {
  fm_address = "10.114.170.57"
  skip_verify = true
  api_token = "eyJhbGciOiJIUzI1NiJ9.eyJ0b2tlbklkIjoiOTEyNTM2MTA2NDc3NjM0MiIsInN1YiI6InRlcnJhZm9ybS10b2tlbiIsImlhdCI6MTc2OTc0MDcyMiwiZXhwIjoxNzc4ODEyNzIyfQ.OZ23mXtCxWaI9CS5_z8o1mmz42HcWk3wdaDqgoiakIw"
}

resource "gigamon_esxi_image" "vseries-6-12" {
  file_name = "/home/vgopu/gigamon-gigavue-vseries-node-6.12.00-550748_amd64.ova"
  timeout = 240
}

resource "gigamon_esxi_monitoring_domain" "terraform-md" {
  alias                           = "terraform-md"
}

# vCenter connection associated with the Monitoring Domain.
resource "gigamon_esxi_connection" "terraform-conn" {
  alias                = "terraform-conn"
  monitoring_domain_id = gigamon_esxi_monitoring_domain.terraform-md.id
  vcenter_address      = "10.203.226.100"
  username             = "vgopu@vsphere.local"
  password             = "1Gigamon#"
}

data "gigamon_esxi_datacenter" "terraform-dc" {
  connection_id    = gigamon_esxi_connection.terraform-conn.id
  data_center_name = "fm-terraform-dev"
}

data "gigamon_esxi_cluster" "terraform-cluster" {
  connection_id     = gigamon_esxi_connection.terraform-conn.id
  data_center_moref = data.gigamon_esxi_datacenter.terraform-dc.data_center_moref
  cluster_name      = "ClusterUno"
}

data "gigamon_esxi_hosts" "terraform-hosts" {
  connection_id     = gigamon_esxi_connection.terraform-conn.id
  data_center_moref = data.gigamon_esxi_datacenter.terraform-dc.data_center_moref

  cluster_moref = [
    data.gigamon_esxi_cluster.terraform-cluster.cluster_moref,
  ]
  hostname = [
    "10.115.169.56"
  ]

}

resource "gigamon_esxi_fabric" "terraform-fabric" {
  name = "terraform-fabric"
  connection_id = gigamon_esxi_connection.terraform-conn.id
  datacenter_moref = data.gigamon_esxi_datacenter.terraform-dc.data_center_moref
  image_id = gigamon_esxi_image.vseries-6-12.id
  dynamic "host_vm_spec" {
    for_each = data.gigamon_esxi_hosts.terraform-hosts.host_details
    content {
      host_moref = host_vm_spec.value.host_moref
      host_name = host_vm_spec.value.hostname
      datastore_moref = host_vm_spec.value.datastore_cluster_moref.fm_terraform_ds
      admin_password = "gigamon123A!!"
      name = "Terraform-VSeries"
      management_interface = {
        network_moref = host_vm_spec.value.network_moref.VM-Network
      }
      tunnel_interface = {
        network_moref = host_vm_spec.value.network_moref.VM-Network
      }
    }
  }
}

resource "gigamon_monitoring_session" "terraform-ms" {
  alias                = "terraform-ms"
  connection_id        = gigamon_esxi_connection.terraform-conn.id
  monitoring_domain_id = gigamon_esxi_monitoring_domain.terraform-md.id
  description          = "Terraform MS"

  depends_on = [
    gigamon_esxi_fabric.terraform-fabric,
  ]
}

resource "gigamon_trafficmap" "terraform-map" {
  name                  = "terraform-map"
  monitoring_session_id = gigamon_monitoring_session.terraform-ms.id
  comment               = "Pass all IPv4 traffic"

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