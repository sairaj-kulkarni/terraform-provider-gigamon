// Copyright (c) Gigamon, Inc.

// Implements the datasource for the inventory, i.e. gets the VCentter inventory info
// Provides data sources to get the following from FM managed MD / Connections
// Datacenter/Cluster/network/switchs/datastore MORef from their names
// Details of the host including the MORef and other details

package esxidatasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"gigamon.com/terraform-provider-gigamon/internal/fmclient"
	"gigamon.com/terraform-provider-gigamon/internal/utils/fmesxi"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &EsxiDataCenter{}
var _ datasource.DataSource = &EsxiCluster{}
var _ datasource.DataSource = &EsxiDataStore{}
var _ datasource.DataSource = &EsxiDataStoreCluster{}
var _ datasource.DataSource = &EsxiNetworks{}
var _ datasource.DataSource = &EsxiPortGroups{}
var _ datasource.DataSource = &EsxiHosts{}

// Hosts Datastore structs
func NewEsxiHosts() datasource.DataSource {
	return &EsxiHosts{}
}

// EsxiHosts Get the Hosts ref given the set of hosts
type EsxiHosts struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// EsxiHostsModel Defines the model for the Hosts datastore

type EsxiHostDetails struct {
	HostRef    types.String `tfsdk:"host_moref"`
	ClusterRef types.String `tfsdk:"cluster_moref"`
}

type EsxiHostsModel struct {
	ConnectionId    types.String `tfsdk:"connection_id"`
	DataCenterRef   types.String `tfsdk:"data_center_moref"`
	ClusterRef      types.List   `tfsdk:"cluster_moref"`
	HostName        types.String `tfsdk:"hostname"`
	HostNamePattern types.String `tfsdk:"hostname_pattern"`
	HostDetails     types.Map    `tfsdk:"host_details"`
}

// Portgroup Datastore structs
func NewEsxiPortGroups() datasource.DataSource {
	return &EsxiPortGroups{}
}

// EsxiPortGroups Get the Portgroup ref given the name
type EsxiPortGroups struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// EsxiPortGroupssModel Defines the model for the PortGroups datastore

type EsxiPortGroupsModel struct {
	ConnectionId  types.String `tfsdk:"connection_id"`
	DataCenterRef types.String `tfsdk:"data_center_moref"`
	PortGroupName types.String `tfsdk:"portgroup_name"`
	PortGroupRef  types.String `tfsdk:"portgroup_moref"`
}

// Network Datastore structs
func NewEsxiNetworks() datasource.DataSource {
	return &EsxiNetworks{}
}

// EsxiNetworks Get the Network ref given the name
type EsxiNetworks struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// EsxiNetworksModel Defines the model for the Networks datastore

type EsxiNetworksModel struct {
	ConnectionId  types.String `tfsdk:"connection_id"`
	DataCenterRef types.String `tfsdk:"data_center_moref"`
	NetworkName   types.String `tfsdk:"network_name"`
	NetworkRef    types.String `tfsdk:"network_moref"`
}

// DatastoreCluster Datastore Cluster structs
func NewEsxiDataStoreCluster() datasource.DataSource {
	return &EsxiDataStoreCluster{}
}

// EsxiDataStoreCluster Get the DataStore Cluster ref given the name
type EsxiDataStoreCluster struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// EsxiDataStoreClusterModel Defines the model for the data store cluster datasource

type EsxiDataStoreClusterModel struct {
	ConnectionId         types.String `tfsdk:"connection_id"`
	DataCenterRef        types.String `tfsdk:"data_center_moref"`
	DataStoreClusterName types.String `tfsdk:"datastore_cluster_name"`
	DataStoreClusterRef  types.String `tfsdk:"datastore_cluster_moref"`
}

// Datastore Datastore structs
func NewEsxiDataStore() datasource.DataSource {
	return &EsxiDataStore{}
}

// EsxiDataStore Get the DataStore ref given the name
type EsxiDataStore struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// EsxiDataStoreModel Defines the model for the data store datasource

type EsxiDataStoreModel struct {
	ConnectionId  types.String `tfsdk:"connection_id"`
	DataCenterRef types.String `tfsdk:"data_center_moref"`
	DataStoreName types.String `tfsdk:"datastore_name"`
	DataStoreRef  types.String `tfsdk:"datastore_moref"`
}

// Cluster Datastore structs
func NewEsxiCluster() datasource.DataSource {
	return &EsxiCluster{}
}

// EsxiCluster Get the Cluster ref given the name
type EsxiCluster struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// EsxiClusterModel Defines the model for the cluster datasource

type EsxiClusterModel struct {
	ConnectionId  types.String `tfsdk:"connection_id"`
	DataCenterRef types.String `tfsdk:"data_center_moref"`
	ClusterName   types.String `tfsdk:"cluster_name"`
	ClusterRef    types.String `tfsdk:"cluster_moref"`
}

// Datacenter Datastore structs
func NewEsxiDataCenter() datasource.DataSource {
	return &EsxiDataCenter{}
}

// EsxiataCenter Get the Datacenter ref given the name
type EsxiDataCenter struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// EsxiDataCenterModel Defines the model for the data center datasource

type EsxiDataCenterModel struct {
	ConnectionId   types.String `tfsdk:"connection_id"`
	DataCenterName types.String `tfsdk:"data_center_name"`
	DataCenterRef  types.String `tfsdk:"data_center_moref"`
}

// Implementation of the hosts DataStore
func (h *EsxiHosts) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_esxi_hosts"
}

func (h *EsxiHosts) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "ESXI Hosts Data Source Model",

		Attributes: map[string]schema.Attribute{
			"connection_id": schema.StringAttribute{
				MarkdownDescription: "Connection ID to use to fetch the details",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"data_center_moref": schema.StringAttribute{
				MarkdownDescription: "MORef of the Data Center where the portgroup is located",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"cluster_moref": schema.ListAttribute{
				MarkdownDescription: "Clusters to which the host must belong",
				Optional:            true,
				ElementType:         types.StringType,
				Validators: []validator.List{
					listvalidator.All(
						listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)),
					),
				},
			},
			"hostname": schema.StringAttribute{
				MarkdownDescription: "hostname for which we want the MORef",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.ConflictsWith(
						path.MatchRoot("hostname_pattern"),
					),
				},
			},
			"hostname_pattern": schema.StringAttribute{
				MarkdownDescription: "Get the MORef for all hosts matching this pattern",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.ConflictsWith(
						path.MatchRoot("hostname"),
					),
				},
			},
			"host_details": schema.MapNestedAttribute{
				MarkdownDescription: "Returns the hostnames and their details as a map",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"host_moref": schema.StringAttribute{
							Computed: true,
						},
						"cluster_moref": schema.StringAttribute{
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func (h *EsxiHosts) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	h.fmClient = fmClient
}

func (h *EsxiHosts) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EsxiHostsModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var hostPattern string
	if data.HostName.ValueString() != "" { // Ensure a full match
		hostPattern = fmt.Sprintf("^%s$", data.HostName.ValueString())
	} else if data.HostNamePattern.ValueString() != "" {
		hostPattern = data.HostNamePattern.ValueString()
	} else { // Ensre everything matches
		hostPattern = ".*"
	}

	clusterTFRef := make([]types.String, 0)
	clusterRef := make([]string, 0)

	// Get the list out of the configuration
	resp.Diagnostics.Append(data.ClusterRef.ElementsAs(ctx, &clusterTFRef, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert it to a list of go strings
	for _, val := range clusterTFRef {
		clusterRef = append(clusterRef, val.ValueString())
	}

	fmResp, err := fmesxi.GetHostsRef(
		ctx,
		data.ConnectionId.ValueString(),
		data.DataCenterRef.ValueString(),
		clusterRef,
		hostPattern,
		h.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to get Hosts details",
			fmt.Sprintf(
				"Unable to get Host details for %s:%s, error is: %s",
				data.DataCenterRef.ValueString(),
				hostPattern,
				err,
			),
		)
		return
	}

	nestedHosts := make(map[string]types.Object)

	attrTypes := map[string]attr.Type{
		"host_moref":    types.StringType,
		"cluster_moref": types.StringType,
	}

	for key, val := range fmResp {
		obj, diags := types.ObjectValue(attrTypes, map[string]attr.Value{
			"host_moref":    types.StringValue(val.HostRef),
			"cluster_moref": types.StringValue(val.ClusterRef),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		nestedHosts[key] = obj
	}

	hostsMap, diags := types.MapValueFrom(
		ctx,
		types.ObjectType{AttrTypes: attrTypes},
		nestedHosts,
	)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.HostDetails = hostsMap

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Implementation of the VDS Portgroup DataStore
func (p *EsxiPortGroups) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_esxi_vds_portgroups"
}

func (p *EsxiPortGroups) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "ESXI VDS Portgroups  Data Source Model",

		Attributes: map[string]schema.Attribute{
			"connection_id": schema.StringAttribute{
				MarkdownDescription: "Connection ID to use to fetch the details",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"data_center_moref": schema.StringAttribute{
				MarkdownDescription: "MORef of the Data Center where the portgroup is located",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"portgroup_name": schema.StringAttribute{
				MarkdownDescription: "Name of the VDS portgroup to fetch the Ref for",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"portgroup_moref": schema.StringAttribute{
				MarkdownDescription: "MORef of the requested VDS portgroup",
				Computed:            true,
			},
		},
	}
}

func (p *EsxiPortGroups) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	p.fmClient = fmClient
}

func (p *EsxiPortGroups) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EsxiPortGroupsModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	pgRef, err := fmesxi.GetVDSPortGroupRef(
		ctx,
		data.ConnectionId.ValueString(),
		data.DataCenterRef.ValueString(),
		data.PortGroupName.ValueString(),
		p.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to get Data Center VDS Portgroup MORef",
			fmt.Sprintf(
				"Unable to get VDS Portgroup  MORef for %s:%s, error is: %s",
				data.DataCenterRef.ValueString(),
				data.PortGroupName.ValueString(),
				err,
			),
		)
		return
	}
	data.PortGroupRef = types.StringValue(pgRef)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Implementation of the Network DataStore
func (n *EsxiNetworks) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_esxi_networks"
}

func (n *EsxiNetworks) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "ESXI Networks Data Source Model",

		Attributes: map[string]schema.Attribute{
			"connection_id": schema.StringAttribute{
				MarkdownDescription: "Connection ID to use to fetch the details",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"data_center_moref": schema.StringAttribute{
				MarkdownDescription: "MORef of the Data Center where the Network is located",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"network_name": schema.StringAttribute{
				MarkdownDescription: "Name of the Network to fetch the Ref for",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"network_moref": schema.StringAttribute{
				MarkdownDescription: "MORef of the requested Network",
				Computed:            true,
			},
		},
	}
}

func (n *EsxiNetworks) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	n.fmClient = fmClient
}

func (n *EsxiNetworks) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EsxiNetworksModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	networkRef, err := fmesxi.GetNetworkRef(
		ctx,
		data.ConnectionId.ValueString(),
		data.DataCenterRef.ValueString(),
		data.NetworkName.ValueString(),
		n.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to get Data Center Network MORef",
			fmt.Sprintf(
				"Unable to get Network MORef for %s:%s, error is: %s",
				data.DataCenterRef.ValueString(),
				data.NetworkName.ValueString(),
				err,
			),
		)
		return
	}
	data.NetworkRef = types.StringValue(networkRef)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Implementation of the DatastoreCluster DataStore
func (c *EsxiDataStoreCluster) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_esxi_datastore_cluster"
}

func (c *EsxiDataStoreCluster) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "ESXI Datastore Cluster Data Source Model",

		Attributes: map[string]schema.Attribute{
			"connection_id": schema.StringAttribute{
				MarkdownDescription: "Connection ID to use to fetch the details",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"data_center_moref": schema.StringAttribute{
				MarkdownDescription: "MORef of the Data Center where the DScluster is located",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"datastore_cluster_name": schema.StringAttribute{
				MarkdownDescription: "Name of the Datastore cluster to fetch the Ref for",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"datastore_cluster_moref": schema.StringAttribute{
				MarkdownDescription: "MORef of the requested Datastore Cluster",
				Computed:            true,
			},
		},
	}
}

func (c *EsxiDataStoreCluster) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	c.fmClient = fmClient
}

func (c *EsxiDataStoreCluster) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EsxiDataStoreClusterModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	datastoreClusterRef, err := fmesxi.GetDataStoreClusterRef(
		ctx,
		data.ConnectionId.ValueString(),
		data.DataCenterRef.ValueString(),
		data.DataStoreClusterName.ValueString(),
		c.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to get Data Center Datastore MORef",
			fmt.Sprintf(
				"Unable to get Datastore  Cluster MORef for %s:%s, error is: %s",
				data.DataCenterRef.ValueString(),
				data.DataStoreClusterName.ValueString(),
				err,
			),
		)
		return
	}
	data.DataStoreClusterRef = types.StringValue(datastoreClusterRef)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Implementation of the Datastore DataStore
func (d *EsxiDataStore) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_esxi_datastore"
}

func (d *EsxiDataStore) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "ESXI Datastore Data Source Model",

		Attributes: map[string]schema.Attribute{
			"connection_id": schema.StringAttribute{
				MarkdownDescription: "Connection ID to use to fetch the DataCenter details",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"data_center_moref": schema.StringAttribute{
				MarkdownDescription: "MORef of the Data Center where the cluster is located",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"datastore_name": schema.StringAttribute{
				MarkdownDescription: "Name of the Datastore to fetch the Ref for",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"datastore_moref": schema.StringAttribute{
				MarkdownDescription: "MORef of the requested Datastore",
				Computed:            true,
			},
		},
	}
}

func (d *EsxiDataStore) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.fmClient = fmClient
}

func (d *EsxiDataStore) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EsxiDataStoreModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	datastoreRef, err := fmesxi.GetDataStoreRef(
		ctx,
		data.ConnectionId.ValueString(),
		data.DataCenterRef.ValueString(),
		data.DataStoreName.ValueString(),
		d.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to get Data Center Datastore MORef",
			fmt.Sprintf(
				"Unable to get Datastore  MORef for %s:%s, error is: %s",
				data.DataCenterRef.ValueString(),
				data.DataStoreName.ValueString(),
				err,
			),
		)
		return
	}
	data.DataStoreRef = types.StringValue(datastoreRef)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Implementation of the Cluster DataStore
func (c *EsxiCluster) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_esxi_cluster"
}

func (c *EsxiCluster) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "ESXI Cluster Data Source Model",

		Attributes: map[string]schema.Attribute{
			"connection_id": schema.StringAttribute{
				MarkdownDescription: "Connection ID to use to fetch the DataCenter details",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"data_center_moref": schema.StringAttribute{
				MarkdownDescription: "MORef of the Data Center where the cluster is located",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"cluster_name": schema.StringAttribute{
				MarkdownDescription: "Name of the Cluster to fetch the Ref for",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"cluster_moref": schema.StringAttribute{
				MarkdownDescription: "MORef of the requested cluster",
				Computed:            true,
			},
		},
	}
}

func (c *EsxiCluster) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	c.fmClient = fmClient
}

func (c *EsxiCluster) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EsxiClusterModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	clusterRef, err := fmesxi.GetClusterRef(
		ctx,
		data.ConnectionId.ValueString(),
		data.DataCenterRef.ValueString(),
		data.ClusterName.ValueString(),
		c.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to get Data Center Cluster MORef",
			fmt.Sprintf(
				"Unable to get Clusterr MORef for %s:%s, error is: %s",
				data.DataCenterRef.ValueString(),
				data.ClusterName.ValueString(),
				err,
			),
		)
		return
	}
	data.ClusterRef = types.StringValue(clusterRef)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Implementation for the DataCenter Datastore
func (d *EsxiDataCenter) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_esxi_datacenter"
}

func (d *EsxiDataCenter) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "ESXI Data Center Data Source Model",

		Attributes: map[string]schema.Attribute{
			"connection_id": schema.StringAttribute{
				MarkdownDescription: "Connection ID to use to fetch the DataCenter details",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"data_center_name": schema.StringAttribute{
				MarkdownDescription: "Name of the Data Center to fetch the Ref for",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"data_center_moref": schema.StringAttribute{
				MarkdownDescription: "MORef for the given data center name",
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
		},
	}
}

func (d *EsxiDataCenter) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.fmClient = fmClient
}

func (d *EsxiDataCenter) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EsxiDataCenterModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	dcRef, err := fmesxi.GetDataCenterRef(
		ctx,
		data.ConnectionId.ValueString(),
		data.DataCenterName.ValueString(),
		d.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to get Data Center MORef",
			fmt.Sprintf("Unable to get Data Center MORef for %s, error is: %s", data.DataCenterName.ValueString(), err),
		)
		return
	}
	data.DataCenterRef = types.StringValue(dcRef)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
