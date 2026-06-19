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

// Implements the Data Source for reading Cloud SSL Keys details from FM

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
var _ datasource.DataSource = &CloudSSLKeysDataSource{}
var _ datasource.DataSourceWithConfigure = &CloudSSLKeysDataSource{}

func NewCloudSSLKeysDataSource() datasource.DataSource {
	return &CloudSSLKeysDataSource{}
}

type CloudSSLKeysDataSource struct {
	fmClient *fmclient.FmClient
}

// CloudSSLKeysDSModel describes the data source model
type CloudSSLKeysDSModel struct {
	Alias         types.String `tfsdk:"alias"`
	KeyStoreAlias types.String `tfsdk:"key_store_alias"`
	Metadata      types.Object `tfsdk:"metadata"`
}

// FM response from GET API
type fmSSLKeyResponse struct {
	CloudSSLKey fmSSLKey `json:"cloudSslKey"`
}

type fmSSLKey struct {
	Alias         string `json:"alias"`
	KeyStoreAlias string `json:"keyStoreAlias"`
	InstalledOn   string `json:"installedOn"`
	Certificate   bool   `json:"certificate"`
	PrivateKey    bool   `json:"privateKey"`
	CommonName    string `json:"cn"`
	Organization  string `json:"o"`
	Expiry        string `json:"expiry"`
	KeyType       string `json:"keyType"`
}

var sslKeyMetadataType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"common_name":  types.StringType,
		"organization": types.StringType,
		"expiry":       types.StringType,
		"key_type":     types.StringType,
		"certificate":  types.BoolType,
		"private_key":  types.BoolType,
		"installed_on": types.StringType,
	},
}

func (d *CloudSSLKeysDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_ssl_keys"
}

func (d *CloudSSLKeysDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source to read Cloud SSL Keys (private key + certificate) details from FM.",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Alias of the Cloud SSL Keys entry in FM",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"key_store_alias": schema.StringAttribute{
				MarkdownDescription: "Key store alias",
				Computed:            true,
			},
			"metadata": schema.SingleNestedAttribute{
				MarkdownDescription: "SSL key metadata from FM",
				Computed:            true,
				Attributes: map[string]schema.Attribute{
					"common_name": schema.StringAttribute{
						Computed: true,
					},
					"organization": schema.StringAttribute{
						Computed: true,
					},
					"expiry": schema.StringAttribute{
						Computed: true,
					},
					"key_type": schema.StringAttribute{
						Computed: true,
					},
					"certificate": schema.BoolAttribute{
						Computed: true,
					},
					"private_key": schema.BoolAttribute{
						Computed: true,
					},
					"installed_on": schema.StringAttribute{
						Computed: true,
					},
				},
			},
		},
	}
}

func (d *CloudSSLKeysDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *CloudSSLKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data CloudSSLKeysDSModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	alias := strings.TrimSpace(data.Alias.ValueString())
	if alias == "" {
		resp.Diagnostics.AddError("Missing alias", "alias must not be empty")
		return
	}

	var fmResp fmSSLKeyResponse

	respBytes, err := d.fmClient.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/ssl/keys/%s", alias),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read Cloud SSL Keys",
			fmt.Sprintf("Alias %q error: %v", alias, err),
		)
		return
	}

	if err := json.Unmarshal(respBytes, &fmResp); err != nil {
		resp.Diagnostics.AddError(
			"Unable to parse Cloud SSL Keys response",
			fmt.Sprintf("Alias %q error: %v", alias, err),
		)
		return
	}

	meta := fmResp.CloudSSLKey

	data.Alias = types.StringValue(meta.Alias)
	data.KeyStoreAlias = types.StringValue(meta.KeyStoreAlias)

	metadata, diags := types.ObjectValue(
		sslKeyMetadataType.AttrTypes,
		map[string]attr.Value{
			"common_name":  types.StringValue(meta.CommonName),
			"organization": types.StringValue(meta.Organization),
			"expiry":       types.StringValue(meta.Expiry),
			"key_type":     types.StringValue(meta.KeyType),
			"certificate":  types.BoolValue(meta.Certificate),
			"private_key":  types.BoolValue(meta.PrivateKey),
			"installed_on": types.StringValue(meta.InstalledOn),
		},
	)
	if diags.HasError() {
		resp.Diagnostics.AddError("Failed to build SSL keys metadata", "")
		return
	}
	data.Metadata = metadata

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
