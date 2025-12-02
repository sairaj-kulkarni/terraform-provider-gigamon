// Copyright (c) Gigamon, Inc.

// Implements utility functions that interact with FM and provides the responses.

package esxiutils

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"

	"terraform-provider-gigamon/internal/fmclient"
)

// Data struct for the response of VMWare ESXI get on the monitoring domain for various
// inventory objects

// DataStore response
type FmDataStoreResp struct {
	Name                   string `json:"name"`
	Ref                    string `json:"ref"`
	DataCenterName         string `json:"datacenterName"`
	DataCenterRef          string `json:"datacenterRef"`
	DataStoreClusterName   string `json:"datastoreClusterName"`
	DataStoreClusterRef    string `json:"datastoreClusterRef"`
	DataStoreClusterMember bool   `json:"datastoreClusterMember"`
}

// Network Response
type FmNetworksResp struct {
	Name           string `json:"name"`
	Ref            string `json:"ref"`
	DataCenterName string `json:"datacenterName"`
	DataCenterRef  string `json:"datacenterRef"`
}

// Distributed Switch Response
type FmVDSPortGroups struct {
	Name string `json:"name"`
	Ref  string `json:"ref"`
}

type FmDistributedSwitchResp struct {
	DataCenterName string            `json:"datacenterName"`
	DataCenterRef  string            `json:"datacenterRef"`
	PortGroups     []FmVDSPortGroups `json:"portGroups"`
}

// FM Host Response
type FmHostResp struct {
	Name           string `json:"name"`
	Ref            string `json:"ref"`
	DataCenterName string `json:"datacenterName"`
	DataCenterRef  string `json:"datacenterRef"`
	ClusterName    string `json:"clusterName"`
	ClusterRef     string `json:"clusterRef"`
}

// Returend data for host datastore request
type HostDSResp struct {
	HostRef    string
	ClusterRef string
}

// Returns the set of host Ref mapping to the hosts that we are querying
// The returned hosts are filtered by
// Datacenter which is requested
//
//	restricted to the set of clusters (if the cluster is specified)
//	retricted to patterns matching with the host_pattern
//	  Only one of host_name or host_name_pattern must be specified
func GetHostsRef(
	ctx context.Context,
	connectionId, datacenterRef string,
	clusterRef []string,
	hostPattern string,
	client *fmclient.FmClient,
) (map[string]HostDSResp, error) {

	retHosts := make(map[string]HostDSResp)

	re, err := regexp.Compile(hostPattern)
	if err != nil {
		return nil, fmt.Errorf("Given pattern: %s is wrong, and does not compile: %s", hostPattern, err)
	}

	fmHostData := struct {
		Hosts []FmHostResp `json:"hosts"`
	}{
		Hosts: make([]FmHostResp, 0),
	}
	resp, err := client.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/vmware/fabricDeployment/hosts"),
		map[string]string{"connId": connectionId},
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("Get request of portgrop calls with: %s failed: %s", connectionId, err)
	}
	err = json.Unmarshal(resp, &fmHostData)
	if err != nil {
		return nil, fmt.Errorf("Unable to convert resp to struct: %s error is: %s", string(resp), err)
	}

	// Check if the required VDS Portgroup is there and return its MORef
	for _, hData := range fmHostData.Hosts {
		if hData.DataCenterRef == datacenterRef &&
			(len(clusterRef) == 0 || slices.Contains(clusterRef, hData.ClusterRef)) &&
			re.FindString(hData.Name) != "" {
			retHosts[hData.Name] = HostDSResp{
				HostRef:    hData.Ref,
				ClusterRef: hData.ClusterRef,
			}
		}
	}
	if len(retHosts) == 0 {
		return nil, fmt.Errorf("Unable to find hostname: %s in FM Connection: %s", hostPattern, connectionId)
	}
	return retHosts, nil
}

// Returns the VDS Portgroup MORef given the name.
func GetVDSPortGroupRef(
	ctx context.Context,
	connectionId, datacenterRef, portgroupName string,
	client *fmclient.FmClient,
) (string, error) {

	fmVDSData := struct {
		DistributedSwitches []FmDistributedSwitchResp `json:"distributedSwitches"`
	}{
		DistributedSwitches: make([]FmDistributedSwitchResp, 0),
	}
	resp, err := client.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/vmware/fabricDeployment/distributedSwitches"),
		map[string]string{"connId": connectionId},
		nil,
		nil,
		"",
	)
	if err != nil {
		return "", fmt.Errorf("Get request of portgrop calls with: %s failed: %s", connectionId, err)
	}
	err = json.Unmarshal(resp, &fmVDSData)
	if err != nil {
		return "", fmt.Errorf("Unable to convert resp to struct: %s error is: %s", string(resp), err)
	}

	// Check if the required VDS Portgroup is there and return its MORef
	for _, vData := range fmVDSData.DistributedSwitches {
		if vData.DataCenterRef == datacenterRef {
			for _, vPortgrp := range vData.PortGroups {
				if vPortgrp.Name == portgroupName {
					return vPortgrp.Ref, nil
				}
			}
		}
	}
	return "", fmt.Errorf("Unable to find portGroup: %s in FM Connection: %s", portgroupName, connectionId)
}

// Returns the network  MORef given the name. This is essentially for the VSS portgroups
// that are organized as networks.
func GetNetworkRef(
	ctx context.Context,
	connectionId, datacenterRef, networkName string,
	client *fmclient.FmClient,
) (string, error) {

	fmNetData := struct {
		Networks []FmNetworksResp `json:"networks"`
	}{
		Networks: make([]FmNetworksResp, 0),
	}
	resp, err := client.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/vmware/fabricDeployment/networks"),
		map[string]string{"connId": connectionId},
		nil,
		nil,
		"",
	)
	if err != nil {
		return "", fmt.Errorf("Get request of network calls with: %s failed: %s", connectionId, err)
	}
	err = json.Unmarshal(resp, &fmNetData)
	if err != nil {
		return "", fmt.Errorf("Unable to convert resp to struct: %s error is: %s", string(resp), err)
	}

	// Check if the required Network is there and return its MORef
	for _, nData := range fmNetData.Networks {
		if nData.DataCenterRef == datacenterRef && nData.Name == networkName {
			return nData.Ref, nil
		}
	}
	return "", fmt.Errorf("Unable to find Network: %s in FM Connection: %s", networkName, connectionId)
}

// Returns the Data Center MORef given the name. Returns the MORef if the DC is found in
// FM inventory, otherwise returns an error. The DC will only be found if there is at least
// one host on that DC.
func GetDataCenterRef(
	ctx context.Context,
	connectionId, dataCenterName string,
	client *fmclient.FmClient,
) (string, error) {

	fmHostData := struct {
		Hosts []FmHostResp `json:"hosts"`
	}{
		Hosts: make([]FmHostResp, 0),
	}
	resp, err := client.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/vmware/fabricDeployment/hosts"),
		map[string]string{"connId": connectionId},
		nil,
		nil,
		"",
	)
	if err != nil {
		return "", fmt.Errorf("Get request of host calls with: %s failed: %s", connectionId, err)
	}
	err = json.Unmarshal(resp, &fmHostData)
	if err != nil {
		return "", fmt.Errorf("Unable to convert resp to struct: %s error is: %s", string(resp), err)
	}

	// Check if the required DC is there and return its MORef
	for _, hData := range fmHostData.Hosts {
		if hData.DataCenterName == dataCenterName {
			return hData.DataCenterRef, nil
		}
	}
	return "", fmt.Errorf("Unable to find Dc: %s in FM Connection: %s", dataCenterName, connectionId)
}

// Returns the Cluster  MORef given the cluster name and DC MORef. Returns the MORef if the
// Cluster is found in FM inventory, otherwise returns an error. The Cluster will only be
// found if there is at least one host on that Cluster.
func GetClusterRef(
	ctx context.Context,
	connectionId, dataCenterRef, clusterName string,
	client *fmclient.FmClient,
) (string, error) {

	fmHostData := struct {
		Hosts []FmHostResp `json:"hosts"`
	}{
		Hosts: make([]FmHostResp, 0),
	}
	resp, err := client.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/vmware/fabricDeployment/hosts"),
		map[string]string{"connId": connectionId},
		nil,
		nil,
		"",
	)
	if err != nil {
		return "", fmt.Errorf("Get request of host calls with: %s failed: %s", connectionId, err)
	}
	err = json.Unmarshal(resp, &fmHostData)
	if err != nil {
		return "", fmt.Errorf("Unable to convert resp to struct: %s error is: %s", string(resp), err)
	}

	// Check if the required DC is there and return its MORef
	for _, hData := range fmHostData.Hosts {
		if hData.DataCenterRef == dataCenterRef && hData.ClusterName == clusterName {
			return hData.ClusterRef, nil
		}
	}
	return "", fmt.Errorf("Unable to find Cluster: %s in Datacenter: %s", clusterName, dataCenterRef)
}

// Returns the DAtastore  MORef given the datastore name and DC MORef. Returns the MORef if the
// datastore is found in FM inventory, otherwise returns an error. The datastore will only be
// found if there is at least one host which has attached to that datastore
func GetDataStoreRef(
	ctx context.Context,
	connectionId, dataCenterRef, datastoreName string,
	client *fmclient.FmClient,
) (string, error) {

	fmDataStores := struct {
		Datastores []FmDataStoreResp `json:"datastores"`
	}{
		Datastores: make([]FmDataStoreResp, 0),
	}
	resp, err := client.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/vmware/fabricDeployment/datastores"),
		map[string]string{"connId": connectionId},
		nil,
		nil,
		"",
	)
	if err != nil {
		return "", fmt.Errorf("Get request of datastores calls with: %s failed: %s", connectionId, err)
	}
	err = json.Unmarshal(resp, &fmDataStores)
	if err != nil {
		return "", fmt.Errorf("Unable to convert resp to struct: %s error is: %s", string(resp), err)
	}

	// Check if the required datastore is there and return its MORef
	for _, dData := range fmDataStores.Datastores {
		if dData.DataCenterRef == dataCenterRef && dData.Name == datastoreName {
			return dData.Ref, nil
		}
	}
	return "", fmt.Errorf("Unable to find Datastore: %s in Datacenter: %s", datastoreName, dataCenterRef)
}

// Returns the Datastroe cluster  MORef given the datastore cluster name and DC MORef.
// Returns the MORef if the datastore cluster is found in FM inventory
func GetDataStoreClusterRef(
	ctx context.Context,
	connectionId, dataCenterRef, datastoreClusterName string,
	client *fmclient.FmClient,
) (string, error) {

	fmDataStores := struct {
		Datastores []FmDataStoreResp `json:"datastores"`
	}{
		Datastores: make([]FmDataStoreResp, 0),
	}
	resp, err := client.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/vmware/fabricDeployment/datastores"),
		map[string]string{"connId": connectionId},
		nil,
		nil,
		"",
	)
	if err != nil {
		return "", fmt.Errorf("Get request of datastore cluster calls with: %s failed: %s", connectionId, err)
	}
	err = json.Unmarshal(resp, &fmDataStores)
	if err != nil {
		return "", fmt.Errorf("Unable to convert resp to struct: %s error is: %s", string(resp), err)
	}

	// Check if the required datastore cluster is there and return its MORef
	for _, dData := range fmDataStores.Datastores {
		if dData.DataCenterRef == dataCenterRef &&
			dData.DataStoreClusterMember == true &&
			dData.DataStoreClusterName == datastoreClusterName {
			return dData.DataStoreClusterRef, nil
		}
	}
	return "", fmt.Errorf("Unable to find Datastore cluster: %s in Datacenter: %s", datastoreClusterName, dataCenterRef)
}
