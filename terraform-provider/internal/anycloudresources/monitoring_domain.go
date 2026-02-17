// Copyright (c) Gigamon, Inc.

// Implements the Resources for the (Third Party Orchestration) AnyCloud Monitoring Domain

package anycloudresources

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AnyCloudMD{}
var _ resource.ResourceWithImportState = &AnyCloudMD{}
var _ resource.ResourceWithValidateConfig = &AnyCloudMD{}
var _ resource.ResourceWithModifyPlan = &AnyCloudMD{}

// AnyCloud MD resource, which manages monitoring domain for AnyCloud, (Third Party Orchestration)
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
	TappingMethod        types.String `tfsdk:"tapping_method"` // Terraform-only Selector
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
	DualStackPreferIPv6  bool             `json:"dualStackPreferIPv6"`
	UniformTrafficPolicy bool             `json:"uniformTrafficPolicy,omitempty"`
	MTU                  int32            `json:"mtu"`
	ConnectionIds        []string         `json:"connIds,omitempty"`     // Used when we post/patch request
	GetConnectionIds     []AnyCloudMDConn `json:"connections,omitempty"` // Use in the Get only
	Id                   string           `json:"id,omitempty"`
}

// FM request payload when tapping_method == "uctv"
type AnyCloudFmMDRequestUCTV struct {
	Alias               string   `json:"alias,omitempty"`
	Platform            string   `json:"platform,omitempty"`
	UserLaunched        bool     `json:"userLaunched,omitempty"`
	DualStackPreferIPv6 bool     `json:"dualStackPreferIPv6"`
	MTU                 int32    `json:"mtu"`
	ConnectionIds       []string `json:"connIds,omitempty"`
	Id                  string   `json:"id,omitempty"`
}

// FM request payload when tapping_method == "none"
type AnyCloudFmMDRequestNone struct {
	Alias                string   `json:"alias,omitempty"`
	Platform             string   `json:"platform,omitempty"`
	UserLaunched         bool     `json:"userLaunched,omitempty"`
	UniformTrafficPolicy bool     `json:"uniformTrafficPolicy"`
	ConnectionIds        []string `json:"connIds,omitempty"`
	Id                   string   `json:"id,omitempty"`
}

func normalizeTappingMethod(v string) string {
	m := strings.ToLower(strings.TrimSpace(v))
	if m == "" {
		return "uctv"
	}
	return m
}

func (md *AnyCloudMD) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_anycloud_monitoring_domain"
}

func (md *AnyCloudMD) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon Third Party Orchestration (AnyCloud) Monitoring Domain",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name of the monitoring domain",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
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
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"user_launched": schema.BoolAttribute{
				MarkdownDescription: "If true, indicates that the VSeries nodes are launched and managed by the user. Default true",
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			// Terraform-only selector to decide which MD knobs are applicable.
			// Must match the connection tapping_method.
			"tapping_method": schema.StringAttribute{
				MarkdownDescription: "Tapping method selector used to decide which monitoring domain fields are applicable. Must match the connection tapping_method. Allowed values: uctv, none. Default uctv",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("uctv"),
				Validators: []validator.String{
					stringvalidator.OneOf("uctv", "none"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"dual_stack_prefer_ipv6": schema.BoolAttribute{
				MarkdownDescription: "If true, indicates IPv6 tunnels are preferred between UCT‑V and VSeries nodes. Default false. Applicable when tapping_method=uctv",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"uniform_traffic_policy": schema.BoolAttribute{
				MarkdownDescription: "If true, indicates same monitoring session configuration is applied to all VSeries nodes in the monitoring domain. Default false. Applicable when tapping_method=none",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"mtu": schema.Int32Attribute{
				MarkdownDescription: "MTU between UCT‑V and VSeries nodes, when Traffic Acquisition method is UCT-V. Default value is 1450. Applicable when tapping_method=uctv",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(1450),
				Validators: []validator.Int32{
					int32validator.Between(1280, 9000),
				},
			},
			"connection_id": schema.StringAttribute{
				MarkdownDescription: "Connection ID associated with this MD",
				Computed:            true,
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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

func (md *AnyCloudMD) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg AnyCloudMDModel

	// Read Terraform configuration (only what user wrote in HCL)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// tapping_method may be null in config (because default is applied later in plan),
	// so treat null/empty as default "uctv".
	method := "uctv"
	if !cfg.TappingMethod.IsNull() && !cfg.TappingMethod.IsUnknown() {
		if v := strings.TrimSpace(cfg.TappingMethod.ValueString()); v != "" {
			method = strings.ToLower(v)
		}
	}

	// Validate tapping_method value (defensive)
	if method != "uctv" && method != "none" {
		resp.Diagnostics.AddError(
			"Invalid tapping_method",
			fmt.Sprintf("Unsupported tapping_method %q. Allowed values are \"uctv\" and \"none\".", method),
		)
		return
	}

	// Detect whether user explicitly set each knob in config.
	// (If user omitted it, value will be null here, even if schema has Default.)
	dualSet := !cfg.DualStackPreferIPv6.IsNull() && !cfg.DualStackPreferIPv6.IsUnknown()
	mtuSet := !cfg.MTU.IsNull() && !cfg.MTU.IsUnknown()
	uniformSet := !cfg.UniformTrafficPolicy.IsNull() && !cfg.UniformTrafficPolicy.IsUnknown()

	switch method {
	case "none":
		// Only uniform_traffic_policy is applicable
		if dualSet || mtuSet {
			resp.Diagnostics.AddError(
				"Invalid monitoring domain configuration for tapping_method=none",
				"When tapping_method is \"none\", only uniform_traffic_policy is applicable. Remove mtu and dual_stack_prefer_ipv6 from the monitoring domain configuration.",
			)
		}

	case "uctv":
		// Only mtu and dual_stack_prefer_ipv6 are applicable
		if uniformSet {
			resp.Diagnostics.AddError(
				"Invalid monitoring domain configuration for tapping_method=uctv",
				"When tapping_method is \"uctv\", uniform_traffic_policy is not applicable. Remove uniform_traffic_policy from the monitoring domain configuration.",
			)
		}
	}
}

func (md *AnyCloudMD) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// If creating (no prior state), nothing to preserve
	if req.State.Raw.IsNull() {
		return
	}

	var cfg, plan, state AnyCloudMDModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	method := normalizeTappingMethod(plan.TappingMethod.ValueString())

	// If user didn't explicitly configure, preserve state value to avoid drift
	switch method {
	case "none":
		if cfg.MTU.IsNull() || cfg.MTU.IsUnknown() {
			plan.MTU = state.MTU
		}
		if cfg.DualStackPreferIPv6.IsNull() || cfg.DualStackPreferIPv6.IsUnknown() {
			plan.DualStackPreferIPv6 = state.DualStackPreferIPv6
		}

	case "uctv":
		if cfg.UniformTrafficPolicy.IsNull() || cfg.UniformTrafficPolicy.IsUnknown() {
			plan.UniformTrafficPolicy = state.UniformTrafficPolicy
		}
	}

	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

func (md *AnyCloudMD) getMDByAlias(ctx context.Context, alias string) (*AnyCloudFmMD, error) {

	fmMDData := struct {
		MonitoringDomains []AnyCloudFmMD `json:"monitoringDomains"`
	}{}

	mdResp, err := md.fmClient.DoRequest(
		ctx,
		"GET",
		"api/v1.3/cloud/monitoringDomains",
		map[string]string{"platform": "anyCloud"},
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("GET anyCloud monitoring domains failed: %w", err)
	}

	err = json.Unmarshal(mdResp, &fmMDData)
	if err != nil {
		return nil, fmt.Errorf(
			"Unable to convert MD Get resp to struct: %s error is: %w",
			string(mdResp),
			err,
		)
	}

	for i := range fmMDData.MonitoringDomains {
		if fmMDData.MonitoringDomains[i].Alias == alias {
			return &fmMDData.MonitoringDomains[i], nil
		}
	}

	return nil, fmclient.NewFMError(
		fmclient.ObjectNotFound,
		fmt.Sprintf("Unable to find anyCloud MD by alias: %s", alias),
		nil,
	)
}

func (md *AnyCloudMD) getMDByID(ctx context.Context, id string) (*AnyCloudFmMD, error) {
	fmMDData := struct {
		MonitoringDomain AnyCloudFmMD `json:"monitoringDomain"`
	}{}

	// Extract raw UUID from TypedID
	rawID, err := commonutils.UUIDFromTypedID(id)
	if err != nil {
		return nil, fmt.Errorf("Invalid MD id %q: %w", id, err)
	}

	mdResp, err := md.fmClient.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/monitoringDomains/%s", rawID),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("GET anyCloud monitoring domains failed: %w", err)
	}

	err = json.Unmarshal(mdResp, &fmMDData)
	if err != nil {
		return nil, fmt.Errorf(
			"Unable to convert anyCloud MD resp to struct: %s error is: %w",
			string(mdResp),
			err,
		)
	}
	return &fmMDData.MonitoringDomain, nil
}

// Given the MD Alias / ID, get details from FM and updates the TF state
func (md *AnyCloudMD) updateMD(ctx context.Context, data *AnyCloudMDModel, alias, id string) error {

	var err error
	var mdDetails *AnyCloudFmMD

	if alias != "" {
		mdDetails, err = md.getMDByAlias(ctx, alias)
	} else {
		mdDetails, err = md.getMDByID(ctx, id)
	}
	if err != nil {
		return err
	}

	// Make TypedID from raw UUID received from FM
	typedID, err := commonutils.MakeTypedID(commonutils.ModuleMonitoringDomain, commonutils.TypeAnyCloud, mdDetails.Id)
	if err != nil {
		return err
	}
	data.Id = types.StringValue(typedID)

	data.Alias = types.StringValue(mdDetails.Alias)
	data.Platform = types.StringValue(mdDetails.Platform)
	data.UserLaunched = types.BoolValue(mdDetails.UserLaunched)
	data.DualStackPreferIPv6 = types.BoolValue(mdDetails.DualStackPreferIPv6)
	data.MTU = types.Int32Value(mdDetails.MTU)
	data.UniformTrafficPolicy = types.BoolValue(mdDetails.UniformTrafficPolicy)

	if len(mdDetails.GetConnectionIds) != 0 {
		// Make TypedID from raw UUID received from FM
		typedID, err := commonutils.MakeTypedID(commonutils.ModuleConnection, commonutils.TypeAnyCloud, mdDetails.GetConnectionIds[0].Id)
		if err != nil {
			return err
		}
		data.ConnectionId = types.StringValue(typedID)
	} else {
		data.ConnectionId = types.StringValue("Unknown")
	}

	return nil
}

func (md *AnyCloudMD) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AnyCloudMDModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var fmMDData any
	method := normalizeTappingMethod(data.TappingMethod.ValueString())

	switch method {
	case "uctv":
		fmMDData = AnyCloudFmMDRequestUCTV{
			Alias:               data.Alias.ValueString(),
			Platform:            "anyCloud",
			UserLaunched:        true,
			DualStackPreferIPv6: data.DualStackPreferIPv6.ValueBool(),
			MTU:                 data.MTU.ValueInt32(),
		}
	case "none":
		fmMDData = AnyCloudFmMDRequestNone{
			Alias:                data.Alias.ValueString(),
			Platform:             "anyCloud",
			UserLaunched:         true,
			UniformTrafficPolicy: data.UniformTrafficPolicy.ValueBool(),
		}
	default:
		resp.Diagnostics.AddError("Invalid tapping_method", fmt.Sprintf("Unsupported tapping_method %q. Allowed values are \"uctv\" and \"none\".", method))
		return
	}

	jsonData, err := json.Marshal(fmMDData)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to convert Monitoring Domain Create payload to JSON",
			fmt.Sprintf("converting: %v error is: %v", fmMDData, err),
		)
		return
	}

	tflog.Info(ctx, "Creating monitoring domain", map[string]any{
		"struct":   fmMDData,
		"jsonBody": string(jsonData),
	})

	_, err = md.fmClient.DoRequest(
		ctx,
		"POST",
		"api/v1.3/cloud/monitoringDomains",
		nil,
		nil,
		bytes.NewBuffer(jsonData),
		"application/json",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create the monitoring domain for anyCloud",
			fmt.Sprintf("Monitoring Domain Create: %v error is: %v", fmMDData, err),
		)
		return
	}

	err = md.updateMD(ctx, &data, data.Alias.ValueString(), "")
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not get the updated data on anyCloud MD from FM",
			fmt.Sprintf("%v", err),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (md *AnyCloudMD) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AnyCloudMDModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// ID should always be present in state for Read
	if data.Id.IsNull() || data.Id.IsUnknown() || data.Id.ValueString() == "" {
		resp.Diagnostics.AddError(
			"Missing AnyCloud Monitoring Domain ID",
			"Cannot read AnyCloud Monitoring Domain because 'id' is null/unknown/empty in state.",
		)
		return
	}

	err := md.updateMD(ctx, &data, "", data.Id.ValueString())
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) && fmErr.ErrorCode() == fmclient.ObjectNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Unable to read AnyCloud Monitoring Domain",
			fmt.Sprintf("Failed to read AnyCloud Monitoring Domain (id=%s): %v", data.Id.ValueString(), err),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (md *AnyCloudMD) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData AnyCloudMDModel

	// Read Terraform plan and prior state data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Validate ID
	if stateData.Id.IsNull() || stateData.Id.IsUnknown() || stateData.Id.ValueString() == "" {
		resp.Diagnostics.AddError("Missing AnyCloud MD ID", "Cannot update because 'id' is missing in state.")
		return
	}

	// Extract raw UUID from TypedID
	typedID := stateData.Id.ValueString()
	mdId, err := commonutils.UUIDFromTypedID(typedID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid MD id in state", err.Error())
		return
	}
	connId, err := commonutils.UUIDFromTypedID(stateData.ConnectionId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Connection id in state", err.Error())
		return
	}

	var fmMDData any
	method := normalizeTappingMethod(planData.TappingMethod.ValueString())

	switch method {
	case "uctv":
		fmMDData = struct {
			MonitoringDomains []AnyCloudFmMDRequestUCTV `json:"monitoringDomains"`
		}{
			MonitoringDomains: []AnyCloudFmMDRequestUCTV{
				{
					Platform:            stateData.Platform.ValueString(),
					ConnectionIds:       []string{connId},
					Id:                  mdId,
					DualStackPreferIPv6: planData.DualStackPreferIPv6.ValueBool(),
					MTU:                 planData.MTU.ValueInt32(),
				},
			},
		}

	case "none":
		fmMDData = struct {
			MonitoringDomains []AnyCloudFmMDRequestNone `json:"monitoringDomains"`
		}{
			MonitoringDomains: []AnyCloudFmMDRequestNone{
				{
					Platform:             stateData.Platform.ValueString(),
					ConnectionIds:        []string{connId},
					Id:                   mdId,
					UniformTrafficPolicy: planData.UniformTrafficPolicy.ValueBool(),
				},
			},
		}

	default:
		resp.Diagnostics.AddError("Invalid tapping_method", fmt.Sprintf("Unsupported tapping_method %q. Allowed values are \"uctv\" and \"none\".", method))
		return
	}

	jsonData, err := json.Marshal(fmMDData)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to convert Monitoring Domain Update payload to JSON",
			fmt.Sprintf("converting: %v error is: %v", fmMDData, err),
		)
		return
	}

	tflog.Info(ctx, "Updating monitoring domain", map[string]any{
		"struct":   fmMDData,
		"jsonBody": string(jsonData),
	})

	_, err = md.fmClient.DoRequest(
		ctx,
		"PATCH",
		fmt.Sprintf("api/v1.3/cloud/monitoringDomains/%s", mdId),
		nil,
		nil,
		bytes.NewBuffer(jsonData),
		"application/json",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update the AnyCloud monitoring domain",
			fmt.Sprintf("Monitoring Domain update: %v error is: %v", fmMDData, err),
		)
		return
	}

	// Persistant Terraform-only selector from plan into state
	stateData.TappingMethod = planData.TappingMethod

	err = md.updateMD(ctx, &stateData, "", typedID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not get the updated data on MD from FM",
			fmt.Sprintf("%v", err),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &stateData)...)
}

func (md *AnyCloudMD) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AnyCloudMDModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If ID is missing, nothing to delete
	if data.Id.IsNull() || data.Id.IsUnknown() || data.Id.ValueString() == "" {
		return
	}

	// Extract raw UUID from TypedID
	mdId, err := commonutils.UUIDFromTypedID(data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid MD id in state", err.Error())
		return
	}

	_, err = md.fmClient.DoRequest(
		ctx,
		"DELETE",
		fmt.Sprintf("api/v1.3/cloud/monitoringDomains/%s", mdId),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete AnyCloud Monitoring Domain",
			fmt.Sprintf("Unable to delete monitoring domain: %s (%s) error is: %v", data.Alias.ValueString(), data.Id.ValueString(), err),
		)
	}
}

func (md *AnyCloudMD) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	var data AnyCloudMDModel

	alias := strings.TrimSpace(req.ID)
	if alias == "" {
		resp.Diagnostics.AddError("Invalid import id", "Import id cannot be empty. Use the Monitoring Domain alias")
		return
	}

	err := md.updateMD(ctx, &data, alias, "")
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to import AnyCloud Monitoring Domain",
			fmt.Sprintf("Failed to import monitoring domain with alias=%q: %v", alias, err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
