// Copyright (c) Gigamon, Inc.

// Provider package for Gigamon FM product. Implements the provider for cloud functioanlities

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure GigamonProvider satisfies various provider interfaces.
var _ provider.Provider = &GigamonProvider{}

// GigamonProvider is the implementation of Gigamon Provider
type GigamonProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// GigamonProviderModel describes the provider data model.
// User can provide either the UserName/Password if using basic authenticatino or
// ApiToken is using the api_token to authenticate/authorize to FM
// Either one of these are required, and if both are provided we use the ApiToken
type GigamonProviderModel struct {
	FmAddress types.String `tfsdk:"fm_address"`
	UserName  types.String `tfsdk:"user_name"`
	Password  types.String `tfsdk:"password"`
	ApiToken  types.String `tfsdk:"api_token"`
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
			"user_name": schema.StringAttribute{
				MarkdownDescription: "User Name when using basic authentication",
				Optional:            true,
				Description:         "Specify user_name and password if using basic_auth as the mechanism for authentication to FM",
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "password of the user when using basic authentication",
				Optional:            true,
				Sensitive:           true,
			},
			"api_token": schema.StringAttribute{
				MarkdownDescription: "api token generated from FM for use in token based authentication",
				Optional:            true,
				Sensitive:           true,
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
	if (data.UserName.IsNull() || data.Password.IsNull()) && data.ApiToken.IsNull() {
		resp.Diagnostics.AddError("One of api_token or (user_name and passowrd) must be specified",
			fmt.Sprintf("user_name: %s password: %s api_token: %s",
				data.UserName.ValueString(),
				data.Password.ValueString(),
				data.ApiToken.ValueString()))
		return
	}
}

func (p *GigamonProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewGigamonResource,
	}
}

func (p *GigamonProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return nil
}

func (p *GigamonProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewGigamonDataSource,
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
