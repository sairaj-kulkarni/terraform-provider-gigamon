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
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AnyCloudMD{}
var _ resource.ResourceWithImportState = &AnyCloudMD{}
var _ resource.ResourceWithConfigValidators = &AnyCloudMD{}
var _ resource.ResourceWithModifyPlan = &AnyCloudMD{}

// AnyCloud MD resource, which manages monitoring domain for AnyCloud, (Third Party Orchestration)
func NewAnyCloudMD() resource.Resource {
	return &AnyCloudMD{}
}

// AnyCloudMD manages the MD for AnyCloud
type AnyCloudMD struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// Tapping method = uctv
type AnyCloudTappingMethodUCTVModel struct {
	MTU                 types.Int32 `tfsdk:"mtu"`
	DualStackPreferIPv6 types.Bool  `tfsdk:"dual_stack_prefer_ipv6"`
}

// Tapping method = none
type AnyCloudTappingMethodNoneModel struct {
	UniformTrafficPolicy types.Bool `tfsdk:"uniform_traffic_policy"`
}

// AnyCloudMDModel describes the resource data model.
type AnyCloudMDModel struct {
	Alias        types.String `tfsdk:"alias"`
	Platform     types.String `tfsdk:"platform"`
	UserLaunched types.Bool   `tfsdk:"user_launched"`

	// Exactly one of these must be configured
	UCTV types.Object `tfsdk:"uctv"`
	None types.Object `tfsdk:"none"`

	// Computed output derived from which nested config is set: "uctv" or "none"
	TappingMethod types.String `tfsdk:"tapping_method"`
	ConnectionId  types.String `tfsdk:"connection_id"`
	Id            types.String `tfsdk:"id"`
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

func (md *AnyCloudMD) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_anycloud_monitoring_domain"
}

// Exactly one of uctv/none must be configured
func (md *AnyCloudMD) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("uctv"),
			path.MatchRoot("none"),
		),
	}
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

			// Tapping method configuration (exactly one of uctv/none is required). ExactlyOneOf validator enforces overall requiredness
			// Tapping Method = uctv
			"uctv": schema.SingleNestedAttribute{
				MarkdownDescription: "Tapping method as uctv configuration",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"mtu": schema.Int32Attribute{
						MarkdownDescription: "MTU between UCT‑V and VSeries nodes. Default 1450.",
						Optional:            true,
						Computed:            true,
						Default:             int32default.StaticInt32(1450),
						Validators: []validator.Int32{
							int32validator.Between(1280, 9000),
						},
					},
					"dual_stack_prefer_ipv6": schema.BoolAttribute{
						MarkdownDescription: "If true, indicates IPv6 tunnels are preferred between UCT‑V and VSeries nodes. Default false.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
				},
			},
			// Tapping Method = none
			"none": schema.SingleNestedAttribute{
				MarkdownDescription: "Tapping method as none configuration. For Customer Orchestrated Source.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"uniform_traffic_policy": schema.BoolAttribute{
						MarkdownDescription: "If true, indicates same monitoring session configuration is applied to all VSeries nodes in the monitoring domain. Default false.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
				},
			},

			// Computed output for Connection reference
			"tapping_method": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Derived tapping method based on which nested config is set: uctv or none.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"connection_id": schema.StringAttribute{
				MarkdownDescription: "Connection ID associated with this MD",
				Computed:            true,
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

// ModifyPlan derives computed tapping_method from which nested config is set.
// This enables Connection to reference tapping_method during plan / apply
func (md *AnyCloudMD) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Config.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var plan AnyCloudMDModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uctvSet := !plan.UCTV.IsNull() && !plan.UCTV.IsUnknown()
	noneSet := !plan.None.IsNull() && !plan.None.IsUnknown()
	if uctvSet == noneSet {
		resp.Diagnostics.AddError("Invalid configuration", "Exactly one of uctv or none must be specified.")
		return
	}

	if uctvSet {
		plan.TappingMethod = types.StringValue("uctv")
	} else {
		plan.TappingMethod = types.StringValue("none")
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

	// Determine tappingMethod from existing state/config if exists
	tappingMethod := ""
	if !data.TappingMethod.IsNull() && !data.TappingMethod.IsUnknown() && strings.TrimSpace(data.TappingMethod.ValueString()) != "" {
		tappingMethod = strings.ToLower(strings.TrimSpace(data.TappingMethod.ValueString()))
	} else {
		// Infer from nested object present in state
		if !data.UCTV.IsNull() && !data.UCTV.IsUnknown() {
			tappingMethod = "uctv"
		} else if !data.None.IsNull() && !data.None.IsUnknown() {
			tappingMethod = "none"
		}
	}

	uctvAttrTypes := map[string]attr.Type{
		"mtu":                    types.Int32Type,
		"dual_stack_prefer_ipv6": types.BoolType,
	}
	noneAttrTypes := map[string]attr.Type{
		"uniform_traffic_policy": types.BoolType,
	}

	// Populate nested objects and tapping_method
	switch tappingMethod {
	case "uctv":
		uctvObj, diags := types.ObjectValue(uctvAttrTypes, map[string]attr.Value{
			"mtu":                    types.Int32Value(mdDetails.MTU),
			"dual_stack_prefer_ipv6": types.BoolValue(mdDetails.DualStackPreferIPv6),
		})
		if diags.HasError() {
			return fmt.Errorf("failed building uctv object: %v", diags)
		}
		data.UCTV = uctvObj
		data.None = types.ObjectNull(noneAttrTypes)
		data.TappingMethod = types.StringValue("uctv")

	case "none":
		noneObj, diags := types.ObjectValue(noneAttrTypes, map[string]attr.Value{
			"uniform_traffic_policy": types.BoolValue(mdDetails.UniformTrafficPolicy),
		})
		if diags.HasError() {
			return fmt.Errorf("failed building none object: %v", diags)
		}
		data.None = noneObj
		data.UCTV = types.ObjectNull(uctvAttrTypes)
		data.TappingMethod = types.StringValue("none")
	default:
		data.TappingMethod = types.StringNull()
		data.UCTV = types.ObjectNull(uctvAttrTypes)
		data.None = types.ObjectNull(noneAttrTypes)
	}

	if len(mdDetails.GetConnectionIds) != 0 {
		// Make TypedID from raw UUID received from FM
		typedID, err := commonutils.MakeTypedID(commonutils.ModuleConnection, commonutils.TypeAnyCloud, mdDetails.GetConnectionIds[0].Id)
		if err != nil {
			return err
		}
		data.ConnectionId = types.StringValue(typedID)
	} else {
		data.ConnectionId = types.StringNull()
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

	uctvSet := !data.UCTV.IsNull() && !data.UCTV.IsUnknown()
	noneSet := !data.None.IsNull() && !data.None.IsUnknown()

	if uctvSet == noneSet {
		resp.Diagnostics.AddError("Invalid configuration", "Exactly one of uctv or none must be specified.")
		return
	}

	var fmMDData any

	if uctvSet {
		var uctvAttr AnyCloudTappingMethodUCTVModel
		resp.Diagnostics.Append(data.UCTV.As(ctx, &uctvAttr, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}

		data.TappingMethod = types.StringValue("uctv")
		fmMDData = AnyCloudFmMDRequestUCTV{
			Alias:               data.Alias.ValueString(),
			Platform:            "anyCloud",
			UserLaunched:        true,
			DualStackPreferIPv6: uctvAttr.DualStackPreferIPv6.ValueBool(),
			MTU:                 uctvAttr.MTU.ValueInt32(),
		}
	} else {
		var noneAttr AnyCloudTappingMethodNoneModel
		resp.Diagnostics.Append(data.None.As(ctx, &noneAttr, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}

		data.TappingMethod = types.StringValue("none")
		fmMDData = AnyCloudFmMDRequestNone{
			Alias:                data.Alias.ValueString(),
			Platform:             "anyCloud",
			UserLaunched:         true,
			UniformTrafficPolicy: noneAttr.UniformTrafficPolicy.ValueBool(),
		}
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

	// Guard: connection_id must exist for update payload (FM expects it in connIds).
	if stateData.ConnectionId.IsNull() || stateData.ConnectionId.IsUnknown() || strings.TrimSpace(stateData.ConnectionId.ValueString()) == "" {
		resp.Diagnostics.AddError("Missing connection_id", "connection_id not available yet; create connection first.")
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

	uctvSet := !planData.UCTV.IsNull() && !planData.UCTV.IsUnknown()
	noneSet := !planData.None.IsNull() && !planData.None.IsUnknown()
	if uctvSet == noneSet {
		resp.Diagnostics.AddError("Invalid configuration", "Exactly one of uctv or none must be specified.")
		return
	}

	var fmMDData any

	if uctvSet {
		var uctvAttr AnyCloudTappingMethodUCTVModel
		resp.Diagnostics.Append(planData.UCTV.As(ctx, &uctvAttr, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}

		planData.TappingMethod = types.StringValue("uctv")
		fmMDData = struct {
			MonitoringDomains []AnyCloudFmMDRequestUCTV `json:"monitoringDomains"`
		}{
			MonitoringDomains: []AnyCloudFmMDRequestUCTV{
				{
					Platform:            stateData.Platform.ValueString(),
					ConnectionIds:       []string{connId},
					Id:                  mdId,
					DualStackPreferIPv6: uctvAttr.DualStackPreferIPv6.ValueBool(),
					MTU:                 uctvAttr.MTU.ValueInt32(),
				},
			},
		}
	} else {
		var noneAttr AnyCloudTappingMethodNoneModel
		resp.Diagnostics.Append(planData.None.As(ctx, &noneAttr, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}

		planData.TappingMethod = types.StringValue("none")
		fmMDData = struct {
			MonitoringDomains []AnyCloudFmMDRequestNone `json:"monitoringDomains"`
		}{
			MonitoringDomains: []AnyCloudFmMDRequestNone{
				{
					Platform:             stateData.Platform.ValueString(),
					ConnectionIds:        []string{connId},
					Id:                   mdId,
					UniformTrafficPolicy: noneAttr.UniformTrafficPolicy.ValueBool(),
				},
			},
		}
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

	// Preserve computed tapping_method derived from plan for state continuity.
	stateData.TappingMethod = planData.TappingMethod
	// Preserve nested objects from plan into state, then refresh FM values.
	stateData.UCTV = planData.UCTV
	stateData.None = planData.None

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
