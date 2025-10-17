// Provider package for Gigamon FM product. Implements the provider for cloud functioanlities

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
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
type GigamonProviderModel struct {
}

func (p *GigamonProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "gigamon"
	resp.Version = p.version
}

func (p *GigamonProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	return
}

func (p *GigamonProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	return
}

func (p *GigamonProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource {
		NewGigamonResource,
	}
}

func (p *GigamonProvider) EphemeralResources(ctx context.Context) []func() ephemeral.EphemeralResource {
	return nil
}

func (p *GigamonProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource {
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
