# Copyright (c) HashiCorp, Inc.

terraform {
  required_providers {
    gigamon = {
      source = "local/gigamon/gigamon"
    }
  }
}

provider "gigamon" {
  fm_address = "10.114.202.120"
  skip_verify = true
  api_token = "eyJhbGciOiJIUzI1NiJ9.eyJ0b2tlbklkIjoiOTIxNjgzMDk0MjA0ODQ3NSIsInN1YiI6InRmLXRva2VuIiwiaWF0IjoxNzYyMzMwMjk4LCJleHAiOjE3NjQ5MjIyOTh9.WPPhWxx_MeG40RgIJYZVm0zt1v-ahyutPRQzUVWVf_0"
}

resource "gigamon_esxi_monitoring_domain" "my-md" {
  alias = "jana-md"
}

resource "gigamon_esxi_connection" "my-conn" {
  alias = "jana-conn"
  monitoring_domain_id = gigamon_esxi_monitoring_domain.my-md.id
  vcenter_address = "10.115.202.13"
  username = "administrator@vsphere.local"
  password = "Gigamon123!"
}

data "gigamon_esxi_datacenter" "my-dc" {
  connection_id = gigamon_esxi_connection.my-conn.id
  data_center_name = "Datacenter"
}

data "gigamon_esxi_cluster" "my-cluster" {
  connection_id = gigamon_esxi_connection.my-conn.id
  data_center_moref = data.gigamon_esxi_datacenter.my-dc.data_center_moref
  cluster_name = "ClusterUno"
}

data "gigamon_esxi_cluster" "my-cluster-1" {
  connection_id = gigamon_esxi_connection.my-conn.id
  data_center_moref = data.gigamon_esxi_datacenter.my-dc.data_center_moref
  cluster_name = "ClusterTres"
}
data "gigamon_esxi_datastore" "my-datastore" {
  connection_id = gigamon_esxi_connection.my-conn.id
  data_center_moref = data.gigamon_esxi_datacenter.my-dc.data_center_moref
  datastore_name = "datastore_10.115.201.43"
}
data "gigamon_esxi_datastore_cluster" "my-d-cluster" {
  connection_id = gigamon_esxi_connection.my-conn.id
  data_center_moref = data.gigamon_esxi_datacenter.my-dc.data_center_moref
  datastore_cluster_name = "DatastoreCluster"
}
data "gigamon_esxi_networks" "my-net" {
  connection_id = gigamon_esxi_connection.my-conn.id
  data_center_moref = data.gigamon_esxi_datacenter.my-dc.data_center_moref
  network_name = "VM Network"
}
data "gigamon_esxi_vds_portgroups" "my-pgrp" {
  connection_id = gigamon_esxi_connection.my-conn.id
  data_center_moref = data.gigamon_esxi_datacenter.my-dc.data_center_moref
  portgroup_name = "VDS-ClusterTres-Management-Network"
}

data "gigamon_esxi_hosts" "my-hosts" {
  connection_id = gigamon_esxi_connection.my-conn.id
  data_center_moref = data.gigamon_esxi_datacenter.my-dc.data_center_moref
  #cluster_moref = [
    #data.gigamon_esxi_cluster.my-cluster.cluster_moref,
    #data.gigamon_esxi_cluster.my-cluster-1.cluster_moref
  #]
  # hostname_pattern = "10.115"
  # hostname = "10.115.201.45"
}

resource "gigamon_esxi_fabric" "my-fabric" {
  name = "my-fabric"
  connection_id = "abc"
  datacenter_moref = "def"
  form_factor = "small"
  image_id = "abc"
  host_vm_spec = {
    host1 = {
	  hostname = "host1"
	  host_moref = "abc"
	  name = "vseries"
	  management_interface_spec = {
	    interface_name = "abc"
		interface_moref = "def"
	  }
	  tunnel_interface_spec = {
	    interface_name = "ijk"
		 interface_moref = "lkj"
	  }
	}
    host2 = {
	  hostname = "host2"
	  host_moref = "abc"
	  name = "vseries"
	  management_interface_spec = {
	    interface_name = "abc"
		interface_moref = "def"
	  }
	  tunnel_interface_spec = {
	    interface_name = "ijk"
		 interface_moref = "lkj"
	  }
	}
  }
}
   

