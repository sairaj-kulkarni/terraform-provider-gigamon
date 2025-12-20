// Copyright (c) Gigamon, Inc.

// Implements the Resrouces for the ESXI Fabric (VSeries) resources

package esxiresources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &EsxiFabric{}

// Esxi Fabric resoruce, which manages the images for ESXI platform
func NewEsxiFabric() resource.Resource {
	return &EsxiFabric{}
}

// EsxiFabric manages the Fabric for the ESXI platform
type EsxiFabric struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// EsxiIntfSpec describes the interface details of the Vseries spec
type EsxiIntfSpec struct {
	Ref         types.String `tfsdk:"network_moref"`
	AddressMode types.String `tfsdk:"address_assignment_mode"`
}

// EsxiVmSpec describes the spec for a VM on ESXI
type EsxiVMSpec struct {
	HostRef  types.String `tfsdk:"host_moref"`
	HostName types.String `tfsdk:"host_name"`
	VmName   types.String `tfsdk:"name"`
	VMId     types.String `tfsdk:"vseries_node_id"`
	Status   types.String `tfsdk:"status"`
	Version  types.String `tfsdk:"version"`
}

// EsxiFabricModel describes the fabric resource data model.
type EsxiFabricModel struct {
	Name                types.String  `tfsdk:"name"`
	ConnectionId        types.String  `tfsdk:"connection_id"`
	DatacenterRef       types.String  `tfsdk:"datacenter_moref"`
	FormFactor          types.String  `tfsdk:"form_factor"`
	ImageId             types.String  `tfsdk:"image_id"`
	VmFolder            types.String  `tfsdk:"vm_folder"`
	DataStoreClusterRef types.String  `tfsdk:"datastore_cluster_moref"`
	DiskFormat          types.String  `tfsdk:"disk_format"`
	MgmtIntf            *EsxiIntfSpec `tfsdk:"management_interface_spec"`
	TunnelIntf          *EsxiIntfSpec `tfsdk:"tunnel_interface_spec"`
	AdminPassword       types.String  `tfsdk:"admin_password"`
	HostSpec            []*EsxiVMSpec `tfsdk:"host_vm_spec"`
	Id                  types.String  `tfsdk:"id"`
	Timeout             types.Int32   `tfsdk:"timeout"`
}

// Go structs for managing the fabric deployment for VMWare ESXI Platform

// ObjectSpec used to represent the general spec for an object in VMWare ESXI environment
type ObjectSpec struct {
	Ref  string `json:"vcKey,omitempty"` // Represents the MORef of the object in Vcenter
	Name string `json:"name,omitempty"`  // Name of the object (user provided)
}

// InterfaceSpec is to represent an network interface NIC of the VM
type InterfaceSpec struct {
	NetworkSpec ObjectSpec `json:"intfRef"` // The network to which this NIC is assigned to
	AddressMode string     `json:"ipType"`  // Whether it is DHCP/static assignment
}

// VMSpec represents the details of the VM Spec that is used to spin up for a VSeries instance
type VMSpec struct {
	HostSpec             *ObjectSpec    `json:"hostRef"`                        // Host specification on which this VM is spun up
	VMName               string         `json:"vmNodeName"`                     // Name assigned by user for this Vseries VM
	DiskFormat           string         `json:"diskFormat"`                     // Format of the disk
	DataStoreSpec        *ObjectSpec    `json:"datastoreRef,omitempty"`         // VM DataStore
	DataStoreClusterSpec *ObjectSpec    `json:"datastoreClusterRef,omitemptry"` // VM Datstore Cluster
	ManagementIntfSpec   *InterfaceSpec `json:"intfMgmt"`                       // Management NIC network details
	TunnelIntfSpec       *InterfaceSpec `json:"intfTunnel"`                     // Tunnel NIC network details
	VMFolder             string         `json:"vmFolder"`                       // Folder to hold the VM files
	AdminPassword        string         `json:"adminPassword"`
}

// FabricDeployment represents the struct that is passed to FM to create a fabric
type FabricDeployment struct {
	ConnectionId    string     `json:"connId"`     // Connection ID on which to create this fabric
	DataCenterSpec  ObjectSpec `json:"dcRef"`      // DataCenter spec for this fabric creation
	VseriesNodeSpec []VMSpec   `json:"hostSpecs"`  // Vseries Node Spec
	ImageId         string     `json:"imageName"`  // Vseries Image name/version
	FormFactor      string     `json:"formFactor"` // Vseries form factor
}

// Vseriesnode represents the GET on FM to fetch the Vsereis Node details
type VseriesNode struct {
	Id           string   `json:"nodeId"`    // ID assigned to this vseries node by FM
	VMName       string   `json:"name"`      // The Name user assigned to this Vseries Node
	ManagementIp string   `json:"mgmtIp"`    // Vseries Node management IP address
	TunnelIps    []string `json:"tunnelIps"` // Vseries Node tunnel IP address list
	Version      string   `json:"version"`   // Version on the Vseries node
	Status       string   `json:"status"`    // Health status of the Vseries Node
}

// Helper Functions to map between Go struct and TF structs and handle the FM interaction
func convertTFConfig(data *EsxiFabricModel, fabricId string) *FabricDeployment {
	fabricDeploy := &FabricDeployment{
		ConnectionId: data.ConnectionId.ValueString(),
		DataCenterSpec: ObjectSpec{
			Ref: data.DatacenterRef.ValueString(),
		},
		ImageId:         data.ImageId.ValueString(),
		FormFactor:      data.FormFactor.ValueString(),
		VseriesNodeSpec: make([]VMSpec, 0, len(data.HostSpec)),
	}
	for _, hSpec := range data.HostSpec {
		vmSpec := VMSpec{
			DiskFormat: data.DiskFormat.ValueString(),
			VMFolder:   data.VmFolder.ValueString(),
			HostSpec: &ObjectSpec{
				Ref:  hSpec.HostRef.ValueString(),
				Name: hSpec.HostName.ValueString(),
			},
			VMName: fmt.Sprintf("%s-%s", fabricId, hSpec.VmName.ValueString()),
			ManagementIntfSpec: &InterfaceSpec{
				NetworkSpec: ObjectSpec{
					Ref: data.MgmtIntf.Ref.ValueString(),
				},
				AddressMode: data.MgmtIntf.AddressMode.ValueString(),
			},
			TunnelIntfSpec: &InterfaceSpec{
				NetworkSpec: ObjectSpec{
					Ref: data.TunnelIntf.Ref.ValueString(),
				},
				AddressMode: data.TunnelIntf.AddressMode.ValueString(),
			},
			DataStoreClusterSpec: &ObjectSpec{
				Ref: data.DataStoreClusterRef.ValueString(),
			},
			AdminPassword: data.AdminPassword.ValueString(),
		}
		fabricDeploy.VseriesNodeSpec = append(fabricDeploy.VseriesNodeSpec, vmSpec)
	}
	return fabricDeploy
}

// Given the FM Fabric Deployment payload struct, deploys it and returns any error
// encountered
func deployFabric(ctx context.Context, fabricData *FabricDeployment, fmClient *fmclient.FmClient) error {

	jsonData, err := json.Marshal(fabricData)
	if err != nil {
		return fmt.Errorf("Unable to marshal fabric data: %v \nerror: %s", fabricData, err)
	}

	_, err = fmClient.DoRequest(
		ctx,
		"POST",
		"api/v1.3/cloud/vmware/fabricDeployment/vseriesNodes/",
		nil,
		nil,
		bytes.NewBuffer(jsonData),
		"application/json",
	)

	if err != nil {
		return fmt.Errorf("Error in creating fabric:: %v \nerror: %s", fabricData, err)
	}

	return nil
}

// Delete a given Vseries Node from the fabric
func deleteVseriesNode(ctx context.Context, connectionId, nodeId string, fmClient *fmclient.FmClient) error {
	_, err := fmClient.DoRequest(
		ctx,
		"DELETE",
		fmt.Sprintf(
			"/api/v1.3/cloud/vmware/fabricDeployment/vseriesNodes/%s/%s",
			connectionId,
			nodeId,
		),
		nil,
		nil,
		nil,
		"",
	)
	return err
}

// Given the fabricID get the Vsereis nodes that are part of this fabric Deployment and their
// details
func getVseriesDetails(
	ctx context.Context,
	fabricId string,
	connectionId string,
	client *fmclient.FmClient,
) (
	map[string]VseriesNode,
	error,
) {
	fmVseriesData := struct {
		VseriesNodeData []VseriesNode `json:"vseriesNodes"`
	}{
		VseriesNodeData: make([]VseriesNode, 0),
	}
	retMap := make(map[string]VseriesNode)
	resp, err := client.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/vmware/fabricDeployment/vseriesNodes"),
		map[string]string{"connId": connectionId},
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("Get request of vseriesNodes calls with: %s failed: %s", connectionId, err)
	}
	err = json.Unmarshal(resp, &fmVseriesData)
	if err != nil {
		return nil, fmt.Errorf("Unable to convert resp to struct: %s error is: %s", string(resp), err)
	}
	for _, vData := range fmVseriesData.VseriesNodeData {
		splitName := strings.SplitN(vData.VMName, "-", 2)
		if len(splitName) != 2 || splitName[0] != fabricId {
			continue
		}
		retMap[splitName[1]] = VseriesNode{
			Id:           vData.Id,
			VMName:       splitName[1],
			ManagementIp: vData.ManagementIp,
			TunnelIps:    vData.TunnelIps,
			Version:      vData.Version,
			Status:       vData.Status,
		}
	}
	return retMap, nil
}

func (f *EsxiFabric) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_esxi_fabric"
}

func (f *EsxiFabric) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon Esxi Fabric",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the fabric",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"connection_id": schema.StringAttribute{
				MarkdownDescription: "Connection ID on which this fabric is to be deployed",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"admin_password": schema.StringAttribute{
				MarkdownDescription: "Admin Passowrd for the Vseries Nodes",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"datacenter_moref": schema.StringAttribute{
				MarkdownDescription: "Vcenter MORef of the datacanter on which to deploy",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"form_factor": schema.StringAttribute{
				MarkdownDescription: "Form Factor for the Vseries Nodes",
				Required:            true,
			},

			"image_id": schema.StringAttribute{
				MarkdownDescription: "Vseries Image to be loaded on the fabric nodes",
				Required:            true,
			},

			"vm_folder": schema.StringAttribute{
				MarkdownDescription: "Folder where we store the VM files",
				Required:            true,
			},

			"datastore_cluster_moref": schema.StringAttribute{
				MarkdownDescription: "Datastore cluster where the VM storage is allocated",
				Required:            true,
			},

			"disk_format": schema.StringAttribute{
				MarkdownDescription: "disk format for the VM",
				Required:            true,
			},

			"management_interface_spec": schema.SingleNestedAttribute{
				MarkdownDescription: "Management Interface spec common to all nodes in the fabric. Can be overrirden on a specific host, by providing the host specific Management Interface details",
				Required:            true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
				Attributes: map[string]schema.Attribute{
					"network_moref": schema.StringAttribute{
						MarkdownDescription: "Vcenter MORefof the management network",
						Required:            true,
					},
					"address_assignment_mode": schema.StringAttribute{
						MarkdownDescription: "Scheme for IP address assignment DHCP/Static",
						Required:            true,
					},
				},
			},
			"tunnel_interface_spec": schema.SingleNestedAttribute{
				MarkdownDescription: "Tunnel Interface spec common to all nodes in the fabric. Can be overrirden on a specific host, by providing the host specific Management Interface details",
				Required:            true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
				Attributes: map[string]schema.Attribute{
					"network_moref": schema.StringAttribute{
						MarkdownDescription: "Vcenter MORefof the tunnel  network",
						Required:            true,
					},
					"address_assignment_mode": schema.StringAttribute{
						MarkdownDescription: "Scheme for IP address assignment DHCP/Static",
						Optional:            true,
					},
				},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this Fabric for later use",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"timeout": schema.Int32Attribute{
				MarkdownDescription: "Maximum time to wait for the Vseries Nodes to become ok",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(900),
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.RequiresReplace(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"host_vm_spec": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"host_name": schema.StringAttribute{
							MarkdownDescription: "Host name for this host",
							Required:            true,
						},
						"host_moref": schema.StringAttribute{
							MarkdownDescription: "Host MORef for this host",
							Required:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "Name of the vseries node spun up on this host",
							Required:            true,
						},
						"vseries_node_id": schema.StringAttribute{
							MarkdownDescription: "ID of the Vseries Node from FM",
							Computed:            true,
						},
						"status": schema.StringAttribute{
							MarkdownDescription: "Status of the Vseries Node from FM",
							Computed:            true,
						},
						"version": schema.StringAttribute{
							MarkdownDescription: "Version of the Vseries Node from FM",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (f *EsxiFabric) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	fmClient, ok := req.ProviderData.(*fmclient.FmClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *fmclient.FmClient, got: %T. Report the issue to Gigamon", req.ProviderData),
		)
		return
	}
	f.fmClient = fmClient
}

func getRandomString(charSet []byte, stringLen int) string {
	result := make([]byte, stringLen)
	for i := 0; i < stringLen; i++ {
		result[i] = charSet[rand.Intn(len(charSet))]
	}
	return string(result)
}

func (f *EsxiFabric) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EsxiFabricModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	charSet := []byte("abcdefghijklmnopqrstuvwxyz")

	fabricId := getRandomString(charSet, 8)

	// Convert the TF data to FM Payload struct
	fmData := convertTFConfig(&data, fabricId)

	// Create the fabric
	timeout := data.Timeout.ValueInt32()
	myCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	err := deployFabric(myCtx, fmData, f.fmClient)

	if err != nil {
		resp.Diagnostics.AddError(
			"Could not create the fabric",
			fmt.Sprintf("%v", err),
		)
		return
	}

	data.Id = types.StringValue(fabricId)

	// We need to wait till the Vseries Nodes go to OK state (at least one of them)
	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// Get the Vsereis Node details and update the TF computed results
			vseriesNodeData, err := getVseriesDetails(
				myCtx,
				fabricId,
				fmData.ConnectionId,
				f.fmClient,
			)
			if err != nil {
				resp.Diagnostics.AddError(
					"Could not udate the fabric details",
					fmt.Sprintf("%v", err),
				)
				return
			}

			err = nil
			gotOk := false
			for _, host := range data.HostSpec {
				details, ok := vseriesNodeData[host.VmName.ValueString()]
				if !ok {
					err = fmt.Errorf("Not able to find %s in the returned vseriesnodes", host.VmName.ValueString())
					continue
				}
				host.VMId = types.StringValue(details.Id)
				host.Status = types.StringValue(details.Status)
				host.Version = types.StringValue(details.Version)
				if strings.ToLower(details.Status) == "ok" {
					gotOk = true
				}
			}
			if err != nil {
				resp.Diagnostics.AddError(
					"Error in updating the details of the fabric nodes",
					fmt.Sprintf("%v", err),
				)
				return
			}
			if gotOk {
				resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
				return
			}
		case <-myCtx.Done():
			resp.Diagnostics.AddError(
				"Timeout before the Vseries nodes could get to OK state",
				"Please increase the timeout, or check for errors in bringing up the fabric",
			)
			return
		}
	}
}

func (f *EsxiFabric) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EsxiFabricModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	fabricId := data.Id.ValueString()
	connectionId := data.ConnectionId.ValueString()

	// Get the Vsereis Node details and update the TF computed results
	vseriesNodeData, err := getVseriesDetails(
		ctx,
		fabricId,
		connectionId,
		f.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not udate the fabric details",
			fmt.Sprintf("%v", err),
		)
		return
	}

	err = nil
	for _, host := range data.HostSpec {
		details, ok := vseriesNodeData[host.VmName.ValueString()]
		if !ok {
			err = fmt.Errorf("Not able to find %s in the returned vseriesnodes. %w", host.VmName.ValueString(), err)
			continue
		}
		host.VMId = types.StringValue(details.Id)
		host.Status = types.StringValue(details.Status)
		host.Version = types.StringValue(details.Version)
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error in updating the details of the fabric nodes",
			fmt.Sprintf("%v", err),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (f *EsxiFabric) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Esxi Fabric does not support any modifications",
		"ESXI Fabric can only be created/deleted. They cannot be modified",
	)
}

func (f *EsxiFabric) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EsxiFabricModel
	var err error

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	connectionId := data.ConnectionId.ValueString()
	err = nil
	for _, vHost := range data.HostSpec {
		apiErr := deleteVseriesNode(
			ctx,
			connectionId,
			vHost.VMId.ValueString(),
			f.fmClient,
		)
		if apiErr != nil {
			err = fmt.Errorf(
				"Unable to delete node: %d error: %s. %w",
				vHost.VMId.ValueString(),
				apiErr,
				err,
			)
		}
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Error in Deleting the fabric Vseries Nodes",
			fmt.Sprintf("%v", err),
		)
	}
	return
}
