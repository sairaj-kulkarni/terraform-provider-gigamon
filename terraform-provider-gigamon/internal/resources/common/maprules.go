// Copyright (c) Gigamon, Inc.

// Implements the various map rule types that we support and also the conversion from
// TF to Golang struct

package commonresources

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
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
	Pos          types.Int32  `tfsdk:"nested_level_depth"`
	SrcAddr      types.String `tfsdk:"source_address"`
	SrcAddrStart types.String `tfsdk:"source_address_start"`
	SrcAddrEnd   types.String `tfsdk:"source_address_end"`
	SrcAddrMask  types.String `tfsdk:"source_address_mask"`
}

// The above rules are combined to provide a generic Rule type where any one of the above
// can be specified. This should hold only one of them, and that is enforced by the schema
type RuleTypeModel struct {
	EtherType *EtherTypeModel `tfsdk:"ethernet_type"`
	L2SrcAddr *L2MacAddrModel `tfsdk:"l2_src_mac"`
	L2DstAddr *L2MacAddrModel `tfsdk:"l2_dst_mac"`
}

// The model for the rules, which is a combination of a rule ID and a set of rules from the
// above rules
type RulesModel struct {
	RuleId  types.Int32     `tfsdk:"rule_id"`
	Matches []RuleTypeModel `tfsdk:"matches"`
}

// RuleSetModel which is a ruleset, which contains a rule set ID, the aepID which is used
// to direct the traffic hitting thi ruleset, and the actual pass/drop rules of this ruleset
type RuleSetModel struct {
	RuleSetId types.Int32  `tfsdk:"rule_set_id"`
	Priority  types.Int32  `tfsdk:"priority"`
	AepId     types.Int32  `tfsdk:"aep_id"`
	PassRules []RulesModel `tfsdk:"pass_rules"`
	DropRules []RulesModel `tfsdk:"drop_rules"`
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
	Pos      int    `json:"pos,omitempty"`
	Value    int    `json:"value"`
	ValueMax int    `json:"valueMax,ignoreempty"`
}

type L2MacAddr struct {
	Type     string `json:"type"`
	Pos      int    `json:"pos,omitempty"`
	Value    string `json:"value"`
	ValueMax string `json:"valueMax,ignoreempty"`
	Mask     string `json:"Mask,omitempty"`
}

// This is the rules struct which will be used in the match field of the rules. We use
// embedded struct to ensure that when we marshall the json, we do not add any new layer
// and the fields in this are promoted to the outer layer

// Json marshalling will be default omit the field if the reference is nil

type RuleType struct {
	*EtherType
	*L2MacAddr // This will contain any L2 MAC rule whether it is SRC or DST
}

type Rules struct {
	RuleId  int        `json:"ruleId"`
	Matches []RuleType `json:"matches"`
}

type RuleSet struct {
	RuleSetId int     `json:"ruleSetId"`
	Priority  int     `json:"priority"`
	AepId     int     `json:"aepid"`
	PassRules []Rules `json:"passRules"`
	DropRules []Rules `json:"dropRules"`
}

type Map struct {
	Comment  string    `json:"comment"`
	Enable   bool      `json:"enable"`
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
		    },
		    "ether_type_start": schema.Int32Attribute{
			    MarkdownDescription: "The start range of the ether type to match",
			    Optional:            true,
		    },
		    "ether_type_end": schema.Int32Attribute{
			    MarkdownDescription: "The end range of the ether type byte to match",
			    Optional:            true,
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
		    },
		    "source_address_mask": schema.StringAttribute{
			    MarkdownDescription: "If specified this is applied to source_address to get the range of MAC addresses to match",
			    Optional:            true,
		    },
		    "source_address_start": schema.StringAttribute{
			    MarkdownDescription: "The start range of the MAC Address to match",
			    Optional:            true,
		    },
		    "source_address_end": schema.StringAttribute{
			    MarkdownDescription: "The end range of the MAC Address to match",
			    Optional:            true,
		    },
	    },
	}
}

// Comibine all the above rule schemas into a map rule schema.
func MatchesSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
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
			"rule_id": schema.Int32Attribute{
				MarkdownDescription: "ID of this rule set, 1-5",
				Required:            true,
			},
			"matches": MatchesSchema(),
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
				NestedObject:        RulesSchema(),
				Validators: []validator.List{
					listvalidator.AtLeastOneOf(path.Expressions{
						path.MatchRelative().AtParent().AtName("drop_rules"),
					}...),
				},
			},
			"drop_rules": schema.ListNestedAttribute{
				MarkdownDescription: "List of drop rules for this map",
				Optional:            true,
				NestedObject:        RulesSchema(),
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
