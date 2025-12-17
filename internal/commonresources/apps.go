// Copyright (c) Gigamon, Inc.

// Implements the APP Resrouces that are common across all environment

package commonresources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DedupConfig{}
var _ resource.Resource = &Slicing{}

// Dedup Config app resoruce, which manages the dedup configuration
//  Dedup configuration is applied globally across all dedup instances in a MD.
func NewDedupConfig() resource.Resource {
	return &DedupConfig{}
}

// Slicing app resoruce, which manages the slicing applications
func NewSlicing() resource.Resource {
	return &Slicing{}
}

// Dedup config manages the dedup app config on a per MD basis
type DedupConfig struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// Slicing manages the slicing app
type Slicing struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// DedupConfig App model. The ID provided would be dedupconfig:<monitoring domain id>
type DedupConfigModel struct {
	MonitoringDomainId types.String `tfsdk:"monitoring_domain_id"`
	Action              types.String `tfsdk:"action"`
	Timer               types.Int32  `tfsdk:"timer"`
	IPTClass            types.String `tfsdk:"ipv6_traffic_class"`
	IPTos               types.String `tfsdk:"ipv4_tos_field"`
	TCPSeq              types.String `tfsdk:"tcp_sequence"`
	Vlan                types.String `tfsdk:"vlan"`
	Id                  types.String `tfsdk:"id"`
}

// Slicing App model
type SlicingModel struct {
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`
	Alias               types.String `tfsdk:"alias"`
	Id                  types.String `tfsdk:"id"`
	Protocol types.String `tfsdk:"protocol"`
	Offset types.Int32 `tfsdk:"offset"`
}

// Dedup Config Application TF Hooks
func (decfg *DedupConfig) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dedup_md_config"
}

func (decfg *DedupConfig) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon Dedup Config Schema",

		Attributes: map[string]schema.Attribute{
			"monitoring_domain_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring domain ID for this dedup config",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"action": schema.StringAttribute{
				MarkdownDescription: "Action to take on the duplicate packets",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("drop"),
				Validators: []validator.String{
					stringvalidator.OneOf("drop", "count"),
				},
			},
			"timer": schema.Int32Attribute{
				MarkdownDescription: "Time to wait for duplicates in micro seconds",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(50000),
				Validators: []validator.Int32{
					int32validator.AtLeast(10),
					int32validator.AtMost(500000),
				},
			},
			"ipv6_traffic_class": schema.StringAttribute{
				MarkdownDescription: "include or ignore the IPv6 Traffic Class filed",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("include"),
				Validators: []validator.String{
					stringvalidator.OneOf("include", "ignore"),
				},
			},
			"ipv4_tos_field": schema.StringAttribute{
				MarkdownDescription: "include or ignore the IPv4 TOS/DSCP field",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("include"),
				Validators: []validator.String{
					stringvalidator.OneOf("include", "ignore"),
				},
			},
			"tcp_sequence": schema.StringAttribute{
				MarkdownDescription: "include or ignore the TCP Sequence Number field",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("include"),
				Validators: []validator.String{
					stringvalidator.OneOf("include", "ignore"),
				},
			},
			"vlan": schema.StringAttribute{
				MarkdownDescription: "include or ignore the VLAN ID field in l2 header",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("ignore"),
				Validators: []validator.String{
					stringvalidator.OneOf("include", "ignore"),
				},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this Monitoring Session for later use",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Initial Configure call, to initialize the Provider
func (decfg *DedupConfig) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	decfg.fmClient = fmClient
}

// Create a FM DS from the TF DS and return the same
func (decfg *DedupConfig) createFMStruct(data *DedupConfigModel) *FMDedupConfig {
	return &FMDedupConfig{
		Action:   data.Action.ValueString(),
		Timer:    data.Timer.ValueInt32(),
		IPTClass: data.IPTClass.ValueString(),
		IPTos:    data.IPTos.ValueString(),
		Vlan:     data.Vlan.ValueString(),
		TCPSeq:   data.TCPSeq.ValueString(),
	}
}

// Update the TF Data from the FM struct
func (decfg *DedupConfig) updateTFStruct(data *DedupConfigModel, fmData *FMDedupConfig) {
	data.Action = types.StringValue(fmData.Action)
	data.IPTClass = types.StringValue(fmData.IPTClass)
	data.IPTos = types.StringValue(fmData.IPTos)
	data.Vlan = types.StringValue(fmData.Vlan)
	data.TCPSeq = types.StringValue(fmData.TCPSeq)
	data.Timer = types.Int32Value(fmData.Timer)
}

// Create call for new Dedup Config Object
// This is a MD single instance in FM, and there is no need to create as it is already present
// just do a PUT to update the values as desired by the user, and return our ID for this
func (decfg *DedupConfig) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DedupConfigModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Copy the TF Types over to regular GO types
	fmData := decfg.createFMStruct(&data)
	gsData := GsParams{
		GsParamsName: "gsParams",
		Dedup: *fmData,
	}

	err := SetGsParams(
		ctx,
	    data.MonitoringDomainId.ValueString(),
		&gsData,
		decfg.fmClient,
	)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Set the Dedup parameters",
			fmt.Sprintf("error while setting the dedup parameters: %s", err),
		)
		return
	}

	data.Id = types.StringValue(fmt.Sprintf("dedup:%s", data.MonitoringDomainId.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (decfg *DedupConfig) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DedupConfigModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	gsParams, err := GetGsParams(
		ctx,
		data.MonitoringDomainId.ValueString(),
		decfg.fmClient,
	)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Get  dedup parameters",
			fmt.Sprintf("dedup configuration get failed: %s", err),
		)
		return
	}

	// Save updated data into Terraform state
	decfg.updateTFStruct(&data, &gsParams.Dedup)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (decfg *DedupConfig) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {

	var planData, stateData DedupConfigModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Copy the TF Types over to regular GO types and get the content body
	fmData := decfg.createFMStruct(&planData)

	gsData := GsParams{
		GsParamsName: "gsParams",
		Dedup: *fmData,
	}

	err := SetGsParams(
		ctx,
	    planData.MonitoringDomainId.ValueString(),
		&gsData,
		decfg.fmClient,
	)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Update the Dedup parameters",
			fmt.Sprintf("error while updating the dedup parameters: %s", err),
		)
		return
	}

	decfg.updateTFStruct(&stateData, fmData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &stateData)...)
}

func (decfg *DedupConfig) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Nothing to do in delete, as this is a permanent singleton object in FM
}

// Slicing Application TF Hooks
func (s *Slicing) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_slicing"
}

func (s *Slicing) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon APP Slicing Schema",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name for this slicing application",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"monitoring_session_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring session ID on which to deploy this APP",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"protocol": schema.StringAttribute{
				MarkdownDescription: "Protocol to check and skip before applying the offset",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("none"),
				Validators: []validator.String{
					stringvalidator.OneOf("none", "ipv4", "ipv6", "udp", "tcp", "ftp-data", "https", "ssh", "gtp", "gtp-ipv4", "gtp-udp", "gtp-tcp"),
				},
			},
			"offset": schema.Int32Attribute{
				MarkdownDescription: "Offset at which to slice.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(64),
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this Monitoring Session for later use",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Initial Configure call, to initialize the Provider
func (s *Slicing) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	s.fmClient = fmClient
}

// Create a FM DS from the TF DS and return the same
func (s *Slicing) createFMStruct(data *SlicingModel) *FMSlicing {
	return &FMSlicing{
		Alias:    data.Alias.ValueString(),
		Name:     "slicing",
		Protocol:   data.Protocol.ValueString(),
		Offset:   data.Offset.ValueInt32(),
		Id:       data.Id.ValueString(),
	}
}

// Update the TF Data from the FM struct
func (s *Slicing) updateTFStruct(data *SlicingModel, fmData *FMSlicing) {
	data.Protocol = types.StringValue(fmData.Protocol)
	data.Offset = types.Int32Value(fmData.Offset)
	if fmData.Id != "" {
		data.Id = types.StringValue(fmData.Id)
	}
}

// Create call for new Slicing App Instance
func (s *Slicing) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SlicingModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Copy the TF Types over to regular GO types and get the content body
	fmData := s.createFMStruct(&data)

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType:  "application",
				Operation:   "create",
				Application: fmData,
			},
		},
	}

	id, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		s.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create slicing app",
			fmt.Sprintf("app creation failed: %s", err),
		)
		return
	}

	data.Id = types.StringValue(id)

	// Deploy the MS if it is not already deployed
	err = deployIfNeeded(ctx, s.fmClient, data.MonitoringSessionId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to deploy MS",
			fmt.Sprintf("unable to deploy MS. error is %s", err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (s *Slicing) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SlicingModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	slicingData := FMSlicing{}

	ok, err := GetMSAppData(
		ctx,
	    data.MonitoringSessionId.ValueString(),
		data.Id.ValueString(),
		"slicing",
		"",
		&slicingData,
		s.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Get Slicing App details",
			fmt.Sprintf("unable to get Slicing App details. error is %s", err),
		)
		return
	}
	if !ok {
		resp.State.RemoveResource(ctx)
		return
	}

	// Save updated data into Terraform state
	s.updateTFStruct(&data, &slicingData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (s *Slicing) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData SlicingModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Copy the TF Types over to regular GO types and get the content body
	fmData := s.createFMStruct(&planData)

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType:  "application",
				Operation:   "update",
				Application: fmData,
			},
		},
	}

	_, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		planData.MonitoringSessionId.ValueString(),
		s.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update slicing app",
			fmt.Sprintf("app update failed: %s", err),
		)
		return
	}
	s.updateTFStruct(&stateData, fmData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &stateData)...)
}

func (s *Slicing) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SlicingModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "application",
				Operation:  "delete",
				Application: FMSlicing{
					Id: data.Id.ValueString(),
					Name: "Application",
				},
			},
		},
	}

	_, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		s.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete slicing app",
			fmt.Sprintf("app deeltion failed: %s", err),
		)
	}
}
