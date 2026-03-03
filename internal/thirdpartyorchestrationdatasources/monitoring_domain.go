// Copyright (c) Gigamon, Inc.

// Implements the Data Sources for the Third Party Orchestration, Monitoring Domain
// Data Source is used when Monitoring Domain is created outside Terraform
// Terraform needs to read the Monitoring Domain details to proceed with Monitoring Session

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
var _ datasource.DataSource = &ThirdPartyOrchestrationMDDataSource{}
var _ datasource.DataSourceWithConfigure = &ThirdPartyOrchestrationMDDataSource{}

// NewThirdPartyOrchestrationMDDataSource creates the Third Party Orchestration, Monitoring Domain data source
func NewThirdPartyOrchestrationMDDataSource() datasource.DataSource {
	return &ThirdPartyOrchestrationMDDataSource{}
}

// ThirdPartyOrchestrationMDDataSource reads an existing Third Party Orchestration Monitoring Domain created outside Terraform
// (e.g., UI or other automation) and exposes it as a data source
type ThirdPartyOrchestrationMDDataSource struct {
	fmClient *fmclient.FmClient
}

// ThirdPartyOrchestrationMDDataSourceModel describes the data source model
// alias is the only input. Everything else is computed
type ThirdPartyOrchestrationMDDataSourceModel struct {
	Alias                types.String `tfsdk:"alias"`
	Platform             types.String `tfsdk:"platform"`
	UserLaunched         types.Bool   `tfsdk:"user_launched"`
	DualStackPreferIPv6  types.Bool   `tfsdk:"dual_stack_prefer_ipv6"`
	UniformTrafficPolicy types.Bool   `tfsdk:"uniform_traffic_policy"`
	MTU                  types.Int32  `tfsdk:"mtu"`
	ConnectionId         types.String `tfsdk:"connection_id"`
	Id                   types.String `tfsdk:"id"`
}

// FM response for Monitoring Domain Alias
type ThirdPartyOrchestrationMDConn struct {
	Id    string `json:"id,omitempty"`
	Alias string `json:"alias,omitempty"`
}

type ThirdPartyOrchestrationFmMD struct {
	Alias                string                          `json:"alias,omitempty"`
	Id                   string                          `json:"id,omitempty"`
	Platform             string                          `json:"platform,omitempty"`
	UserLaunched         bool                            `json:"userLaunched,omitempty"`
	DualStackPreferIPv6  bool                            `json:"dualStackPreferIPv6"`
	UniformTrafficPolicy bool                            `json:"uniformTrafficPolicy,omitempty"`
	MTU                  int32                           `json:"mtu"`
	GetConnectionIds     []ThirdPartyOrchestrationMDConn `json:"connections,omitempty"`
}

func (ds *ThirdPartyOrchestrationMDDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_third_party_orchestration_monitoring_domain"
}

func (ds *ThirdPartyOrchestrationMDDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Read-only data source for an existing Gigamon Third Party Orchestration Monitoring Domain (created via UI or other automation).",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Monitoring Domain alias to look up.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[A-Za-z0-9_-]+$`),
						`Invalid characters (Only alphanumeric, "-" and "_" are allowed) monitoring domain name`,
					),
				},
			},

			// Computed outputs
			"id": schema.StringAttribute{
				MarkdownDescription: "TypedID of this Monitoring Domain (monitoringDomain::thirdPartyOrchestration::<uuid>)",
				Computed:            true,
			},
			"platform": schema.StringAttribute{
				MarkdownDescription: "Platform on which the monitoring domain exists",
				Computed:            true,
			},
			"user_launched": schema.BoolAttribute{
				MarkdownDescription: "True if VSeries nodes are launched/managed by user.",
				Computed:            true,
			},
			"dual_stack_prefer_ipv6": schema.BoolAttribute{
				MarkdownDescription: "True if IPv6 tunnels are preferred between UCT‑V and VSeries nodes.",
				Computed:            true,
			},
			"uniform_traffic_policy": schema.BoolAttribute{
				MarkdownDescription: "True if same monitoring session config applies to all VSeries nodes in the MD.",
				Computed:            true,
			},
			"mtu": schema.Int32Attribute{
				MarkdownDescription: "MTU between UCT‑V and VSeries nodes.",
				Computed:            true,
			},
			"connection_id": schema.StringAttribute{
				MarkdownDescription: "TypedID of the associated Connection (connection::thirdPartyOrchestration::<uuid>)",
				Computed:            true,
			},
		},
	}
}

func (ds *ThirdPartyOrchestrationMDDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	fmClient, ok := req.ProviderData.(*fmclient.FmClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *fmclient.FmClient, got: %T. Report the issue to Gigamon", req.ProviderData),
		)
		return
	}

	ds.fmClient = fmClient
}

// getMDByAlias fetches Third Party Orchestration, Monitoring Domain by alias
func (ds *ThirdPartyOrchestrationMDDataSource) getMDByAlias(ctx context.Context, alias string) (*ThirdPartyOrchestrationFmMD, error) {
	fmMDData := struct {
		MonitoringDomains []ThirdPartyOrchestrationFmMD `json:"monitoringDomains"`
	}{}

	mdResp, err := ds.fmClient.DoRequest(
		ctx,
		"GET",
		"api/v1.3/cloud/monitoringDomains",
		map[string]string{"platform": "anyCloud"},
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("Get request of Third Party Orchestration Monitoring Domain: %s, failed with error %w", alias, err)
	}

	if err := json.Unmarshal(mdResp, &fmMDData); err != nil {
		return nil, fmt.Errorf(
			"unable to convert MD Get resp to struct: %s error: %w",
			string(mdResp),
			err,
		)
	}

	for i := range fmMDData.MonitoringDomains {
		if fmMDData.MonitoringDomains[i].Alias == alias {
			return &fmMDData.MonitoringDomains[i], nil
		}
	}

	return nil, fmclient.NewFMError(fmclient.ObjectNotFound, fmt.Sprintf("Unable to find Third Party Orchestration Monitoring Domain by alias: %s", alias), nil)
}

func (ds *ThirdPartyOrchestrationMDDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ThirdPartyOrchestrationMDDataSourceModel

	if ds.fmClient == nil {
		resp.Diagnostics.AddError("Provider not configured", "FM client is nil.")
		return
	}

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	alias := strings.TrimSpace(data.Alias.ValueString())
	mdDetails, err := ds.getMDByAlias(ctx, alias)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read Third Party Orchestration Monitoring Domain",
			fmt.Sprintf("look up by alias=%q failed: %v", alias, err),
		)
		return
	}

	// Decorate MD ID into TF TypedID
	mdTypedID, err := commonutils.MakeTypedID(commonutils.ModuleMonitoringDomain, commonutils.TypeThirdPartyOrchestration, mdDetails.Id)
	if err != nil {
		resp.Diagnostics.AddError("Unable to build typed MD ID", err.Error())
		return
	}
	data.Id = types.StringValue(mdTypedID)

	// Copy fields
	data.Platform = types.StringValue(mdDetails.Platform)
	data.UserLaunched = types.BoolValue(mdDetails.UserLaunched)
	data.DualStackPreferIPv6 = types.BoolValue(mdDetails.DualStackPreferIPv6)
	data.UniformTrafficPolicy = types.BoolValue(mdDetails.UniformTrafficPolicy)
	data.MTU = types.Int32Value(mdDetails.MTU)

	// Connection (typed) if present, else null
	if len(mdDetails.GetConnectionIds) > 0 && mdDetails.GetConnectionIds[0].Id != "" {
		connTypedID, err := commonutils.MakeTypedID(commonutils.ModuleConnection, commonutils.TypeThirdPartyOrchestration, mdDetails.GetConnectionIds[0].Id)
		if err != nil {
			resp.Diagnostics.AddError("Unable to build typed Connection ID", err.Error())
			return
		}
		data.ConnectionId = types.StringValue(connTypedID)
	} else {
		data.ConnectionId = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
