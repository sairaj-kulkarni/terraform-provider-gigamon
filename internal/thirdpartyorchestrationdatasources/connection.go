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

// Implements the Data Sources for the Third Party Orchestration Connection
// Data Source is used when Monitoring Domain and Connection is created outside Terraform
// Terraform needs to read the Connection details to proceed with Monitoring Session

package thirdpartyorchestrationdatasources

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ datasource.DataSource = &ThirdPartyOrchestrationConnectionDataSource{}
var _ datasource.DataSourceWithConfigure = &ThirdPartyOrchestrationConnectionDataSource{}

// NewThirdPartyConnectionDataSource creates the Third Party Orchestration, Connection data source
func NewThirdPartyOrchestrationConnectionDataSource() datasource.DataSource {
	return &ThirdPartyOrchestrationConnectionDataSource{}
}

// ThirdPartyOrchestrationConnectionDataSource reads an existing Third Party Orchestration, Connection created outside Terraform
// (e.g., UI or other automation) and exposes it as a data source
type ThirdPartyOrchestrationConnectionDataSource struct {
	fmClient *fmclient.FmClient
}

// ThirdPartyOrchestrationConnectionDSModel describes the data source model
// Inputs: alias, monitoring_domain_id. Everything else is computed
type ThirdPartyOrchestrationConnectionDSModel struct {
	Alias              types.String `tfsdk:"alias"`
	MonitoringDomainId types.String `tfsdk:"monitoring_domain_id"`
	TappingMethod      types.String `tfsdk:"tapping_method"`
	Id                 types.String `tfsdk:"id"`
	Status             types.String `tfsdk:"status"`
}

// FM response for Connection alias
type ThirdPartyOrchestrationFmConnection struct {
	MonitoringDomainId string `json:"monitoringDomainId"`
	TappingMethod      string `json:"tappingMethod"`
	Alias              string `json:"alias"`
	Id                 string `json:"id,omitempty"`
	Status             string `json:"status,omitempty"`
}

func (ds *ThirdPartyOrchestrationConnectionDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_third_party_orchestration_connection"
}

func (ds *ThirdPartyOrchestrationConnectionDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read-only Third Party Orchestration Connection lookup scoped by Monitoring Domain ID (TypedID) (wiring enforced in provider).",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Connection alias to look up",
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
				MarkdownDescription: "TypedID of the Monitoring Domain this connection must belong to (monitoringDomain::thirdPartyOrchestration::<uuid>).",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},

			// Computed outputs
			"id": schema.StringAttribute{
				MarkdownDescription: "TypedID of the associated Connection (connection::thirdPartyOrchestration::<uuid>).",
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

func (ds *ThirdPartyOrchestrationConnectionDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// getConnectionByAliasAndMD fetches Third Party Orchestration Connection by alias
func (ds *ThirdPartyOrchestrationConnectionDataSource) getConnectionByAliasAndMD(ctx context.Context, alias string, mdTypedID string) (*ThirdPartyOrchestrationFmConnection, error) {

	// Convert MD TypedID -> raw UUID (FM uses raw UUID)
	mdId, err := commonutils.UUIDFromTypedID(mdTypedID)
	if err != nil {
		return nil, fmt.Errorf("invalid monitoring_domain_id %q (expected TypedID): %w", mdTypedID, err)
	}

	fmConnectionData := struct {
		ThirdPartyOrchestrationFmConnections []ThirdPartyOrchestrationFmConnection `json:"anyCloudConnections"`
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
		return nil, fmt.Errorf("Get request of Third Party Orchestration Connection: %s, failed with error %w", alias, err)
	}

	err = json.Unmarshal(connResp, &fmConnectionData)
	if err != nil {
		return nil, fmt.Errorf("Unable to convert connResp to struct: %s error is: %w", string(connResp), err)
	}

	// Match by alias + monitoringDomainId
	for i := range fmConnectionData.ThirdPartyOrchestrationFmConnections {
		c := &fmConnectionData.ThirdPartyOrchestrationFmConnections[i]
		if c.Alias == alias && c.MonitoringDomainId == mdId {
			return c, nil
		}
	}

	return nil, fmclient.NewFMError(fmclient.ObjectNotFound, fmt.Sprintf("Unable to find Third Party Orchestration Connection by alias=%q for monitoring_domain_id=%q", alias, mdTypedID), nil)
}

func (ds *ThirdPartyOrchestrationConnectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ThirdPartyOrchestrationConnectionDSModel

	if ds.fmClient == nil {
		resp.Diagnostics.AddError("Provider not configured", "FM client is nil.")
		return
	}

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	alias := strings.TrimSpace(data.Alias.ValueString())
	mdTypedID := data.MonitoringDomainId.ValueString()

	connDetails, err := ds.getConnectionByAliasAndMD(ctx, alias, mdTypedID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read Third Party Orchestration Connection",
			fmt.Sprintf("lookup by alias=%q failed: %v", alias, err),
		)
		return
	}

	connTypedID, err := commonutils.MakeTypedID(commonutils.ModuleConnection, commonutils.TypeThirdPartyOrchestration, connDetails.Id)
	if err != nil {
		resp.Diagnostics.AddError("Unable to build typed Connection ID", err.Error())
		return
	}

	data.Alias = types.StringValue(alias)
	data.Id = types.StringValue(connTypedID)
	data.TappingMethod = types.StringValue(connDetails.TappingMethod)
	data.Status = types.StringValue(connDetails.Status)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}
