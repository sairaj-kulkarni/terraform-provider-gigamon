// Copyright (c) Gigamon, Inc.

// Implements the Data Sources for the (Third Party Orchestration) Any Cloud Connection
// Data Source is used when Monitoring Domain and Connection is created outside Terraform
// Terraform needs to read the Connection details to proceed with Monitoring Session

package anyclouddatasources

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ datasource.DataSource = &AnyCloudConnectionDataSource{}
var _ datasource.DataSourceWithConfigure = &AnyCloudConnectionDataSource{}

// NewAnyCloudConnectionDataSource creates the AnyCloud Connection data source
func NewAnyCloudConnectionDataSource() datasource.DataSource {
	return &AnyCloudConnectionDataSource{}
}

// AnyCloudConnectionDataSource reads an existing AnyCloud Connection created outside Terraform
// (e.g., UI or other automation) and exposes it as a data source
type AnyCloudConnectionDataSource struct {
	fmClient *fmclient.FmClient
}

// AnyCloudConnectionDSModel describes the data source model
// Inputs: alias, monitoring_domain_id. Everything else is computed
type AnyCloudConnectionDSModel struct {
	Alias              types.String `tfsdk:"alias"`
	MonitoringDomainId types.String `tfsdk:"monitoring_domain_id"`
	TappingMethod      types.String `tfsdk:"tapping_method"`
	Id                 types.String `tfsdk:"id"`
	Status             types.String `tfsdk:"status"`
}

// FM response for Connection alias
type AnyCloudFmConnection struct {
	MonitoringDomainId string `json:"monitoringDomainId"`
	TappingMethod      string `json:"tappingMethod"`
	Alias              string `json:"alias"`
	Id                 string `json:"id,omitempty"`
	Status             string `json:"status,omitempty"`
}

func (ds *AnyCloudConnectionDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_anycloud_connection"
}

func (ds *AnyCloudConnectionDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read-only AnyCloud Connection lookup scoped by Monitoring Domain ID (TypedID) (wiring enforced in provider).",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Connection alias to lookup",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[A-Za-z0-9_-]+$`),
						`Invalid characters (Only alphanumeric, "-" and "_" are allowed)`,
					),
				},
			},

			// Wiring input (TypedID)
			"monitoring_domain_id": schema.StringAttribute{
				MarkdownDescription: "TypedID of the Monitoring Domain this connection must belong to (monitoringDomain::anyCloud::<uuid>).",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},

			// Computed outputs
			"id": schema.StringAttribute{
				MarkdownDescription: "TypedID of the associated Connection (connection::anyCloud::<uuid>).",
				Computed:            true,
			},
			"tapping_method": schema.StringAttribute{
				MarkdownDescription: "Tapping method reported by FM.",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Connectivity status of this connection",
				Computed:            true,
			},
		},
	}
}

func (ds *AnyCloudConnectionDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	fmClient, ok := req.ProviderData.(*fmclient.FmClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *fmclient.FmClient, got: %T", req.ProviderData),
		)
		return
	}
	ds.fmClient = fmClient
}

// getConnectionByAliasAndMD fetches AnyCloud Connection by alias
func (ds *AnyCloudConnectionDataSource) getConnectionByAliasAndMD(ctx context.Context, alias string, mdTypedID string) (*AnyCloudFmConnection, error) {

	// Convert MD TypedID -> raw UUID (FM uses raw UUID)
	mdId, err := commonutils.UUIDFromTypedID(mdTypedID)
	if err != nil {
		return nil, fmt.Errorf("invalid monitoring_domain_id (expected TypedID): %w", err)
	}

	fmConnectionData := struct {
		AnyCloudFmConnections []AnyCloudFmConnection `json:"anyCloudConnections"`
	}{}

	connResp, err := ds.fmClient.DoRequest(
		ctx,
		"GET",
		"api/v1.3/cloud/anyCloud/connections",
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("Get request of AnyCloud Connection: %s, failed with error %w", alias, err)
	}

	err = json.Unmarshal(connResp, &fmConnectionData)
	if err != nil {
		return nil, fmt.Errorf("Unable to convert connResp to struct: %s error is: %w", string(connResp), err)
	}

	// Match by alias + monitoringDomainId
	for i := range fmConnectionData.AnyCloudFmConnections {
		c := &fmConnectionData.AnyCloudFmConnections[i]
		if c.Alias == alias && c.MonitoringDomainId == mdId {
			return c, nil
		}
	}

	return nil, fmt.Errorf("Unable to find %s in FM Response %s and JSON Struct %v for AnyCloud Connection", alias, string(connResp), fmConnectionData)
}

func (ds *AnyCloudConnectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AnyCloudConnectionDSModel

	if ds.fmClient == nil {
		resp.Diagnostics.AddError("Provider not configured", "FM client is nil.")
		return
	}

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	alias := data.Alias.ValueString()
	mdTypedID := data.MonitoringDomainId.ValueString()

	connDetails, err := ds.getConnectionByAliasAndMD(ctx, alias, mdTypedID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read AnyCloud Connection",
			fmt.Sprintf("lookup by alias=%q failed: %v", alias, err),
		)
		return
	}

	connTypedID, err := commonutils.MakeTypedID(commonutils.ModuleConnection, commonutils.TypeAnyCloud, connDetails.Id)
	if err != nil {
		resp.Diagnostics.AddError("Unable to build typed Connection ID", err.Error())
		return
	}

	data.Id = types.StringValue(connTypedID)
	data.TappingMethod = types.StringValue(connDetails.TappingMethod)
	data.Status = types.StringValue(connDetails.Status)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}
