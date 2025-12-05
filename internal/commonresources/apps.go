// Copyright (c) Gigamon, Inc.

// Implements the APP Resrouces that are common across all environment

package commonresources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
var _ resource.Resource = &Dedup{}
var _ resource.Resource = &Slicing{}

// Dedup app resoruce, which manages the dedup applications
func NewDedup() resource.Resource {
	return &Dedup{}
}

// Slicing app resoruce, which manages the slicing applications
func NewSlicing() resource.Resource {
	return &Slicing{}
}

// Dedup manages the dedup app
type Dedup struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// Slicing manages the slicing app
type Slicing struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// Dedup App model
type DedupModel struct {
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`
	Alias               types.String `tfsdk:"alias"`
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

// Dedup Application TF Hooks
func (de *Dedup) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_dedup"
}

func (de *Dedup) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon APP Dedup Schema",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name for this dedup application",
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
func (de *Dedup) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	de.fmClient = fmClient
}

// Create a FM DS from the TF DS and return the same
func (de *Dedup) createFMStruct(data *DedupModel) *FMDedup {
	return &FMDedup{
		Alias:    data.Alias.ValueString(),
		Name:     "dedup",
		Action:   data.Action.ValueString(),
		Timer:    data.Timer.ValueInt32(),
		IPTClass: data.IPTClass.ValueString(),
		IPTos:    data.IPTos.ValueString(),
		Vlan:     data.Vlan.ValueString(),
		TCPSeq:   data.TCPSeq.ValueString(),
		Id:       data.Id.ValueString(),
	}
}

// Update the TF Data from the FM struct
func (de *Dedup) updateTFStruct(data *DedupModel, fmData *FMDedup) {
	data.Action = types.StringValue(fmData.Action)
	data.IPTClass = types.StringValue(fmData.IPTClass)
	data.IPTos = types.StringValue(fmData.IPTos)
	data.Vlan = types.StringValue(fmData.Vlan)
	data.TCPSeq = types.StringValue(fmData.TCPSeq)
	data.Timer = types.Int32Value(fmData.Timer)
	if fmData.Id != "" {
		data.Id = types.StringValue(fmData.Id)
	}
}

// Create call for new monitoring session
func (de *Dedup) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DedupModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Copy the TF Types over to regular GO types and get the content body
	fmData := de.createFMStruct(&data)

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
		de.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create dedup app",
			fmt.Sprintf("app creation failed: %s", err),
		)
		return
	}

	data.Id = types.StringValue(id)

	// Deploy the MS if it is not already deployed
	err = deployIfNeeded(ctx, de.fmClient, data.MonitoringSessionId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to deploy MS",
			fmt.Sprintf("unable to deploy MS. error is %s", err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (de *Dedup) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DedupModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dedupData := FMDedup{}

	ok, err := GetMSAppData(
		ctx,
	    data.MonitoringSessionId.ValueString(),
		data.Id.ValueString(),
		"dedup",
		"",
		&dedupData,
		de.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Get App details",
			fmt.Sprintf("unable to get App details. error is %s", err),
		)
		return
	}
	if !ok {
		resp.State.RemoveResource(ctx)
		return
	}

	// Save updated data into Terraform state
	de.populateDedupData(&dedupData, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (de *Dedup) populateDedupData(dedupData *FMDedup, data *DedupModel) {
	if dedupData.Action == "" {
		dedupData.Action = "drop"
	}
	if dedupData.IPTClass == "" {
		dedupData.IPTClass = "include"
	}
	if dedupData.IPTos == "" {
		dedupData.IPTos = "include"
	}
	if dedupData.TCPSeq == "" {
		dedupData.TCPSeq = "include"
	}
	if dedupData.Vlan == "" {
		dedupData.Vlan = "ignore"
	}
	if dedupData.Timer == 0 {
		dedupData.Timer = 50000
	}
	data.Action = types.StringValue(dedupData.Action)
	data.IPTClass = types.StringValue(dedupData.IPTClass)
	data.IPTos = types.StringValue(dedupData.IPTos)
	data.TCPSeq = types.StringValue(dedupData.TCPSeq)
	data.Vlan = types.StringValue(dedupData.Vlan)
	data.Timer = types.Int32Value(dedupData.Timer)
}


func (de *Dedup) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Dedup APP does not support any modifications",
		"Dedup App can only be created/deleted. They cannot be modified",
	)
}

func (de *Dedup) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DedupModel

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
				Application: FMDedup{
					Id: data.Id.ValueString(),
				},
			},
		},
	}

	_, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		de.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete dedup app",
			fmt.Sprintf("app deeltion failed: %s", err),
		)
	}
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
			"Unable to create slicing app",
			fmt.Sprintf("app creation failed: %s", err),
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
