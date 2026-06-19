//  Copyright (c) 2017-2026 Gigamon, Inc. All rights reserved.
//
//  Author: Gigamon Terraform Team (gigamon-terraform-team@gigamon.com)
//
//  This program is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, version 3 of the License.
//
//  This program is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with this program. If not, see <https://www.gnu.org/licenses/>

// Implements the functionality related to tapping_method as uctv which is needed in Monitoring Session

package commonresources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// UCT-V Filtering Rules
type FMUctVFilter struct {
	Name     string `json:"name,omitempty"`
	Relation string `json:"relation,omitempty"`
	Value    string `json:"value,omitempty"`
}

type FMUctVFilteringRule struct {
	RuleName  string         `json:"ruleName,omitempty"`
	Action    string         `json:"action,omitempty"`
	Direction string         `json:"direction,omitempty"`
	Priority  int64          `json:"priority,omitempty"`
	Filters   []FMUctVFilter `json:"filters,omitempty"`
}

type FMUctVFilteringPolicy struct {
	Rules []FMUctVFilteringRule `json:"rules,omitempty"`
}

type UctvFilterModel struct {
	Name     types.String `tfsdk:"name"`
	Relation types.String `tfsdk:"relation"`
	Value    types.String `tfsdk:"value"`
}

type UctvFilteringRuleModel struct {
	RuleName  types.String      `tfsdk:"rule_name"`
	Action    types.String      `tfsdk:"action"`
	Direction types.String      `tfsdk:"direction"`
	Priority  types.Int64       `tfsdk:"priority"`
	Filters   []UctvFilterModel `tfsdk:"filters"`
}

type UctvFilteringPolicyModel struct {
	Rules []UctvFilteringRuleModel `tfsdk:"rules"`
}

type FMMonSessUCTV struct {
	UCTVFilteringEnabled             bool `json:"uctVFilteringEnabled,omitempty"`
	UCTVMirrorTrafficEnabled         bool `json:"uctVMirrorTrafficEnabled,omitempty"`
	UCTVPrecryptionEnabled           bool `json:"uctVPrecryptionEnabled,omitempty"`
	UCTVPrecryptionFilteringEnabled  bool `json:"uctVPrecryptionFilteringEnabled,omitempty"`
	SecureTunnelOnMirrorEnabled      bool `json:"secureTunnelOnMirrorEnabled,omitempty"`
	SecureTunnelOnPrecryptionEnabled bool `json:"secureTunnelOnPrecryptionEnabled,omitempty"`

	// Mirror filtering rules
	UctVFilteringPolicy *FMUctVFilteringPolicy `json:"uctVFilteringPolicy,omitempty"`
}

// Traffic Acquisition models
type MirroringModel struct {
	MirroringFilteringEnabled types.Bool `tfsdk:"mirroring_filtering_enabled"`
	SecureTunnelsEnabled      types.Bool `tfsdk:"secure_tunnels_enabled"`
	// Filtering Rules
	UctvFilteringPolicy types.Object `tfsdk:"uctv_filtering_policy"`
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

// TF attr.Type trees for uctv_filtering_policy, needed for ObjectValue/ObjectNull
func uctvFilteringPolicyAttrTypes() (policyAttrTypes, ruleAttrTypes, filterAttrTypes map[string]attr.Type) {
	filterAttrTypes = map[string]attr.Type{
		"name":     types.StringType,
		"relation": types.StringType,
		"value":    types.StringType,
	}
	filterObjType := types.ObjectType{AttrTypes: filterAttrTypes}

	ruleAttrTypes = map[string]attr.Type{
		"rule_name": types.StringType,
		"action":    types.StringType,
		"direction": types.StringType,
		"priority":  types.Int64Type,
		"filters":   types.ListType{ElemType: filterObjType},
	}

	ruleObjType := types.ObjectType{AttrTypes: ruleAttrTypes}

	policyAttrTypes = map[string]attr.Type{
		"rules": types.ListType{ElemType: ruleObjType},
	}

	return policyAttrTypes, ruleAttrTypes, filterAttrTypes
}

func trafficAcqAttrTypes() (map[string]attr.Type, map[string]attr.Type, map[string]attr.Type) {
	policyAttrTypes, _, _ := uctvFilteringPolicyAttrTypes()
	policyObjType := types.ObjectType{AttrTypes: policyAttrTypes}

	mirrorAttrTypes := map[string]attr.Type{
		"secure_tunnels_enabled":      types.BoolType,
		"mirroring_filtering_enabled": types.BoolType,
		"uctv_filtering_policy":       policyObjType,
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

// UCT-V Filtering Schema
func UctvFilteringPolicyAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:            true,
		MarkdownDescription: "UCT-V filtering policy. If provided, rules must be 1..16 (no empty rules).",
		Attributes: map[string]schema.Attribute{
			"rules": schema.ListNestedAttribute{
				Required: true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(MinUctvPolicyRules),
					listvalidator.SizeAtMost(MaxUctvPolicyRules),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"rule_name": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.LengthBetween(MinRuleNameLen, MaxRuleNameLen),
								stringvalidator.RegexMatches(
									RuleNameRegex,
									"rule_name may contain only alphanumeric, underscore, and dash",
								),
							},
						},
						"action": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf(AllowedActions...),
							},
						},
						"direction": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf(AllowedDirections...),
							},
						},
						"priority": schema.Int64Attribute{
							Required: true,
							Validators: []validator.Int64{
								int64validator.AtLeast(MinRulePriority),
								int64validator.AtMost(MaxRulePriority),
							},
						},
						"filters": schema.ListNestedAttribute{
							Required: true,
							Validators: []validator.List{
								listvalidator.SizeAtLeast(MinUctvRuleFilters),
								listvalidator.SizeAtMost(MaxUctvRuleFilters),
							},
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Required: true,
										Validators: []validator.String{
											stringvalidator.OneOf(AllowedFilterNames...),
										},
									},
									"relation": schema.StringAttribute{
										Required: true,
										Validators: []validator.String{
											stringvalidator.OneOf(AllowedRelations...),
										},
									},
									"value": schema.StringAttribute{
										Required: true,
										Validators: []validator.String{
											stringvalidator.LengthAtLeast(1),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
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
						MarkdownDescription: "True if uctv_filtering_policy is non-empty, otherwise false.",
						Computed:            true,
					},
					"uctv_filtering_policy": UctvFilteringPolicyAttribute(),
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

func DeriveComputedAttributesFromPolicy(ctx context.Context, plan *MonSessModel) error {
	if plan.TrafficAcquisition.IsNull() || plan.TrafficAcquisition.IsUnknown() {
		return nil
	}

	// Decode traffic_acquisition
	var ta TrafficAcquisitionModel
	if diags := plan.TrafficAcquisition.As(ctx, &ta, basetypes.ObjectAsOptions{}); diags.HasError() {
		return fmt.Errorf("Invalid traffic_acquisition")
	}

	if ta.Mirroring.IsNull() || ta.Mirroring.IsUnknown() {
		return nil
	}

	var mirror MirroringModel
	if diags := ta.Mirroring.As(ctx, &mirror, basetypes.ObjectAsOptions{}); diags.HasError() {
		return fmt.Errorf("Invalid traffic_acquisition.mirroring")
	}

	filteringEnabled := false
	if !mirror.UctvFilteringPolicy.IsNull() && !mirror.UctvFilteringPolicy.IsUnknown() {
		var pol UctvFilteringPolicyModel
		if diags := mirror.UctvFilteringPolicy.As(ctx, &pol, basetypes.ObjectAsOptions{}); diags.HasError() {
			return fmt.Errorf("invalid traffic_acquisition.mirroring.uctv_filtering_policy")
		}
		filteringEnabled = len(pol.Rules) > 0
	}

	// Rebuild the mirroring object with computed field set
	policyAttrTypes, _, _ := uctvFilteringPolicyAttrTypes()
	policyNull := types.ObjectNull(policyAttrTypes)

	mirrorAttrTypes, precryptionAttrTypes, taAttrTypes := trafficAcqAttrTypes()

	polObj := mirror.UctvFilteringPolicy
	if polObj.IsNull() || polObj.IsUnknown() {
		polObj = policyNull
	}

	mVals := map[string]attr.Value{
		"secure_tunnels_enabled":      mirror.SecureTunnelsEnabled,
		"mirroring_filtering_enabled": types.BoolValue(filteringEnabled),
		"uctv_filtering_policy":       polObj,
	}
	mirObj, d := types.ObjectValue(mirrorAttrTypes, mVals)
	if d.HasError() {
		return fmt.Errorf("failed building mirroring object: %v", d)
	}

	// Keep precryption as-is
	preObj := ta.Precryption
	if preObj.IsNull() || preObj.IsUnknown() {
		preObj = types.ObjectNull(precryptionAttrTypes)
	}

	taVals := map[string]attr.Value{
		"mirroring":   mirObj,
		"precryption": preObj,
	}
	taObj, d2 := types.ObjectValue(taAttrTypes, taVals)
	if d2.HasError() {
		return fmt.Errorf("failed building traffic_acquisition object: %v", d2)
	}

	plan.TrafficAcquisition = taObj
	return nil
}

// ValidateProtoFilterValues checks that when a filter's name is "proto",
// the value is one of the allowed proto values (TCP, UDP).
func ValidateProtoFilterValues(ctx context.Context, plan MonSessModel) error {
	if plan.TrafficAcquisition.IsNull() || plan.TrafficAcquisition.IsUnknown() {
		return nil
	}

	var ta TrafficAcquisitionModel
	if diags := plan.TrafficAcquisition.As(ctx, &ta, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil // other validators will catch this
	}

	if ta.Mirroring.IsNull() || ta.Mirroring.IsUnknown() {
		return nil
	}

	var mirror MirroringModel
	if diags := ta.Mirroring.As(ctx, &mirror, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil
	}

	if mirror.UctvFilteringPolicy.IsNull() || mirror.UctvFilteringPolicy.IsUnknown() {
		return nil
	}

	var pol UctvFilteringPolicyModel
	if diags := mirror.UctvFilteringPolicy.As(ctx, &pol, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil
	}

	for _, r := range pol.Rules {
		for _, f := range r.Filters {
			if f.Name.ValueString() == FilterProto {
				val := f.Value.ValueString()
				if _, ok := ProtoToFM[val]; !ok {
					return fmt.Errorf(
						"rule %q: proto filter value must be one of %v, got %q",
						r.RuleName.ValueString(), AllowedProtoValues, val,
					)
				}
			}
		}
	}

	return nil
}

// Compute Traffic Acquisition base attributes with default values, and computed based on configuration
// If traffic_acquisition is present,  all 6 attributes are needed in payload
func computeTrafficAcquisitionDefaultAttributes() map[string]any {
	return map[string]any{
		FMUctVMirrorTrafficEnabledKey:         true, // if tapping_method is uctv, mirroring is enabled by default
		FMUctVFilteringEnabledKey:             false,
		FMSecureTunnelOnPrecryptionEnabledKey: false,
		FMUctVPrecryptionEnabledKey:           false,
		FMUctVPrecryptionFilteringEnabledKey:  false,
		FMSecureTunnelOnMirrorEnabledKey:      false,
	}
}

// Returns true if Traffic Acquisition attributes are at default for tapping_method = uctv
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

func buildFilteringPolicy(ctx context.Context, polObj types.Object) (map[string]any, int, error) {
	if polObj.IsNull() || polObj.IsUnknown() {
		return nil, 0, nil
	}

	var pol UctvFilteringPolicyModel
	if diags := polObj.As(ctx, &pol, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, 0, fmt.Errorf("invalid uctv_filtering_policy")
	}

	rules := make([]any, 0, len(pol.Rules))
	for _, r := range pol.Rules {
		filters := make([]any, 0, len(r.Filters))
		for _, f := range r.Filters {
			fValue := f.Value.ValueString()
			// Convert user-facing proto names (TCP/UDP) to FM numeric values
			if f.Name.ValueString() == FilterProto {
				fmVal, ok := ProtoToFM[fValue]
				if !ok {
					return nil, 0, fmt.Errorf("unsupported proto filter value %q; allowed values: %v", fValue, AllowedProtoValues)
				}
				fValue = fmVal
			}
			filters = append(filters, map[string]any{
				FMFilterNameKey:     f.Name.ValueString(),
				FMFilterRelationKey: f.Relation.ValueString(),
				FMFilterValueKey:    fValue,
			})
		}
		rules = append(rules, map[string]any{
			FMRuleNameKey:  r.RuleName.ValueString(),
			FMActionKey:    r.Action.ValueString(),
			FMDirectionKey: r.Direction.ValueString(),
			FMPriorityKey:  r.Priority.ValueInt64(),
			FMFiltersKey:   filters,
		})
	}

	return map[string]any{
		FMRulesKey: rules,
	}, len(pol.Rules), nil
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

	attrs := map[string]any{
		FMUctVMirrorTrafficEnabledKey:    true,
		FMSecureTunnelOnMirrorEnabledKey: secureTunnelsEnabled,
	}

	policyMap, ruleCount, err := buildFilteringPolicy(ctx, mirroringAttrs.UctvFilteringPolicy)
	if err != nil {
		return nil, err
	}

	if ruleCount > 0 {
		attrs[FMUctVFilteringEnabledKey] = true
		attrs[FMUctVFilteringPolicyKey] = policyMap
	} else {
		attrs[FMUctVFilteringEnabledKey] = false
		// omit FMUctVFilteringPolicyKey
	}

	return attrs, nil
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
		FMUctVPrecryptionEnabledKey:           true,
		FMUctVPrecryptionFilteringEnabledKey:  false,
		FMSecureTunnelOnPrecryptionEnabledKey: secureTunnelsEnabled,
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
	taAttrs[FMUctVMirrorTrafficEnabledKey] = false

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

	policyAttrTypes, ruleAttrTypes, filterAttrTypes := uctvFilteringPolicyAttrTypes()
	filterObjType := types.ObjectType{AttrTypes: filterAttrTypes}
	ruleObjType := types.ObjectType{AttrTypes: ruleAttrTypes}

	mirrorAttrTypes, precryptionAttrTypes, taAttrTypes := trafficAcqAttrTypes()

	// If neither is enabled in FM, treat TA as absent
	if !fm.UCTVMirrorTrafficEnabled && !fm.UCTVPrecryptionEnabled {
		return types.ObjectNull(taAttrTypes), diags
	}

	// Mirroring block only if enabled
	var mirroringObj types.Object
	if fm.UCTVMirrorTrafficEnabled {

		// Build policy object (present only when enabled; if nil, keep null)
		policyObj := types.ObjectNull(policyAttrTypes)

		if fm.UctVFilteringPolicy != nil {

			// Build rules list
			ruleVals := make([]attr.Value, 0, len(fm.UctVFilteringPolicy.Rules))
			for _, r := range fm.UctVFilteringPolicy.Rules {

				// Build filters list for this rule
				filterVals := make([]attr.Value, 0, len(r.Filters))
				for _, f := range r.Filters {
					fValue := f.Value
					// Convert FM numeric proto values (6/17) to user-facing names (TCP/UDP)
					if f.Name == FilterProto {
						tfVal, ok := ProtoFromFM[fValue]
						if !ok {
							diags.AddError("Unsupported proto value from FM",
								fmt.Sprintf("FM returned unknown proto filter value %q; expected one of: %s, %s", fValue, FMProtoTCPNumber, FMProtoUDPNumber))
							return types.ObjectNull(taAttrTypes), diags
						}
						fValue = tfVal
					}
					fObj, d := types.ObjectValue(filterAttrTypes, map[string]attr.Value{
						"name":     types.StringValue(f.Name),
						"relation": types.StringValue(f.Relation),
						"value":    types.StringValue(fValue),
					})
					diags.Append(d...)
					filterVals = append(filterVals, fObj)
				}

				fList, d := types.ListValue(filterObjType, filterVals)
				diags.Append(d...)

				rObj, d := types.ObjectValue(ruleAttrTypes, map[string]attr.Value{
					"rule_name": types.StringValue(r.RuleName),
					"action":    types.StringValue(r.Action),
					"direction": types.StringValue(r.Direction),
					"priority":  types.Int64Value(r.Priority),
					"filters":   fList,
				})
				diags.Append(d...)
				ruleVals = append(ruleVals, rObj)
			}

			rList, d := types.ListValue(ruleObjType, ruleVals)
			diags.Append(d...)

			policyObj, d = types.ObjectValue(policyAttrTypes, map[string]attr.Value{
				"rules": rList,
			})
			diags.Append(d...)
		}

		mVals := map[string]attr.Value{
			"secure_tunnels_enabled":      types.BoolValue(fm.SecureTunnelOnMirrorEnabled),
			"mirroring_filtering_enabled": types.BoolValue(fm.UCTVFilteringEnabled),
			"uctv_filtering_policy":       policyObj,
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

// Called from Monitoring Session Read/Create/Update
func ComputeTrafficAcquisitionStateFromFM(
	tappingMethod types.String,
	fmResp FMMonSess,
	configuredTA types.Object,
	preserveConfiguredTA bool,
) (types.Object, diag.Diagnostics) {
	_, _, taAttrTypes := trafficAcqAttrTypes()

	if tappingMethod.IsNull() || tappingMethod.IsUnknown() || tappingMethod.ValueString() != "uctv" {
		return types.ObjectNull(taAttrTypes), nil
	}

	if areTrafficAcquisitionAtDefaults(fmResp) {
		if preserveConfiguredTA && isObjectPresent(configuredTA) {
			return configuredTA, nil
		}
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
