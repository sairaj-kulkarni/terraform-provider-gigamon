// Copyright (c) Gigamon, Inc.

// Implements the various map rule types that we support and also the conversion from
// TF to Golang struct

package commonresources

import (
	"context"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/path"
)

// TF Model for the various rules. The TF Schema model does not directly map into the
// swagger, because it is very difficult to have a common name like "value" or "subset" for
// the user configurable fields. Rather they should be called vlan_id, src_mac_address
// etc. Also for range it is better to speficy the start and end etc. Hence the TF model
// is more tuned towards user consumption and then the provider code has to convert these
// to the actual go structs before doing the JSON encode/decode to send/receive from FM

// Ethernet Type match rules
type EtherTypeModel struct {
	Type           types.String `tfsdk:"type"`
	Pos            types.Int32  `tfsdk:"nested_level_count"`
	EtherType      types.Int32  `tfsdk:"ether_type"`
	EtherTypeStart types.Int32  `tfsdk:"ether_type_start"`
	EtherTypeEnd   types.Int32  `tfsdk:"ether_type_end"`
}

// Match on L2 SRC/DST MAC
type L2MacAddrModel struct {
	Type         types.String `tfsdk:"type"`
	Pos          types.Int32  `tfsdk:"nested_level_count"`
	SrcAddr      types.String `tfsdk:"source_address"`
	SrcAddrStart types.String `tfsdk:"source_address_start"`
	SrcAddrEnd   types.String `tfsdk:"source_address_end"`
	SrcAddrMask  types.String `tfsdk:"source_address_mask"`
}

// The model for the rules, which is a combination of a rule ID and a set of rules from the
// above rules
type RulesModel struct {
	EtherType  *EtherTypeModel `tfsdk:"ether_type"`
	L2SrcMac *L2MacAddrModel `tfsdk:"l2_src_mac"`
	L2DstMac *L2MacAddrModel `tfsdk:"l2_dst_mac"`
}

type RuleGroupModel struct {
	RuleId  types.Int32     `tfsdk:"rule_id"`
	Rules []RulesModel `tfsdk:"rules"`
}

// RuleSetModel which is a ruleset, which contains a rule set ID, the aepID which is used
// to direct the traffic hitting thi ruleset, and the actual pass/drop rules of this ruleset
type RuleSetModel struct {
	RuleSetId types.Int32  `tfsdk:"rule_set_id"`
	Priority  types.Int32  `tfsdk:"priority"`
	AepId     types.Int32  `tfsdk:"aep_id"`
	PassRules []RuleGroupModel `tfsdk:"pass_rules"`
	DropRules []RuleGroupModel `tfsdk:"drop_rules"`
}

// MapModel, consists of a set of rulesets and an ID that is got from FM
type MapModel struct {
	Comment  types.String   `tfsdk:"comment"`
	Enable   types.Bool     `tfsdk:"enable"`
	RuleSets []RuleSetModel `tfsdk:"rule_sets"`
	Id       types.String   `tfsdk:"id"`
}

// GO Struct for the rules
type EtherType struct {
	Type     string `json:"type"`
	Pos      int32    `json:"pos,omitempty"`
	Value    int32   `json:"value"`
	ValueMax int32    `json:"valueMax,omitempty"`
}

type L2MacAddr struct {
	Type     string `json:"type"`
	Pos      int32    `json:"pos,omitempty"`
	Value    string `json:"value"`
	ValueMax string `json:"valueMax,omitempty"`
	Mask     string `json:"Mask,omitempty"`
}

// This is the rules struct which will be used in the match field of the rules. We use
// embedded struct to ensure that when we marshall the json, we do not add any new layer
// and the fields in this are promoted to the outer layer

// Json marshalling will be default omit the field if the reference is nil

type RuleGroups struct {
	RuleId  int32        `json:"ruleId"`
	Matches []any `json:"matches"`
}

type RuleSet struct {
	RuleSetId int32     `json:"ruleSetId"`
	Priority  int32     `json:"priority"`
	AepId     int32     `json:"aepid"`
	PassRules []RuleGroups `json:"passRules"`
	DropRules []RuleGroups `json:"dropRules"`
}

type MapGo struct {
	Comment  string    `json:"comment,omitempty"`
	Enable   bool      `json:"enable,omitempty"`
	RuleSets []RuleSet `json:"ruleSets"`
}

// Definition of our Rules Schema

func EtherTypeSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
		    "type": schema.StringAttribute{
			    MarkdownDescription: "The type of the rule. Auto specified not configured by user",
			    Computed:            true,
			    Default:             stringdefault.StaticString("etherType"),
		    },
		    "nested_level_count": schema.Int32Attribute{
			    MarkdownDescription: "In case of multi-VLAN tagged packet, the level at which to match the Ethertype/TPID. 0 implies any position",
			    Optional:            true,
			    Computed:            true,
			    Default:             int32default.StaticInt32(0),
		    },
		    "ether_type": schema.Int32Attribute{
			    MarkdownDescription: "The value of the ether type byte to match",
			    Optional:            true,
				Validators: []validator.Int32{
					int32validator.ConflictsWith(path.Expressions{
						path.MatchRelative().AtParent().AtName("ether_type_start"),
					}...),
				},
		    },
		    "ether_type_start": schema.Int32Attribute{
			    MarkdownDescription: "The start range of the ether type to match",
			    Optional:            true,
		    },
		    "ether_type_end": schema.Int32Attribute{
			    MarkdownDescription: "The end range of the ether type byte to match",
			    Optional:            true,
				Validators: []validator.Int32{
					int32validator.AlsoRequires(path.Expressions{
						path.MatchRelative().AtParent().AtName("ether_type_start"),
					}...),
				},
		    },
		},
	}
}

func L2MacSchema(macType string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
		    "type": schema.StringAttribute{
			    MarkdownDescription: "The type of the rule. Auto specified not configured by user",
			    Computed:            true,
			    Default:             stringdefault.StaticString(macType),
		    },
		    "nested_level_count": schema.Int32Attribute{
			    MarkdownDescription: "In case of MAC-in-MAC packet, the level at which to match the MAC Address. 0 implies any position",
			    Optional:            true,
			    Computed:            true,
			    Default:             int32default.StaticInt32(0),
		    },
		    "source_address": schema.StringAttribute{
			    MarkdownDescription: "The value of the MAC Address to match",
			    Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$`),
						"must be a valid MAC address format (e.g., 00:1A:2B:3C:4D:5E)",
					),
				},
		    },
		    "source_address_mask": schema.StringAttribute{
			    MarkdownDescription: "If specified this is applied to source_address to get the range of MAC addresses to match",
			    Optional:            true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.Expressions{
						path.MatchRelative().AtParent().AtName("source_address"),
					}...),
				},
		    },
		    "source_address_start": schema.StringAttribute{
			    MarkdownDescription: "The start range of the MAC Address to match",
			    Optional:            true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRelative().AtParent().AtName("source_address"),
					}...),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$`),
						"must be a valid MAC address format (e.g., 00:1A:2B:3C:4D:5E)",
					),
				},
		    },
		    "source_address_end": schema.StringAttribute{
			    MarkdownDescription: "The end range of the MAC Address to match",
			    Optional:            true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.Expressions{
						path.MatchRelative().AtParent().AtName("source_address_start"),
					}...),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$`),
						"must be a valid MAC address format (e.g., 00:1A:2B:3C:4D:5E)",
					),
				},
		    },
	    },
	}
}

// Comibine all the above rule schemas into a map rule schema.
func MatchesSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"ethernet_type": EtherTypeSchema(),
			"l2_src_mac":    L2MacSchema("macSrc"),
			"l2_dst_mac":    L2MacSchema("macDst"),
		},
	}
}

func RulesSchema() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"ether_type": EtherTypeSchema(),
			"l2_src_mac": L2MacSchema("macSrc"),
			"l2_dst_mac": L2MacSchema("macDst"),
		},
	}
}

func RuleGroupSchema() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"rule_id": schema.Int32Attribute{
				MarkdownDescription: "ID of this rule set, 1-5",
				Required:            true,
			},
			"rules": schema.ListNestedAttribute{
				MarkdownDescription: "List of rules for this pass/drop set",
				Required: true,
				NestedObject: RulesSchema(),
		     },
		 },
	}
}

// Define the schema for the RuleSet which is a nested object within the map schema
func RuleSetSchema() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"rule_set_id": schema.Int32Attribute{
				MarkdownDescription: "ID of this rule set, 1-5",
				Required:            true,
				Validators: []validator.Int32{
					int32validator.AtLeast(1),
					int32validator.AtMost(5),
				},
			},
			"priority": schema.Int32Attribute{
				MarkdownDescription: "Priority of this rule, the lower the number higher the priority",
				Required:            true,
				Validators: []validator.Int32{
					int32validator.AtLeast(1),
					int32validator.AtMost(5),
				},
			},
			"aep_id": schema.Int32Attribute{
				MarkdownDescription: "the AEP endpoint ID for this ruleset. Used to connect the output of this to the tool/app using the link object",
				Required:            true,
				Validators: []validator.Int32{
					int32validator.AtLeast(1),
					int32validator.AtMost(63),
				},
			},
			"pass_rules": schema.ListNestedAttribute{
				MarkdownDescription: "List of pass rules for this map",
				Optional:            true,
				NestedObject:        RuleGroupSchema(),
				Validators: []validator.List{
					listvalidator.AtLeastOneOf(path.Expressions{
						path.MatchRelative().AtParent().AtName("drop_rules"),
					}...),
				},
			},
			"drop_rules": schema.ListNestedAttribute{
				MarkdownDescription: "List of drop rules for this map",
				Optional:            true,
				NestedObject:        RuleGroupSchema(),
			},
		},
	}
}

// Finally the complete MAP schema
func MapSchema() schema.Schema {
	return schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon Map Schema",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this Monitoring Session for later use",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"comment": schema.StringAttribute{
				MarkdownDescription: "Comment for this map",
				Optional:            true,
			},
			"enable": schema.BoolAttribute{
				MarkdownDescription: "Whether this map is enabled or not",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"rule_sets": schema.ListNestedAttribute{
				MarkdownDescription: "List of rule sets in this map",
				Required:            true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
					listvalidator.SizeAtMost(5),
				},
				NestedObject: RuleSetSchema(),
			},
		},
	}
}

func ModelEtherTypeToGo (ctx context.Context, etherModel *EtherTypeModel) *EtherType {
	etherType := EtherType{
		Type: etherModel.Type.ValueString(),
		Pos: etherModel.Pos.ValueInt32(),
	}
	if etherModel.EtherType.ValueInt32() != 0 {
		etherType.Value = etherModel.EtherType.ValueInt32()
	} else {
		etherType.Value = etherModel.EtherTypeStart.ValueInt32()
		etherType.ValueMax = etherModel.EtherTypeEnd.ValueInt32()
	}
		
	return &etherType
}

func ModelL2MacToGo(ctx context.Context, l2MacModel *L2MacAddrModel) *L2MacAddr {
	l2MacAddr := L2MacAddr {
		Type: l2MacModel.Type.ValueString(),
		Pos: l2MacModel.Pos.ValueInt32(),
	}
	if l2MacModel.SrcAddr.ValueString() != "" {
		l2MacAddr.Value = l2MacModel.SrcAddr.ValueString()
		if l2MacModel.SrcAddrMask.ValueString() != "" {
			l2MacAddr.Mask = l2MacModel.SrcAddrMask.ValueString()
		}
	} else {
		l2MacAddr.Value = l2MacModel.SrcAddrStart.ValueString()
		l2MacAddr.ValueMax = l2MacModel.SrcAddrEnd.ValueString()
	}
	return &l2MacAddr
}


func updateGoRules (ctx context.Context, ruleGroupModel *RuleGroupModel) RuleGroups {
	goRuleGroups := RuleGroups {
		RuleId: ruleGroupModel.RuleId.ValueInt32(),
	}

	for _, ruleModel := range ruleGroupModel.Rules {
		goRule := make([]any,0)
	    if ruleModel.EtherType != nil {
			goRule = append(goRule, ModelEtherTypeToGo(ctx, ruleModel.EtherType))
	    }

	    if ruleModel.L2SrcMac != nil {
		    goRule = append(goRule, ModelL2MacToGo(ctx, ruleModel.L2SrcMac))
	    }

	    if ruleModel.L2DstMac != nil {
		    goRule = append(goRule, ModelL2MacToGo(ctx, ruleModel.L2DstMac))
	    }
		goRuleGroups.Matches = goRule
	}
	return goRuleGroups
}

func ModelMapToGoMap (ctx context.Context, data *MapModel) *MapGo {
	goMap := MapGo {
		Comment: data.Comment.ValueString(),
		Enable: data.Enable.ValueBool(),
		RuleSets: make([]RuleSet, 0),
	}

	// Copy over the elements of the map
	for _, modelRuleSet := range data.RuleSets {
		goRuleSet := RuleSet {
			RuleSetId: modelRuleSet.RuleSetId.ValueInt32(),
			Priority: modelRuleSet.Priority.ValueInt32(),
			AepId: modelRuleSet.AepId.ValueInt32(),
			PassRules: make([]RuleGroups,0),
			DropRules: make([]RuleGroups,0),
		}
		for _, passRuleGroupModel := range modelRuleSet.PassRules {
			goRuleSet.PassRules = append(goRuleSet.PassRules, updateGoRules(ctx, &passRuleGroupModel))
		}
		for _, dropRuleGroupModel := range modelRuleSet.DropRules {
			goRuleSet.DropRules = append(goRuleSet.DropRules, updateGoRules(ctx, &dropRuleGroupModel))
		}
		goMap.RuleSets = append(goMap.RuleSets, goRuleSet)
	}
	return &goMap
}

