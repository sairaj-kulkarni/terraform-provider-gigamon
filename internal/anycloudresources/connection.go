// Copyright (c) Gigamon, Inc.

// Implements the Resources for (Third Party Orchestratation) AnyCloud Cloud Connection

package anycloudresources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AnyCloudConnection{}
var _ resource.ResourceWithImportState = &AnyCloudConnection{}

// AnyCloud Connection resoruce, which manages the images for the Third Party Orchestratation platform
func NewAnyCloudConnection() resource.Resource {
	return &AnyCloudConnection{}
}

// AnyCloud Connetion manages the connection for the Third Party Orchestratation platform
type AnyCloudConnection struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// AnyCloudConnectionModel describes the resource data model.
type AnyCloudConnectionModel struct {
	MonitoringDomainId types.String `tfsdk:"monitoring_domain_id"`
	TappingMethod      types.String `tfsdk:"tapping_method"`
	Alias              types.String `tfsdk:"alias"`
	Id                 types.String `tfsdk:"id"`
	Status             types.String `tfsdk:"status"`
}

// FM response for Connection API
type AnyCloudFmConnection struct {
	MonitoringDomainId string `json:"monitoringDomainId"`
	TappingMethod      string `json:"tappingMethod"`
	Alias              string `json:"alias"`
	Id                 string `json:"id,omitempty"`
	Status             string `json:"status,omitempty"`
}

func (c *AnyCloudConnection) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_anycloud_connection"
}

func (c *AnyCloudConnection) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon Third Party Orchestratation (Any Cloud) Connection",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name of the Connection",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"monitoring_domain_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring Domain ID to attach this connection to",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"tapping_method": schema.StringAttribute{
				MarkdownDescription: "Type of tapping method to use",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("uctv"),
				Validators: []validator.String{
					stringvalidator.OneOf([]string{"uctv", "none"}...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Connectivity status of this connection",
				Computed:            true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this Connection for later use",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (c *AnyCloudConnection) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (c *AnyCloudConnection) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
}

func (c *AnyCloudConnection) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
}

func (c *AnyCloudConnection) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

func (c *AnyCloudConnection) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func (c *AnyCloudConnection) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
}
