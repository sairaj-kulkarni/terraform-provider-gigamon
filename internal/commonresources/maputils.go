// Copyright (c) Gigamon, Inc.

// Implements the various map rule types that we support and also the conversion from
// TF to Golang struct

package commonresources

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/fmclient"
	// "github.com/hashicorp/terraform-plugin-log/tflog"
)

// TF Model for the various rules. The TF Schema model does not directly map into the
// swagger, because it is very difficult to have a common name like "value" or "subset" for
// the user configurable fields. Rather they should be called vlan_id, src_mac_address
// etc. Also for range it is better to speficy the start and end etc. Hence the TF model
// is more tuned towards user consumption and then the provider code has to convert these
// to the actual go structs before doing the JSON encode/decode to send/receive from FM

// Overall rule strucutrie is as follows
// RuleElements -> This is an element of a rule, like ethertype=0x800 or ipversion = v4 etc.
// Rule         -> This is a set of RuleElements with and AND between them. This implies
//                 that rule elements cannot have repeatition of the same element as and AND
//                 with different value for the same element would not in any case match
// RuleSet      -> Is an array of dropRules and passRules where each dropRule and passRule is
//                 is a rule, with an OR between each of the elements in the drop or pass

// First define all the individual rule elements
// Ethernet Type match rules
type EtherTypeModel struct {
	Type           types.String `tfsdk:"type"`
	Pos            types.Int32  `tfsdk:"nested_level_count"`
	EtherType      types.String `tfsdk:"ether_type"`
	EtherTypeStart types.String `tfsdk:"ether_type_start"`
	EtherTypeEnd   types.String `tfsdk:"ether_type_end"`
}

// Match on L2 SRC/DST MAC
type L2MacAddrModel struct {
	Type         types.String `tfsdk:"type"`
	Pos          types.Int32  `tfsdk:"nested_level_count"`
	MacAddr      types.String `tfsdk:"mac_address"`
	MacAddrStart types.String `tfsdk:"mac_address_start"`
	MacAddrEnd   types.String `tfsdk:"mac_address_end"`
	MacAddrMask  types.String `tfsdk:"mac_address_mask"`
}

// Match on IP Version (IPv4 / IPv6)
type IpVersionModel struct {
	Type      types.String `tfsdk:"type"`
	Pos       types.Int32  `tfsdk:"nested_level_count"`
	IpVersion types.String `tfsdk:"ip_version"` // "v4" or "v6"
}

// The model for the rules, which is a combination of the above rule elements with an OR between
// them. This will translate to one element of passRule/dropRule in the swagger with the
// elements of the struct representing one element of the matches array
type RulesModel struct {
	RuleId    types.Int32     `tfsdk:"rule_id"`
	EtherType *EtherTypeModel `tfsdk:"ether_type"`
	L2SrcMac  *L2MacAddrModel `tfsdk:"l2_src_mac"`
	L2DstMac  *L2MacAddrModel `tfsdk:"l2_dst_mac"`
	IpVersion *IpVersionModel `tfsdk:"ip_version"`
}

// RuleSetModel which is a ruleset, which contains a rule set ID, the aepID which is used
// to direct the traffic hitting thi ruleset, and the actual pass/drop rules of this ruleset
type RuleSetModel struct {
	RuleSetId types.String `tfsdk:"rule_set_id"`
	Priority  types.Int32  `tfsdk:"priority"`
	AepId     types.Int32  `tfsdk:"aep_id"`
	PassRules []RulesModel `tfsdk:"pass_rules"`
	DropRules []RulesModel `tfsdk:"drop_rules"`
}

// MAC filter list entries for macFilterList.Pass
type MacFilterEntryModel struct {
	MacAddress types.String `tfsdk:"mac_address"`
}

type MacFilterListModel struct {
	Pass []MacFilterEntryModel `tfsdk:"pass"`
}

// MapModel, consists of a set of rulesets and an ID that is got from FM
type MapModel struct {
	Name                types.String       `tfsdk:"name"`
	Comment             types.String       `tfsdk:"comment"`
	Enable              types.Bool         `tfsdk:"enable"`
	RuleSets            []RuleSetModel     `tfsdk:"rule_sets"`
	MonitoringSessionId types.String       `tfsdk:"monitoring_session_id"`
	Id                  types.String       `tfsdk:"id"`
	MacFilterList       MacFilterListModel `tfsdk:"-"`
}

// GO Struct for the rules
// First define the rule elements, and here we will mimnic the backend and use generic keys
// like value, valueMax etc.
type EtherTypeGo struct {
	Type     string `json:"type"`
	Pos      int32  `json:"pos,omitempty"`
	Value    string `json:"value"`
	ValueMax string `json:"valueMax,omitempty"`
}

type L2MacAddrGo struct {
	Type     string `json:"type"`
	Pos      int32  `json:"pos,omitempty"`
	Value    string `json:"value"`
	ValueMax string `json:"valueMax,omitempty"`
	Mask     string `json:"mask,omitempty"`
}

type IpVersionGo struct {
	Type  string `json:"type"`
	Pos   int32  `json:"pos,omitempty"`
	Value string `json:"value"` // "v4" or "v6"
}

// RulesGo represent a rule, which is an element in the pass/drop rules array in the swagger.
// Matches here is got from the RulesModel, where each non-null element of the RulesModel
// will translate to an elemen in the matches array

type RulesGo struct {
	RuleId  int32 `json:"ruleId"`
	Matches []any `json:"matches"`
}

// RuleSetGo represents a ruleSet in the swagger.
type RuleSetGo struct {
	RuleSetId string    `json:"ruleSetId"`
	Priority  int32     `json:"priority"`
	AepId     int32     `json:"aepId"`
	PassRules []RulesGo `json:"passRules"`
	DropRules []RulesGo `json:"dropRules"`
}

// MAC filter list element in macFilterList.Pass
type MacFilterEntryGo struct {
	Id         int32  `json:"id,omitempty"`
	MacAddress string `json:"macAddress"`
}

type MacFilterListGo struct {
	Pass []MacFilterEntryGo `json:"Pass"`
}

type MapGo struct {
	Name          string           `json:"name,omitempty"`
	Comment       string           `json:"comment,omitempty"`
	Enable        bool             `json:"enable,omitempty"`
	RuleSets      []RuleSetGo      `json:"ruleSets,omitempty"`
	MacFilterList *MacFilterListGo `json:"macFilterList,omitempty"`
	Id            string           `json:"id,omitempty"`
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
			"ether_type": schema.StringAttribute{
				MarkdownDescription: "The value of the ether type byte to match",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRelative().AtParent().AtName("ether_type_start"),
					}...),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^0[xX][0-9a-fA-F]{1,4}$`),
						"mut be a hexadecimal 2 byte value e.g. 0x800 or 0x3600",
					),
					stringvalidator.AtLeastOneOf(path.Expressions{
						path.MatchRelative().AtParent().AtName("ether_type_start"),
					}...),
				},
			},
			"ether_type_start": schema.StringAttribute{
				MarkdownDescription: "The start range of the ether type to match",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^0[xX][0-9a-fA-F]{1,4}$`),
						"mut be a hexadecimal 2 byte value e.g. 0x800 or 0x3600",
					),
					stringvalidator.AlsoRequires(path.Expressions{
						path.MatchRelative().AtParent().AtName("ether_type_end"),
					}...),
				},
			},
			"ether_type_end": schema.StringAttribute{
				MarkdownDescription: "The end range of the ether type byte to match",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.Expressions{
						path.MatchRelative().AtParent().AtName("ether_type_start"),
					}...),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^0[xX][0-9a-fA-F]{1,4}$`),
						"mut be a hexadecimal 2 byte value e.g. 0x800 or 0x3600",
					),
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
			"mac_address": schema.StringAttribute{
				MarkdownDescription: "The value of the MAC Address to match",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$`),
						"must be a valid MAC address format (e.g., 00:1A:2B:3C:4D:5E)",
					),
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRelative().AtParent().AtName("mac_address_start"),
					}...),
					stringvalidator.AtLeastOneOf(path.Expressions{
						path.MatchRelative().AtParent().AtName("mac_address_start"),
					}...),
				},
			},
			"mac_address_mask": schema.StringAttribute{
				MarkdownDescription: "If specified this is applied to mac_address to get the range of MAC addresses to match",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("FF:FF:FF:FF:FF:FF"),
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.Expressions{
						path.MatchRelative().AtParent().AtName("mac_address"),
					}...),
				},
			},
			"mac_address_start": schema.StringAttribute{
				MarkdownDescription: "The start range of the MAC Address to match",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$`),
						"must be a valid MAC address format (e.g., 00:1A:2B:3C:4D:5E)",
					),
					stringvalidator.AlsoRequires(path.Expressions{
						path.MatchRelative().AtParent().AtName("mac_address_end"),
					}...),
				},
			},
			"mac_address_end": schema.StringAttribute{
				MarkdownDescription: "The end range of the MAC Address to match",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.Expressions{
						path.MatchRelative().AtParent().AtName("mac_address_start"),
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

// IP Version rule schema.
func IpVersionSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				MarkdownDescription: "The type of the rule. Auto specified; not configured by user.",
				Computed:            true,
				Default:             stringdefault.StaticString("ipVer"),
			},
			"nested_level_count": schema.Int32Attribute{
				MarkdownDescription: "For tunneled or stacked headers, the level at which to match the IP version. 0 implies any position.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
			},
			"ip_version": schema.StringAttribute{
				MarkdownDescription: "IP version to match (v4 or v6).",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("v4", "v6"),
				},
			},
		},
	}
}

// Comibine all the above rule schemas into a map rule schema.
func RulesSchema() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"rule_id": schema.Int32Attribute{
				MarkdownDescription: "ID of this rule set, 1-5",
				Required:            true,
			},
			"ether_type": EtherTypeSchema(),
			"l2_src_mac": L2MacSchema("macSrc"),
			"l2_dst_mac": L2MacSchema("macDst"),
			"ip_version": IpVersionSchema(),
		},
	}
}

// Define the schema for the RuleSet which is a nested object within the map schema
func RuleSetSchema() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"rule_set_id": schema.StringAttribute{
				MarkdownDescription: "ID of this rule set, 1-5",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[1-5]$`),
						"mut be a string beteen 1 and 5",
					),
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
					int32validator.AtLeast(2),
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
			"name": schema.StringAttribute{
				MarkdownDescription: "Name for this map",
				Required:            true,
			},
			"monitoring_session_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring session on which this map is createrd",
				Required:            true,
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

// Convert the rule elements in the Model to their go structs

// ModelEtherTypeToGo convert the ethertype element from TF model to Go struct
func ModelEtherTypeToGo(ctx context.Context, etherModel *EtherTypeModel) *EtherTypeGo {
	etherType := EtherTypeGo{
		Type: etherModel.Type.ValueString(),
		Pos:  etherModel.Pos.ValueInt32(),
	}
	if etherModel.EtherType.ValueString() != "" {
		etherType.Value = etherModel.EtherType.ValueString()
	} else {
		etherType.Value = etherModel.EtherTypeStart.ValueString()
		etherType.ValueMax = etherModel.EtherTypeEnd.ValueString()
	}

	return &etherType
}

// ModelLeMacToGo Convert the L2MAC address (either SRC or DST) from TF model to go struct
func ModelL2MacToGo(ctx context.Context, l2MacModel *L2MacAddrModel) *L2MacAddrGo {
	l2MacAddr := L2MacAddrGo{
		Type: l2MacModel.Type.ValueString(),
		Pos:  l2MacModel.Pos.ValueInt32(),
	}
	if l2MacModel.MacAddr.ValueString() != "" {
		l2MacAddr.Value = l2MacModel.MacAddr.ValueString()
		if l2MacModel.MacAddrMask.ValueString() != "" {
			l2MacAddr.Mask = l2MacModel.MacAddrMask.ValueString()
		}
	} else {
		l2MacAddr.Value = l2MacModel.MacAddrStart.ValueString()
		l2MacAddr.ValueMax = l2MacModel.MacAddrEnd.ValueString()
	}
	return &l2MacAddr
}

// ModelIpVersionToGo convert the ip_version element from TF model to Go struct
func ModelIpVersionToGo(ctx context.Context, ipModel *IpVersionModel) *IpVersionGo {
	return &IpVersionGo{
		Type:  ipModel.Type.ValueString(),      // "ipVer"
		Pos:   ipModel.Pos.ValueInt32(),        // nesting level
		Value: ipModel.IpVersion.ValueString(), // "v4" or "v6"
	}
}

// ModelRulesToGoRules convert from TF Model rules to Go struct rules
func ModelRulesToGoRules(ctx context.Context, rulesModel *RulesModel) RulesGo {
	goRules := RulesGo{
		RuleId:  rulesModel.RuleId.ValueInt32(),
		Matches: make([]any, 0),
	}

	if rulesModel.EtherType != nil {
		goRules.Matches = append(
			goRules.Matches,
			ModelEtherTypeToGo(ctx, rulesModel.EtherType),
		)
	}

	if rulesModel.L2SrcMac != nil {
		goRules.Matches = append(
			goRules.Matches,
			ModelL2MacToGo(ctx, rulesModel.L2SrcMac),
		)
	}

	if rulesModel.L2DstMac != nil {
		goRules.Matches = append(
			goRules.Matches,
			ModelL2MacToGo(ctx, rulesModel.L2DstMac),
		)
	}

	if rulesModel.IpVersion != nil {
		goRules.Matches = append(
			goRules.Matches,
			ModelIpVersionToGo(ctx, rulesModel.IpVersion),
		)
	}

	return goRules
}

// ModelMapToGoMap - Top level function to convert a TF model map to go lang struct
func ModelMapToGoMap(ctx context.Context, data *MapModel) *MapGo {
	goMap := MapGo{
		Comment:  data.Comment.ValueString(),
		Enable:   data.Enable.ValueBool(),
		Name:     data.Name.ValueString(),
		RuleSets: make([]RuleSetGo, 0),
		Id:       data.Id.ValueString(),
	}

	// Copy over the elements of the map
	for _, modelRuleSet := range data.RuleSets {
		goRuleSet := RuleSetGo{
			RuleSetId: modelRuleSet.RuleSetId.ValueString(),
			Priority:  modelRuleSet.Priority.ValueInt32(),
			AepId:     modelRuleSet.AepId.ValueInt32(),
			PassRules: make([]RulesGo, 0),
			DropRules: make([]RulesGo, 0),
		}
		for _, passRulesModel := range modelRuleSet.PassRules {
			goRuleSet.PassRules = append(
				goRuleSet.PassRules,
				ModelRulesToGoRules(ctx, &passRulesModel),
			)
		}
		for _, dropRulesModel := range modelRuleSet.DropRules {
			goRuleSet.DropRules = append(
				goRuleSet.DropRules,
				ModelRulesToGoRules(ctx, &dropRulesModel),
			)
		}
		goMap.RuleSets = append(goMap.RuleSets, goRuleSet)
	}

	// Drive macFilterList only when the TF model explicitly set it.
	//
	// Semantics:
	// - data.MacFilterList.Pass == nil           ⇒ omit macFilterList entirely (FM keeps current value)
	// - data.MacFilterList.Pass != nil, len>0    ⇒ set these MACs
	// - data.MacFilterList.Pass != nil, len == 0 ⇒ clear macFilterList on FM
	if data.MacFilterList.Pass != nil {
		entries := make([]MacFilterEntryGo, 0, len(data.MacFilterList.Pass))
		for i, e := range data.MacFilterList.Pass {
			entries = append(entries, MacFilterEntryGo{
				Id:         int32(i + 1),
				MacAddress: e.MacAddress.ValueString(),
			})
		}
		goMap.MacFilterList = &MacFilterListGo{Pass: entries}
	}

	return &goMap
}

func GoMacFilterListToModel(macList *MacFilterListGo) MacFilterListModel {
	// If FM did not send macFilterList at all, macList is nil.
	if macList == nil || macList.Pass == nil {
		return MacFilterListModel{Pass: nil}
	}

	model := MacFilterListModel{
		Pass: make([]MacFilterEntryModel, 0, len(macList.Pass)),
	}
	for _, e := range macList.Pass {
		model.Pass = append(model.Pass, MacFilterEntryModel{
			MacAddress: types.StringValue(e.MacAddress),
		})
	}
	return model
}

// GetMSMapData - gets the Map details from the MS and returns an error for any errors in
// getting the map, and alo if the map does not exist. If the object does not exist we
// return a specific error code in the FMErrors
func GetMSMapData(
	ctx context.Context,
	monitoringSessId, mapId, mapName, mapType string,
	fmClient *fmclient.FmClient,
) (*MapModel, error) {

	fmResp := struct {
		Alias              string   `json:"alias"`
		Id                 string   `json:"id,omitempty"`
		ConnectionId       []string `json:"connIds"`
		MonitoringDomainId string   `json:"monitoringDomainId"`
		TrafficMaps        []MapGo  `json:"trafficMaps"`
		InclusionMaps      []MapGo  `json:"inclusionMaps"`
		ExclusionMaps      []MapGo  `json:"exclusionMaps"`
	}{
		Id: monitoringSessId,
	}

	err := UpdateMSData(ctx, monitoringSessId, &fmResp, fmClient)
	if err != nil {
		return nil, err
	}

	var myMaps []MapGo
	switch mapType {
	case "trafficMap":
		myMaps = fmResp.TrafficMaps
	case "inclusionMaps":
		myMaps = fmResp.InclusionMaps
	case "exclusionMaps":
		myMaps = fmResp.ExclusionMaps
	default:
		return nil, fmt.Errorf("Internal Error, contact Gigamon. Invalid map type secified %s", mapType)
	}

	// Upfront check: require at least one identifier.
	if mapId == "" && mapName == "" {
		return nil, fmt.Errorf("Internal Error, contact Gigamon. Either mapId or mapName must be specified")
	}

	// Go through and check if this Map is present or not.
	for _, fmMap := range myMaps {
		// If only id is provided, we match on id.
		// If only name is provided, we match on name.
		// If both are provided, we require both to match.
		if (mapId == "" || mapId == fmMap.Id) &&
			(mapName == "" || mapName == fmMap.Name) {

			modelMap := getMapModel(&fmMap)
			modelMap.MonitoringSessionId = types.StringValue(monitoringSessId)

			// copy macFilterList from FM into TF model
			modelMap.MacFilterList = GoMacFilterListToModel(fmMap.MacFilterList)

			for _, goRuleSet := range fmMap.RuleSets {
				modelRuleSet := RuleSetModel{
					RuleSetId: types.StringValue(goRuleSet.RuleSetId),
					Priority:  types.Int32Value(goRuleSet.Priority),
					AepId:     types.Int32Value(goRuleSet.AepId),
				}
				if len(goRuleSet.PassRules) > 0 {
					modelRuleSet.PassRules = make([]RulesModel, 0)
				}
				if len(goRuleSet.DropRules) > 0 {
					modelRuleSet.DropRules = make([]RulesModel, 0)
				}

				for _, goRules := range goRuleSet.PassRules {
					modelRules := RulesModel{
						RuleId: types.Int32Value(goRules.RuleId),
					}
					copyGoRuleGrouptoModel(ctx, &modelRules, &goRules)
					modelRuleSet.PassRules = append(modelRuleSet.PassRules, modelRules)
				}
				for _, goRules := range goRuleSet.DropRules {
					modelRules := RulesModel{
						RuleId: types.Int32Value(goRules.RuleId),
					}
					copyGoRuleGrouptoModel(ctx, &modelRules, &goRules)
					modelRuleSet.DropRules = append(modelRuleSet.DropRules, modelRules)
				}
				modelMap.RuleSets = append(modelMap.RuleSets, modelRuleSet)
			}
			return modelMap, nil
		}
	}

	return nil, fmclient.NewFMError(
		fmclient.ObjectNotFound,
		fmt.Sprintf("%s: %s map not found in the ms", mapType, mapName),
		nil,
	)
}

// getMapModel Create a MAP TF Model object base fromthe given MAP Go lang object
func getMapModel(fmMap *MapGo) *MapModel {

	comment := types.StringNull()
	if fmMap.Comment != "" {
		comment = types.StringValue(fmMap.Comment)
	}

	return &MapModel{
		Name:     types.StringValue(fmMap.Name),
		Comment:  comment,
		Enable:   types.BoolValue(fmMap.Enable),
		Id:       types.StringValue(fmMap.Id),
		RuleSets: make([]RuleSetModel, 0),
		// MacFilterList will be filled by GoMacFilterListToModel
	}
}

// Copy the Rule Groups object from GO model to the corresponding TF model
func copyGoRuleGrouptoModel(
	ctx context.Context,
	modelRules *RulesModel,
	goRules *RulesGo,
) {
	for _, rule := range goRules.Matches {
		ruleElements := rule.(map[string]any)
		ruleType := ruleElements["type"].(string)
		switch ruleType {
		case "etherType":
			modelRules.EtherType = GoEtherTypeToModel(ctx, ruleElements)
		case "macSrc":
			modelRules.L2SrcMac = GoL2MacTypeToModel(ctx, ruleElements, "macSrc")
		case "macDst":
			modelRules.L2DstMac = GoL2MacTypeToModel(ctx, ruleElements, "macDst")
		case "ipVer":
			modelRules.IpVersion = GoIpVersionToModel(ctx, ruleElements)
		}

	}
}

// anyToInt32 is used only when reading FM JSON that was unmarshalled into
// interface{} / map[string]any. encoding/json represents all numbers in this
// case as float64, so we normalize them here. On unexpected types we panic
// so that FM/schema bugs are caught early.
func anyToInt32(v any, field string) int32 {
	switch x := v.(type) {
	case float64:
		return int32(x)
	case int32:
		return x
	default:
		panic(fmt.Sprintf("unexpected type for %s: %T (%v)", field, v, v))
	}
}

func GoEtherTypeToModel(ctx context.Context, ruleElements map[string]any) *EtherTypeModel {
	data := &EtherTypeModel{
		Type: types.StringValue("etherType"),
	}
	if pos, ok := ruleElements["pos"]; ok {
		data.Pos = types.Int32Value(anyToInt32(pos, "matches.pos"))
	} else {
		data.Pos = types.Int32Value(0)
	}
	val, ok := ruleElements["valueMax"]
	if !ok || val.(string) == "" { // Single value and not a range
		data.EtherType = types.StringValue(ruleElements["value"].(string))
	} else {
		data.EtherTypeStart = types.StringValue(ruleElements["value"].(string))
		data.EtherTypeEnd = types.StringValue(ruleElements["valueMax"].(string))
	}
	return data
}

func GoL2MacTypeToModel(ctx context.Context, ruleElements map[string]any, macType string) *L2MacAddrModel {
	data := &L2MacAddrModel{
		Type: types.StringValue(macType),
	}
	if pos, ok := ruleElements["pos"]; ok {
		data.Pos = types.Int32Value(anyToInt32(pos, "matches.pos"))
	} else {
		data.Pos = types.Int32Value(0)
	}
	mask, ok := ruleElements["mask"]
	if !ok {
		data.MacAddrMask = types.StringValue("FF:FF:FF:FF:FF:FF")
	} else {
		data.MacAddrMask = types.StringValue(mask.(string))
	}
	val, ok := ruleElements["valueMax"]
	if !ok || val.(string) == "" {
		data.MacAddr = types.StringValue(ruleElements["value"].(string))
	} else {
		data.MacAddrStart = types.StringValue(ruleElements["valueMax"].(string))
		data.MacAddrEnd = types.StringValue(ruleElements["value"].(string))
	}
	return data
}

func GoIpVersionToModel(ctx context.Context, ruleElements map[string]any) *IpVersionModel {
	data := &IpVersionModel{
		Type: types.StringValue("ipVer"),
	}

	if pos, ok := ruleElements["pos"]; ok {
		data.Pos = types.Int32Value(anyToInt32(pos, "matches.pos"))
	} else {
		data.Pos = types.Int32Value(0)
	}

	if val, ok := ruleElements["value"]; ok {
		data.IpVersion = types.StringValue(val.(string)) // "v4" or "v6"
	} else {
		data.IpVersion = types.StringValue("")
	}

	return data
}
