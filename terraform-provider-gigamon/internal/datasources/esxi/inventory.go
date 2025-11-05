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
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"

	"gigamon.com/terraform-provider-gigamon/internal/fmclient"
	"gigamon.com/terraform-provider-gigamon/internal/utils/fmesxi"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &EsxiDataCenter{}
var _ datasource.DataSource = &EsxiCluster{}
var _ datasource.DataSource = &EsxiDataStore{}

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
	ConnectionId types.String `tfsdk:"connection_id"`
	DataCenterRef types.String `tfsdk:"data_center_moref"`
	DataStoreName  types.String `tfsdk:"datastore_name"`
	DataStoreRef types.String `tfsdk:"datastore_moref"`
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
	ConnectionId types.String `tfsdk:"connection_id"`
	DataCenterRef types.String `tfsdk:"data_center_moref"`
	ClusterName types.String `tfsdk:"cluster_name"`
	ClusterRef types.String `tfsdk:"cluster_moref"`
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
	ConnectionId types.String `tfsdk:"connection_id"`
	DataCenterName types.String `tfsdk:"data_center_name"`
	DataCenterRef types.String `tfsdk:"data_center_moref"`
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
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"datastore_name": schema.StringAttribute{
				MarkdownDescription: "Name of the Datastore to fetch the Ref for",
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"datastore_moref": schema.StringAttribute{
				MarkdownDescription: "MORef of the requested Datastore",
				Computed: true,
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
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"cluster_name": schema.StringAttribute{
				MarkdownDescription: "Name of the Cluster to fetch the Ref for",
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"cluster_moref": schema.StringAttribute{
				MarkdownDescription: "MORef of the requested cluster",
				Computed: true,
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
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"data_center_moref": schema.StringAttribute{
				MarkdownDescription: "MORef for the given data center name",
				Computed: true,
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
