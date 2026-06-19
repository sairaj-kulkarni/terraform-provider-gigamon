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

// Implements the Data Source for reading Cloud CA Certificate details from FM

package securetunnelcertsdatasources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ datasource.DataSource = &CloudCaCertDataSource{}
var _ datasource.DataSourceWithConfigure = &CloudCaCertDataSource{}

func NewCloudCaCertDataSource() datasource.DataSource {
	return &CloudCaCertDataSource{}
}

type CloudCaCertDataSource struct {
	fmClient *fmclient.FmClient
}

// CloudCaCertDSModel describes the data source model
type CloudCaCertDSModel struct {
	Alias        types.String `tfsdk:"alias"`
	Certificates types.List   `tfsdk:"certificates"`
}

// FM response from GET API
type fmCaCertResponse struct {
	Alias        string           `json:"alias"`
	Certificates []fmCertMetadata `json:"certificates"`
}

type fmCertMetadata struct {
	DateNotAfter  string `json:"dateNotAfter"`
	DateNotBefore string `json:"dateNotBefore"`
	Algorithm     string `json:"algo"`
	Version       int64  `json:"version"`
	Issuer        string `json:"issuer"`
	Name          string `json:"name"`
}

var caCertMetadataType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"date_not_after":  types.StringType,
		"date_not_before": types.StringType,
		"algorithm":       types.StringType,
		"version":         types.Int64Type,
		"issuer":          types.StringType,
		"name":            types.StringType,
	},
}

func (d *CloudCaCertDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_ca_cert"
}

func (d *CloudCaCertDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source to read Cloud CA Certificate details from FM.",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Alias of the Cloud CA certificate entry in FM",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"certificates": schema.ListNestedAttribute{
				MarkdownDescription: "Certificate metadata from FM",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"date_not_after": schema.StringAttribute{
							Computed: true,
						},
						"date_not_before": schema.StringAttribute{
							Computed: true,
						},
						"algorithm": schema.StringAttribute{
							Computed: true,
						},
						"version": schema.Int64Attribute{
							Computed: true,
						},
						"issuer": schema.StringAttribute{
							Computed: true,
						},
						"name": schema.StringAttribute{
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func (d *CloudCaCertDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.fmClient = fmClient
}

func (d *CloudCaCertDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data CloudCaCertDSModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	alias := strings.TrimSpace(data.Alias.ValueString())
	if alias == "" {
		resp.Diagnostics.AddError("Missing alias", "alias must not be empty")
		return
	}

	var fmData fmCaCertResponse

	respBytes, err := d.fmClient.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/nodes/caCert/%s", alias),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read Cloud CA Certificate",
			fmt.Sprintf("Alias %q error: %v", alias, err),
		)
		return
	}

	if err := json.Unmarshal(respBytes, &fmData); err != nil {
		resp.Diagnostics.AddError(
			"Unable to parse Cloud CA Certificate response",
			fmt.Sprintf("Alias %q error: %v", alias, err),
		)
		return
	}

	data.Alias = types.StringValue(fmData.Alias)

	elems := make([]attr.Value, 0, len(fmData.Certificates))
	for _, cert := range fmData.Certificates {
		obj, diags := types.ObjectValue(
			caCertMetadataType.AttrTypes,
			map[string]attr.Value{
				"date_not_after":  types.StringValue(cert.DateNotAfter),
				"date_not_before": types.StringValue(cert.DateNotBefore),
				"algorithm":       types.StringValue(cert.Algorithm),
				"version":         types.Int64Value(cert.Version),
				"issuer":          types.StringValue(cert.Issuer),
				"name":            types.StringValue(cert.Name),
			},
		)
		if diags.HasError() {
			resp.Diagnostics.AddError("Failed to build certificate metadata", "")
			return
		}
		elems = append(elems, obj)
	}

	listVal, diags := types.ListValue(caCertMetadataType, elems)
	if diags.HasError() {
		resp.Diagnostics.AddError("Failed to build certificates list", "")
		return
	}
	data.Certificates = listVal

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
