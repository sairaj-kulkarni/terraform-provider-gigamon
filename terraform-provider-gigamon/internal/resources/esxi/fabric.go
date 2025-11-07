// Copyright (c) Gigamon, Inc.

// Implements the Resrouces for the ESXI Fabric (VSeries) resources

package esxiresources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-log/tflog"

	"gigamon.com/terraform-provider-gigamon/internal/fmclient"

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
	Ref types.String `tfsdk:"network_moref"`
	AddressMode types.String `tfsdk:"address_assignment_mode"`
}

// EsxiVmSpec describes the spec for a VM on ESXI 
type EsxiVMSpec struct {
	HostName types.String `tfsdk:"host_name"`
	HostRef types.String `tfsdk:"host_moref"`
	VmName types.String `tfsdk:"name"`
	VMId types.String `tfsdk:"vseries_node_id"`
}

// EsxiFabricModel describes the fabric resource data model.
type EsxiFabricModel struct {
	Name types.String `tfsdk:"name"`
	ConnectionId types.String `tfsdk:"connection_id"`
	DatacenterRef types.String `tfsdk:"datacenter_moref"`
	FormFactor types.String `tfsdk:"form_factor"`
	ImageId types.String `tfsdk:"image_id"`
	VmFolder types.String `tfsdk:"vm_folder"`
	DataStoreClusterRef types.String `tfsdk:"datastore_cluster_moref"`
	DiskFormat types.String `tfsdk:"disk_format"`
	MgmtIntf *EsxiIntfSpec `tfsdk:"management_interface_spec"`
	TunnelIntf *EsxiIntfSpec `tfsdk:"tunnel_interface_spec"`
	HostSpec []*EsxiVMSpec `tfsdk:"host_vm_spec"`
	Id types.String `tfsdk:"id"`
}

// Go structs for managing the fabric deployment for VMWare ESXI Platform

// ObjectSpec used to represent the general spec for an object in VMWare ESXI environment
type ObjectSpec struct {
	Ref string `json:"vcKey,omitempty"` // Represents the MORef of the object in Vcenter
	Name string `json:"name,omitempty"` // Name of the object (user provided)
}

// InterfaceSpec is to represent an network interface NIC of the VM
type InterfaceSpec struct {
	NetworkSpec ObjectSpec `json:"intfRef"` // The network to which this NIC is assigned to
	AddressMode string `json:"ipType"` // Whether it is DHCP/static assignment
}

// VMSpec represents the details of the VM Spec that is used to spin up for a VSeries instance
type VMSpec struct {
	HostSpec *ObjectSpec `json:"hostRef"` // Host specification on which this VM is spun up
	VMName string `json:"vmNodeName"` // Name assigned by user for this Vseries VM
	DiskFormat string `json:"diskFormat"` // Format of the disk
	DataStoreSpec *ObjectSpec `json:"datastoreRef,omitempty"` // VM DataStore
	DataStoreClusterSpec *ObjectSpec `json:"datastoreClusterRef,omitemptry"` // VM Datstore Cluster
	ManagementIntfSpec *InterfaceSpec `json:"intfMgmt"` // Management NIC network details
	TunnelIntfSpec *InterfaceSpec `json:"intfTunnel"` // Tunnel NIC network details
	VMFolder string `json:"vmFolder"` // Folder to hold the VM files
}

// FabricDeployment represents the struct that is passed to FM to create a fabric
type FabricDeployment struct {
	ConnectionId string `json:"connId"` // Connection ID on which to create this fabric
	DataCenterSpec ObjectSpec `json:"dcRef"` // DataCenter spec for this fabric creation
	VseriesNodeSpec []VMSpec `json:"hostSpecs"` // Vseries Node Spec
	ImageId string `json:"imageName"` // Vseries Image name/version
	FormFactor string `json:"formFactor"` // Vseries form factor
}

// Vseriesnode represents the GET on FM to fetch the Vsereis Node details
type VseriesNode struct {
	Id string `json:"id"` // ID assigned to this vseries node by FM
	VMName string `json:"name"` // The Name user assigned to this Vseries Node
	ConnectionId string `json:"connId"` // Connection ID used to manage this Vseries Node
	ManagementIp string `json:"mgmtIp"` // Vseries Node management IP address
	TunnelIps []string `json:"tunnelIps"` // Vseries Node tunnel IP address list
	Version string `json:"version"` // Version on the Vseries node
	Status string `json:"status"` // Health status of the Vseries Node
}

// Helper Functions to map between Go struct and TF structs and handle the FM interaction
func convertTFConfig(data *EsxiFabricModel) *FabricDeployment {
	fabricDeploy := &FabricDeployment {
		ConnectionId: data.ConnectionId.ValueString(),
		DataCenterSpec: ObjectSpec {
			Ref: data.DatacenterRef.ValueString(),
		},
		ImageId: data.ImageId.ValueString(),
		FormFactor: data.FormFactor.ValueString(),
		VseriesNodeSpec: make([]VMSpec, 0, len(data.HostSpec)),
	}
	for _, hSpec := range(data.HostSpec) {
		vmSpec := VMSpec {
			DiskFormat: data.DiskFormat.ValueString(),
			VMFolder: data.VmFolder.ValueString(),
			HostSpec: &ObjectSpec {
				Ref: hSpec.HostRef.ValueString(),
				Name: hSpec.HostName.ValueString(),
			},
			VMName: hSpec.VmName.ValueString(),
			ManagementIntfSpec: &InterfaceSpec {
				NetworkSpec: ObjectSpec {
					Ref: data.MgmtIntf.Ref.ValueString(),
				},
		        AddressMode: data.MgmtIntf.AddressMode.ValueString(),
			},
			TunnelIntfSpec: &InterfaceSpec{
				NetworkSpec: ObjectSpec {
					Ref: data.TunnelIntf.Ref.ValueString(),
				},
				AddressMode: data.TunnelIntf.AddressMode.ValueString(),
			},
			DataStoreClusterSpec: &ObjectSpec{
				Ref: data.DataStoreClusterRef.ValueString(),
			},
		}
		fabricDeploy.VseriesNodeSpec = append(fabricDeploy.VseriesNodeSpec, vmSpec)
	}
	return fabricDeploy
}

// Given the FM Fabric Deployment payload struct, deploys it and returns the set of
// Vseries Node ID (in the same order as in the deployment payload) or an error
func deployFabric (ctx context.Context, fabricData *FabricDeployment, fmClient *fmclient.FmClient) ([]string, error) {
	
	jsonData, err := json.Marshal(fabricData)
	if err != nil {
		return nil, fmt.Errorf("Unable to marshal fabric data: %v \nerror: %s", fabricData, err)
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
		return nil, fmt.Errorf("Error in creating fabric:: %v \nerror: %s", fabricData, err)
	}

	// For now just return empty list of Vseries we will update this later
	return nil, nil
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
				Required: true,
				PlanModifiers: []planmodifier.String{
                    stringplanmodifier.RequiresReplace(),
                },
			},

			"datacenter_moref": schema.StringAttribute{
				MarkdownDescription: "Vcenter MORef of the datacanter on which to deploy",
				Required: true,
				PlanModifiers: []planmodifier.String{
                    stringplanmodifier.RequiresReplace(),
                },
			},

			"form_factor": schema.StringAttribute{
				MarkdownDescription: "Form Factor for the Vseries Nodes",
				Required: true,
			},

			"image_id": schema.StringAttribute{
				MarkdownDescription: "Vseries Image to be loaded on the fabric nodes",
				Required: true,
			},

			"vm_folder": schema.StringAttribute{
				MarkdownDescription: "Folder where we store the VM files",
				Required: true,
			},

			"datastore_cluster_moref": schema.StringAttribute{
				MarkdownDescription: "Datastore cluster where the VM storage is allocated",
				Required: true,
			},

			"disk_format": schema.StringAttribute{
				MarkdownDescription: "disk format for the VM",
				Required: true,
			},

			"management_interface_spec": schema.SingleNestedAttribute{
				MarkdownDescription: "Management Interface spec common to all nodes in the fabric. Can be overrirden on a specific host, by providing the host specific Management Interface details",
				Required: true,
				PlanModifiers: []planmodifier.Object{
                    objectplanmodifier.RequiresReplace(),
                },
				Attributes: map[string]schema.Attribute{
					"network_moref": schema.StringAttribute{
						MarkdownDescription:"Vcenter MORefof the management network",
						Required: true,
					},
					"address_assignment_mode": schema.StringAttribute{
						MarkdownDescription:"Scheme for IP address assignment DHCP/Static",
						Required: true,
					},
                },
			},
			"tunnel_interface_spec": schema.SingleNestedAttribute{
				MarkdownDescription: "Tunnel Interface spec common to all nodes in the fabric. Can be overrirden on a specific host, by providing the host specific Management Interface details",
				Required: true,
				PlanModifiers: []planmodifier.Object{
                    objectplanmodifier.RequiresReplace(),
                },
				Attributes: map[string]schema.Attribute{
					"network_moref": schema.StringAttribute{
						MarkdownDescription:"Vcenter MORefof the tunnel  network",
						Required: true,
					},
					"address_assignment_mode": schema.StringAttribute{
						MarkdownDescription:"Scheme for IP address assignment DHCP/Static",
						Optional: true,
					},
                },
			},
			"host_vm_spec": schema.SetNestedAttribute{
				MarkdownDescription: "Spec for the Vseries node on each host in this fabric",
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"host_name": schema.StringAttribute{
							MarkdownDescription: "Host name for this host",
							Required: true,
						},
						"host_moref": schema.StringAttribute{
							MarkdownDescription: "Host MORef for this host",
							Required: true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "Name of the vseries node spun up on this host",
							Required: true,
						},
						"vseries_node_id": schema.StringAttribute{
							MarkdownDescription: "ID of the Vseries Node from FM",
							Computed: true,
						},
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

func (f *EsxiFabric) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EsxiFabricModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Convert the TF data to FM Payload struct
	fmData := convertTFConfig(&data)

	// Create the fabric
	_, err := deployFabric(ctx, fmData, f.fmClient)

	if err != nil {
        resp.Diagnostics.AddError(
             "Could not create the fabric",
		     fmt.Sprintf("%s", err),
	    )
		return
	}

	// For now just simply set the ID field
	data.Id = types.StringValue("My:ID.2")
	for _, host := range data.HostSpec {
	    tflog.Info(ctx, "SEtting  the fabric node id", nil)
		host.VMId = types.StringValue("my-id")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (f *EsxiFabric) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EsxiFabricModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

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

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	return
}
