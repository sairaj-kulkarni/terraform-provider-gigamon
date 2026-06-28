//  Copyright (c) 2017-2026 Gigamon, Inc. All rights reserved.
//
//  Author: Gigamon Terraform Team (gigamon-terraform-team@gigamon.com)
//
//  This program is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, version 3 of the License.
//
//  This program is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with this program. If not, see <https://www.gnu.org/licenses/>

// Implements the datasource for the inventory, i.e. gets the VCentter inventory info
// Provides data sources to get the following from FM managed MD / Connections
// Datacenter/Cluster/network/switchs/datastore MORef from their names
// Details of the host including the MORef and other details

package esxidatasources

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/esxiutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &EsxiDataCenter{}
var _ datasource.DataSource = &EsxiCluster{}
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
type EsxiHostDataModel struct {
	HostRef             types.String            `tfsdk:"host_moref"`
	HostName            types.String            `tfsdk:"hostname"`
	DatastoreRef        map[string]types.String `tfsdk:"datastore_moref"`
	DatastoreClusterRef map[string]types.String `tfsdk:"datastore_cluster_moref"`
	NetworkRef          map[string]types.String `tfsdk:"network_moref"`
	DistributedPGRef    map[string]types.String `tfsdk:"distributed_port_group_moref"`
}

type EsxiHostsModel struct {
	ConnectionId    types.String                 `tfsdk:"connection_id"`
	DatacenterRef   types.String                 `tfsdk:"data_center_moref"`
	ClusterRef      types.List                   `tfsdk:"cluster_moref"`
	Hostname        types.List                   `tfsdk:"hostname"`
	HostnamePattern types.String                 `tfsdk:"hostname_pattern"`
	HostDetails     map[string]EsxiHostDataModel `tfsdk:"host_details"`
}

func EsxiHostDataSchema() schema.MapNestedAttribute {
	return schema.MapNestedAttribute{
		MarkdownDescription: "Details of the specified hosts",
		Computed:            true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"host_moref": schema.StringAttribute{
					MarkdownDescription: "MORef of this host",
					Computed:            true,
				},
				"hostname": schema.StringAttribute{
					MarkdownDescription: "name of this host",
					Computed:            true,
				},
				"datastore_moref": schema.MapAttribute{
					MarkdownDescription: "Map of Datastore names to their moref",
					ElementType:         types.StringType,
					Computed:            true,
				},
				"datastore_cluster_moref": schema.MapAttribute{
					MarkdownDescription: "Map of Datastore Cluster names to their moref",
					ElementType:         types.StringType,
					Computed:            true,
				},
				"network_moref": schema.MapAttribute{
					MarkdownDescription: "Map of Network names to their moref",
					ElementType:         types.StringType,
					Computed:            true,
				},
				"distributed_port_group_moref": schema.MapAttribute{
					MarkdownDescription: "Map of Distributed PG names to their moref",
					ElementType:         types.StringType,
					Computed:            true,
				},
			},
		},
	}
}

// Implementation of the hosts DataStore

// Copy from the TF model the input data to the GO model
func (h *EsxiHosts) ConvertTFtoGO(
	ctx context.Context,
	data *EsxiHostsModel,
	goStruct *esxiutils.GoHosts,
) diag.Diagnostics {
	goStruct.ConnectionId = data.ConnectionId.ValueString()
	goStruct.DatacenterRef = data.DatacenterRef.ValueString()
	goStruct.HostnamePattern = data.HostnamePattern.ValueString()
	diags := data.ClusterRef.ElementsAs(ctx, &goStruct.ClusterRef, false)
	diags.Append(data.Hostname.ElementsAs(ctx, &goStruct.Hostname, false)...)
	return diags
}

// Convert from GOlang model to TF model
func (h *EsxiHosts) ConvertGOtoTF(
	ctx context.Context,
	data *EsxiHostsModel,
	goStruct *esxiutils.GoHosts,
) {
	r := strings.NewReplacer(".", "-", " ", "-")
	data.HostDetails = make(map[string]EsxiHostDataModel)
	for hostname, hostdata := range goStruct.HostDetails {
		tfHost := EsxiHostDataModel{
			HostRef:             types.StringValue(hostdata.HostRef),
			HostName:            types.StringValue(hostname),
			DatastoreRef:        make(map[string]types.String),
			NetworkRef:          make(map[string]types.String),
			DatastoreClusterRef: make(map[string]types.String),
			DistributedPGRef:    make(map[string]types.String),
		}
		data.HostDetails[r.Replace(hostname)] = tfHost
		for name, ref := range hostdata.DatastoreRef {
			tfHost.DatastoreRef[r.Replace(name)] = types.StringValue(ref)
		}
		for name, ref := range hostdata.DatastoreClusterRef {
			tfHost.DatastoreClusterRef[r.Replace(name)] = types.StringValue(ref)
		}
		for name, ref := range hostdata.NetworkRef {
			tfHost.NetworkRef[r.Replace(name)] = types.StringValue(ref)
		}
		for name, ref := range hostdata.DistributedPGRef {
			tfHost.DistributedPGRef[r.Replace(name)] = types.StringValue(ref)
		}
	}
}

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
			"hostname": schema.ListAttribute{
				MarkdownDescription: "hostname for which we want the MORef",
				Optional:            true,
				ElementType:         types.StringType,
				Validators: []validator.List{
					listvalidator.All(
						listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)),
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
					stringvalidator.AtLeastOneOf(path.Expressions{
						path.MatchRelative().AtParent().AtName("hostname"),
					}...),
				},
			},
			"host_details": EsxiHostDataSchema(),
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

	// Convert the TF request to a Golang struct
	fmData := esxiutils.GoHosts{}
	resp.Diagnostics.Append(h.ConvertTFtoGO(ctx, &data, &fmData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call the FM functions to gather the required data
	err := esxiutils.GetHostDetails(ctx, &fmData, h.fmClient)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to get Hosts Data",
			fmt.Sprintf(
				"Unable to get Hosts Data. error is: %s",
				err,
			),
		)
		return
	}

	// Convert from FM GO format to TF model format
	h.ConvertGOtoTF(ctx, &data, &fmData)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Implementation of the Cluster DataStore

// EsxiCluster Get the Cluster ref given the name
type EsxiCluster struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// Datacenter Datastore structs
func NewEsxiCluster() datasource.DataSource {
	return &EsxiCluster{}
}

// EsxiClusterModel Defines the model for the cluster datasource

type EsxiClusterModel struct {
	ConnectionId  types.String `tfsdk:"connection_id"`
	DataCenterRef types.String `tfsdk:"data_center_moref"`
	ClusterName   types.String `tfsdk:"cluster_name"`
	ClusterRef    types.String `tfsdk:"cluster_moref"`
}

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

	clusterRef, err := esxiutils.GetClusterRef(
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

	dcRef, err := esxiutils.GetDataCenterRef(
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
