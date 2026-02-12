// Copyright (c) Gigamon, Inc.

// Implements the Resrouces for the ESXI Fabric (VSeries) resources

package esxiresources

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/esxiutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// EsxiIntfSpec describes the interface details of the Vseries spec
type EsxiInterfaceModel struct {
	Ref           types.String `tfsdk:"network_moref"`
	AddressMode   types.String `tfsdk:"address_assignment_mode"`
	Mtu           types.Int32  `tfsdk:"mtu"`
	IPAddress     types.String `tfsdk:"ip_address"`
	IPAddressMask types.String `tfsdk:"ip_address_mask"`
	GatewayIP     types.String `tfsdk:"gateway_ip"`
}

// EsxiVmSpec describes the spec for a VM on ESXI
type EsxiVMSpec struct {
	HostRef             types.String        `tfsdk:"host_moref"`
	HostName            types.String        `tfsdk:"host_name"`
	DiskFormat          types.String        `tfsdk:"disk_format"`
	DatastoreRef        types.String        `tfsdk:"datastore_moref"`
	DatastoreClusterRef types.String        `tfsdk:"datastore_cluster_moref"`
	ClusterRef          types.String        `tfsdk:"cluster_moref"`
	MgmtInterface       EsxiInterfaceModel  `tfsdk:"management_interface"`
	TunnelInterface     *EsxiInterfaceModel `tfsdk:"tunnel_interface"`
	VMFolder            types.String        `tfsdk:"vm_folder"`
	NameServer          []types.String      `tfsdk:"name_server"`
	AdminPassword       types.String        `tfsdk:"admin_password"`
	VmName              types.String        `tfsdk:"name"`
	VMId                types.String        `tfsdk:"vseries_node_id"`
	Status              types.String        `tfsdk:"status"`
	Version             types.String        `tfsdk:"version"`
}

// EsxiFabricModel describes the fabric resource data model.
type EsxiFabricModel struct {
	Name          types.String  `tfsdk:"name"`
	ConnectionId  types.String  `tfsdk:"connection_id"`
	DatacenterRef types.String  `tfsdk:"datacenter_moref"`
	FormFactor    types.String  `tfsdk:"form_factor"`
	ImageId       types.String  `tfsdk:"image_id"`
	HostSpec      []*EsxiVMSpec `tfsdk:"host_vm_spec"`
	Id            types.String  `tfsdk:"id"`
	Timeout       types.Int32   `tfsdk:"timeout"`
}

// TF Schema for management interface spec for the Vseries Node Spec
func EsxiIntfSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		PlanModifiers: []planmodifier.Object{
			objectplanmodifier.RequiresReplace(),
		},
		Attributes: map[string]schema.Attribute{
			"network_moref": schema.StringAttribute{
				MarkdownDescription: "MORef for the  network",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"address_assignment_mode": schema.StringAttribute{
				MarkdownDescription: "Address assignment mode DHCP/Static",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("DHCP"),
				Validators: []validator.String{
					stringvalidator.OneOf("DHCP", "Static"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mtu": schema.Int32Attribute{
				MarkdownDescription: "MTU of the network Interface",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(1500),
				Validators: []validator.Int32{
					int32validator.AtLeast(1280),
					int32validator.AtMost(9000),
				},
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.RequiresReplace(),
					int32planmodifier.UseStateForUnknown(),
				},
			},
			"ip_address": schema.StringAttribute{
				MarkdownDescription: "Ip Address for the interface, when using Static mode of address assignment",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ip_address_mask": schema.StringAttribute{
				MarkdownDescription: "Address mask in 255.255.0.0 format for the network ip address",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"gateway_ip": schema.StringAttribute{
				MarkdownDescription: "Gatway IP when using static address assignment",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func EsxiHostSpecSchema() schema.NestedBlockObject {
	return schema.NestedBlockObject{
		Attributes: map[string]schema.Attribute{
			"host_moref": schema.StringAttribute{
				MarkdownDescription: "Host MORef on which this Vseries Node is spun up",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"host_name": schema.StringAttribute{
				MarkdownDescription: "Host name on which this Vseries Node is spun up",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"datastore_moref": schema.StringAttribute{
				MarkdownDescription: "Datastore MORef on which this vseries Nodes is hosted",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"datastore_cluster_moref": schema.StringAttribute{
				MarkdownDescription: "Datastore cluster MOref to host this vseris node",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.AtLeastOneOf(path.Expressions{
						path.MatchRelative().AtParent().AtName("datastore_moref"),
					}...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cluster_moref": schema.StringAttribute{
				MarkdownDescription: "Cluster to whcih this host belongs",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"admin_password": schema.StringAttribute{
				MarkdownDescription: "Admin password for the Vseries Node",
				Required:            true,
				Sensitive:           true,
				WriteOnly:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"disk_format": schema.StringAttribute{
				MarkdownDescription: "disk format to be used for the Vseries node",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("thin"),
				Validators: []validator.String{
					stringvalidator.OneOf("thin", "thick", "eagerZeroedThick"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"management_interface": EsxiIntfSchema(),
			"tunnel_interface":     EsxiIntfSchema(),
			"vm_folder": schema.StringAttribute{
				MarkdownDescription: "Folder on which this VM files are placed",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("/"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name_server": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name for this Vseris Node VM",
				Required:            true,
			},
			"vseries_node_id": schema.StringAttribute{
				MarkdownDescription: "Node ID of the Vseries Node VM",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Status of the Vseries Node VM",
				Computed:            true,
			},
			"version": schema.StringAttribute{
				MarkdownDescription: "Version of Gigamon Software on Vseries Node",
				Computed:            true,
			},
		},
	}
}

// The complete fabric vseries node spec schema
func FabficModelSchema() schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Gigamon ESXI Fabric Deployment Schema",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name given by the user for this deployment",
				Required:            true,
			},
			"connection_id": schema.StringAttribute{
				MarkdownDescription: "connection_id on which this fabric is to be deployed",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"datacenter_moref": schema.StringAttribute{
				MarkdownDescription: "Data center MORef to deploy the Vseries nodes",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"image_id": schema.StringAttribute{
				MarkdownDescription: "Image to load on the Vseries nodes",
				Required:            true,
			},
			"form_factor": schema.StringAttribute{
				MarkdownDescription: "Form factor of the VMs to deploy",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("Small"),
				Validators: []validator.String{
					stringvalidator.OneOf("Small", "Medium", "Large"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "ID of this fabric deployment for later use",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"timeout": schema.Int32Attribute{
				MarkdownDescription: "Timeout for this resource creation (in seconds)",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(900),
				Validators: []validator.Int32{
					int32validator.AtLeast(300),
					int32validator.AtMost(36000),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"host_vm_spec": schema.ListNestedBlock{
				MarkdownDescription: "list of host specs nested block schema",
				NestedObject:        EsxiHostSpecSchema(),
			},
		},
	}
}

var _ resource.Resource = &EsxiFabric{}

// EsxiFabric manages the Fabric for ESXI deployments
type EsxiFabric struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// ESXI resources for fabric management
func NewEsxiFabric() resource.Resource {
	return &EsxiFabric{}
}

func (f *EsxiFabric) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_esxi_fabric"
}

func (f *EsxiFabric) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = FabficModelSchema()
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

// Get the latest data from FM for this deployment and update the TF model data
func (f *EsxiFabric) UpdateGOtoTF(
	ctx context.Context,
	goData *esxiutils.EsxiFabric,
	tfData *EsxiFabricModel,
	deploymentId string,
) (int, error) {

	healthyCount, err := esxiutils.GetDeploymentUpdate(ctx, deploymentId, goData, f.fmClient)
	if err != nil {
		return healthyCount, err
	}

	// Copy the FM returned data back to the TF model
	tfData.ConnectionId = types.StringValue(goData.ConnectionId)
	tfData.DatacenterRef = types.StringValue(goData.DatacenterRef.VcKey)
	tfData.FormFactor = types.StringValue(goData.FormFactor)
	tfData.ImageId = types.StringValue(goData.ImageId)
	tfData.HostSpec = make([]*EsxiVMSpec, 0, 0)
	for _, fmHost := range goData.HostSpecs {
		tfHost := &EsxiVMSpec{}
		tfHost.HostRef = types.StringValue(fmHost.HostRef.VcKey)
		tfHost.HostName = types.StringValue(fmHost.HostRef.Name)
		if fmHost.DiskFormat == "" {
			tfHost.DiskFormat = types.StringValue("thin")
		} else {
			tfHost.DiskFormat = types.StringValue(fmHost.DiskFormat)
		}
		if fmHost.DatastoreRef != nil {
			tfHost.DatastoreRef = types.StringValue(fmHost.DatastoreRef.VcKey)
		} else {
			tfHost.DatastoreRef = types.StringNull()
		}
		if fmHost.DatastoreClusterRef != nil {
			tfHost.DatastoreClusterRef = types.StringValue(fmHost.DatastoreClusterRef.VcKey)
		} else {
			tfHost.DatastoreClusterRef = types.StringNull()
		}
		if fmHost.ClusterRef != nil {
			tfHost.ClusterRef = types.StringValue(fmHost.ClusterRef.VcKey)
		} else {
			tfHost.ClusterRef = types.StringNull()
		}
		lenDnsNames := len(fmHost.NameServer)
		if lenDnsNames > 0 {
			tfHost.NameServer = make([]types.String, lenDnsNames, lenDnsNames)
			for index := range lenDnsNames {
				tfHost.NameServer[index] = types.StringValue(fmHost.NameServer[index])
			}
		}
		tfHost.VMFolder = types.StringValue(fmHost.VmFolder)
		tfHost.VmName = types.StringValue(fmHost.VmName)
		tfHost.VMId = types.StringValue(fmHost.VMId)
		tfHost.Status = types.StringValue(fmHost.Status)
		tfHost.Version = types.StringValue(fmHost.Version)
		tfHost.MgmtInterface = EsxiInterfaceModel{}
		copyGOtoTFInterface(&fmHost.MgmtInterface, &tfHost.MgmtInterface)
		if fmHost.TunnelInterface != nil {
			tfHost.TunnelInterface = &EsxiInterfaceModel{}
			copyGOtoTFInterface(fmHost.TunnelInterface, tfHost.TunnelInterface)
		}
		tfData.HostSpec = append(tfData.HostSpec, tfHost)
	}
	return healthyCount, nil
}

func copyGOtoTFInterface(source *esxiutils.EsxiInterfaceSpec, dest *EsxiInterfaceModel) {
	dest.Ref = types.StringValue(source.NetworkRef.VcKey)
	dest.AddressMode = types.StringValue(source.AddressMode)
	dest.Mtu = types.Int32Value(source.Mtu)
	if source.IPAddress != "" {
		dest.IPAddress = types.StringValue(source.IPAddress)
	} else {
		dest.IPAddress = types.StringNull()
	}
	if source.IPAddressMask != "" {
		dest.IPAddressMask = types.StringValue(source.IPAddressMask)
	} else {
		dest.IPAddressMask = types.StringNull()
	}
	if source.GatewayIP != "" {
		dest.GatewayIP = types.StringValue(source.GatewayIP)
	} else {
		dest.GatewayIP = types.StringNull()
	}
}

func (f *EsxiFabric) ConvertTFtoGO(
	ctx context.Context,
	data *EsxiFabricModel,
	goData *esxiutils.EsxiFabric,
) {
	//Extract Raw UUID from TypedId
	connId, err := commonutils.UUIDFromTypedID(data.ConnectionId.ValueString())
	if err != nil {
		return
	}

	goData.ConnectionId = connId
	goData.DatacenterRef.VcKey = data.DatacenterRef.ValueString()
	goData.ImageId = data.ImageId.ValueString()
	goData.FormFactor = data.FormFactor.ValueString()
	goData.HostSpecs = make([]*esxiutils.EsxiHostSpec, 0, 0)
	for _, tfHost := range data.HostSpec {
		lenDnsServers := len(tfHost.NameServer)
		goHost := &esxiutils.EsxiHostSpec{
			NameServer: make([]string, lenDnsServers, lenDnsServers),
		}
		goHost.HostRef.VcKey = tfHost.HostRef.ValueString()
		goHost.HostRef.Name = tfHost.HostName.ValueString()
		goHost.DiskFormat = tfHost.DiskFormat.ValueString()
		if tfHost.DatastoreRef.ValueString() != "" {
			goHost.DatastoreRef = &esxiutils.ObjectRef{
				VcKey: tfHost.DatastoreRef.ValueString(),
			}
		}
		if tfHost.DatastoreClusterRef.ValueString() != "" {
			goHost.DatastoreClusterRef = &esxiutils.ObjectRef{
				VcKey: tfHost.DatastoreClusterRef.ValueString(),
			}
		}
		if tfHost.ClusterRef.ValueString() != "" {
			goHost.ClusterRef = &esxiutils.ObjectRef{
				VcKey: tfHost.ClusterRef.ValueString(),
			}
		}
		for index := range lenDnsServers {
			goHost.NameServer[index] = tfHost.NameServer[index].ValueString()
		}
		goHost.VmFolder = tfHost.VMFolder.ValueString()
		goHost.VmName = tfHost.VmName.ValueString()
		goHost.AdminPassword = tfHost.AdminPassword.ValueString()
		goHost.MgmtInterface = esxiutils.EsxiInterfaceSpec{
			NetworkRef: esxiutils.ObjectRef{
				VcKey: tfHost.MgmtInterface.Ref.ValueString(),
			},
			AddressMode:   tfHost.MgmtInterface.AddressMode.ValueString(),
			Mtu:           tfHost.MgmtInterface.Mtu.ValueInt32(),
			IPAddress:     tfHost.MgmtInterface.IPAddress.ValueString(),
			IPAddressMask: tfHost.MgmtInterface.IPAddressMask.ValueString(),
			GatewayIP:     tfHost.MgmtInterface.GatewayIP.ValueString(),
		}
		goHost.TunnelInterface = &esxiutils.EsxiInterfaceSpec{
			NetworkRef: esxiutils.ObjectRef{
				VcKey: tfHost.TunnelInterface.Ref.ValueString(),
			},
			AddressMode:   tfHost.TunnelInterface.AddressMode.ValueString(),
			Mtu:           tfHost.TunnelInterface.Mtu.ValueInt32(),
			IPAddress:     tfHost.TunnelInterface.IPAddress.ValueString(),
			IPAddressMask: tfHost.TunnelInterface.IPAddressMask.ValueString(),
			GatewayIP:     tfHost.TunnelInterface.GatewayIP.ValueString(),
		}
		goData.HostSpecs = append(goData.HostSpecs, goHost)
	}
}

func (f *EsxiFabric) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var cfgData, data EsxiFabricModel

	goData := &esxiutils.EsxiFabric{}

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfgData)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Copy the password value from the config data to the plan data
	for index, tfHost := range data.HostSpec {
		tfHost.AdminPassword = cfgData.HostSpec[index].AdminPassword
	}
	f.ConvertTFtoGO(ctx, &data, goData)

	// Start a new context with this resource speicfic timeout
	timeout := data.Timeout.ValueInt32()
	myCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	deploymentId, err := esxiutils.DeployFabric(myCtx, goData, f.fmClient)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to deploy fabric",
			fmt.Sprintf("error when deploying fabric: %v", err),
		)
		return
	}
	data.Id = types.StringValue(deploymentId)

	// Update it now and then wait till the Vseries nodes are spun up. Update
	// again with the latest data
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	// Check for an updated status every 30 seconds
	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// Get the Vsereis Node details and update the TF computed results
			count, err := f.UpdateGOtoTF(myCtx, goData, &data, deploymentId)
			if err != nil {
				resp.Diagnostics.AddError(
					"Unable to deploy fabric",
					fmt.Sprintf("error when waiting for fabrc to become ready: %v", err),
				)
				return
			}
			if count == 0 {
				resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
				return
			}

		case <-myCtx.Done():
			resp.Diagnostics.AddError(
				"Timeout before the fabric nodes could get to OK state",
				"Please increase the timeout, or check for errors in bringing up the fabric",
			)
			return
		}
	}
}

func (f *EsxiFabric) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EsxiFabricModel
	goData := &esxiutils.EsxiFabric{}

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	f.ConvertTFtoGO(ctx, &data, goData)

	// Read from FM and update our Model data
	_, err := f.UpdateGOtoTF(ctx, goData, &data, data.Id.ValueString())
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) {
			if fmErr.ErrorCode() == fmclient.ObjectNotFound {
				resp.State.RemoveResource(ctx)
				return
			}
		}
		resp.Diagnostics.AddError(
			"Unable to read fabric data",
			fmt.Sprintf("error when reading fabric data: %v", err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (f *EsxiFabric) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var cfgData, planData, stateData EsxiFabricModel
	var planGoStruct esxiutils.EsxiFabric

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfgData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Copy the password value from the config data to the plan data
	for index, tfHost := range planData.HostSpec {
		tfHost.AdminPassword = cfgData.HostSpec[index].AdminPassword
	}

	// Convert the plan(what we want to do) and the state(where we are currently) to the
	// corresponding go struct and do a diff between them to see what we have to change

	// Esxi Fabric can have the following changes
	// VM Name change - Customer can change the name of the Vseries node at any time
	// VM Image - For upgrade customer can change the VM Image to the new version. Along with
	//   with the upgrade, customer can also change the DiskFormat, FormFacotr and NameServer
	//   Please note: These fields can be changed only along with upgrade.
	//     Upgrade is always done at the deployment level and applies to all the specs in the
	//     deployment
	// Delete a spec - One or more spec can be deleted, in which case the corresponding
	//   specs in the Fm will also be deleted
	// Add a spec (not yet supported in FM). we will add this support once that is available

	// Start a new context with this resource speicfic timeout
	timeout := planData.Timeout.ValueInt32()
	myCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	f.ConvertTFtoGO(myCtx, &planData, &planGoStruct)
	deploymentId := stateData.Id.ValueString()
	changeSpec, err := esxiutils.GetDiff(
		myCtx,
		&planGoStruct,
		deploymentId,
		f.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read fabric data/inconcsitent data  during update",
			fmt.Sprintf("error when reading/finding diff of fabric data: %v", err),
		)
		return
	}
	tflog.Info(myCtx, "***** Dumping the changespec fields *******", map[string]any{
		"changeSpec": changeSpec,
	})

	// Implement the required Diffs
	// name Changes
	err = esxiutils.ChangeVmName(myCtx, changeSpec.VmNameChanges, f.fmClient)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to change VM Node namesduring update",
			fmt.Sprintf("error when changing VM names: %v", err),
		)
		return
	}

	// Upgrade requests
	err = esxiutils.UpgradeVms(myCtx, changeSpec.UpgradeVMs, f.fmClient)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to upgrade VM Node during update",
			fmt.Sprintf("error when upgrading VMs:  %v", err),
		)
		return
	}

	// Delete Changes
	err = esxiutils.DeleteVms(myCtx, changeSpec.DeleteVMs, f.fmClient)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Delete VM Node during update",
			fmt.Sprintf("error when deleting VMs:  %v", err),
		)
		return
	}

	// New Spec Create Changes
	err = esxiutils.AddNewSpecs(
		myCtx,
		changeSpec.AddVMs,
		&planGoStruct,
		deploymentId,
		f.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create VM Node during update",
			fmt.Sprintf("error when creating VMs:  %v", err),
		)
		return
	}

	// Check for an updated status every 30 seconds
	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// Get the Vsereis Node details and update the TF computed results
			count, err := f.UpdateGOtoTF(myCtx, &planGoStruct, &stateData, deploymentId)
			if err != nil {
				resp.Diagnostics.AddError(
					"Unable to deploy fabric",
					fmt.Sprintf("error when waiting for fabrc to become ready: %v", err),
				)
				return
			}
			if count == 0 {
				resp.Diagnostics.Append(resp.State.Set(ctx, &stateData)...)
				return
			}

		case <-myCtx.Done():
			resp.Diagnostics.AddError(
				"Timeout before the fabric nodes could get to OK state",
				"Please increase the timeout, or check for errors in bringing up the fabric",
			)
			return
		}
	}
}

func (f *EsxiFabric) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EsxiFabricModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	err := esxiutils.DeleteFabric(ctx, data.Id.ValueString(), f.fmClient)
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) {
			if fmErr.ErrorCode() == fmclient.ObjectNotFound {
				return
			}
		}
		resp.Diagnostics.AddError(
			"Unable to Delete the fabric",
			fmt.Sprintf("unable to delete fabric. error is %v", err),
		)
		return
	}
}
