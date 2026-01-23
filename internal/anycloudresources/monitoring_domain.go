// Copyright (c) Gigamon, Inc.

// Implements the Resources for the (Third Party Orchestratation) Any Cloud Monitoring Domain

package anycloudresources

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AnyCloudMD{}
var _ resource.ResourceWithImportState = &AnyCloudMD{}

// AnyCloud MD resoruce, which manages the images for AnyCloud, (Third Party Orchestratation)
func NewAnyCloudMD() resource.Resource {
	return &AnyCloudMD{}
}

// AnyCloudMD manages the MD for AnyCloud
type AnyCloudMD struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// AnyCloudMDModel describes the resource data model.
type AnyCloudMDModel struct {
	Alias                types.String `tfsdk:"alias"`
	Platform             types.String `tfsdk:"platform"`
	UserLaunched         types.Bool   `tfsdk:"user_launched"`
	DualStackPreferIPv6  types.Bool   `tfsdk:"dual_stack_prefer_ipv6"`
	UniformTrafficPolicy types.Bool   `tfsdk:"uniform_traffic_policy"`
	MTU                  types.Int32  `tfsdk:"mtu"`
	ConnectionId         types.String `tfsdk:"connection_id"`
	Id                   types.String `tfsdk:"id"`
}

type AnyCloudMDConn struct {
	Id    string `json:"id,omitempty"`
	Alias string `json:"alias,omitempty"`
}

// FM request/response for Monitoring Domains
type AnyCloudFmMD struct {
	Alias                string           `json:"alias,omitempty"`
	Platform             string           `json:"platform,omitempty"`
	UserLaunched         bool             `json:"userLaunched,omitempty"`
	DualStackPreferIPv6  bool             `json:"dualStackPreferIPv6,omitempty"`
	UniformTrafficPolicy bool             `json:"uniformTrafficPolicy,omitempty"`
	MTU                  int32            `json:"mtu,omitempty"`
	ConnectionIds        []string         `json:"connIds,omitempty"`     // Used when we post/patch request
	GetConnectionIds     []AnyCloudMDConn `json:"connections,omitempty"` // Use in the Get only
	Id                   string           `json:"id,omitempty"`
}

func (md *AnyCloudMD) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_anycloud_monitoring_domain"
}

func (md *AnyCloudMD) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon Third Party Orchestratation (Any Cloud) Monitoring Domain",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name of the monitoring domain",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[A-Za-z0-9_-]+$`),
						`Invalid characters (Only alphanumeric, "-" and "_" are allowed) monitoring domain name`,
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"platform": schema.StringAttribute{
				MarkdownDescription: "Platform on which the monitoring domain has been created",
				Computed:            true,
			},
			"user_launched": schema.BoolAttribute{
				MarkdownDescription: "If true, indicates that the vseries nodes are launched and managed by the user. Default true",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"dual_stack_prefer_ipv6": schema.BoolAttribute{
				MarkdownDescription: "If true, indicates IPv6 tunnels are preferred between UCT‑V and V Series nodes. Default false",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"uniform_traffic_policy": schema.BoolAttribute{
				MarkdownDescription: "If true, indicates same monitoring session configuration is applied to all V Series Nodes in the monitoring domain. Default false",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"mtu": schema.Int32Attribute{
				MarkdownDescription: "MTU between UCT‑V and V Series nodes, when Traffic Acquisiotn method is UCT-V. Default value is 1450",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(1450),
				Validators: []validator.Int32{
					int32validator.Between(1280, 9000),
				},
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.RequiresReplace(),
				},
			},
			"connection_id": schema.StringAttribute{
				MarkdownDescription: "Connection ID associated with this MD",
				Computed:            true,
				Optional:            true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this Monitoring Domain for later use",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (md *AnyCloudMD) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	md.fmClient = fmClient
}

func (md *AnyCloudMD) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
}

func (md *AnyCloudMD) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
}

func (md *AnyCloudMD) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

func (md *AnyCloudMD) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func (md *AnyCloudMD) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
}
