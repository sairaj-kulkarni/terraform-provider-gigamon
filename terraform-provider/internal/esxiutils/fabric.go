// Copyright (c) Gigamon, Inc.

// Implements utility functions that interact with ESXI Fabric APIs

package esxiutils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Data struct for the response of VMWare ESXI get on the monitoring domain for various
// inventory objects

// FM Host Response
type FmHostData struct {
	Name           string   `json:"name"`
	Ref            string   `json:"ref"`
	DatacenterName string   `json:"datacenterName"`
	DatacenterRef  string   `json:"datacenterRef"`
	ClusterName    string   `json:"clusterName"`
	ClusterRef     string   `json:"clusterRef"`
	NetworkRefs    []string `json:"networkRefs"`
	DatastoreRefs  []string `json:"datastoreRefs"`
}

type FmHostResp struct {
	Hosts []FmHostData `json:"hosts"`
}

// FM Networks response of interest to us
type FmNetworkData struct {
	Name          string   `json:"name"`
	Ref           string   `json:"ref"`
	HostRefs      []string `json:"hostRefs"`
	DatacenterRef string   `json:"datacenterRef"`
}

type FmNetworkResp struct {
	Networks []FmNetworkData `json:"networks"`
}

// FM Distributed Portgrop response of interest to us
type PortgroupData struct {
	Ref  string `json:"ref"`
	Name string `json:"name"`
}

type PortgroupRef struct {
	HostRefs      []string        `json:"hostRefs"`
	Portgroups    []PortgroupData `json:"portGroups"`
	DatacenterRef string          `json:"datacenterRef"`
}

type FmDsPortgroup struct {
	DistributedSwitches []PortgroupRef `json:"distributedSwitches"`
}

// FM Datastore response
type DatastoreData struct {
	Name                   string   `json:"name"`
	Ref                    string   `json:"ref"`
	DatacenterRef          string   `json:"datacenterRef"`
	DatastoreClusterMember bool     `json:"datastoreClusterMember"`
	DatastoreClusterRef    string   `json:"datastoreClusterRef"`
	HostRefs               []string `json:"hostRefs"`
}

type FmDatastoreResp struct {
	Datastores []DatastoreData `json:"datastores"`
}

type FmDatastoreLookup struct {
	Name     string
	HostRefs []string
	Ref      string
	Cluster  bool
}

type FmNetworkLookup struct {
	Name     string
	HostRefs []string
}

type FmDsPortgroupLookup struct {
	Name     string
	HostRefs []string
}

// Host model that is exposed via TF to the user
type FmHostDataModel struct {
	HostRef             string
	DatastoreRef        map[string]string // Map of datastore name to their referneces
	DatastoreClusterRef map[string]string // Map of DS Cluster names to their refernces
	NetworkRef          map[string]string // Map of Network names to their references
	DistributedPGRef    map[string]string // map of Distributed PG names to their refernce
}

// Represents the GO struct for the hosts model
type GoHosts struct {
	ConnectionId    string
	DatacenterRef   string
	ClusterRef      []string
	Hostname        []string
	HostnamePattern string
	HostDetails     map[string]FmHostDataModel
}

// Returns the Data Center MORef given the name. Returns the MORef if the DC is found in
// FM inventory, otherwise returns an error. The DC will only be found if there is at least
// one host on that DC.
func GetDataCenterRef(
	ctx context.Context,
	connectionId, dataCenterName string,
	client *fmclient.FmClient,
) (string, error) {

	fmHostResp := FmHostResp{}

	//Extract Raw UUID from TypedId
	rawID, err := commonutils.UUIDFromTypedID(connectionId)
	if err != nil {
		return "", err
	}

	resp, err := client.DoRequest(
		ctx,
		"GET",
		"api/v1.3/cloud/vmware/fabricDeployment/hosts",
		map[string]string{"connId": rawID},
		nil,
		nil,
		"",
	)
	if err != nil {
		return "", fmt.Errorf("Get request of host calls with: %s failed: %w", connectionId, err)
	}
	err = json.Unmarshal(resp, &fmHostResp)
	if err != nil {
		return "", fmt.Errorf("Unable to convert resp to struct: %s error is: %w", string(resp), err)
	}

	// Check if the required DC is there and return its MORef
	for _, hData := range fmHostResp.Hosts {
		if hData.DatacenterName == dataCenterName {
			return hData.DatacenterRef, nil
		}
	}
	return "", fmt.Errorf("Unable to find Dc: %s in FM Connection: %s", dataCenterName, connectionId)
}

// Returns the Cluster  MORef given the cluster name and DC MORef. Returns the MORef if the
// Cluster is found in FM inventory, otherwise returns an error. The Cluster will only be
// found if there is at least one host on that Cluster.
func GetClusterRef(
	ctx context.Context,
	connectionId, datacenterRef, clusterName string,
	client *fmclient.FmClient,
) (string, error) {

	fmHostResp := FmHostResp{}

	//Extract Raw UUID from TypedId
	rawID, err := commonutils.UUIDFromTypedID(connectionId)
	if err != nil {
		return "", err
	}

	resp, err := client.DoRequest(
		ctx,
		"GET",
		"api/v1.3/cloud/vmware/fabricDeployment/hosts",
		map[string]string{"connId": rawID},
		nil,
		nil,
		"",
	)
	if err != nil {
		return "", fmt.Errorf("Get request of host calls with: %s failed: %w", connectionId, err)
	}
	err = json.Unmarshal(resp, &fmHostResp)
	if err != nil {
		return "", fmt.Errorf("Unable to convert resp to struct: %s error is: %w", string(resp), err)
	}

	// Check if the required DC is there and return its MORef
	for _, hData := range fmHostResp.Hosts {
		if hData.DatacenterRef == datacenterRef && hData.ClusterName == clusterName {
			return hData.ClusterRef, nil
		}
	}
	return "", fmt.Errorf("Unable to find Cluster: %s in Datacenter: %s", clusterName, datacenterRef)
}

func GetHostDetails(
	ctx context.Context,
	fmData *GoHosts,
	client *fmclient.FmClient,
) error {

	// First get the various networks/porgroup/datastore and build up the structs
	datastoreCache, err := GetDatastoreDetails(ctx, fmData, client)
	if err != nil {
		return err
	}

	networkCache, err := GetNetworkDetails(ctx, fmData, client)
	if err != nil {
		return err
	}

	portgroupCache, err := GetPortgroupDetails(ctx, fmData, client)
	if err != nil {
		return err
	}

	// Finally get all the hosts of interest
	err = GetHostCache(ctx, fmData, client, networkCache, portgroupCache, datastoreCache)
	if err != nil {
		return err
	}

	return nil
}

// Checks if the Host present in FM is part of the host specification that the user
// has requested to get data on
func includeHost(fmHost *FmHostData, userSpec *GoHosts) bool {
	if fmHost.DatacenterRef != userSpec.DatacenterRef {
		return false
	}
	if len(userSpec.ClusterRef) > 0 &&
		!slices.Contains(userSpec.ClusterRef, fmHost.ClusterRef) {
		return false
	}
	if len(userSpec.Hostname) > 0 &&
		!slices.Contains(userSpec.Hostname, fmHost.Name) {
		return false
	}
	if userSpec.HostnamePattern != "" {
		match, _ := regexp.MatchString(userSpec.HostnamePattern, fmHost.Name)
		if !match {
			return false
		}
	}
	return true
}

func GetHostCache(
	ctx context.Context,
	fmData *GoHosts,
	client *fmclient.FmClient,
	networkCache map[string]FmNetworkLookup,
	distributedPGCache map[string]FmDsPortgroupLookup,
	datastoreCache map[string]FmDatastoreLookup,
) error {

	fmResp := FmHostResp{}

	//Extract Raw UUID from TypedId
	rawID, err := commonutils.UUIDFromTypedID(fmData.ConnectionId)
	if err != nil {
		return err
	}

	resp, err := client.DoRequest(
		ctx,
		"GET",
		"api/v1.3/cloud/vmware/fabricDeployment/hosts",
		map[string]string{"connId": rawID},
		nil,
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf(
			"Get request for netowrks failed. Connection: %s error: %w",
			fmData.ConnectionId,
			err,
		)
	}
	err = json.Unmarshal(resp, &fmResp)
	if err != nil {
		return fmt.Errorf(
			"Unable to convert resp to struct: %s error is: %w",
			string(resp),
			err,
		)
	}
	fmData.HostDetails = make(map[string]FmHostDataModel)
	for _, host := range fmResp.Hosts {
		if !includeHost(&host, fmData) {
			continue
		}
		hostCache := FmHostDataModel{
			HostRef:             host.Ref,
			DatastoreRef:        make(map[string]string),
			DatastoreClusterRef: make(map[string]string),
			NetworkRef:          make(map[string]string),
			DistributedPGRef:    make(map[string]string),
		}
		fmData.HostDetails[host.Name] = hostCache

		// Resolve all the networks
		for _, net := range host.NetworkRefs {
			nwCache, ok := networkCache[net]
			if ok && slices.Contains(nwCache.HostRefs, host.Ref) {
				hostCache.NetworkRef[nwCache.Name] = net
				continue
			}
			dsPgCache, ok := distributedPGCache[net]
			if ok && slices.Contains(dsPgCache.HostRefs, host.Ref) {
				hostCache.DistributedPGRef[dsPgCache.Name] = net
				continue
			}
		}

		// Resolve all Datastores
		for _, datastore := range host.DatastoreRefs {
			dsCache, ok := datastoreCache[datastore]
			if ok && slices.Contains(dsCache.HostRefs, host.Ref) {
				if dsCache.Cluster {
					hostCache.DatastoreClusterRef[dsCache.Name] = dsCache.Ref
				} else {
					hostCache.DatastoreRef[dsCache.Name] = dsCache.Ref
				}
				continue
			}
		}
	}
	return nil
}

func GetPortgroupDetails(
	ctx context.Context,
	fmData *GoHosts,
	client *fmclient.FmClient,
) (map[string]FmDsPortgroupLookup, error) {

	fmResp := FmDsPortgroup{}

	//Extract Raw UUID from TypedId
	rawID, err := commonutils.UUIDFromTypedID(fmData.ConnectionId)
	if err != nil {
		return nil, err
	}

	resp, err := client.DoRequest(
		ctx,
		"GET",
		"api/v1.3/cloud/vmware/fabricDeployment/distributedSwitches",
		map[string]string{"connId": rawID},
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"Get request for netowrks failed. Connection: %s error: %w",
			fmData.ConnectionId,
			err,
		)
	}
	err = json.Unmarshal(resp, &fmResp)
	if err != nil {
		return nil, fmt.Errorf(
			"Unable to convert resp to struct: %s error is: %w",
			string(resp),
			err,
		)
	}
	dsPortgroupData := make(map[string]FmDsPortgroupLookup)
	for _, dsElem := range fmResp.DistributedSwitches {
		if dsElem.DatacenterRef != fmData.DatacenterRef {
			continue
		}
		for _, pg := range dsElem.Portgroups {
			dsPortgroupData[pg.Ref] = FmDsPortgroupLookup{
				Name:     pg.Name,
				HostRefs: dsElem.HostRefs,
			}
		}
	}
	return dsPortgroupData, nil
}

func GetNetworkDetails(
	ctx context.Context,
	fmData *GoHosts,
	client *fmclient.FmClient,
) (map[string]FmNetworkLookup, error) {

	fmResp := FmNetworkResp{}

	//Extract Raw UUID from TypedId
	rawID, err := commonutils.UUIDFromTypedID(fmData.ConnectionId)
	if err != nil {
		return nil, err
	}

	resp, err := client.DoRequest(
		ctx,
		"GET",
		"api/v1.3/cloud/vmware/fabricDeployment/networks",
		map[string]string{"connId": rawID},
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"Get request for netowrks failed. Connection: %s error: %w",
			fmData.ConnectionId,
			err,
		)
	}
	err = json.Unmarshal(resp, &fmResp)
	if err != nil {
		return nil, fmt.Errorf(
			"Unable to convert resp to struct: %s error is: %w",
			string(resp),
			err,
		)
	}
	networkData := make(map[string]FmNetworkLookup)
	for _, net := range fmResp.Networks {
		if net.DatacenterRef != fmData.DatacenterRef {
			continue
		}
		networkData[net.Ref] = FmNetworkLookup{
			Name:     net.Name,
			HostRefs: net.HostRefs,
		}
	}
	return networkData, nil
}

func GetDatastoreDetails(
	ctx context.Context,
	fmData *GoHosts,
	client *fmclient.FmClient,
) (map[string]FmDatastoreLookup, error) {

	fmResp := FmDatastoreResp{}

	//Extract Raw UUID from TypedId
	rawID, err := commonutils.UUIDFromTypedID(fmData.ConnectionId)
	if err != nil {
		return nil, err
	}

	resp, err := client.DoRequest(
		ctx,
		"GET",
		"api/v1.3/cloud/vmware/fabricDeployment/datastores",
		map[string]string{"connId": rawID},
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"Get request for datastores failed. Connection: %s error: %w",
			fmData.ConnectionId,
			err,
		)
	}
	err = json.Unmarshal(resp, &fmResp)
	if err != nil {
		return nil, fmt.Errorf(
			"Unable to convert resp to struct: %s error is: %w",
			string(resp),
			err,
		)
	}
	datastoreMap := make(map[string]FmDatastoreLookup)
	var ref string
	var isCluster bool
	for _, datastore := range fmResp.Datastores {
		if datastore.DatacenterRef != fmData.DatacenterRef {
			continue
		}
		if datastore.DatastoreClusterMember {
			ref = datastore.DatastoreClusterRef
			isCluster = true
		} else {
			ref = datastore.Ref
			isCluster = false
		}
		datastoreMap[datastore.Ref] = FmDatastoreLookup{
			HostRefs: datastore.HostRefs,
			Name:     datastore.Name,
			Ref:      ref,
			Cluster:  isCluster,
		}
	}
	return datastoreMap, nil
}

// Fabric Deployment Management
// Go struct for the fabric model in ESXI

type ObjectRef struct {
	VcKey string `json:"vcKey,omitempty"`
	Name  string `json:"name,omitempty"`
}

type EsxiFabric struct {
	ConnectionId  string          `json:"connId,omitempty"`
	DatacenterRef ObjectRef       `json:"dcRef,omitempty"`
	ImageId       string          `json:"imageName,omitempty"`
	FormFactor    string          `json:"formFactor,omitempty"`
	HostSpecs     []*EsxiHostSpec `json:"hostSpecs"`
}

type DnsServer struct {
	DnsName string `json:"nameserver,omitempty"`
}

type EsxiHostSpec struct {
	HostRef             ObjectRef          `json:"hostRef"`
	VmName              string             `json:"vmNodeName,omitempty"`
	DiskFormat          string             `json:"diskFormat,omitempty"`
	DatastoreRef        *ObjectRef         `json:"datastoreRef,omitempty"`
	DatastoreClusterRef *ObjectRef         `json:"datastoreClusterRef,omitempty"`
	ClusterRef          *ObjectRef         `json:"clusterRef,omitempty"`
	MgmtInterface       EsxiInterfaceSpec  `json:"intfMgmt"`
	TunnelInterface     *EsxiInterfaceSpec `json:"intfTunnel,omitempty"`
	VmFolder            string             `json:"vmFolder,omitempty"`
	AdminPassword       string             `json:"adminPassword,omitempty"`
	NameServer          []DnsServer        `json:"nameServerConfig,omitempty"`
	// The below are the node dynamic data that is got from FM and updated here
	VMId         string   `json:"vm_id,omitempty"`
	Status       string   `json:"status,omitempty"`
	Version      string   `json:"version,omitempty"`
	ManagementIP string   `json:"management_interface_ip,omitempty"`
	DataIPs      []string `json:"data_interface_ips,omitempty"`
}

type EsxiInterfaceSpec struct {
	NetworkRef    ObjectRef `json:"intfRef,omitempty"`
	AddressMode   string    `json:"ipType,omitempty"`
	Mtu           int32     `json:"mtu,omitempty"`
	IPAddress     string    `json:"ipAddress,omitempty"`
	IPAddressMask string    `json:"ipAddressMask,omitempty"`
	GatewayIP     string    `json:"gatewayIp,omitempty"`
	Ipv6PrefixLen int32     `json:"ipv6PrefixLen,omitempty"`
}

// Go structs for the fabric deployment get response

type EsxiNodeData struct {
	ManagementIP string   `json:"mgmtIp,omitempty"`
	Version      string   `json:"version,omitempty"`
	DataIPs      []string `json:"dataIps,omitempty"`
	Name         string   `json:"name,omitempty"`
	Status       string   `json:"status,omitempty"`
	NodeId       string   `json:"nodeId,omitempty"`
}

type DeploymentSpec struct {
	ConnectionId  string       `json:"connId,omitempty"`
	DatacenterRef ObjectRef    `json:"dcRef"`
	ImageId       string       `json:"imageName,omitempty"`
	FormFactor    string       `json:"formFactor,omitempty"`
	HostSpec      EsxiHostSpec `json:"hostSpec,omotempty"`
}

type DeploymentData struct {
	Node EsxiNodeData   `json:"node"`
	Spec DeploymentSpec `json:"spec"`
}

type DeploymentResp struct {
	DeploymentId string           `json:"deploymentId,omitempty"`
	Deployments  []DeploymentData `json:"deployments,omitempty"`
}

// Set if struct and functions to do a diff between two spec (intent versus state) and return
// the set of actions to take to go from state to the intent

// Represents those spec for which we have to do a name change
type NameChangeSpec struct {
	NodeId       string `json:"nodeId,omitempty"`
	Name         string `json:"name,omitempty"`
	ConnectionId string `json:"connectionId,omitempty"`
}

// Represents the data for nodes that we want to delete
type DeleteNodeSpec struct {
	NodeId       string
	ConnectionId string
}

// These are new nodes that need to be added, i.e. nodes in the plan whcih are not there
// in the spec
type AddNodeSpec struct {
	Index int // Index into the Plan Struct of the node specs to be added
}

// These are the nodes that need to be upgraded. In general we will upgrade all the nodes
// in the deployment but we can ignore those nodes that need to be added or deleted
type UpgradeNodeSpec struct {
	NodeId     string      `json:"nodeId,omitempty"`
	FormFactor string      `json:"formFactor,omitempty"`
	NameServer []DnsServer `json:"nameServerConfig,omitempty"`
}

type UpgradeSpec struct {
	ImageId            string            `json:"imageName,omitempty"`
	MonitoringDomainId string            `json:"monitoringDomainId,omitempty"`
	ConnectionId       string            `json:"-"`
	UpgradeName        string            `json:"upgradeName,omitempty"`
	NodeDetails        []UpgradeNodeSpec `json:"nodeDetails,omitempty"`
}

type StateToIntent struct {
	VmNameChanges []NameChangeSpec // List of VM for which we have to apply name changes
	DeleteVMs     []DeleteNodeSpec // List of Vseries Spec  to delete
	AddVMs        []AddNodeSpec    // List of Vseries Spec to add
	UpgradeVMs    *UpgradeSpec     // List of Vseries specs to add
}

func GetDiff(
	ctx context.Context,
	intentSpec *EsxiFabric,
	deploymentId string,
	client *fmclient.FmClient,
) (*StateToIntent, error) {

	fmResp := DeploymentResp{}

	respData, err := client.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf(
			"api/v1.3/cloud/vmware/fabricDeployment/vseriesNodes/deployment/%s",
			deploymentId,
		),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(respData, &fmResp); err != nil {
		return nil, fmt.Errorf(
			"Unable to decode deployment get response: %s , err: %w",
			string(respData),
			err,
		)
	}

	changeSpec := &StateToIntent{
		VmNameChanges: []NameChangeSpec{},
		DeleteVMs:     []DeleteNodeSpec{},
		AddVMs:        []AddNodeSpec{},
		UpgradeVMs: &UpgradeSpec{
			ImageId:      intentSpec.ImageId,
			ConnectionId: intentSpec.ConnectionId,
			UpgradeName:  fmt.Sprintf("TF-upgrade-%s", uuid.New().String()),
			NodeDetails:  []UpgradeNodeSpec{},
		},
	}

	respSpecProcessed := make([]bool, len(fmResp.Deployments))
	var respIndex, inIndex int
	var inHost *EsxiHostSpec
	var respDeploy DeploymentData
	var nodeFound bool

	for inIndex, inHost = range intentSpec.HostSpecs {
		nodeFound = false
		for respIndex, respDeploy = range fmResp.Deployments {
			if inHost.HostRef.VcKey == respDeploy.Spec.HostSpec.HostRef.VcKey {
				nodeFound = true
				break
			}
		}
		if !nodeFound {
			// This spec is in Plan and not in FM, so add this
			changeSpec.AddVMs = append(
				changeSpec.AddVMs,
				AddNodeSpec{
					Index: inIndex,
				},
			)
			continue
		}
		respSpecProcessed[respIndex] = true
		// Check if there is any changes that need to be done for this spec
		if inHost.VmName != respDeploy.Node.Name { // The name needs to be updated
			changeSpec.VmNameChanges = append(
				changeSpec.VmNameChanges,
				NameChangeSpec{
					NodeId:       respDeploy.Node.NodeId,
					Name:         inHost.VmName,
					ConnectionId: respDeploy.Spec.ConnectionId,
				},
			)
		}
		err := checkUpgrade(inHost, &respDeploy, intentSpec, changeSpec)
		if err != nil {
			return nil, err
		}
	}
	for index, val := range respSpecProcessed {
		if !val {
			// This is present in FM but not in our intent, so delete if from FM
			changeSpec.DeleteVMs = append(
				changeSpec.DeleteVMs,
				DeleteNodeSpec{
					NodeId:       fmResp.Deployments[index].Node.NodeId,
					ConnectionId: fmResp.Deployments[index].Spec.ConnectionId,
				},
			)
		}
	}
	return changeSpec, nil
}

// Check if this node needs to be upgraded. If not then also make sure that none of
// other parameters that cannot be changed without an upgrade is changed
func checkUpgrade(
	inHost *EsxiHostSpec,
	respDeploy *DeploymentData,
	intentSpec *EsxiFabric,
	changeSpec *StateToIntent,
) error {

	// Check if we have to upgrade this node
	if intentSpec.ImageId != respDeploy.Spec.ImageId {
		changeSpec.UpgradeVMs.NodeDetails = append(
			changeSpec.UpgradeVMs.NodeDetails,
			UpgradeNodeSpec{
				NodeId:     respDeploy.Node.NodeId,
				FormFactor: intentSpec.FormFactor,
				NameServer: inHost.NameServer,
			},
		)
		return nil
	}
	return nil
}

// Change the names of the given nodes
func ChangeVmName(
	ctx context.Context,
	changes []NameChangeSpec,
	client *fmclient.FmClient,
) error {

	for _, changeSpec := range changes {
		jsonData, err := json.Marshal(changeSpec)
		if err != nil {
			return fmt.Errorf(
				"Unable to encode chagneName spec Json: %v, error: %s",
				changeSpec,
				err,
			)
		}

		_, err = client.DoRequest(
			ctx,
			"PATCH",
			"api/v1.3/cloud/vmware/fabricNode/rename",
			nil,
			nil,
			bytes.NewBuffer(jsonData),
			"application/json",
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func DeleteVms(
	ctx context.Context,
	deleteVms []DeleteNodeSpec,
	client *fmclient.FmClient,
) error {

	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()
	var globalErr error

outerLoop:
	for _, deleteSpec := range deleteVms {
		for {
			_, err := client.DoRequest(
				ctx,
				"DELETE",
				fmt.Sprintf(
					"/api/v1.3/cloud/vmware/fabricDeployment/vseriesNodes/%s/%s",
					deleteSpec.ConnectionId,
					deleteSpec.NodeId,
				),
				nil,
				nil,
				nil,
				"",
			)
			if err == nil {
				continue outerLoop
			}
			var fmErr *fmclient.FMErrors
			if errors.As(err, &fmErr) {
				errCode := fmErr.ErrorCode()
				if errCode == fmclient.ObjectNotFound {
					continue outerLoop
				}
				if errCode != fmclient.RequestConflict {
					globalErr = errors.Join(
						fmt.Errorf(
							"Unable to delete %s VM, %w",
							deleteSpec.NodeId,
							err,
						),
						globalErr,
					)
					continue outerLoop
				}
			} else {
				globalErr = errors.Join(
					fmt.Errorf(
						"Unable to delete %s VM, %w",
						deleteSpec.NodeId,
						err,
					),
					globalErr,
				)
				continue outerLoop
			}
			select {
			case <-ticker.C:
				break
			case <-ctx.Done():
				globalErr = errors.Join(
					fmt.Errorf(
						"Timeout while deleteing the nodes",
					),
					globalErr,
				)
				return globalErr
			}
		}
	}
	return globalErr
}

func UpgradeVms(
	ctx context.Context,
	upgradeVms *UpgradeSpec,
	client *fmclient.FmClient,
) error {

	if len(upgradeVms.NodeDetails) == 0 {
		return nil // Nothing to do
	}

	// Get the Monitoring Domain ID
	connDetails, err := GetConnectionById(ctx, upgradeVms.ConnectionId, client)
	if err != nil {
		return err
	}
	upgradeVms.MonitoringDomainId = connDetails.MonitoringDomainId

	jsonData, err := json.Marshal(upgradeVms)
	if err != nil {
		return fmt.Errorf(
			"Unable to encode upgradeSpec spec Json: %v, error: %w",
			upgradeVms,
			err,
		)
	}

	_, err = client.DoRequest(
		ctx,
		"POST",
		"api/v1.3/cloud/vmware/fabricDeployment/vseriesNodes/upgrade",
		nil,
		nil,
		bytes.NewBuffer(jsonData),
		"application/json",
	)
	if err != nil {
		return err
	}
	return nil
}

func AddNewSpecs(
	ctx context.Context,
	addVms []AddNodeSpec,
	intentSpec *EsxiFabric,
	deploymentId string,
	client *fmclient.FmClient,
) error {
	if len(addVms) == 0 {
		return nil
	}

	diffSpec := &EsxiFabric{
		ConnectionId: intentSpec.ConnectionId,
		DatacenterRef: ObjectRef{
			VcKey: intentSpec.DatacenterRef.VcKey,
			Name:  intentSpec.DatacenterRef.Name,
		},
		ImageId:    intentSpec.ImageId,
		FormFactor: intentSpec.FormFactor,
		HostSpecs:  []*EsxiHostSpec{},
	}

	// Now add into the above fabric spec, all the nodes that we have to now add
	for _, addVmIndex := range addVms {
		diffSpec.HostSpecs = append(
			diffSpec.HostSpecs,
			intentSpec.HostSpecs[addVmIndex.Index],
		)
	}

	// Now deploy this set of VMs
	jsonData, err := json.Marshal(diffSpec)
	if err != nil {
		return err
	}
	_, err = client.DoRequest(
		ctx,
		"PATCH",
		fmt.Sprintf(
			"api/v1.3/cloud/vmware/fabricDeployment/vseriesNodes/deployment/%s",
			deploymentId,
		),
		nil,
		nil,
		bytes.NewBuffer(jsonData),
		"application/json",
	)
	return err
}

// Get the deployment node/spec details from FM and fill it up in the Golang TF model struct
// Returns the number of Vseries node that are not in "ok" state

func GetDeploymentUpdate(
	ctx context.Context,
	deploymentId string,
	inSpec *EsxiFabric,
	client *fmclient.FmClient,
) (int, error) {

	fmResp := DeploymentResp{}
	vseriesNotReady := 0

	respData, err := client.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf(
			"api/v1.3/cloud/vmware/fabricDeployment/vseriesNodes/deployment/%s",
			deploymentId,
		),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return vseriesNotReady, err
	}
	if err := json.Unmarshal(respData, &fmResp); err != nil {
		return vseriesNotReady, fmt.Errorf(
			"Unable to decode deployment get response: %s , err: %w",
			string(respData),
			err,
		)
	}

	// Now match the input spec to the received response and update the appropriate fields

	// Handle the special case of no deployment spec at all
	if len(fmResp.Deployments) == 0 {
		inSpec.HostSpecs = nil
		return vseriesNotReady, fmt.Errorf("No nodes foundi n this deploymen")
	}

	// Copy the outer data from the first element
	inSpec.ConnectionId = fmResp.Deployments[0].Spec.ConnectionId
	inSpec.DatacenterRef.VcKey = fmResp.Deployments[0].Spec.DatacenterRef.VcKey
	inSpec.ImageId = fmResp.Deployments[0].Spec.ImageId
	inSpec.FormFactor = fmResp.Deployments[0].Spec.FormFactor

	inSpecProcessed := make([]bool, len(inSpec.HostSpecs))
	respSpecProcessed := make([]bool, len(fmResp.Deployments))
	var respIndex, inIndex int
	var inHost *EsxiHostSpec
	var respDeploy DeploymentData
	var nodeFound bool

	for inIndex, inHost = range inSpec.HostSpecs {
		nodeFound = false
		for respIndex, respDeploy = range fmResp.Deployments {
			if inHost.HostRef.VcKey == respDeploy.Spec.HostSpec.HostRef.VcKey {
				nodeFound = true
				break
			}
		}
		if !nodeFound {
			// This spec is in Plan/previous state and not in FM. So remove it out of the
			// state or plan to let TF know that it has to add it back
			continue
		}
		inSpecProcessed[inIndex] = true
		respSpecProcessed[respIndex] = true
		// Copy the spec data and also the dynamic node data over
		status := copyFields(ctx, &respDeploy, inHost)
		if status != "ok" {
			vseriesNotReady += 1
		}
	}

	// For any entry in the deploy result that is not there in the spec, add it to the spec
	for index, present := range respSpecProcessed {
		if !present {
			inHost = &EsxiHostSpec{}
			copyFields(ctx, &fmResp.Deployments[index], inHost)
			inSpec.HostSpecs = append(inSpec.HostSpecs, inHost)
		}
	}

	// These entries are in spec but not in FM, so they need to be removed to reflect
	// current state
	offset := 0
	for index, present := range inSpecProcessed {
		if !present {
			removeIndex := index - offset
			offset += 1
			inSpec.HostSpecs = append(
				inSpec.HostSpecs[0:removeIndex],
				inSpec.HostSpecs[removeIndex+1:]...,
			)
		}
	}

	return vseriesNotReady, nil
}

// Copies the FM response from deployment call, into the spec model
func copyFields(ctx context.Context, deployData *DeploymentData, specData *EsxiHostSpec) string {
	deploySpec := deployData.Spec.HostSpec
	deployNodeData := deployData.Node
	specData.HostRef.VcKey = deploySpec.HostRef.VcKey
	specData.HostRef.Name = deploySpec.HostRef.Name
	specData.VmName = deployNodeData.Name
	specData.DiskFormat = deploySpec.DiskFormat
	if deploySpec.DatastoreRef != nil {
		specData.DatastoreRef = &ObjectRef{
			VcKey: deploySpec.DatastoreRef.VcKey,
			Name:  deploySpec.DatastoreRef.Name,
		}
	} else {
		specData.DatastoreRef = nil
	}
	if deploySpec.DatastoreClusterRef != nil {
		specData.DatastoreClusterRef = &ObjectRef{
			VcKey: deploySpec.DatastoreClusterRef.VcKey,
			Name:  deploySpec.DatastoreClusterRef.Name,
		}
	} else {
		specData.DatastoreClusterRef = nil
	}
	if deploySpec.ClusterRef != nil {
		specData.ClusterRef = &ObjectRef{
			VcKey: deploySpec.ClusterRef.VcKey,
			Name:  deploySpec.ClusterRef.Name,
		}
	} else {
		specData.ClusterRef = nil
	}
	copyInterfaceSpec(&deploySpec.MgmtInterface, &specData.MgmtInterface)
	if deploySpec.TunnelInterface != nil {
		specData.TunnelInterface = &EsxiInterfaceSpec{}
		copyInterfaceSpec(deploySpec.TunnelInterface, specData.TunnelInterface)
	} else {
		specData.TunnelInterface = nil
	}
	if len(deploySpec.NameServer) != 0 {
		specData.NameServer = deploySpec.NameServer
	}
	specData.VmFolder = deploySpec.VmFolder
	specData.VMId = deployNodeData.NodeId
	specData.Status = deployNodeData.Status
	specData.Version = deployNodeData.Version
	specData.ManagementIP = deployNodeData.ManagementIP
	specData.DataIPs = deployNodeData.DataIPs
	return strings.ToLower(specData.Status)
}

func copyInterfaceSpec(source, dest *EsxiInterfaceSpec) {
	dest.NetworkRef = ObjectRef{
		VcKey: source.NetworkRef.VcKey,
		Name:  source.NetworkRef.Name,
	}
	dest.AddressMode = source.AddressMode
	dest.Mtu = source.Mtu
	dest.IPAddress = source.IPAddress
	dest.IPAddressMask = source.IPAddressMask
	dest.GatewayIP = source.GatewayIP
	dest.Ipv6PrefixLen = source.Ipv6PrefixLen
}

func DeployFabric(
	ctx context.Context,
	specData *EsxiFabric,
	client *fmclient.FmClient,
) (string, error) {

	deploymentResp := struct {
		DeploymentId string `json:"deploymentId"`
	}{}

	jsonData, err := json.Marshal(specData)
	if err != nil {
		return "", fmt.Errorf(
			"Unable to encode fabric spec Json: %v, error: %w",
			specData,
			err,
		)
	}

	respData, err := client.DoRequest(
		ctx,
		"POST",
		"api/v1.3/cloud/vmware/fabricDeployment/vseriesNodes",
		nil,
		nil,
		bytes.NewBuffer(jsonData),
		"application/json",
	)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(respData, &deploymentResp); err != nil {
		return "", fmt.Errorf(
			"Unable to decode update response: %s , err: %w",
			string(respData),
			err,
		)
	}
	return deploymentResp.DeploymentId, nil
}

// Delete the deployment
func DeleteFabric(
	ctx context.Context,
	deploymentId string,
	client *fmclient.FmClient,
) error {

	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()
	for {
		_, err := client.DoRequest(
			ctx,
			"DELETE",
			fmt.Sprintf(
				"api/v1.3/cloud/vmware/fabricDeployment/vseriesNodes/deployment/%s",
				deploymentId,
			),
			nil,
			nil,
			nil,
			"",
		)
		if err != nil {
			var fmErr *fmclient.FMErrors
			if errors.As(err, &fmErr) {
				errCode := fmErr.ErrorCode()
				if errCode == fmclient.ObjectNotFound {
					return nil
				}
				if errCode != fmclient.RequestConflict {
					return err
				}
			}
		} else {
			return nil
		}
		select {
		case <-ticker.C:
			break
		case <-ctx.Done():
			return fmt.Errorf(
				"Timeout while deleting the fabric deployment: %s",
				deploymentId,
			)
		}
	}
}
