// Copyright (c) Gigamon, Inc.

// Implements the APP Resrouces that are common across all environment

package commonresources

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
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

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DedupConfig{}
var _ resource.Resource = &Slicing{}
var _ resource.Resource = &Dedup{}
var _ resource.Resource = &Masking{}

// Dedup Config app resoruce, which manages the dedup configuration
//
//	Dedup configuration is applied globally across all dedup instances in a MD.
func NewDedupConfig() resource.Resource {
	return &DedupConfig{}
}

// Slicing app resoruce, which manages the slicing applications
func NewSlicing() resource.Resource {
	return &Slicing{}
}

// Dedup app resource, which manages the dedup application instances
func NewDedup() resource.Resource {
	return &Dedup{}
}

// Masking app resource, which manages the masking application instances
func NewMasking() resource.Resource {
	return &Masking{}
}

// Dedup config manages the dedup app config on a per MD basis
type DedupConfig struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// Slicing manages the slicing app
type Slicing struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// Dedup manages the dedup app instance on a monitoring session
type Dedup struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// Masking manages the masking app instance on a monitoring session
type Masking struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// DedupConfig App model. The ID provided would be dedupconfig:<monitoring domain id>
type DedupConfigModel struct {
	MonitoringDomainId types.String `tfsdk:"monitoring_domain_id"`
	Action             types.String `tfsdk:"action"`
	Timer              types.Int32  `tfsdk:"timer"`
	IPTClass           types.String `tfsdk:"ipv6_traffic_class"`
	IPTos              types.String `tfsdk:"ipv4_tos_field"`
	TCPSeq             types.String `tfsdk:"tcp_sequence"`
	Vlan               types.String `tfsdk:"vlan"`
	Id                 types.String `tfsdk:"id"`
}

// Slicing App model
type SlicingModel struct {
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`
	Alias               types.String `tfsdk:"alias"`
	Id                  types.String `tfsdk:"id"`
	Protocol            types.String `tfsdk:"protocol"`
	Offset              types.Int32  `tfsdk:"offset"`
}

// Masking App Model
type MaskingModel struct {
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`
	Alias               types.String `tfsdk:"alias"`
	Id                  types.String `tfsdk:"id"`
	Protocol            types.String `tfsdk:"protocol"`
	Offset              types.Int32  `tfsdk:"offset"`
	Length              types.Int32  `tfsdk:"length"`
	Pattern             types.String `tfsdk:"pattern"`
	ContentType         types.String `tfsdk:"content_type"`
}

// Dedup App Model
type DedupModel struct {
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`
	Alias               types.String `tfsdk:"alias"`
	Id                  types.String `tfsdk:"id"`
	Description         types.String `tfsdk:"description"`
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
		Dedup:        *fmData,
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
			fmt.Sprintf("error while setting the dedup parameters: %v", err),
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
			fmt.Sprintf("dedup configuration get failed: %v", err),
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
		Dedup:        *fmData,
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
			fmt.Sprintf("error while updating the dedup parameters: %v", err),
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
		Protocol: data.Protocol.ValueString(),
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

	err := GetMSAppData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		data.Id.ValueString(),
		"slicing",
		"",
		&slicingData,
		s.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		tflog.Info(ctx, "Slicing app data read failed ******", nil)
		if errors.As(err, &fmErr) {
			if fmErr.ErrorCode() == fmclient.ObjectNotFound {
				resp.State.RemoveResource(ctx)
				return
			}
		}
		tflog.Info(ctx, "Not a not found error Slicing app data read failed ******", nil)
		resp.Diagnostics.AddError(
			"Unable to Get Slicing App details",
			fmt.Sprintf("unable to get Slicing App details. error is %v", err),
		)
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
					Id:   data.Id.ValueString(),
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

// Dedup Application TF Hooks
func (d *Dedup) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_dedup"
}

func (d *Dedup) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
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
			"description": schema.StringAttribute{
				MarkdownDescription: "Optional description for this dedup app",
				Optional:            true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this App instance for later use",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Initial Configure call, to initialize the Provider
func (d *Dedup) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	d.fmClient = fmClient
}

// Create a FM DS from the TF DS and return the same
func (d *Dedup) createFMStruct(data *DedupModel) *FMDedup {
	return &FMDedup{
		Alias:       data.Alias.ValueString(),
		Name:        "dedup",
		Description: data.Description.ValueString(),
		Id:          data.Id.ValueString(),
	}
}

// Update the TF Data from the FM struct
func (d *Dedup) updateTFStruct(data *DedupModel, fmData *FMDedup) {
	data.Description = types.StringValue(fmData.Description)
	if fmData.Id != "" {
		data.Id = types.StringValue(fmData.Id)
	}
}

// Create call for new Dedup App Instance
func (d *Dedup) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DedupModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fmData := d.createFMStruct(&data)

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
		d.fmClient,
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
	err = deployIfNeeded(ctx, d.fmClient, data.MonitoringSessionId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to deploy MS",
			fmt.Sprintf("unable to deploy MS. error is %s", err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *Dedup) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DedupModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dedupData := FMDedup{}

	err := GetMSAppData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		data.Id.ValueString(),
		"dedup",
		"",
		&dedupData,
		d.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		tflog.Info(ctx, "**** Dedup app data read failed ******", nil)
		if errors.As(err, &fmErr) {
			if fmErr.ErrorCode() == fmclient.ObjectNotFound {
				resp.State.RemoveResource(ctx)
				return
			}
		}
		tflog.Info(ctx, "*** Not a not found error dedup app data read failed ******", nil)
		resp.Diagnostics.AddError(
			"Unable to Get Dedup App details",
			fmt.Sprintf("unable to get Dedup App details. error is %v", err),
		)
		return
	}
	tflog.Info(ctx, "**** Dedup app data read SUCCESS  ******", nil)

	d.updateTFStruct(&data, &dedupData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *Dedup) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData DedupModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fmData := d.createFMStruct(&planData)

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
		d.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update dedup app",
			fmt.Sprintf("app update failed: %s", err),
		)
		return
	}

	d.updateTFStruct(&stateData, fmData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &stateData)...)
}

func (d *Dedup) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DedupModel

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
					Id:   data.Id.ValueString(),
					Name: "Application",
				},
			},
		},
	}

	_, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		d.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete dedup app",
			fmt.Sprintf("app deletion failed: %s", err),
		)
	}
}

// Masking Application TF Hooks
func (m *Masking) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_masking"
}

func (m *Masking) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Gigamon APP Masking Schema",
		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name for this masking application",
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
				MarkdownDescription: "If specified the offset if calcualted from the end of this protocol header, if none, the offset starts from the first byte f the packet",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("none"),
				Validators: []validator.String{
					stringvalidator.OneOf("none", "ipv4", "ipv6", "udp", "tcp", "ftp-data", "https", "ssh", "gtp", "gtp-ipv4", "gtp-udp", "gtp-tcp", "sip"),
				},
			},
			"offset": schema.Int32Attribute{
				MarkdownDescription: "Offset at which to start masking, relative to the protocol field value",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(64),
			},
			"length": schema.Int32Attribute{
				MarkdownDescription: "Number of bytes to mask from offset. Not valid for protocol sip, but required otherwise",
				Optional:            true,
				Validators: []validator.Int32{
					int32validator.AtLeast(1),
				},
			},
			"pattern": schema.StringAttribute{
				MarkdownDescription: "one byte hex value, which is the pattern to be written. Not valid for sip protocol but required otherwise",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^0[xX][0-9a-fA-F]{2}$`),
						"mut be a hexadecimal 1 byte value e.g. 0x08 or 0xFF",
					),
				},
			},
			"content_type": schema.StringAttribute{
				MarkdownDescription: "For SIP, indicates which packets to mask. Must if protocol is sip",
				Optional:            true,
			},

			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this App instance for later use",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Initial Configure call, to initialize the Provider
func (m *Masking) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	m.fmClient = fmClient
}

// Create a FM DS from the TF DS and return the same
func (m *Masking) createFMStruct(data *MaskingModel) *FMMasking {
	return &FMMasking{
		Alias:       data.Alias.ValueString(),
		Name:        "masking",
		Protocol:    data.Protocol.ValueString(),
		Offset:      data.Offset.ValueInt32(),
		Length:      data.Length.ValueInt32(),
		Pattern:     data.Pattern.ValueString(),
		ContentType: data.ContentType.ValueString(),
		Id:          data.Id.ValueString(),
	}
}

// Update the TF Data from the FM struct
func (m *Masking) updateTFStruct(data *MaskingModel, fmData *FMMasking) {
	data.Protocol = types.StringValue(fmData.Protocol)
	data.Offset = types.Int32Value(fmData.Offset)
	if fmData.Protocol == "sip" {
	    data.ContentType = types.StringValue(fmData.ContentType)
	} else {
	    data.Length = types.Int32Value(fmData.Length)
	    data.Pattern = types.StringValue(fmData.Pattern)
	}
	if fmData.Id != "" {
		data.Id = types.StringValue(fmData.Id)
	}
}

// Validates the input parameters
func (m *Masking) validateParams(data *MaskingModel) error {
	if data.Protocol.ValueString() == "sip" {
		if data.ContentType.IsNull() || data.ContentType.IsUnknown() {
			return fmt.Errorf("for sip protocol, the content_type parameter must be specified")
		}
		if !data.Length.IsNull() || !data.Pattern.IsNull() {
			return fmt.Errorf(
			    "For sip protocol, the fields length and pattern are not allowed",
			)
		}
	} else {
		if data.Length.IsNull() || data.Pattern.IsNull() {
			return fmt.Errorf(
				"for all non sip protocols, the length and pattern field is mandatory",
			)
		}
		if !data.ContentType.IsNull() {
			return fmt.Errorf("for non sip protocols, the content_type field is not valid")
		}
	}
	return nil
}

// Create call for new Masking App Instance
func (m *Masking) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MaskingModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := m.validateParams(&data)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid parameters specified",
			fmt.Sprintf("Invalid parameters for masking app: %s", err),
		)
		return
	}

	fmData := m.createFMStruct(&data)

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
		m.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create masking app",
			fmt.Sprintf("app creation failed: %s", err),
		)
		return
	}

	data.Id = types.StringValue(id)

	// Deploy the MS if it is not already deployed
	err = deployIfNeeded(ctx, m.fmClient, data.MonitoringSessionId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to deploy MS",
			fmt.Sprintf("unable to deploy MS. error is %s", err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (m *Masking) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MaskingModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	maskingData := FMMasking{}

	err := GetMSAppData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		data.Id.ValueString(),
		"masking",
		"",
		&maskingData,
		m.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) {
			if fmErr.ErrorCode() == fmclient.ObjectNotFound {
				resp.State.RemoveResource(ctx)
				return
			}
		}
		resp.Diagnostics.AddError(
			"Unable to Get Masking App details",
			fmt.Sprintf("unable to get Masking App details. error is %v", err),
		)
		return
	}

	m.updateTFStruct(&data, &maskingData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (m *Masking) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData MaskingModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := m.validateParams(&planData)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update masking app",
			fmt.Sprintf("app update failed: %s", err),
		)
		return
	}

	fmData := m.createFMStruct(&planData)

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType:  "application",
				Operation:   "update",
				Application: fmData,
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		planData.MonitoringSessionId.ValueString(),
		m.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update masking app",
			fmt.Sprintf("app update failed: %s", err),
		)
		return
	}

	m.updateTFStruct(&stateData, fmData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &stateData)...)
}

func (m *Masking) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MaskingModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "application",
				Operation:  "delete",
				Application: FMMasking{
					Id:   data.Id.ValueString(),
					Name: "Application",
				},
			},
		},
	}

	_, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		m.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete masking app",
			fmt.Sprintf("app deletion failed: %s", err),
		)
	}
}
