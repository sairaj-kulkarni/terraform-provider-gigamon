// Copyright (c) Gigamon, Inc.

// Implements the datasource for the inventory, i.e. gets the VCentter inventory info
// Provides data sources to get the following from FM managed MD / Connections
// Datacenter/Cluster/network/switchs/datastore MORef from their names
// Details of the host including the MORef and other details


package esxidatasources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"gigamon.com/terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &EsxiDataCenter{}

func NewEsxiDataCenter() datasource.DataSource {
	return &EsxiDataCenter{}
}

// EsxiDataCenter Get the Datacenter ref given the name
type EsxiDataCenter struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// EsxiDataCenterModel Defines the model for the data center datasource

type EsxiInventoryModel struct {
	ConnectionId types.String `tfsdk:"connection_id"`
	DataCenterName types.String `tfsdk:"data_center_name"`
	Id                    types.String `tfsdk:"id"`
}

func (i *EsxiInventory) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_esxi_inventory"
}

type FmDataStoreResp struct {
	Name string `json:"name"`
	Ref string `json:"ref"`
	DataCenterName string `json:"datacenterName"`
	DataCenterRef string `json:"datacenterRef"`
	DataStoreClusterName string `json:"datastoreClusterName"`
	DataStoreClusterRef string `json:"datastoreClusterRef"`
}

type FmNetworksResp struct {
	Name string `json:"name"`
	Ref string `json:"ref"`
	DataCenterName string `json:"datacenterName"`
	DataCenterRef string `json:"datacenterRef"`
}

type FmDistributedSwitchResp struct {
	Name string `json:"name"`
	Ref string `json:"ref"`
	DataCenterName string `json:"datacenterName"`
	DataCenterRef string `json:"datacenterRef"`
}

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
func (i *EsxiInventory) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "ESXI Inventory Data Source Model",

		Attributes: map[string]schema.Attribute{
			"connection_id": schema.StringAttribute{
				MarkdownDescription: "ID of the connection for which we want the inventory",
				Required:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Identifier for this data source instance",
				Computed:            true,
			},
		},
	}
}

func (i *EsxiInventory) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	i.fmClient = fmClient
}

func (i *EsxiInventory) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EsxiInventoryModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	data.Id = types.StringValue("example-id")

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "read a data source")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
