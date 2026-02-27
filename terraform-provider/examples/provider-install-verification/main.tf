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


# Create a monitoring domain. The Vsereis fabric is deployed in this Monitoring Domain.
resource "gigamon_esxi_monitoring_domain" "my-md" {
  alias = "jana-md"
  use_public_ip_for_notifications = true
}

# This represents the connection the vSphere. Please use Vault and do not expose the password
# in plain text in production environments. The connection is associated with the Monitoring
# domain created above.

resource "gigamon_esxi_connection" "my-conn" {
  alias = "jana-conn-test"
  monitoring_domain_id = gigamon_esxi_monitoring_domain.my-md.id
  maximum_nodes_per_host = 5
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
  cluster_name = "ClusterDeux"
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
  hostname = [
    # "10.115.201.45",
    "10.115.201.46",
  ]
}

# Upload the Vseries Image to FM.
resource "gigamon_esxi_image" "vseries-6-14" {
  file_name = "/home/jana/gigamon-gigavue-vseries-node-6.14.00-563398_amd64.ova"

  # Adjust the timeout to the needed value based on the size of the file and network speed
  timeout = 240
}

resource "gigamon_esxi_image" "vseries-6-14-01" {
  file_name = "/home/jana/gigamon-gigavue-vseries-node-6.14.00-564867_amd64.ova"

  # Adjust the timeout to the needed value based on the size of the file and network speed
  timeout = 240
}

# Prepare the host_vm_spec map, from the data source response.
# this is a key:value where the key is the host-MORef and the value is the parameters of
# the host spec object
locals {
  hostspec = {
    for host, host_spec in data.gigamon_esxi_hosts.my-hosts.host_details: host_spec.host_moref =>   {
      host_name = host_spec.hostname
      host_moref = host_spec.host_moref
      datastore_cluster_moref = host_spec.datastore_cluster_moref.datastore_qnap2tb
      admin_password = "gigamon123A!"
      name_server = [
        "8.8.8.8",
        "8.8.4.4",
      ]
      name = host_spec.hostname
      management_interface = {
        network_moref = host_spec.network_moref.VM-Network
      }
      tunnel_interface = {
         network_moref = host_spec.network_moref.VM-Network
         ipv6_prefix_length = "64"
      }
    }
  }
}

resource "gigamon_esxi_fabric" "my-fabric" {
  name = "my-fabric"
  connection_id = gigamon_esxi_connection.my-conn.id
  datacenter_moref = data.gigamon_esxi_datacenter.my-dc.data_center_moref
  image_id = gigamon_esxi_image.vseries-6-14.id
  # image_id = gigamon_esxi_image.vseries-6-14-01.id
  host_vm_spec = local.hostspec
  form_factor = "Small"
}
