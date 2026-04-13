// Copyright (c) Gigamon, Inc.

// Implements the various map rule types that we support and also the conversion from
// TF to Golang struct

package commonresources

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

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

	"terraform-provider-gigamon/internal/commonutils"
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

// Match on DSCP (IPv4 or IPv6, depending on internal type).
type DscpModel struct {
	Type  types.String `tfsdk:"type"` // computed: "ip4Dscp" or "ip6Dscp"
	Pos   types.Int32  `tfsdk:"nested_level_count"`
	Value types.String `tfsdk:"dscp"` // af11..af43, ef
}

// Match on VM Name prefix (source or destination).
type VmNameRuleModel struct {
	Type   types.String `tfsdk:"type"`           // "srcVmPrefix" or "dstVmPrefix" (computed)
	Prefix types.String `tfsdk:"vm_name_prefix"` // required
}

// Match on VM Tag (source or destination).
// FM JSON: { "type": "srcVmTag"/"dstVmTag", "name": "<tag-key-or-name>", "value": "<tag-value-or-category>" }.
type VmTagRuleModel struct {
	Type     types.String `tfsdk:"type"`      // "srcVmTag" or "dstVmTag" (computed)
	TagName  types.String `tfsdk:"tag_name"`  // Tag key (or tag name in vSphere)
	TagValue types.String `tfsdk:"tag_value"` // Tag value (or tag category in vSphere)
}

// Match on IPv4 Source/Destination (common model).
type Ipv4AddrRuleModel struct {
	Type       types.String `tfsdk:"type"`               // "ip4Src" or "ip4Dst" (computed)
	Pos        types.Int32  `tfsdk:"nested_level_count"` // 0..3, default 0
	Address    types.String `tfsdk:"address"`            // required (min)
	AddressMax types.String `tfsdk:"address_max"`        // optional range max
	CidrMask   types.String `tfsdk:"cidr_mask"`          // optional 1..32, as string
	NetMask    types.String `tfsdk:"netmask"`            // optional dotted-decimal
}

// Match on IPv6 Source/Destination (common model).
type Ipv6AddrRuleModel struct {
	Type       types.String `tfsdk:"type"`               // "ip6Src" or "ip6Dst" (computed)
	Pos        types.Int32  `tfsdk:"nested_level_count"` // 0..3, default 0
	Address    types.String `tfsdk:"address"`            // required (min)
	AddressMax types.String `tfsdk:"address_max"`        // optional range max
	CidrMask   types.String `tfsdk:"cidr_mask"`          // optional 1..128, as string
	NetMask    types.String `tfsdk:"netmask"`            // optional 128-bit IPv6 mask
}

// Match on IP Fragmentation (IPv4).
type Ip4FragRuleModel struct {
	Type  types.String `tfsdk:"type"`               // computed: "ip4Frag"
	Pos   types.Int32  `tfsdk:"nested_level_count"` // 0..3, default 0
	Value types.String `tfsdk:"mode"`               // unfragmented_only, any_fragment, non_first_fragments, first_fragment_only, first_or_unfragmented
}

// Match on IPv4 Protocol Number (0–255).
type Ip4ProtoRuleModel struct {
	Type           types.String `tfsdk:"type"`
	Pos            types.Int32  `tfsdk:"nested_level_count"`
	ProtocolMin    types.Int32  `tfsdk:"protocol_min"`
	ProtocolMax    types.Int32  `tfsdk:"protocol_max"`
	ProtocolSubset types.String `tfsdk:"protocol_subset"`
}

// The model for the rules, which is a combination of the above rule elements with an OR between
// them. This will translate to one element of passRule/dropRule in the swagger with the
// elements of the struct representing one element of the matches array
type RulesModel struct {
	RuleId            types.Int32        `tfsdk:"rule_id"`
	EtherType         *EtherTypeModel    `tfsdk:"ether_type"`
	L2SrcMac          *L2MacAddrModel    `tfsdk:"l2_src_mac"`
	L2DstMac          *L2MacAddrModel    `tfsdk:"l2_dst_mac"`
	IpVersion         *IpVersionModel    `tfsdk:"ip_version"`
	Ipv4Source        *Ipv4AddrRuleModel `tfsdk:"ipv4_source"`
	Ipv4Destination   *Ipv4AddrRuleModel `tfsdk:"ipv4_destination"`
	Ipv6Source        *Ipv6AddrRuleModel `tfsdk:"ipv6_source"`
	Ipv6Destination   *Ipv6AddrRuleModel `tfsdk:"ipv6_destination"`
	VmNameSource      *VmNameRuleModel   `tfsdk:"vm_name_source"`
	VmNameDestination *VmNameRuleModel   `tfsdk:"vm_name_destination"`
	VmTagSource       *VmTagRuleModel    `tfsdk:"vm_tag_source"`
	VmTagDestination  *VmTagRuleModel    `tfsdk:"vm_tag_destination"`
	Ipv4Dscp          *DscpModel         `tfsdk:"ipv4_dscp"`
	Ipv6Dscp          *DscpModel         `tfsdk:"ipv6_dscp"`
	Ip4Frag           *Ip4FragRuleModel  `tfsdk:"ipv4_fragmentation"`
	Ip4Protocol       *Ip4ProtoRuleModel `tfsdk:"ipv4_protocol"`
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
	Description         types.String       `tfsdk:"description"`
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

type DscpGo struct {
	Type  string `json:"type"`          // "ip4Dscp" or "ip6Dscp"
	Pos   int32  `json:"pos,omitempty"` // 0..3
	Value string `json:"value"`         // "af11", "ef", ...
}

type VmNameGo struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type VmTagGo struct {
	Type  string `json:"type"`  // "srcVmTag"
	Name  string `json:"name"`  // tag-1
	Value string `json:"value"` // workload
}

type Ip4AddrGo struct {
	Type     string `json:"type"`               // "ip4Src" or "ip4Dst"
	Pos      int32  `json:"pos,omitempty"`      // 0..3
	Value    string `json:"value"`              // min/address
	ValueMax string `json:"valueMax,omitempty"` // optional range max
	CidrMask string `json:"cidrMask,omitempty"` // optional "1".."32"
	NetMask  string `json:"netMask,omitempty"`  // optional dotted-decimal
}

type Ip6AddrGo struct {
	Type     string `json:"type"`               // "ip6Src" or "ip6Dst"
	Pos      int32  `json:"pos,omitempty"`      // 0..3
	Value    string `json:"value"`              // min/address
	ValueMax string `json:"valueMax,omitempty"` // optional range max
	CidrMask string `json:"cidrMask,omitempty"` // optional "1".."128"
	NetMask  string `json:"netMask,omitempty"`  // optional IPv6 mask string
}

type Ip4FragGo struct {
	Type  string `json:"type"`          // "ip4Frag"
	Pos   int32  `json:"pos,omitempty"` // 0..3
	Value string `json:"value"`         // "noFrag", "allFrag", "allFragNoFirst", "firstFrag", "firstOrNoFrag"
}

type Ip4ProtoGo struct {
	Type     string `json:"type"`               // "ip4Proto"
	Pos      int32  `json:"pos,omitempty"`      // 0..3
	Value    string `json:"value"`              // min
	ValueMax string `json:"valueMax,omitempty"` // max, optional
	Subset   string `json:"subset,omitempty"`   // "none" | "even" | "odd"
}

var ipv4Regex = regexp.MustCompile(
	`^((25[0-5]|2[0-4][0-9]|[0-1]?[0-9]{1,2})\.){3}(25[0-5]|2[0-4][0-9]|[0-1]?[0-9]{1,2})$`,
)

// Very simple IPv6 address regex: allow compressed form; we rely on FM for deep validation.
var ipv6Regex = regexp.MustCompile(`^[0-9A-Fa-f:]+$`)

// IPv6 netmask: 8 hextets of uppercase hex (FM uses uppercase).
var ipv6NetmaskRegex = regexp.MustCompile(`^([0-9A-F]{4}:){7}[0-9A-F]{4}$`)

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

func dscpSchema(fmType string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				MarkdownDescription: "Internal rule type; auto-specified, not configured by user.",
				Computed:            true,
				Default:             stringdefault.StaticString(fmType), // "ip4Dscp" or "ip6Dscp"
			},
			"nested_level_count": schema.Int32Attribute{
				MarkdownDescription: "For tunneled/stacked IP headers, which header's DSCP to match. 0=any, 1=outer, 2=second, 3=third.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
			},
			"dscp": schema.StringAttribute{
				MarkdownDescription: "DSCP code point to match (AFxx or EF).",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						"af11", "af12", "af13",
						"af21", "af22", "af23",
						"af31", "af32", "af33",
						"af41", "af42", "af43",
						"ef",
					),
				},
			},
		},
	}
}

// VM Name (prefix) rule schema.
func VmNameSchema(fmType string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				MarkdownDescription: "Internal rule type; set automatically.",
				Computed:            true,
				Default:             stringdefault.StaticString(fmType), // "srcVmPrefix" or "dstVmPrefix"
			},
			"vm_name_prefix": schema.StringAttribute{
				MarkdownDescription: "Prefix of the VM name to match. For vSphere, this is the VM name; for clouds this is the VM name as shown in GigaVUE‑FM. Wildcards not supported; prefix match only.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
		},
	}
}

// VM Tag rule schema (vSphere Tag Name + Category).
func VmTagSchema(fmType string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				MarkdownDescription: "Internal rule type; set automatically.",
				Computed:            true,
				Default:             stringdefault.StaticString(fmType), // "srcVmTag" or "dstVmTag"
			},
			"tag_name": schema.StringAttribute{
				MarkdownDescription: "Tag key. In vSphere this is the tag *name*; in AWS/Azure/GCP this is the tag key.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"tag_value": schema.StringAttribute{
				MarkdownDescription: "Tag value. In vSphere this corresponds to the tag *category*.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
		},
	}
}

func ipv4AddrSchema(fmType string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				MarkdownDescription: "Internal rule type; auto specified, not configured by user.",
				Computed:            true,
				Default:             stringdefault.StaticString(fmType), // "ip4Src" or "ip4Dst"
			},
			"nested_level_count": schema.Int32Attribute{
				MarkdownDescription: "For tunneled/stacked IPv4 headers, which header to inspect. 0=any, 1=outer, 2=second, 3=third.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
				Validators: []validator.Int32{
					int32validator.AtLeast(0),
					int32validator.AtMost(3),
				},
			},
			"address": schema.StringAttribute{
				MarkdownDescription: "IPv4 address to match (start of range or network address).",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						ipv4Regex,
						"must be a valid IPv4 address",
					),
				},
			},
			"address_max": schema.StringAttribute{
				MarkdownDescription: "Upper end of IPv4 range (inclusive). Mutually exclusive with cidr_mask and netmask.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRelative().AtParent().AtName("cidr_mask"),
						path.MatchRelative().AtParent().AtName("netmask"),
					}...),
					stringvalidator.RegexMatches(
						ipv4Regex,
						"must be a valid IPv4 address",
					),
				},
			},
			"cidr_mask": schema.StringAttribute{
				MarkdownDescription: "CIDR prefix length (1-32) applied to address. Mutually exclusive with address_max and netmask.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRelative().AtParent().AtName("address_max"),
						path.MatchRelative().AtParent().AtName("netmask"),
					}...),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^(?:[1-9]|[1-2][0-9]|3[0-2])$`),
						"must be an integer between 1 and 32",
					),
				},
			},
			"netmask": schema.StringAttribute{
				MarkdownDescription: "Dotted-decimal netmask applied to address. Mutually exclusive with address_max and cidr_mask.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRelative().AtParent().AtName("address_max"),
						path.MatchRelative().AtParent().AtName("cidr_mask"),
					}...),
					stringvalidator.RegexMatches(
						regexp.MustCompile(
							`^(128|192|224|240|248|252|254|255)\.0\.0\.0$|^255\.(0|128|192|224|240|248|252|254)\.0\.0$|^255\.255\.(0|128|192|224|240|248|252|254)\.0$|^255\.255\.255\.(0|128|192|224|240|248|252|254)$`,
						),
						"must be a valid contiguous IPv4 netmask",
					),
				},
			},
		},
	}
}

func ipv6AddrSchema(fmType string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				MarkdownDescription: "Internal rule type; auto specified, not configured by user.",
				Computed:            true,
				Default:             stringdefault.StaticString(fmType), // "ip6Src" or "ip6Dst"
			},
			"nested_level_count": schema.Int32Attribute{
				MarkdownDescription: "For tunneled/stacked IPv6 headers, which header to inspect. 0=any, 1=outer, 2=second, 3=third.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
				Validators: []validator.Int32{
					int32validator.AtLeast(0),
					int32validator.AtMost(3),
				},
			},
			"address": schema.StringAttribute{
				MarkdownDescription: "IPv6 address to match (start of range or network address).",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						ipv6Regex,
						"must be a valid IPv6 address format",
					),
				},
			},
			"address_max": schema.StringAttribute{
				MarkdownDescription: "Upper end of IPv6 range (inclusive). Mutually exclusive with cidr_mask and netmask.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRelative().AtParent().AtName("cidr_mask"),
						path.MatchRelative().AtParent().AtName("netmask"),
					}...),
					stringvalidator.RegexMatches(
						ipv6Regex,
						"must be a valid IPv6 address format",
					),
				},
			},
			"cidr_mask": schema.StringAttribute{
				MarkdownDescription: "CIDR prefix length (1-128) applied to address. Mutually exclusive with address_max and netmask.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRelative().AtParent().AtName("address_max"),
						path.MatchRelative().AtParent().AtName("netmask"),
					}...),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^(?:[1-9][0-9]?|1[01][0-9]|12[0-8])$`),
						"must be an integer between 1 and 128",
					),
				},
			},
			"netmask": schema.StringAttribute{
				MarkdownDescription: "IPv6 netmask applied to address (128-bit mask, e.g. FFFF:FFFF:FFFF:FFFF:0000:0000:0000:0000). Mutually exclusive with address_max and cidr_mask.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRelative().AtParent().AtName("address_max"),
						path.MatchRelative().AtParent().AtName("cidr_mask"),
					}...),
					stringvalidator.RegexMatches(
						ipv6NetmaskRegex,
						"must be an 8-hextet IPv6 mask in uppercase hex",
					),
				},
			},
		},
	}
}

// IP fragmentation rule schema (IPv4).
func ip4FragSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				MarkdownDescription: "Internal rule type; auto specified, not configured by user.",
				Computed:            true,
				Default:             stringdefault.StaticString("ip4Frag"),
			},
			"nested_level_count": schema.Int32Attribute{
				MarkdownDescription: "For tunneled/stacked IPv4 headers, which header's fragmentation bits to inspect. 0=any, 1=outer, 2=second, 3=third.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
				Validators: []validator.Int32{
					int32validator.AtLeast(0),
					int32validator.AtMost(3),
				},
			},
			"mode": schema.StringAttribute{
				MarkdownDescription: "Which IP fragments to match.\n" +
					"- `unfragmented_only`: only packets that are not fragmented.\n" +
					"- `any_fragment`: any fragment of a fragmented packet (first and later fragments).\n" +
					"- `non_first_fragments`: only non-first fragments of fragmented packets.\n" +
					"- `first_fragment_only`: only the first fragment of fragmented packets.\n" +
					"- `first_or_unfragmented`: unfragmented packets or the first fragment of fragmented packets.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						"unfragmented_only",
						"any_fragment",
						"non_first_fragments",
						"first_fragment_only",
						"first_or_unfragmented",
					),
				},
			},
		},
	}
}

type ip4ProtoRangeValidator struct{}

func (v ip4ProtoRangeValidator) Description(ctx context.Context) string {
	return "protocol_max must be greater than protocol_min when both are set"
}

func (v ip4ProtoRangeValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v ip4ProtoRangeValidator) ValidateInt32(
	ctx context.Context,
	req validator.Int32Request,
	resp *validator.Int32Response,
) {
	// If max is null/unknown, nothing to validate.
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	// Read the entire ipv4_protocol block into the real model.
	var parent Ip4ProtoRuleModel
	diags := req.Config.GetAttribute(ctx, req.Path.ParentPath(), &parent)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if parent.ProtocolMin.IsNull() || parent.ProtocolMin.IsUnknown() {
		return
	}

	min := parent.ProtocolMin.ValueInt32()
	max := req.ConfigValue.ValueInt32()

	// Enforce strict inequality: max must be > min.
	if max <= min {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid IPv4 protocol range",
			fmt.Sprintf("protocol_max (%d) must be greater than protocol_min (%d)", max, min),
		)
	}
}

func ip4ProtoSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				MarkdownDescription: "Internal rule type; auto specified, not configured by user.",
				Computed:            true,
				Default:             stringdefault.StaticString("ip4Proto"),
			},
			"nested_level_count": schema.Int32Attribute{
				MarkdownDescription: "For tunneled/stacked IPv4 headers, which header's protocol field to inspect. 0=any, 1=outer, 2=second, 3=third.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
				Validators: []validator.Int32{
					int32validator.AtLeast(0),
					int32validator.AtMost(3),
				},
			},
			"protocol_min": schema.Int32Attribute{
				MarkdownDescription: "Lower bound (inclusive) of IPv4 protocol number to match (0–255).",
				Required:            true,
				Validators: []validator.Int32{
					int32validator.AtLeast(0),
					int32validator.AtMost(255),
				},
			},
			"protocol_max": schema.Int32Attribute{
				MarkdownDescription: "Upper bound (inclusive) of IPv4 protocol number (0–255). If omitted, only protocol_min is matched.",
				Optional:            true,
				Validators: []validator.Int32{
					int32validator.AtLeast(0),
					int32validator.AtMost(255),
					ip4ProtoRangeValidator{},
				},
			},
			"protocol_subset": schema.StringAttribute{
				MarkdownDescription: "Restrict matches within [protocol_min, protocol_max] to `all` (no parity filter), only `even`, or only `odd` protocol numbers. `even`/`odd` require protocol_max.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("all"),
				Validators: []validator.String{
					stringvalidator.OneOf("all", "even", "odd"),
					// For even/odd we require max to be present.
					stringvalidator.AlsoRequires(path.Expressions{
						path.MatchRelative().AtParent().AtName("protocol_max"),
					}...),
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
			"ether_type":          EtherTypeSchema(),
			"l2_src_mac":          L2MacSchema("macSrc"),
			"l2_dst_mac":          L2MacSchema("macDst"),
			"ip_version":          IpVersionSchema(),
			"ipv4_source":         ipv4AddrSchema("ip4Src"),
			"ipv4_destination":    ipv4AddrSchema("ip4Dst"),
			"ipv6_source":         ipv6AddrSchema("ip6Src"),
			"ipv6_destination":    ipv6AddrSchema("ip6Dst"),
			"vm_name_source":      VmNameSchema("srcVmPrefix"),
			"vm_name_destination": VmNameSchema("dstVmPrefix"),
			"vm_tag_source":       VmTagSchema("srcVmTag"),
			"vm_tag_destination":  VmTagSchema("dstVmTag"),
			"ipv4_dscp":           dscpSchema("ip4Dscp"),
			"ipv6_dscp":           dscpSchema("ip6Dscp"),
			"ipv4_fragmentation":  ip4FragSchema(),
			"ipv4_protocol":       ip4ProtoSchema(),
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
					listvalidator.SizeAtLeast(1),
				},
			},
			"drop_rules": schema.ListNestedAttribute{
				MarkdownDescription: "List of drop rules for this map",
				Optional:            true,
				NestedObject:        RulesSchema(),
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
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
			"description": schema.StringAttribute{
				MarkdownDescription: "Description for this map",
				Optional:            true,
				Validators: []validator.String{
					// If description is set, it must be non-empty
					stringvalidator.LengthAtLeast(1),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name for this map",
				Required:            true,
			},
			"monitoring_session_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring session on which this map is created",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
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

// ModelDscpToGo convert the dscp element from TF model to Go struct
func ModelDscpToGo(_ context.Context, m *DscpModel) *DscpGo {
	return &DscpGo{
		Type:  m.Type.ValueString(),  // "ip4Dscp" or "ip6Dscp"
		Pos:   m.Pos.ValueInt32(),    // nested_level_count
		Value: m.Value.ValueString(), // dscp value
	}
}

// ModelVmNameToGo converts the vm_name element from TF model to Go struct.
func ModelVmNameToGo(_ context.Context, m *VmNameRuleModel) *VmNameGo {
	return &VmNameGo{
		Type:  m.Type.ValueString(),   // "srcVmPrefix" or "dstVmPrefix"
		Value: m.Prefix.ValueString(), // prefix
	}
}

// ModelVmTagToGo converts the vm_tag element from TF model to Go struct.
func ModelVmTagToGo(_ context.Context, m *VmTagRuleModel) *VmTagGo {
	return &VmTagGo{
		Type:  m.Type.ValueString(),     // "srcVmTag" or "dstVmTag"
		Name:  m.TagName.ValueString(),  // key / tag name
		Value: m.TagValue.ValueString(), // value / tag category
	}
}

func ModelIpv4AddrToGo(_ context.Context, m *Ipv4AddrRuleModel) *Ip4AddrGo {
	ip := &Ip4AddrGo{
		Type:  m.Type.ValueString(),
		Pos:   m.Pos.ValueInt32(),
		Value: m.Address.ValueString(),
	}

	if v := m.AddressMax.ValueString(); v != "" {
		ip.ValueMax = v
	}
	if v := m.CidrMask.ValueString(); v != "" {
		ip.CidrMask = v
	}
	if v := m.NetMask.ValueString(); v != "" {
		ip.NetMask = v
	}

	return ip
}

func ModelIpv6AddrToGo(_ context.Context, m *Ipv6AddrRuleModel) *Ip6AddrGo {
	ip := &Ip6AddrGo{
		Type:  m.Type.ValueString(),
		Pos:   m.Pos.ValueInt32(),
		Value: m.Address.ValueString(),
	}

	if v := m.AddressMax.ValueString(); v != "" {
		ip.ValueMax = v
	}
	if v := m.CidrMask.ValueString(); v != "" {
		ip.CidrMask = v
	}
	if v := m.NetMask.ValueString(); v != "" {
		ip.NetMask = v
	}

	return ip
}

// Mapping from TF-friendly names to backend ip4Frag values.
var tfIp4FragToBackend = map[string]string{
	"unfragmented_only":     "noFrag",
	"any_fragment":          "allFrag",
	"non_first_fragments":   "allFragNoFirst",
	"first_fragment_only":   "firstFrag",
	"first_or_unfragmented": "firstOrNoFrag",
}

func ModelIp4FragToGo(_ context.Context, m *Ip4FragRuleModel) *Ip4FragGo {
	mode := m.Value.ValueString()
	backend, ok := tfIp4FragToBackend[mode]
	if !ok {
		// Should not happen due to validators; fall back to unfragmented_only.
		backend = "noFrag"
	}

	return &Ip4FragGo{
		Type:  m.Type.ValueString(), // "ip4Frag"
		Pos:   m.Pos.ValueInt32(),   // nested_level_count
		Value: backend,              // noFrag, allFrag, ...
	}
}

func ModelIp4ProtoToGo(_ context.Context, m *Ip4ProtoRuleModel) *Ip4ProtoGo {
	min := m.ProtocolMin.ValueInt32()

	var maxStr string
	if !m.ProtocolMax.IsNull() && !m.ProtocolMax.IsUnknown() {
		maxStr = strconv.FormatInt(int64(m.ProtocolMax.ValueInt32()), 10)
	}

	subset := m.ProtocolSubset.ValueString()
	if subset == "" || subset == "all" {
		subset = "none" // FM encoding for “no subset filter”
	}

	return &Ip4ProtoGo{
		Type:     m.Type.ValueString(), // "ip4Proto"
		Pos:      m.Pos.ValueInt32(),
		Value:    strconv.FormatInt(int64(min), 10),
		ValueMax: maxStr,
		Subset:   subset, // "none", "even", or "odd"
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

	if rulesModel.Ipv4Source != nil {
		goRules.Matches = append(goRules.Matches, ModelIpv4AddrToGo(ctx, rulesModel.Ipv4Source))
	}
	if rulesModel.Ipv4Destination != nil {
		goRules.Matches = append(goRules.Matches, ModelIpv4AddrToGo(ctx, rulesModel.Ipv4Destination))
	}

	if rulesModel.Ipv6Source != nil {
		goRules.Matches = append(goRules.Matches, ModelIpv6AddrToGo(ctx, rulesModel.Ipv6Source))
	}
	if rulesModel.Ipv6Destination != nil {
		goRules.Matches = append(goRules.Matches, ModelIpv6AddrToGo(ctx, rulesModel.Ipv6Destination))
	}

	if rulesModel.VmNameSource != nil {
		goRules.Matches = append(
			goRules.Matches,
			ModelVmNameToGo(ctx, rulesModel.VmNameSource),
		)
	}
	if rulesModel.VmNameDestination != nil {
		goRules.Matches = append(
			goRules.Matches,
			ModelVmNameToGo(ctx, rulesModel.VmNameDestination),
		)
	}

	if rulesModel.VmTagSource != nil {
		goRules.Matches = append(
			goRules.Matches,
			ModelVmTagToGo(ctx, rulesModel.VmTagSource),
		)
	}
	if rulesModel.VmTagDestination != nil {
		goRules.Matches = append(
			goRules.Matches,
			ModelVmTagToGo(ctx, rulesModel.VmTagDestination),
		)
	}

	if rulesModel.Ipv4Dscp != nil {
		goRules.Matches = append(goRules.Matches, ModelDscpToGo(ctx, rulesModel.Ipv4Dscp))
	}

	if rulesModel.Ipv6Dscp != nil {
		goRules.Matches = append(goRules.Matches, ModelDscpToGo(ctx, rulesModel.Ipv6Dscp))
	}

	if rulesModel.Ip4Frag != nil {
		goRules.Matches = append(goRules.Matches, ModelIp4FragToGo(ctx, rulesModel.Ip4Frag))
	}

	if rulesModel.Ip4Protocol != nil {
		goRules.Matches = append(goRules.Matches, ModelIp4ProtoToGo(ctx, rulesModel.Ip4Protocol))
	}

	return goRules
}

// ModelMapToGoMap - Top level function to convert a TF model map to go lang struct
func ModelMapToGoMap(ctx context.Context, data *MapModel) *MapGo {

	var rawID string
	if v := data.Id.ValueString(); v != "" {
		id, err := commonutils.UUIDFromTypedID(v)
		if err == nil {
			rawID = id
		}
	}

	goMap := MapGo{
		Comment:  data.Description.ValueString(),
		Enable:   data.Enable.ValueBool(),
		Name:     data.Name.ValueString(),
		RuleSets: make([]RuleSetGo, 0),
		Id:       rawID,
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

			// Normalize Id: always store typed ID in the TF model.
			var typ commonutils.Type
			switch mapType {
			case "trafficMap":
				typ = commonutils.TypeTrafficMap
			case "inclusionMaps":
				typ = commonutils.TypeInclusionMap
			case "exclusionMaps":
				typ = commonutils.TypeExclusionMap
			}

			typedID, err := commonutils.MakeTypedID(commonutils.ModuleMap, typ, fmMap.Id)
			if err != nil {
				return nil, err
			}
			modelMap.Id = types.StringValue(typedID)

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

	var description types.String
	if fmMap.Comment != "" {
		description = types.StringValue(fmMap.Comment)
	} else {
		description = types.StringNull()
	}

	return &MapModel{
		Name:        types.StringValue(fmMap.Name),
		Description: description,
		Enable:      types.BoolValue(fmMap.Enable),
		Id:          types.StringValue(fmMap.Id),
		RuleSets:    make([]RuleSetModel, 0),
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
		case "ip4Src":
			modelRules.Ipv4Source = GoIpv4AddrToModel(ruleElements, "ip4Src")
		case "ip4Dst":
			modelRules.Ipv4Destination = GoIpv4AddrToModel(ruleElements, "ip4Dst")
		case "ip6Src":
			modelRules.Ipv6Source = GoIpv6AddrToModel(ruleElements, "ip6Src")
		case "ip6Dst":
			modelRules.Ipv6Destination = GoIpv6AddrToModel(ruleElements, "ip6Dst")
		case "srcVmPrefix":
			modelRules.VmNameSource = GoVmNameToModel(ctx, ruleElements)
		case "dstVmPrefix":
			modelRules.VmNameDestination = GoVmNameToModel(ctx, ruleElements)
		case "srcVmTag":
			modelRules.VmTagSource = GoVmTagToModel(ctx, ruleElements)
		case "dstVmTag":
			modelRules.VmTagDestination = GoVmTagToModel(ctx, ruleElements)
		case "ip4Dscp":
			modelRules.Ipv4Dscp = GoDscpToModel(ruleElements)
		case "ip6Dscp":
			modelRules.Ipv6Dscp = GoDscpToModel(ruleElements)
		case "ip4Frag":
			modelRules.Ip4Frag = GoIp4FragToModel(ruleElements)
		case "ip4Proto":
			modelRules.Ip4Protocol = GoIp4ProtoToModel(ruleElements)
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
		data.MacAddrStart = types.StringValue(ruleElements["value"].(string))
		data.MacAddrEnd = types.StringValue(ruleElements["valueMax"].(string))
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

func GoDscpToModel(ruleElements map[string]any) *DscpModel {
	m := &DscpModel{
		Type: types.StringValue(ruleElements["type"].(string)), // "ip4Dscp" or "ip6Dscp"
	}

	if pos, ok := ruleElements["pos"]; ok {
		m.Pos = types.Int32Value(anyToInt32(pos, "matches.pos"))
	} else {
		m.Pos = types.Int32Value(0)
	}

	if v, ok := ruleElements["value"]; ok {
		m.Value = types.StringValue(v.(string))
	} else {
		m.Value = types.StringValue("")
	}

	return m
}

func GoVmNameToModel(_ context.Context, ruleElements map[string]any) *VmNameRuleModel {
	data := &VmNameRuleModel{
		Type: types.StringValue(ruleElements["type"].(string)), // "srcVmPrefix" or "dstVmPrefix"
	}

	if val, ok := ruleElements["value"]; ok {
		data.Prefix = types.StringValue(val.(string))
	} else {
		data.Prefix = types.StringValue("")
	}

	return data
}

func GoVmTagToModel(_ context.Context, ruleElements map[string]any) *VmTagRuleModel {
	data := &VmTagRuleModel{
		Type: types.StringValue(ruleElements["type"].(string)), // "srcVmTag" or "dstVmTag"
	}

	if name, ok := ruleElements["name"]; ok {
		data.TagName = types.StringValue(name.(string))
	} else {
		data.TagName = types.StringValue("")
	}

	if val, ok := ruleElements["value"]; ok {
		data.TagValue = types.StringValue(val.(string))
	} else {
		data.TagValue = types.StringValue("")
	}

	return data
}

func GoIpv4AddrToModel(ruleElements map[string]any, fmType string) *Ipv4AddrRuleModel {
	m := &Ipv4AddrRuleModel{
		Type: types.StringValue(fmType),
	}

	if pos, ok := ruleElements["pos"]; ok {
		m.Pos = types.Int32Value(anyToInt32(pos, "matches.pos"))
	} else {
		m.Pos = types.Int32Value(0)
	}

	if v, ok := ruleElements["value"]; ok {
		m.Address = types.StringValue(v.(string))
	}

	if v, ok := ruleElements["valueMax"]; ok && v.(string) != "" {
		m.AddressMax = types.StringValue(v.(string))
	}

	if v, ok := ruleElements["cidrMask"]; ok {
		m.CidrMask = types.StringValue(fmt.Sprint(anyToInt32(v, "matches.cidrMask")))
	}

	if v, ok := ruleElements["netMask"]; ok && v.(string) != "" {
		m.NetMask = types.StringValue(v.(string))
	}

	return m
}

func GoIpv6AddrToModel(ruleElements map[string]any, fmType string) *Ipv6AddrRuleModel {
	m := &Ipv6AddrRuleModel{
		Type: types.StringValue(fmType),
	}

	if pos, ok := ruleElements["pos"]; ok {
		m.Pos = types.Int32Value(anyToInt32(pos, "matches.pos"))
	} else {
		m.Pos = types.Int32Value(0)
	}

	if v, ok := ruleElements["value"]; ok {
		m.Address = types.StringValue(v.(string))
	}

	if v, ok := ruleElements["valueMax"]; ok && v.(string) != "" {
		m.AddressMax = types.StringValue(v.(string))
	}

	if v, ok := ruleElements["cidrMask"]; ok {
		// FM sends numeric cidrMask as float64, we normalize via anyToInt32.
		m.CidrMask = types.StringValue(fmt.Sprint(anyToInt32(v, "matches.cidrMask")))
	}

	if v, ok := ruleElements["netMask"]; ok && v.(string) != "" {
		m.NetMask = types.StringValue(v.(string))
	}

	return m
}

var backendIp4FragToTf = map[string]string{
	"noFrag":         "unfragmented_only",
	"allFrag":        "any_fragment",
	"allFragNoFirst": "non_first_fragments",
	"firstFrag":      "first_fragment_only",
	"firstOrNoFrag":  "first_or_unfragmented",
}

func GoIp4FragToModel(ruleElements map[string]any) *Ip4FragRuleModel {
	m := &Ip4FragRuleModel{
		Type: types.StringValue("ip4Frag"),
	}

	if pos, ok := ruleElements["pos"]; ok {
		m.Pos = types.Int32Value(anyToInt32(pos, "matches.pos"))
	} else {
		m.Pos = types.Int32Value(0)
	}

	if v, ok := ruleElements["value"]; ok {
		backend := v.(string)
		if tf, ok2 := backendIp4FragToTf[backend]; ok2 {
			m.Value = types.StringValue(tf)
		} else {
			// Fallback; should not happen
			m.Value = types.StringValue("unfragmented_only")
		}
	} else {
		m.Value = types.StringValue("unfragmented_only")
	}

	return m
}

func GoIp4ProtoToModel(ruleElements map[string]any) *Ip4ProtoRuleModel {
	m := &Ip4ProtoRuleModel{
		Type: types.StringValue("ip4Proto"),
	}

	// pos (0..3)
	if pos, ok := ruleElements["pos"]; ok {
		m.Pos = types.Int32Value(anyToInt32(pos, "matches.pos"))
	} else {
		m.Pos = types.Int32Value(0)
	}

	// value -> protocol_min (numeric; use anyToInt32)
	if v, ok := ruleElements["value"]; ok {
		m.ProtocolMin = types.Int32Value(anyToInt32(v, "matches.value"))
	}

	// valueMax -> protocol_max (numeric; use anyToInt32)
	if v, ok := ruleElements["valueMax"]; ok {
		m.ProtocolMax = types.Int32Value(anyToInt32(v, "matches.valueMax"))
	}

	// subset -> protocol_subset; FM "none" becomes TF "all"
	subset := "all"
	if v, ok := ruleElements["subset"]; ok {
		s := v.(string)
		if s != "" && s != "none" {
			subset = s // "even" or "odd"
		}
	}
	m.ProtocolSubset = types.StringValue(subset)

	return m
}

// ---------- Map FM update helpers (traffic / inclusion / exclusion) ----------
// MapKind represents the FM entityType for maps in the MS update API.
// These strings MUST match what FM expects in the JSON "entityType" field.
type MapKind string

const (
	MapKindTraffic   MapKind = "trafficMap"
	MapKindInclusion MapKind = "inclusionMap"
	MapKindExclusion MapKind = "exclusionMap"
)

// applyMSMapUpdate is the central helper used by all map resources.
// It builds a commonutils.UpdateReq with a single "map" object and
// calls the /cloud/monitoringSessions/{id}/update API.
//
// On "create", it returns the raw FM UUID of the new map (for typed ID).
// On "update" and "delete", the returned id will be empty.
func applyMSMapUpdate(
	ctx context.Context,
	fm *fmclient.FmClient,
	monitoringSessId string,
	kind MapKind,
	op string, // "create" | "update" | "delete"
	goMap *MapGo,
) (string, error) {
	if goMap == nil {
		return "", fmt.Errorf("applyMSMapUpdate: goMap is nil")
	}

	req := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: string(kind),
				Operation:  op,
				Map:        goMap,
			},
		},
	}

	id, err := commonutils.UpdateMonSess(ctx, &req, monitoringSessId, fm)
	if err != nil {
		return "", err
	}
	return id, nil
}

// CreateMSMap converts the TF model into MapGo and creates a map
// of the requested kind within the given Monitoring Session.
//
// Returns raw FM UUID of the newly-created map.
func CreateMSMap(
	ctx context.Context,
	kind MapKind,
	model *MapModel,
	fm *fmclient.FmClient,
) (string, error) {
	if model == nil {
		return "", fmt.Errorf("CreateMSMap: model is nil")
	}
	msID := model.MonitoringSessionId.ValueString()
	goMap := ModelMapToGoMap(ctx, model)

	return applyMSMapUpdate(ctx, fm, msID, kind, "create", goMap)
}

// UpdateMSMap updates an existing map (traffic / inclusion / exclusion)
// based on the contents of the TF model.
func UpdateMSMap(
	ctx context.Context,
	kind MapKind,
	model *MapModel,
	fm *fmclient.FmClient,
) error {
	if model == nil {
		return fmt.Errorf("UpdateMSMap: model is nil")
	}
	msID := model.MonitoringSessionId.ValueString()
	goMap := ModelMapToGoMap(ctx, model)

	_, err := applyMSMapUpdate(ctx, fm, msID, kind, "update", goMap)
	return err
}

// DeleteMSMap deletes an existing map by raw FM UUID.
//
// rawID must be the untyped FM map id (output of UUIDFromTypedID).
func DeleteMSMap(
	ctx context.Context,
	kind MapKind,
	monitoringSessId string,
	rawID string,
	fm *fmclient.FmClient,
) error {
	if rawID == "" {
		return fmt.Errorf("DeleteMSMap: rawID is empty")
	}
	goMap := &MapGo{Id: rawID}

	_, err := applyMSMapUpdate(ctx, fm, monitoringSessId, kind, "delete", goMap)
	return err
}
