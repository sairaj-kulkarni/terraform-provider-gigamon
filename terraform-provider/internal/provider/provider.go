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

// Provider package for Gigamon FM product. Implements the provider for cloud functioanlities

package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/commondatasources"
	"terraform-provider-gigamon/internal/commonresources"
	"terraform-provider-gigamon/internal/esxidatasources"
	"terraform-provider-gigamon/internal/esxiresources"
	"terraform-provider-gigamon/internal/fmclient"
	"terraform-provider-gigamon/internal/securetunnelcertsdatasources"
	"terraform-provider-gigamon/internal/securetunnelcertsresources"
	"terraform-provider-gigamon/internal/thirdpartyorchestrationdatasources"
	"terraform-provider-gigamon/internal/thirdpartyorchestrationresources"
)

// Ensure GigamonProvider satisfies various provider interfaces.
var _ provider.Provider = &GigamonProvider{}

// GigamonProvider is the implementation of Gigamon Provider
type GigamonProvider struct {
	fmClient *fmclient.FmClient // Handle to the FM Client HTTP handler instance
	// Will be set in the main from the release build process
	version string
}

// GigamonProviderModel describes the provider data model.
// User can provide either the UserName/Password if using basic authenticatino or
// ApiToken is using the api_token to authenticate/authorize to FM
// Either one of these are required, and if both are provided we use the ApiToken
type GigamonProviderModel struct {
	FmAddress  types.String `tfsdk:"fm_address"`
	ApiToken   types.String `tfsdk:"api_token"`
	SkipVerify types.Bool   `tfsdk:"skip_verify"`
}

func (p *GigamonProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "gigamon"
	resp.Version = p.version
}

func (p *GigamonProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"fm_address": schema.StringAttribute{
				MarkdownDescription: "FM address, either the IP numerical address or DNS name",
				Required:            true,
			},
			"api_token": schema.StringAttribute{
				MarkdownDescription: "api token generated from FM for use in token based authentication. If FM_API_TOKEN environment variable is set, it takes precedence",
				Optional:            true,
				Sensitive:           true,
			},
			"skip_verify": schema.BoolAttribute{
				MarkdownDescription: "Skip FM certificate valdiation, default false",
				Optional:            true,
			},
		},
	}
}

func (p *GigamonProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data GigamonProviderModel

	// Extract our config from configrequest and update TF with any errors
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	apiToken := os.Getenv("FM_API_TOKEN")
	if apiToken == "" {
		if data.ApiToken.IsUnknown() {
			resp.Diagnostics.AddError(
				"Missing API Token",
				"The provider api_token is unknown. Set a concrete api_token in provider configuration or set FM_API_TOKEN environment variable.",
			)
			return
		}

		apiToken = data.ApiToken.ValueString()
	}

	if apiToken == "" {
		resp.Diagnostics.AddError(
			"Missing API Token",
			"Set api_token in provider configuration or set FM_API_TOKEN environment variable.",
		)
		return
	}

	fmClient, err := fmclient.NewFmClient(
		ctx,
		apiToken,
		data.FmAddress.ValueString(),
		data.SkipVerify.ValueBool(),
		p.version,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"FM Connection/Valdiation failed",
			fmt.Sprintf("Error when connecting to FM: %v", err),
		)
		return
	}
	resp.DataSourceData = fmClient
	resp.ResourceData = fmClient
	resp.ActionData = fmClient
	resp.ListResourceData = fmClient
	resp.EphemeralResourceData = fmClient

	p.fmClient = fmClient
}

func (p *GigamonProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		esxiresources.NewEsxiImage,
		esxiresources.NewEsxiMD,
		esxiresources.NewEsxiConnection,
		esxiresources.NewEsxiFabric,
		esxiresources.NewEsxiVmSelection,

		// Third Party Orchestration
		thirdpartyorchestrationresources.NewThirdPartyOrchestrationMD,
		thirdpartyorchestrationresources.NewThirdPartyOrchestrationConnection,

		// Secure Tunnels Certs
		securetunnelcertsresources.NewSecureTunnelCertsApply,
		securetunnelcertsresources.NewCloudCaCert,
		securetunnelcertsresources.NewCloudSSLKeys,

		// Common Resources
		commonresources.NewMonSess,
		commonresources.NewDedupConfig,
		commonresources.NewSlicing,
		commonresources.NewMasking,
		commonresources.NewDedup,
		commonresources.NewHeaderStripping,
		commonresources.NewLoadBalancing,
		commonresources.NewAmx,
		commonresources.NewTrafficMap,
		commonresources.NewInclusionMap,
		commonresources.NewExclusionMap,
		commonresources.NewLink,
		commonresources.NewTunnelIn,
		commonresources.NewTunnelOut,
		commonresources.NewRawEndpoint,
		commonresources.NewEndpointIfaceMapping,
	}
}

func (p *GigamonProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return nil
}

func (p *GigamonProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		esxidatasources.NewEsxiDataCenter,
		esxidatasources.NewEsxiCluster,
		esxidatasources.NewEsxiHosts,

		commondatasources.NewVSeriesInterfaces,

		// Third Party Orchestration
		thirdpartyorchestrationdatasources.NewThirdPartyOrchestrationMDDataSource,
		thirdpartyorchestrationdatasources.NewThirdPartyOrchestrationConnectionDataSource,

		// Secure Tunnel Certificates
		securetunnelcertsdatasources.NewCloudCaCertDataSource,
		securetunnelcertsdatasources.NewCloudSSLKeysDataSource,
	}
}

func (p *GigamonProvider) Functions(ctx context.Context) []func() function.Function {
	return nil
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &GigamonProvider{
			version: version,
		}
	}
}
