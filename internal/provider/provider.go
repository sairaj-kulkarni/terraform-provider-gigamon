// Copyright (c) Gigamon, Inc.

// Provider package for Gigamon FM product. Implements the provider for cloud functioanlities

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/commonactions"
	"terraform-provider-gigamon/internal/commonresources"
	"terraform-provider-gigamon/internal/esxidatasources"
	"terraform-provider-gigamon/internal/esxiresources"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure GigamonProvider satisfies various provider interfaces.
var _ provider.ProviderWithActions = &GigamonProvider{}

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
				MarkdownDescription: "api token generated from FM for use in token based authentication",
				Required:            true,
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

	fmClient, err := fmclient.NewFmClient(
		ctx,
		data.ApiToken.ValueString(),
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
		commonresources.NewMonSess,
		commonresources.NewDedupConfig,
		commonresources.NewSlicing,
		commonresources.NewMasking,
		commonresources.NewDedup,
		commonresources.NewTrafficMap,
		commonresources.NewLink,
	}
}

func (p *GigamonProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return nil
}

func (p *GigamonProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		esxidatasources.NewEsxiDataCenter,
		esxidatasources.NewEsxiCluster,
		esxidatasources.NewEsxiDataStore,
		esxidatasources.NewEsxiDataStoreCluster,
		esxidatasources.NewEsxiNetworks,
		esxidatasources.NewEsxiPortGroups,
		esxidatasources.NewEsxiHosts,
	}
}

func (p *GigamonProvider) Functions(ctx context.Context) []func() function.Function {
	return nil
}

func (p *GigamonProvider) Actions(_ctx context.Context) []func() action.Action {
	return []func() action.Action{
		commonactions.NewPosition,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &GigamonProvider{
			version: version,
		}
	}
}
