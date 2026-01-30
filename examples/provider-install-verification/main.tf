# Copyright (c) Gigamon Inc

# This is an example Terraform/Opentofu file, for Gigamon cloud fabric deployment.
#  Platform/cloud provider   - VMWare ESXI
#  Traffic Acquistion Method - Port Mirroring

# Define the usage of Gigamon Provider. For now refering to it from a local source
# i.e. the local file system
terraform {
  required_providers {
    gigamon = {
      source = "tf-proj.gigamon.com/gigamon/gigamon"
    }
  }
}

# Provide the provider required parameters. Currently we support api_token based
# authentication to FM. The user must login to FM and generate the token and provide it
# here.

# please note that in this example the api_token and other sensitive information like
# passwords are provided in plain text. This is only for sample and production environment
# should use secure mecahnisms like vault

provider "gigamon" {
  fm_address = "10.114.84.25"

  # skip_verify is default false, which implies that the certificate presented by FM must be
  # a valid certificate and will be verified. For demo purpose this is skipped, but should not
  # be set in productino environment
  skip_verify = true

  # this token is generated using FM API, via  the user management section. For best
  # security rotate this token often and also use mecahnisms like vault to prevent exposing
  # this in plain text in the configuration files
  api_token = "eyJhbGciOiJIUzI1NiJ9.eyJ0b2tlbklkIjoiNDczMTkwMjk3MzIzMDI4MyIsInN1YiI6ImphbmEtdG9rZW4iLCJpYXQiOjE3Njk3NDgwOTEsImV4cCI6MTc3NzUyNDA5MX0.psb4Qq6vsvuZgGFjAgNcshKz0z94nSCHC7_jT-1oHxk"
}

# Upload the Vseries Image to FM.
resource "gigamon_esxi_image" "vseries-6-14" {
  file_name = "/home/jana/gigamon-gigavue-vseries-node-6.14.00-563398_amd64.ova"

  # Adjust the timeout to the needed value based on the size of the file and network speed
  timeout = 240
}

# Create a monitoring domain. The Vsereis fabric is deployed in this Monitoring Domain.
resource "gigamon_esxi_monitoring_domain" "my-md" {
  alias = "jana-md"
  use_public_ip_for_notifications = true
}

# This represents the connection the vSphere. Please use Vault and do not expose the password
# in plain text in production environments. The connection is associated with the Monitoring
# domain created above.

resource "gigamon_esxi_connection" "my-conn" {
  alias = "jana-conn"
  monitoring_domain_id = gigamon_esxi_monitoring_domain.my-md.id
  vcenter_address = "10.115.202.13"
  username = "administrator@vsphere.local"
  password = "Gigamon123!"
}

# Once the connection is setup, FM will do an inventory collection. This will allow
# us to query FM to get the details of the various objects like host/clsuter/datastore
# from FM.

# while it is possible to query these directly from vSpehere also, it may be better to
# query these from FM, to ensure that FM and this configuration files are in sync.

# the below datastore calls, fetch the required information like Datacenter, Datastore,
# hosts etc. which are needed for creating the fabric (i.e. Vseries deployment)

# In the below example, the monitoring Domain is used to monitor
# all the hosts in the cluster ClusterUno belonging to the datacenter Datacenter.
# The Vseries nodes management and tunnel interfaces are connected to the VM Network


# Get the datacenter MORef for the specified datacenter
data "gigamon_esxi_datacenter" "my-dc" {
  connection_id = gigamon_esxi_connection.my-conn.id
  data_center_name = "Datacenter"
}

# Gets the cluster MORef
data "gigamon_esxi_cluster" "my-cluster" {
  connection_id = gigamon_esxi_connection.my-conn.id
  data_center_moref = data.gigamon_esxi_datacenter.my-dc.data_center_moref
  cluster_name = "ClusterUno"
}

# Get the Datastore cluster on which we are going to deploy the Vseries
data "gigamon_esxi_datastore_cluster" "my-ds-cluster" {
  connection_id = gigamon_esxi_connection.my-conn.id
  data_center_moref = data.gigamon_esxi_datacenter.my-dc.data_center_moref
  datastore_cluster_name = "DatastoreCluster"
}

# Get the network MORef which is used to connect the Vseries management/tunnel interfaces
data "gigamon_esxi_networks" "my-net" {
  connection_id = gigamon_esxi_connection.my-conn.id
  data_center_moref = data.gigamon_esxi_datacenter.my-dc.data_center_moref
  network_name = "VM Network"
}

# Get the list of hosts
data "gigamon_esxi_hosts" "my-hosts" {
  connection_id = gigamon_esxi_connection.my-conn.id
  data_center_moref = data.gigamon_esxi_datacenter.my-dc.data_center_moref

  # cluster_moref is used to specify the which hosts to fetch. If left empty it will fetch
  # all the hosts in the datacenter. It is possible to also spceify hostname or hostpattern
  # to restrict the hosts further

  cluster_moref = [
    data.gigamon_esxi_cluster.my-cluster.cluster_moref,
  ]
  hostname = "10.115.201.43"
}

data "gigamon_esxi_hosts" "my-hosts-1" {
  connection_id = gigamon_esxi_connection.my-conn.id
  data_center_moref = data.gigamon_esxi_datacenter.my-dc.data_center_moref

  # cluster_moref is used to specify the which hosts to fetch. If left empty it will fetch
  # all the hosts in the datacenter. It is possible to also spceify hostname or hostpattern
  # to restrict the hosts further

  cluster_moref = [
    data.gigamon_esxi_cluster.my-cluster.cluster_moref,
  ]
  hostname = "10.115.201.44"
}

# Setting up the VSeries Fabric
resource "gigamon_esxi_fabric" "my-fabric" {
  name = "my-fabric"
  connection_id = gigamon_esxi_connection.my-conn.id
  datacenter_moref = data.gigamon_esxi_datacenter.my-dc.data_center_moref
  form_factor = "small"
  image_id = gigamon_esxi_image.vseries-6-14.id
  vm_folder = "/"
  datastore_cluster_moref = data.gigamon_esxi_datastore_cluster.my-ds-cluster.datastore_cluster_moref
  disk_format = "thick"

  # This is the password of the user gigamon on the VSereis that are spun up
  admin_password = "Gigamon123A!!"

  management_interface_spec = {
    network_moref = data.gigamon_esxi_networks.my-net.network_moref
	address_assignment_mode = "DHCP"
  }
  tunnel_interface_spec = {
    network_moref = data.gigamon_esxi_networks.my-net.network_moref
	address_assignment_mode = "DHCP"
  }
  dynamic "host_vm_spec"  {
    for_each = data.gigamon_esxi_hosts.my-hosts.host_details
	  content {
	    host_moref = host_vm_spec.value.host_moref
	    host_name = host_vm_spec.key

		# This is the name assigned to the vseries that is spun up on this host
	    name = host_vm_spec.key
	}
  }
}

# 
# Creates a Monitoring Session
resource "gigamon_esxi_monitoring_session" "my-ms" {
  alias = "jana-ms"
  connection_id = gigamon_esxi_connection.my-conn.id
  monitoring_domain_id = gigamon_esxi_monitoring_domain.my-md.id
  description = "MY MS"
  depends_on = [
    gigamon_esxi_fabric.my-fabric,
  ]
    
}

# Configure the dedup app parameters in this MD
resource "gigamon_dedup_md_config" "my-dedup-config"{
  monitoring_domain_id = gigamon_esxi_monitoring_domain.my-md.id
  action = "count"
  timer = 45000
}

# Create a Dedup App in this MS
resource "gigamon_app_dedup" "my-dedup" {
  monitoring_session_id = gigamon_esxi_monitoring_session.my-ms.id
  alias = "jana-dedup"
  description = "this is a good dedup app used for testing"
}

# Create a Masking App in this MS
resource "gigamon_app_masking" "my-masking" {
  monitoring_session_id = gigamon_esxi_monitoring_session.my-ms.id
  alias = "jana-masking"
  length = 6
  pattern = "0xFF"
}

# Create the APP Slicing
resource "gigamon_app_slicing" "my-slicing" {
  monitoring_session_id = gigamon_esxi_monitoring_session.my-ms.id
  alias = "jana-slicing"
  # offset = 128
  protocol = "udp"
}

resource "gigamon_trafficmap" "my-map" {
  name = "jana-map"
  monitoring_session_id = gigamon_esxi_monitoring_session.my-ms.id
  comment = "My trial map"
  rule_sets = [
	{
	  rule_set_id = "2"
	  priority = 1
	  aep_id = 2
	  pass_rules = [
	    {
	      rule_id = 1
		  ether_type = {
		    ether_type = "0x1600"
		  }
	    }
	  ]
	},
    {
	  rule_set_id = "1"
	  priority = 2
	  aep_id = 3
	  pass_rules = [
	    {
		  rule_id = 3
		  ether_type = {
		    ether_type = "0x400"
		  }
		},
		{
		  rule_id = 4
		  l2_src_mac = {
		    mac_address = "22:33:44:11:55:66"
		  },
		  l2_dst_mac = {
		    mac_address = "22:11:33:44:55:66"
		  }
		}
	  ]
	  drop_rules = [
	    {
		  rule_id = 1
	      ether_type = {
		    ether_type = "0x800"
		  },
		  l2_src_mac = {
		    mac_address = "aa:bb:cc:dd:ee:ff"
	      }
		},
		{
		  rule_id = 2
		  ether_type = {
		    ether_type = "0x900"
		  },
		  l2_dst_mac = {
		    mac_address = "11:22:33:44:55:66"
		  }
		}
      ]
	},
  ]
}

action "gigamon_ms_position" "position-objects" {
  provider = gigamon
  config {
    monitoring_session_ids = [
	  gigamon_esxi_monitoring_session.my-ms.id,
    ]
  }
}
