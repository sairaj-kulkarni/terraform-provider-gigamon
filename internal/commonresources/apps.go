// Copyright (c) Gigamon, Inc.

// Implements the APP Resrouces that are common across all environment

package commonresources

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
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
var _ resource.Resource = &HeaderStripping{}

// Dedup Config app resoruce, which manages the dedup configuration
//
// Dedup configuration is applied globally across all dedup instances in a MD.
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

// Header Stripping app resource, which manages header stripping instances
func NewHeaderStripping() resource.Resource {
	return &HeaderStripping{}
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

// HeaderStripping manages the header stripping app instance on a monitoring session
type HeaderStripping struct {
	fmClient *fmclient.FmClient
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

// Per‑protocol TF config for Header Stripping (nested blocks)
type HeaderStrippingVxlanConfig struct {
	VxlanId types.Int32 `tfsdk:"vxlan_id"`
}

type HeaderStrippingVlanConfig struct {
	VlanHeader types.String `tfsdk:"vlan_header"`
}

type HeaderStrippingFm6000TsConfig struct {
	TimestampFormat types.String `tfsdk:"timestamp_format"`
}

type HeaderStrippingErspanConfig struct {
	FlowId types.Int32 `tfsdk:"flow_id"`
}

type HeaderStrippingGenericConfig struct {
	Ah1              types.String `tfsdk:"ah1"`
	Offset           types.String `tfsdk:"offset"`
	OffsetRangeValue types.Int32  `tfsdk:"offset_range_value"`
	HeaderCount      types.Int32  `tfsdk:"header_count"`
	CustomLen        types.Int32  `tfsdk:"custom_len"`
	Ah2              types.String `tfsdk:"ah2"`
}

// Header Stripping App Model (top‑level TF resource model)
type HeaderStrippingModel struct {
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`
	Alias               types.String `tfsdk:"alias"`
	Id                  types.String `tfsdk:"id"`

	// Computed from which block is set, like tunnel "type"
	Protocol types.String `tfsdk:"protocol"`

	// Blocks with parameters
	Vxlan    *HeaderStrippingVxlanConfig    `tfsdk:"vxlan"`
	Vlan     *HeaderStrippingVlanConfig     `tfsdk:"vlan"`
	Fm6000Ts *HeaderStrippingFm6000TsConfig `tfsdk:"fm6000_ts"`
	Erspan   *HeaderStrippingErspanConfig   `tfsdk:"erspan"`
	Generic  *HeaderStrippingGenericConfig  `tfsdk:"generic"`

	// Simple protocols (no per‑protocol attributes, just presence)
	Gtp          *HeaderStrippingEmptyConfig `tfsdk:"gtp"`
	Isl          *HeaderStrippingEmptyConfig `tfsdk:"isl"`
	Mpls         *HeaderStrippingEmptyConfig `tfsdk:"mpls"`
	MplsPlusVlan *HeaderStrippingEmptyConfig `tfsdk:"mpls_plus_vlan"`
	Vntag        *HeaderStrippingEmptyConfig `tfsdk:"vntag"`
	Geneve       *HeaderStrippingEmptyConfig `tfsdk:"geneve"`
}

// Per‑protocol sub‑structs for Header Stripping (FM payload)
type FMHeaderStrippingVxlan struct {
	VxlanId int32 `json:"vxlanId"` // 0 is valid, so no ,omitempty
}

type FMHeaderStrippingVlan struct {
	VlanHeader string `json:"vlanHeader"` // "outer"/"all"
}

type FMHeaderStrippingFm6000Ts struct {
	TimestampFormat string `json:"timestampFormat"`
}

type FMHeaderStrippingErspan struct {
	ErspanFlowId int32 `json:"erspanFlowId"` // required when erspan block exists
}

type FMHeaderStrippingGeneric struct {
	Ah1              string `json:"ah1"`                        // "none"/"eth"/...
	Offset           string `json:"offset"`                     // "start"/"end"/"offsetRange"
	OffsetRangeValue *int32 `json:"offsetRangeValue,omitempty"` // 0 is valid when offsetRange
	HeaderCount      int32  `json:"headerCount,omitempty"`
	CustomLen        int32  `json:"customLen,omitempty"`
	Ah2              string `json:"ah2"` // "none"/"eth"/...
}

// FM payload struct for Header Stripping (used in /monitoringSessions/{id}/update)
type FMHeaderStripping struct {
	Alias    string `json:"alias,omitempty"`
	Name     string `json:"name,omitempty"`
	Protocol string `json:"protocol,omitempty"`

	Vxlan    *FMHeaderStrippingVxlan    `json:"vxlan,omitempty"`
	Vlan     *FMHeaderStrippingVlan     `json:"vlan,omitempty"`
	Fm6000Ts *FMHeaderStrippingFm6000Ts `json:"fm6000Ts,omitempty"`
	Erspan   *FMHeaderStrippingErspan   `json:"erspan,omitempty"`
	Generic  *FMHeaderStrippingGeneric  `json:"generic,omitempty"`

	Id string `json:"id,omitempty"`
}

type HeaderStrippingEmptyConfig struct{}

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

	typedID, err := commonutils.MakeTypedID(commonutils.ModuleApp, commonutils.TypeDedup, data.MonitoringDomainId.ValueString())
	if err != nil {
		return
	}
	data.Id = types.StringValue(typedID)

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
			"Unable to Get dedup parameters",
			fmt.Sprintf("dedup configuration get failed: %v", err),
		)
		return
	}

	// Save updated data into Terraform state
	decfg.updateTFStruct(&data, &gsParams.Dedup)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (decfg *DedupConfig) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData DedupConfigModel

	// Read desired config from the plan
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build FM payload from the plan
	fmData := decfg.createFMStruct(&planData)

	gsData := GsParams{
		GsParamsName: "gsParams",
		Dedup:        *fmData,
	}

	// Send plan to FM
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

	decfg.updateTFStruct(&planData, fmData)

	// Final state = plan (normalized via updateTFStruct)
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
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
	var planData SlicingModel

	// Read desired values from the plan
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
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

	// Let FM override computed/FM-owned fields (protocol, offset, id)
	s.updateTFStruct(&planData, fmData)

	// Final state = plan + FM-owned overrides
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
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

	if fmData.Description != "" {
		data.Description = types.StringValue(fmData.Description)
	}

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
	tflog.Info(ctx, "**** Dedup app data read SUCCESS ******", nil)

	d.updateTFStruct(&data, &dedupData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *Dedup) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData DedupModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
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

	// Overlay FM-owned fields (id, description normalization)
	d.updateTFStruct(&planData, fmData)

	// Final state = plan + FM overrides
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
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
	var planData MaskingModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
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

	// Let FM override computed/FM-owned fields
	m.updateTFStruct(&planData, fmData)

	// Final state = plan + FM-owned overrides
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
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

func (h *HeaderStripping) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_header_stripping"
}

func simpleHSBlock(desc string) schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: desc,
		Attributes:          map[string]schema.Attribute{},
	}
}

func vxlanHSBlock() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: "VXLAN header stripping config (protocol = vxlan)",

		Attributes: map[string]schema.Attribute{
			"vxlan_id": schema.Int32Attribute{
				MarkdownDescription: "24‑bit VXLAN ID to strip; 0 strips all VXLAN IDs",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
				Validators: []validator.Int32{
					int32validator.AtLeast(0),
					int32validator.AtMost(16777215),
				},
			},
		},
	}
}

func vlanHSBlock() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: "VLAN header stripping config (protocol = vlan)",
		Attributes: map[string]schema.Attribute{
			"vlan_header": schema.StringAttribute{
				MarkdownDescription: "Target VLAN header(s) to remove",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("all"),
				Validators: []validator.String{
					stringvalidator.OneOf("outer", "all"),
				},
			},
		},
	}
}

func fm6000TsHSBlock() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: "FM6000Ts header stripping config (protocol = fm6000Ts)",
		Attributes: map[string]schema.Attribute{
			"timestamp_format": schema.StringAttribute{
				MarkdownDescription: "Timestamp format",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("none"),
				Validators: []validator.String{
					stringvalidator.OneOf("none"),
				},
			},
		},
	}
}

func erspanHSBlock() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: "ERSPAN header stripping config (protocol = erspan)",
		Attributes: map[string]schema.Attribute{
			"flow_id": schema.Int32Attribute{
				MarkdownDescription: "ERSPAN flow ID; 0 matches all flows",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
				Validators: []validator.Int32{
					int32validator.AtLeast(0),
					int32validator.AtMost(1023),
				},
			},
		},
	}
}

func genericHSBlock() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: "Generic header stripping config (protocol = generic)",
		Attributes: map[string]schema.Attribute{
			"ah1": schema.StringAttribute{
				MarkdownDescription: "First anchor header",
				Optional:            true, // was Required: true
				Validators: []validator.String{
					stringvalidator.OneOf("none", "eth", "vlan", "mpls", "ipv4", "ipv6"),
				},
			},
			"offset": schema.StringAttribute{
				MarkdownDescription: "Offset mode: start, end, offsetRange",
				Optional:            true, // was Required: true
				Validators: []validator.String{
					stringvalidator.OneOf("start", "end", "offsetRange"),
				},
			},
			"offset_range_value": schema.Int32Attribute{
				MarkdownDescription: "Offset from AH1 when offset = offsetRange (0–1500); 0 is valid",
				Optional:            true,
				Validators: []validator.Int32{
					int32validator.AtLeast(0),
					int32validator.AtMost(1500),
				},
			},
			"header_count": schema.Int32Attribute{
				MarkdownDescription: "Number of headers to remove",
				Optional:            true,
				Validators: []validator.Int32{
					int32validator.AtLeast(1),
					int32validator.AtMost(32),
				},
			},
			"custom_len": schema.Int32Attribute{
				MarkdownDescription: "Length (bytes) of header to strip",
				Optional:            true,
				Validators: []validator.Int32{
					int32validator.AtLeast(1),
					int32validator.AtMost(1500),
				},
			},
			"ah2": schema.StringAttribute{
				MarkdownDescription: "Second anchor header",
				Optional:            true, // was Required: true
				Validators: []validator.String{
					stringvalidator.OneOf("none", "eth", "vlan", "mpls"),
				},
			},
		},
	}
}

func (h *HeaderStripping) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("vxlan"),
			path.MatchRoot("vlan"),
			path.MatchRoot("fm6000_ts"),
			path.MatchRoot("erspan"),
			path.MatchRoot("generic"),
			path.MatchRoot("gtp"),
			path.MatchRoot("isl"),
			path.MatchRoot("mpls"),
			path.MatchRoot("mpls_plus_vlan"),
			path.MatchRoot("vntag"),
			path.MatchRoot("geneve"),
		),
	}
}

func (h *HeaderStripping) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Gigamon APP Header Stripping Schema",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name for this header stripping application",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("headerStrip"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
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
				MarkdownDescription: "Header Stripping protocol (computed from the selected block).",
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						"vxlan",
						"vlan",
						"fm6000Ts",
						"erspan",
						"generic",
						"gtp",
						"isl",
						"mpls",
						"mplsPlusVlan",
						"vntag",
						"geneve",
					),
				},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this App instance for later use",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},

		Blocks: map[string]schema.Block{
			// parameterized protocols
			"vxlan":     vxlanHSBlock(),
			"vlan":      vlanHSBlock(),
			"fm6000_ts": fm6000TsHSBlock(),
			"erspan":    erspanHSBlock(),
			"generic":   genericHSBlock(),

			// simple protocols (presence = select protocol)
			"gtp":            simpleHSBlock("GTP header stripping (protocol = gtp)"),
			"isl":            simpleHSBlock("ISL header stripping (protocol = isl)"),
			"mpls":           simpleHSBlock("MPLS header stripping (protocol = mpls)"),
			"mpls_plus_vlan": simpleHSBlock("MPLS+VLAN header stripping (protocol = mplsPlusVlan)"),
			"vntag":          simpleHSBlock("VN‑tag header stripping (protocol = vntag)"),
			"geneve":         simpleHSBlock("Geneve header stripping (protocol = geneve)"),
		},
	}
}

func inferHeaderStripProtocol(
	vx *HeaderStrippingVxlanConfig,
	vl *HeaderStrippingVlanConfig,
	ts *HeaderStrippingFm6000TsConfig,
	er *HeaderStrippingErspanConfig,
	ge *HeaderStrippingGenericConfig,
	gtp *HeaderStrippingEmptyConfig,
	isl *HeaderStrippingEmptyConfig,
	mpls *HeaderStrippingEmptyConfig,
	mplsPlusVlan *HeaderStrippingEmptyConfig,
	vntag *HeaderStrippingEmptyConfig,
	geneve *HeaderStrippingEmptyConfig,
) string {
	switch {
	case vx != nil:
		return "vxlan"
	case vl != nil:
		return "vlan"
	case ts != nil:
		return "fm6000Ts"
	case er != nil:
		return "erspan"
	case ge != nil:
		return "generic"
	case gtp != nil:
		return "gtp"
	case isl != nil:
		return "isl"
	case mpls != nil:
		return "mpls"
	case mplsPlusVlan != nil:
		return "mplsPlusVlan"
	case vntag != nil:
		return "vntag"
	case geneve != nil:
		return "geneve"
	default:
		return ""
	}
}

func (h *HeaderStripping) createFMStruct(data *HeaderStrippingModel) *FMHeaderStripping {
	p := inferHeaderStripProtocol(
		data.Vxlan,
		data.Vlan,
		data.Fm6000Ts,
		data.Erspan,
		data.Generic,
		data.Gtp,
		data.Isl,
		data.Mpls,
		data.MplsPlusVlan,
		data.Vntag,
		data.Geneve,
	)

	if p != "" {
		data.Protocol = types.StringValue(p)
	}

	fm := &FMHeaderStripping{
		Alias:    data.Alias.ValueString(),
		Name:     "headerStrip",
		Protocol: p,
		Id:       data.Id.ValueString(),
	}

	switch p {
	case "vxlan":
		var vxid int32
		if data.Vxlan != nil && !data.Vxlan.VxlanId.IsNull() && !data.Vxlan.VxlanId.IsUnknown() {
			vxid = data.Vxlan.VxlanId.ValueInt32()
		}
		fm.Vxlan = &FMHeaderStrippingVxlan{
			VxlanId: vxid,
		}

	case "vlan":
		var vh string
		if data.Vlan != nil && !data.Vlan.VlanHeader.IsNull() && !data.Vlan.VlanHeader.IsUnknown() {
			vh = data.Vlan.VlanHeader.ValueString()
		}
		fm.Vlan = &FMHeaderStrippingVlan{
			VlanHeader: vh, // will be "all" if user omitted it
		}

	case "fm6000Ts":
		ts := "none"
		if data.Fm6000Ts != nil && !data.Fm6000Ts.TimestampFormat.IsNull() && !data.Fm6000Ts.TimestampFormat.IsUnknown() && data.Fm6000Ts.TimestampFormat.ValueString() != "" {
			ts = data.Fm6000Ts.TimestampFormat.ValueString()
		}
		fm.Fm6000Ts = &FMHeaderStrippingFm6000Ts{
			TimestampFormat: ts,
		}

	case "erspan":
		var flowId int32
		if data.Erspan != nil && !data.Erspan.FlowId.IsNull() && !data.Erspan.FlowId.IsUnknown() {
			flowId = data.Erspan.FlowId.ValueInt32()
		}
		fm.Erspan = &FMHeaderStrippingErspan{
			ErspanFlowId: flowId,
		}

	case "generic":
		if data.Generic != nil {
			// OffsetRangeValue: pointer so we can omit the field entirely if user didn't set it.
			var offRangePtr *int32
			if !data.Generic.OffsetRangeValue.IsNull() && !data.Generic.OffsetRangeValue.IsUnknown() {
				v := data.Generic.OffsetRangeValue.ValueInt32()
				offRangePtr = &v
			}

			// HeaderCount, CustomLen: plain int32; 0 means "unset" and is omitted by ,omitempty.
			var hdrCount int32
			if !data.Generic.HeaderCount.IsNull() && !data.Generic.HeaderCount.IsUnknown() {
				hdrCount = data.Generic.HeaderCount.ValueInt32()
			}

			var cLen int32
			if !data.Generic.CustomLen.IsNull() && !data.Generic.CustomLen.IsUnknown() {
				cLen = data.Generic.CustomLen.ValueInt32()
			}

			fm.Generic = &FMHeaderStrippingGeneric{
				Ah1:              data.Generic.Ah1.ValueString(),
				Offset:           data.Generic.Offset.ValueString(),
				OffsetRangeValue: offRangePtr,
				HeaderCount:      hdrCount,
				CustomLen:        cLen,
				Ah2:              data.Generic.Ah2.ValueString(),
			}
		}

	case "gtp", "isl", "mpls", "mplsPlusVlan", "vntag", "geneve":
		// No extra FM blocks; protocol alone is enough.
	}

	return fm
}

// Overlay FM-owned fields into TF state
// Overlay FM-owned fields into TF state
func (h *HeaderStripping) updateTFStruct(data *HeaderStrippingModel, fmData *FMHeaderStripping) {
	if fmData.Protocol != "" {
		data.Protocol = types.StringValue(fmData.Protocol)
	}

	// Clear all blocks first
	data.Vxlan = nil
	data.Vlan = nil
	data.Fm6000Ts = nil
	data.Erspan = nil
	data.Generic = nil
	data.Gtp = nil
	data.Isl = nil
	data.Mpls = nil
	data.MplsPlusVlan = nil
	data.Vntag = nil
	data.Geneve = nil

	if fmData.Vxlan != nil {
		data.Vxlan = &HeaderStrippingVxlanConfig{
			VxlanId: types.Int32Value(fmData.Vxlan.VxlanId),
		}
	}

	if fmData.Vlan != nil {
		data.Vlan = &HeaderStrippingVlanConfig{
			VlanHeader: types.StringValue(fmData.Vlan.VlanHeader),
		}
	}

	if fmData.Fm6000Ts != nil {
		data.Fm6000Ts = &HeaderStrippingFm6000TsConfig{
			TimestampFormat: types.StringValue(fmData.Fm6000Ts.TimestampFormat),
		}
	}

	if fmData.Erspan != nil {
		data.Erspan = &HeaderStrippingErspanConfig{
			FlowId: types.Int32Value(fmData.Erspan.ErspanFlowId),
		}
	}

	if fmData.Generic != nil {
		// offsetRangeValue: pointer in FM, null when absent
		var offRange types.Int32
		if fmData.Generic.OffsetRangeValue != nil {
			offRange = types.Int32Value(*fmData.Generic.OffsetRangeValue)
		} else {
			offRange = types.Int32Null()
		}

		// headerCount/customLen: 0 from FM means "unset" for us,
		// because TF validators require >= 1, so we map 0 -> null
		var hdrCount types.Int32
		if fmData.Generic.HeaderCount != 0 {
			hdrCount = types.Int32Value(fmData.Generic.HeaderCount)
		} else {
			hdrCount = types.Int32Null()
		}

		var cLen types.Int32
		if fmData.Generic.CustomLen != 0 {
			cLen = types.Int32Value(fmData.Generic.CustomLen)
		} else {
			cLen = types.Int32Null()
		}

		data.Generic = &HeaderStrippingGenericConfig{
			Ah1:              types.StringValue(fmData.Generic.Ah1),
			Offset:           types.StringValue(fmData.Generic.Offset),
			OffsetRangeValue: offRange,
			HeaderCount:      hdrCount,
			CustomLen:        cLen,
			Ah2:              types.StringValue(fmData.Generic.Ah2),
		}
	}

	// Simple protocols: re‑set empty blocks
	switch fmData.Protocol {
	case "gtp":
		data.Gtp = &HeaderStrippingEmptyConfig{}
	case "isl":
		data.Isl = &HeaderStrippingEmptyConfig{}
	case "mpls":
		data.Mpls = &HeaderStrippingEmptyConfig{}
	case "mplsPlusVlan":
		data.MplsPlusVlan = &HeaderStrippingEmptyConfig{}
	case "vntag":
		data.Vntag = &HeaderStrippingEmptyConfig{}
	case "geneve":
		data.Geneve = &HeaderStrippingEmptyConfig{}
	}

	if fmData.Id != "" {
		data.Id = types.StringValue(fmData.Id)
	}
}

// validateParams enforces cross-field semantics for the generic block.
func (h *HeaderStripping) validateParams(data *HeaderStrippingModel) error {
	if data.Generic != nil {
		// ah1 required when generic is used
		if data.Generic.Ah1.IsNull() || data.Generic.Ah1.IsUnknown() || data.Generic.Ah1.ValueString() == "" {
			return fmt.Errorf("in generic block, ah1 must be specified")
		}

		// ah2 required when generic is used
		if data.Generic.Ah2.IsNull() || data.Generic.Ah2.IsUnknown() || data.Generic.Ah2.ValueString() == "" {
			return fmt.Errorf("in generic block, ah2 must be specified")
		}

		// offset required when generic is used
		if data.Generic.Offset.IsNull() || data.Generic.Offset.IsUnknown() {
			return fmt.Errorf("in generic block, offset must be specified")
		}
		off := data.Generic.Offset.ValueString()

		if off == "offsetRange" {
			if data.Generic.OffsetRangeValue.IsNull() || data.Generic.OffsetRangeValue.IsUnknown() {
				return fmt.Errorf("in generic block with offset='offsetRange', offset_range_value is required")
			}
		} else {
			if !data.Generic.OffsetRangeValue.IsNull() && !data.Generic.OffsetRangeValue.IsUnknown() {
				return fmt.Errorf("offset_range_value is only valid when offset='offsetRange'")
			}
		}
	}

	return nil
}

func (h *HeaderStripping) Configure(
	ctx context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
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

	h.fmClient = fmClient
}

func (h *HeaderStripping) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data HeaderStrippingModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := h.validateParams(&data); err != nil {
		resp.Diagnostics.AddError(
			"Invalid parameters specified",
			fmt.Sprintf("Invalid parameters for header stripping app: %s", err),
		)
		return
	}

	fmData := h.createFMStruct(&data)

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
		h.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create header stripping app",
			fmt.Sprintf("app creation failed: %s", err),
		)
		return
	}

	data.Id = types.StringValue(id)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (h *HeaderStripping) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data HeaderStrippingModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hs := FMHeaderStripping{}

	err := GetMSAppData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		data.Id.ValueString(),
		"headerStrip", // app type
		"",
		&hs,
		h.fmClient,
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
			"Unable to Get Header Stripping App details",
			fmt.Sprintf("unable to get Header Stripping App details: %v", err),
		)
		return
	}

	h.updateTFStruct(&data, &hs)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (h *HeaderStripping) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData HeaderStrippingModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := h.validateParams(&planData); err != nil {
		resp.Diagnostics.AddError(
			"Unable to update header stripping app",
			fmt.Sprintf("app update failed: %s", err),
		)
		return
	}

	fmData := h.createFMStruct(&planData)

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
		h.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update header stripping app",
			fmt.Sprintf("app update failed: %s", err),
		)
		return
	}

	h.updateTFStruct(&planData, fmData)

	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (h *HeaderStripping) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data HeaderStrippingModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "application",
				Operation:  "delete",
				Application: FMHeaderStripping{
					Id:   data.Id.ValueString(),
					Name: "Application", // matches Slicing/Dedup delete convention
				},
			},
		},
	}

	_, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		h.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete header stripping app",
			fmt.Sprintf("app deletion failed: %s", err),
		)
	}
}
