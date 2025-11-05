// Copyright (c) Gigamon, Inc.

// Implements utility functions that interact with FM and provides the responses.

package fmesxi

import (
	"context"
	"encoding/json"
	"fmt"

	"gigamon.com/terraform-provider-gigamon/internal/fmclient"
)

// Data struct for the response of VMWare ESXI get on the monitoring domain for various
// inventory objects

// DataStore response
type FmDataStoreResp struct {
	Name string `json:"name"`
	Ref string `json:"ref"`
	DataCenterName string `json:"datacenterName"`
	DataCenterRef string `json:"datacenterRef"`
	DataStoreClusterName string `json:"datastoreClusterName"`
	DataStoreClusterRef string `json:"datastoreClusterRef"`
}

// Network Response
type FmNetworksResp struct {
	Name string `json:"name"`
	Ref string `json:"ref"`
	DataCenterName string `json:"datacenterName"`
	DataCenterRef string `json:"datacenterRef"`
}

// Distributed Switch Response
type FmDistributedSwitchResp struct {
	Name string `json:"name"`
	Ref string `json:"ref"`
	DataCenterName string `json:"datacenterName"`
	DataCenterRef string `json:"datacenterRef"`
}

// Host Response
type FmHostResp struct {
	Name string `json:"name"`
	Ref string `json:"ref"`
	DataCenterName string `json:"datacenterName"`
	DataCenterRef string `json:"datacenterRef"`
	ClusterName string `json:"clusterName"`
	ClusterRef string `json:"clusterRef"`
	NetworkRefs []string `json:"networkRefs"`
	DataStoreRefs []string `json:"datastoreRefs"`
}

// Returns the Data Center MORef given the name. Returns the MORef if the DC is found in
// FM inventory, otherwise returns an error. The DC will only be found if there is at least
// one host on that DC.
func GetDataCenterRef(
	ctx context.Context,
	connectionId, dataCenterName  string,
	client *fmclient.FmClient,
) (string, error) {

	fmHostData := struct {
		Hosts []FmHostResp `json:"hosts"`
	} {
		Hosts: make([]FmHostResp, 0),
	}
	resp, err := client.DoRequest(
		ctx,
		"GET",
		0,
		fmt.Sprintf("api/v1.3/cloud/vmware/fabricDeployment/hosts"),
		map[string]string {"connId": connectionId},
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
	connectionId, dataCenterRef, clusterName  string,
	client *fmclient.FmClient,
) (string, error) {

	fmHostData := struct {
		Hosts []FmHostResp `json:"hosts"`
	} {
		Hosts: make([]FmHostResp, 0),
	}
	resp, err := client.DoRequest(
		ctx,
		"GET",
		0,
		fmt.Sprintf("api/v1.3/cloud/vmware/fabricDeployment/hosts"),
		map[string]string {"connId": connectionId},
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
		if hData.DataCenterRef == dataCenterRef && hData.ClusterName == clusterName{
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
	connectionId, dataCenterRef, datastoreName  string,
	client *fmclient.FmClient,
) (string, error) {

	fmDataStores := struct {
		Datastores []FmDataStoreResp `json:"datastores"`
	} {
		Datastores: make([]FmDataStoreResp, 0),
	}
	resp, err := client.DoRequest(
		ctx,
		"GET",
		0,
		fmt.Sprintf("api/v1.3/cloud/vmware/fabricDeployment/datastores"),
		map[string]string {"connId": connectionId},
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
		if dData.DataCenterRef == dataCenterRef && dData.Name == datastoreName{
			return dData.Ref, nil
		}
	}
	return "", fmt.Errorf("Unable to find Datastore: %s in Datacenter: %s", datastoreName, dataCenterRef)
}
