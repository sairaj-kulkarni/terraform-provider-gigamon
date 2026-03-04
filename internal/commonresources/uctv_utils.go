// Copyright (c) Gigamon, Inc.

// Implements the functionality related to tapping_method as uctv which is needed in Monitoring Session

package commonresources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

type FMMonSessUCTV struct {
	UCTVFilteringEnabled             bool `json:"uctVFilteringEnabled,omitempty"`
	UCTVMirrorTrafficEnabled         bool `json:"uctVMirrorTrafficEnabled,omitempty"`
	UCTVPrecryptionEnabled           bool `json:"uctVPrecryptionEnabled,omitempty"`
	UCTVPrecryptionFilteringEnabled  bool `json:"uctVPrecryptionFilteringEnabled,omitempty"`
	SecureTunnelOnMirrorEnabled      bool `json:"secureTunnelOnMirrorEnabled,omitempty"`
	SecureTunnelOnPrecryptionEnabled bool `json:"secureTunnelOnPrecryptionEnabled,omitempty"`
}

// Traffic Acquisition models
type MirroringModel struct {
	MirroringFilteringEnabled types.Bool `tfsdk:"mirroring_filtering_enabled"`
	SecureTunnelsEnabled      types.Bool `tfsdk:"secure_tunnels_enabled"`
	// Filtering Rules
}

type PrecryptionModel struct {
	PrecryptionFilteringEnabled types.Bool `tfsdk:"precryption_filtering_enabled"`
	SecureTunnelsEnabled        types.Bool `tfsdk:"secure_tunnels_enabled"`
	// Filtering Rules
	// AppRules
}

type TrafficAcquisitionModel struct {
	Mirroring   types.Object `tfsdk:"mirroring"`
	Precryption types.Object `tfsdk:"precryption"`
}

func trafficAcqAttrTypes() (map[string]attr.Type, map[string]attr.Type, map[string]attr.Type) {
	mirrorAttrTypes := map[string]attr.Type{
		"secure_tunnels_enabled":      types.BoolType,
		"mirroring_filtering_enabled": types.BoolType,
	}
	precryptionAttrTypes := map[string]attr.Type{
		"secure_tunnels_enabled":        types.BoolType,
		"precryption_filtering_enabled": types.BoolType,
	}
	taAttrTypes := map[string]attr.Type{
		"mirroring":   types.ObjectType{AttrTypes: mirrorAttrTypes},
		"precryption": types.ObjectType{AttrTypes: precryptionAttrTypes},
	}
	return mirrorAttrTypes, precryptionAttrTypes, taAttrTypes
}

// TrafficAcquisitionSchemaAttribute returns the nested schema for traffic_acquisition
func TrafficAcquisitionSchemaAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: "Optional traffic acquisition configuration. If set, at least one of mirroring/precryption must be configured. Both are allowed.",
		Optional:            true,
		Attributes: map[string]schema.Attribute{
			"mirroring": schema.SingleNestedAttribute{
				MarkdownDescription: "UCT-V Mirroring traffic acquisition.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"mirroring_filtering_enabled": schema.BoolAttribute{
						MarkdownDescription: "Is filtering enabled for UCTV Mirroring.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
					"secure_tunnels_enabled": schema.BoolAttribute{
						MarkdownDescription: "Enable/disable Secure Tunnels for mirroring.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
				},
			},
			"precryption": schema.SingleNestedAttribute{
				MarkdownDescription: "Precryption traffic acquisition",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"precryption_filtering_enabled": schema.BoolAttribute{
						MarkdownDescription: "Is filtering enabled for Precryption.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
					"secure_tunnels_enabled": schema.BoolAttribute{
						MarkdownDescription: "Enable/disable Secure Tunnels for precryption.",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
				},
			},
		},
	}
}

// ValidateTrafficAcquisitionConfig enforces ModifyPlan gating for traffic_acquisition.
// - If traffic_acquisition is null/unknown => no gating
// - If traffic_acquisition is set => tapping_method must be "uctv"
// - Must configure at least one of mirroring/precryption
func ValidateTrafficAcquisitionConfig(
	ctx context.Context,
	req resource.ModifyPlanRequest,
	resp *resource.ModifyPlanResponse,
	plan MonSessModel,
) {
	// Only gate when traffic_acquisition is configured
	if plan.TrafficAcquisition.IsNull() || plan.TrafficAcquisition.IsUnknown() {
		return
	}

	if plan.TappingMethod.IsNull() || plan.TappingMethod.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("tapping_method"),
			"tapping_method is required when traffic_acquisition is set",
			"Set tapping_method as uctv or remove traffic_acquisition.",
		)
		return
	}

	if plan.TappingMethod.ValueString() != "uctv" {
		resp.Diagnostics.AddAttributeError(
			path.Root("traffic_acquisition"),
			"traffic_acquisition is only supported for UCTV",
			fmt.Sprintf(
				"tapping_method is %q; set tapping_method = \"uctv\" or remove traffic_acquisition.",
				plan.TappingMethod.ValueString(),
			),
		)
		return
	}

	// Validate at least one of mirroring/precryption is set
	var mirroring types.Object
	var precryption types.Object

	resp.Diagnostics.Append(
		req.Config.GetAttribute(ctx, path.Root("traffic_acquisition").AtName("mirroring"), &mirroring)...,
	)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(
		req.Config.GetAttribute(ctx, path.Root("traffic_acquisition").AtName("precryption"), &precryption)...,
	)
	if resp.Diagnostics.HasError() {
		return
	}

	mirrSet := !mirroring.IsNull() && !mirroring.IsUnknown()
	precSet := !precryption.IsNull() && !precryption.IsUnknown()

	if !mirrSet && !precSet {
		resp.Diagnostics.AddAttributeError(
			path.Root("traffic_acquisition"),
			"Invalid traffic_acquisition",
			"traffic_acquisition is set, but neither mirroring nor precryption is configured. Set at least one.",
		)
		return
	}
}

// Compute Traffic Acquisition base attributes with default values, and computed based on configuration
// If traffic_acquisition is present,  all 6 attributes are needed in payload
func computeTrafficAcquisitionDefaultAttributes() map[string]any {
	return map[string]any{
		"uctVMirrorTrafficEnabled":         true, // if tapping_method is uctv, mirroring is enabled by default
		"uctVFilteringEnabled":             false,
		"secureTunnelOnPrecryptionEnabled": false,
		"uctVPrecryptionEnabled":           false,
		"uctVPrecryptionFilteringEnabled":  false,
		"secureTunnelOnMirrorEnabled":      false,
	}
}

// Returns true if Traffic Acquistion attributes are at default for tapping_method = uctv
func areTrafficAcquisitionAtDefaults(fm FMMonSess) bool {
	return fm.UCTVMirrorTrafficEnabled &&
		!fm.UCTVFilteringEnabled &&
		!fm.UCTVPrecryptionEnabled &&
		!fm.UCTVPrecryptionFilteringEnabled &&
		!fm.SecureTunnelOnMirrorEnabled &&
		!fm.SecureTunnelOnPrecryptionEnabled
}

func isObjectPresent(obj types.Object) bool {
	return !obj.IsNull() && !obj.IsUnknown()
}

// Computes UCT-V mirroring related payload keys
func computeMirroringAttributes(ctx context.Context, mirroringObj types.Object) (map[string]any, error) {
	var mirroringAttrs MirroringModel
	diags := mirroringObj.As(ctx, &mirroringAttrs, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, fmt.Errorf("invalid traffic_acquisition.mirroring block")
	}

	secureTunnelsEnabled := !mirroringAttrs.SecureTunnelsEnabled.IsNull() &&
		!mirroringAttrs.SecureTunnelsEnabled.IsUnknown() &&
		mirroringAttrs.SecureTunnelsEnabled.ValueBool()

	// Filtering to be added
	return map[string]any{
		"uctVMirrorTrafficEnabled":    true,
		"uctVFilteringEnabled":        false,
		"secureTunnelOnMirrorEnabled": secureTunnelsEnabled,
	}, nil
}

// Computes Precryption related payload keys
func computePrecryptionAttributes(ctx context.Context, precryptionObj types.Object) (map[string]any, error) {
	var precryptionAttrs PrecryptionModel
	diags := precryptionObj.As(ctx, &precryptionAttrs, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, fmt.Errorf("invalid traffic_acquisition.precryption block")
	}

	secureTunnelsEnabled := !precryptionAttrs.SecureTunnelsEnabled.IsNull() &&
		!precryptionAttrs.SecureTunnelsEnabled.IsUnknown() &&
		precryptionAttrs.SecureTunnelsEnabled.ValueBool()

	// Filtering to be added
	return map[string]any{
		"uctVPrecryptionEnabled":           true,
		"uctVPrecryptionFilteringEnabled":  false,
		"secureTunnelOnPrecryptionEnabled": secureTunnelsEnabled,
	}, nil
}

func merge(dst, src map[string]any) {
	for key, value := range src {
		dst[key] = value
	}
}

// Computes mirroring and precryption keys independently
// It returns a map containing base attributes and mode specific overrides
func computeTrafficAcquisitionAttributes(ctx context.Context, taObj types.Object) (map[string]any, error) {
	if taObj.IsNull() || taObj.IsUnknown() {
		return nil, nil
	}

	var ta TrafficAcquisitionModel
	diags := taObj.As(ctx, &ta, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, fmt.Errorf("invalid traffic_acquisition block")
	}

	mirrorSet := isObjectPresent(ta.Mirroring)
	precryptionSet := isObjectPresent(ta.Precryption)

	// Compute traffic acquisition default attributes with default values
	taAttrs := computeTrafficAcquisitionDefaultAttributes()

	// Set all attributes to false
	taAttrs["uctVMirrorTrafficEnabled"] = false

	if mirrorSet {
		mirrorAttrs, err := computeMirroringAttributes(ctx, ta.Mirroring)
		if err != nil {
			return nil, err
		}
		merge(taAttrs, mirrorAttrs)
	}

	if precryptionSet {
		precryptionAttrs, err := computePrecryptionAttributes(ctx, ta.Precryption)
		if err != nil {
			return nil, err
		}
		merge(taAttrs, precryptionAttrs)
	}

	return taAttrs, nil
}

func buildTrafficAcquisitionFromFM(
	fm FMMonSess,
) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	mirrorAttrTypes, precryptionAttrTypes, taAttrTypes := trafficAcqAttrTypes()

	// If neither is enabled in FM, treat TA as absent
	if !fm.UCTVMirrorTrafficEnabled && !fm.UCTVPrecryptionEnabled {
		return types.ObjectNull(taAttrTypes), diags
	}

	// Mirroring block only if enabled
	var mirroringObj types.Object
	if fm.UCTVMirrorTrafficEnabled {
		mVals := map[string]attr.Value{
			"secure_tunnels_enabled":      types.BoolValue(fm.SecureTunnelOnMirrorEnabled),
			"mirroring_filtering_enabled": types.BoolValue(fm.UCTVFilteringEnabled),
		}
		o, d := types.ObjectValue(mirrorAttrTypes, mVals)
		diags.Append(d...)
		mirroringObj = o
	} else {
		mirroringObj = types.ObjectNull(mirrorAttrTypes)
	}

	// Precryption block only if enabled
	var precryptionObj types.Object
	if fm.UCTVPrecryptionEnabled {
		pVals := map[string]attr.Value{
			"secure_tunnels_enabled":        types.BoolValue(fm.SecureTunnelOnPrecryptionEnabled),
			"precryption_filtering_enabled": types.BoolValue(fm.UCTVPrecryptionFilteringEnabled),
		}
		o, d := types.ObjectValue(precryptionAttrTypes, pVals)
		diags.Append(d...)
		precryptionObj = o
	} else {
		precryptionObj = types.ObjectNull(precryptionAttrTypes)
	}

	taVals := map[string]attr.Value{
		"mirroring":   mirroringObj,
		"precryption": precryptionObj,
	}
	taObj, d := types.ObjectValue(taAttrTypes, taVals)
	diags.Append(d...)

	return taObj, diags
}

// Called from Monitoring Session Create
func AddTrafficAcquisitionIntoPayload(
	ctx context.Context,
	payload map[string]any,
	taObj types.Object,
) error {

	if !isObjectPresent(taObj) {
		return nil
	}

	taPayload, err := computeTrafficAcquisitionAttributes(ctx, taObj)
	if err != nil {
		return err
	}

	merge(payload, taPayload)
	return nil
}

// Called from Monitoring Session Read
func ComputeTrafficAcquisitionStateFromFM(
	tappingMethod types.String,
	fmResp FMMonSess,
) (types.Object, diag.Diagnostics) {
	_, _, taAttrTypes := trafficAcqAttrTypes()

	if tappingMethod.IsNull() || tappingMethod.IsUnknown() || tappingMethod.ValueString() != "uctv" {
		return types.ObjectNull(taAttrTypes), nil
	}

	if areTrafficAcquisitionAtDefaults(fmResp) {
		return types.ObjectNull(taAttrTypes), nil
	}

	// Build TA attributes from FM Response and return
	return buildTrafficAcquisitionFromFM(fmResp)
}

// Called from Monitoring Session Update
func ApplyTrafficAcquisitionUpdatesToPayload(
	ctx context.Context,
	payload map[string]any,
	planTA types.Object,
	stateTA types.Object,
	planTappingMethod types.String,
) error {
	taIsSet := isObjectPresent(planTA)
	taWasSet := isObjectPresent(stateTA)

	// If traffic_acquisition is present, compute and include mirroring and precryption attributes
	if taIsSet {
		taPayload, err := computeTrafficAcquisitionAttributes(ctx, planTA)
		if err != nil {
			return err
		}
		merge(payload, taPayload)
		return nil
	}

	// If traffic_acquisition is removed and tapping method is uctv, set attributes to defaults
	if taWasSet &&
		!planTappingMethod.IsNull() && !planTappingMethod.IsUnknown() &&
		planTappingMethod.ValueString() == "uctv" {
		merge(payload, computeTrafficAcquisitionDefaultAttributes())
	}

	// traffic_acquisition is not present in state and plan
	return nil
}
