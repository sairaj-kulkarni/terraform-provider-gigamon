// Copyright (c) Gigamon, Inc.

// Implements the Tunnel resources that are common across all environments.

package commonresources

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// NewTunnelOut creates a new OUT (egress) tunnel resource.
func NewTunnelOut() resource.Resource {
	return &tunnelOutResource{}
}

// NewTunnelIn creates a new IN (ingress) tunnel resource.
func NewTunnelIn() resource.Resource {
	return &tunnelInResource{}
}

// tunnelOutResource manages an egress tunnel endpoint instance within a Monitoring Session.
type tunnelOutResource struct {
	fmClient *fmclient.FmClient // FM http client instance
}

// tunnelInResource manages an ingress tunnel endpoint instance within a Monitoring Session.
type tunnelInResource struct {
	fmClient *fmclient.FmClient // FM http client instance
}

// Ensure resources satisfy the framework interfaces.
var _ resource.Resource = &tunnelOutResource{}
var _ resource.Resource = &tunnelInResource{}

// ---------- Type-specific nested configs ----------

type L2GreConfig struct {
	Key types.Int32 `tfsdk:"key"` // GRE key
}

// type UdpGreConfig struct {
// 	Key types.Int32 `tfsdk:"key"` // GRE key over UDP
// }

type VxlanConfig struct {
	Vni             types.Int32 `tfsdk:"vni"` // VXLAN Network Identifier
	DestinationPort types.Int32 `tfsdk:"destination_port"`
}

type GeneveConfig struct {
	Vni             types.Int32 `tfsdk:"vni"`              // Geneve VNI
	DestinationPort types.Int32 `tfsdk:"destination_port"` // UDP dest port
}

type ErspanConfig struct {
	FlowId types.Int32 `tfsdk:"flow_id"` // ERSPAN Flow ID
}

type TlsPcapngConfig struct {
	EnableMtls      types.Bool   `tfsdk:"enable_mtls"`
	TlsKeyAlias     types.String `tfsdk:"tls_key_alias"`
	TlsCipher       types.String `tfsdk:"tls_cipher"`
	TlsVersion      types.String `tfsdk:"tls_version"`
	TlsSAck         types.String `tfsdk:"tls_sack"`
	TlsSynRetries   types.Int32  `tfsdk:"tls_syn_retries"`
	TlsDelayAck     types.String `tfsdk:"tls_delay_ack"`
	SourcePort      types.Int32  `tfsdk:"source_port"`
	DestinationPort types.Int32  `tfsdk:"destination_port"`
}

type UdpConfig struct {
	SourcePort      types.Int32 `tfsdk:"source_port"`
	DestinationPort types.Int32 `tfsdk:"destination_port"`
}

// ---------- TF Models ----------

// TunnelOutModel describes the Terraform model for the egress tunnel resource.
type TunnelOutModel struct {
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`
	Alias               types.String `tfsdk:"alias"`

	// MS-level tunnel instance ID (returned by MS update API).
	Id types.String `tfsdk:"id"`

	// Type and direction
	Type             types.String `tfsdk:"type"`              // Computed from blocks
	TrafficDirection types.String `tfsdk:"traffic_direction"` // always "out"

	// Common fields for egress tunnels
	Description types.String `tfsdk:"description"`
	RemoteIP    types.String `tfsdk:"remote_ip"`  // peer IP
	Mtu         types.Int32  `tfsdk:"mtu"`        // bytes
	Ttl         types.Int32  `tfsdk:"ttl"`        // hops
	Dscp        types.Int32  `tfsdk:"dscp"`       // 0–63
	Prec        types.Int32  `tfsdk:"prec"`       // 0–7
	FlowLabel   types.Int32  `tfsdk:"flow_label"` // IPv6 flow label

	// Type-specific blocks (exactly one must be set)
	L2Gre     *L2GreConfig     `tfsdk:"l2gre"`
	Vxlan     *VxlanConfig     `tfsdk:"vxlan"`
	TlsPcapng *TlsPcapngConfig `tfsdk:"tls_pcapng"`
	Udp       *UdpConfig       `tfsdk:"udp"`
}

// TunnelInModel describes the Terraform model for the ingress tunnel resource.
type TunnelInModel struct {
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`
	Alias               types.String `tfsdk:"alias"`

	// MS-level tunnel instance ID (returned by MS update API).
	Id types.String `tfsdk:"id"`

	// Type and direction
	Type             types.String `tfsdk:"type"`              // Computed from blocks
	TrafficDirection types.String `tfsdk:"traffic_direction"` // always "in"

	// Common fields for ingress tunnels
	Description types.String `tfsdk:"description"`
	RemoteIP    types.String `tfsdk:"remote_ip"` // peer IP if applicable

	// Type-specific blocks
	L2Gre *L2GreConfig `tfsdk:"l2gre"`
	// UdpGre    *UdpGreConfig    `tfsdk:"udpgre"`
	Vxlan     *VxlanConfig     `tfsdk:"vxlan"`
	Geneve    *GeneveConfig    `tfsdk:"geneve"`
	Erspan    *ErspanConfig    `tfsdk:"erspan"`
	TlsPcapng *TlsPcapngConfig `tfsdk:"tls_pcapng"`
}

// FMTunnel is the FM representation of a tunnel instance.
type FMTunnel struct {
	Type             string `json:"type,omitempty"`
	Id               string `json:"id,omitempty"`
	Alias            string `json:"alias,omitempty"`
	Description      string `json:"description,omitempty"`
	TrafficDirection string `json:"trafficDirection,omitempty"`
	IpVersion        string `json:"ipVersion,omitempty"`
	AdminState       string `json:"adminState,omitempty"`

	RemoteIP  string `json:"remoteIP,omitempty"`
	Mtu       int32  `json:"mtu,omitempty"`
	Ttl       int32  `json:"ttl,omitempty"`
	Dscp      int32  `json:"dscp,omitempty"`
	Prec      int32  `json:"prec,omitempty"`
	FlowLabel int32  `json:"flowLabel,omitempty"`

	// Type-specific (non-TLS)
	Key     int32 `json:"key,omitempty"`   // L2GRE/UDPGRE key
	Vni     int32 `json:"vni,omitempty"`   // VXLAN / Geneve VNI
	SPort   int32 `json:"sport,omitempty"` // source L4 port
	DPort   int32 `json:"dport,omitempty"` // dest L4 port
	FlowId  int32 `json:"flowId,omitempty"`
	Multi   bool  `json:"multiTunnel,omitempty"`
	NumTuns int32 `json:"numTunnels,omitempty"`

	// TLS-PCAPNG (TcpTunnel) specific (FM wire format)
	Mtls       string `json:"mtls,omitempty"` // "enable"/"disable"
	KeyAlias   string `json:"keyAlias,omitempty"`
	Cipher     string `json:"cipher,omitempty"`
	TlsVersion string `json:"tlsVersion,omitempty"`
	SAck       string `json:"sAck,omitempty"` // "enable"/"disable"
	SynRetries int32  `json:"synRetries,omitempty"`
	DelayAck   string `json:"delayAck,omitempty"` // "enable"/"disable"
}

// ---------------------- Metadata ----------------------

func (r *tunnelOutResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tunnel_out"
}

func (r *tunnelInResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tunnel_in"
}

// ---------------------- Schema helpers ----------------

func l2GreBlock() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: "L2GRE tunnel parameters.",
		Attributes: map[string]schema.Attribute{
			"key": schema.Int32Attribute{
				MarkdownDescription: "L2GRE key (0–4294967295).",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
				Validators: []validator.Int32{
					int32validator.AtLeast(0),
				},
			},
		},
	}
}

// func udpGreBlock() schema.SingleNestedBlock {
// 	return schema.SingleNestedBlock{
// 		MarkdownDescription: "UDPGRE tunnel parameters.",
// 		Attributes: map[string]schema.Attribute{
// 			"key": schema.Int32Attribute{
// 				MarkdownDescription: "UDPGRE key (1–4294967295).",
// 				Optional:            true,
// 				Validators: []validator.Int32{
// 					int32validator.AtLeast(1),
// 				},
// 			},
// 		},
// 	}
// }

func vxlanBlock() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: "VXLAN tunnel parameters.",
		Attributes: map[string]schema.Attribute{
			"vni": schema.Int32Attribute{
				MarkdownDescription: "VXLAN Network Identifier (1–16777215).",
				Optional:            true,
				Validators: []validator.Int32{
					int32validator.Between(1, 16777215),
				},
			},
			"destination_port": schema.Int32Attribute{
				MarkdownDescription: "Destination UDP port for this VXLAN tunnel (1–65535).",
				Optional:            true, // keep Optional; enforce required later via common validator
				Validators: []validator.Int32{
					int32validator.Between(1, 65535),
				},
			},
		},
	}
}

func geneveBlock() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: "Geneve tunnel parameters.",
		Attributes: map[string]schema.Attribute{
			"vni": schema.Int32Attribute{
				MarkdownDescription: "Geneve VNI (1–16777215).",
				Optional:            true,
				Validators: []validator.Int32{
					int32validator.Between(1, 16777215),
				},
			},
			"destination_port": schema.Int32Attribute{
				MarkdownDescription: "Destination UDP port for this Geneve tunnel (1–65535).",
				Optional:            true,
				Validators: []validator.Int32{
					int32validator.Between(1, 65535),
				},
			},
		},
	}
}

func erspanBlock() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: "ERSPAN tunnel parameters.",
		Attributes: map[string]schema.Attribute{
			"flow_id": schema.Int32Attribute{
				MarkdownDescription: "ERSPAN Flow ID (1–1023).",
				Optional:            true,
				Validators: []validator.Int32{
					int32validator.Between(1, 1023),
				},
			},
		},
	}
}

func baseTlsPcapngBlock() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: "TLS-PCAPNG tunnel parameters.",
		Attributes: map[string]schema.Attribute{
			"enable_mtls": schema.BoolAttribute{
				MarkdownDescription: "Enable mTLS for this TLS-PCAPNG tunnel.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"tls_key_alias": schema.StringAttribute{
				MarkdownDescription: "Key alias for this TLS-PCAPNG tunnel.",
				Optional:            true,
			},
			"tls_cipher": schema.StringAttribute{
				MarkdownDescription: "Cipher suite label for this TLS-PCAPNG tunnel.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tls_version": schema.StringAttribute{
				MarkdownDescription: "TLS version label for this TLS-PCAPNG tunnel (e.g., TLS1.3).",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tls_sack": schema.StringAttribute{
				MarkdownDescription: "Selective ACK state for this TLS-PCAPNG tunnel (enable/disable).",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tls_syn_retries": schema.Int32Attribute{
				MarkdownDescription: "SYN retries for this TLS-PCAPNG tunnel (1–6).",
				Optional:            true,
				Computed:            true,
				Validators: []validator.Int32{
					int32validator.Between(1, 6),
				},
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.UseStateForUnknown(),
				},
			},
			"tls_delay_ack": schema.StringAttribute{
				MarkdownDescription: "Delay ACK state for this TLS-PCAPNG tunnel (enable/disable).",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func tlsPcapngOutBlock() schema.SingleNestedBlock {
	b := baseTlsPcapngBlock()

	b.Attributes["source_port"] = schema.Int32Attribute{
		MarkdownDescription: "Source L4 port for this TLS-PCAPNG egress tunnel (1–65535).",
		Optional:            true,
		Computed:            true,
		Validators: []validator.Int32{
			int32validator.Between(1, 65535),
		},
	}
	b.Attributes["destination_port"] = schema.Int32Attribute{
		MarkdownDescription: "Destination L4 port for this TLS-PCAPNG egress tunnel (1–65535).",
		Optional:            true,
		Validators: []validator.Int32{
			int32validator.Between(1, 65535),
		},
	}

	return b
}

func tlsPcapngInBlock() schema.SingleNestedBlock {
	b := baseTlsPcapngBlock()

	b.Attributes["source_port"] = schema.Int32Attribute{
		MarkdownDescription: "Source L4 port for this TLS-PCAPNG ingress tunnel (1–65535).",
		Optional:            true,
		Computed:            true,
		Validators: []validator.Int32{
			int32validator.Between(1, 65535),
		},
	}
	b.Attributes["destination_port"] = schema.Int32Attribute{
		MarkdownDescription: "Destination L4 port for this TLS-PCAPNG ingress tunnel (1–65535).",
		Optional:            true,
		Validators: []validator.Int32{
			int32validator.Between(1, 65535),
		},
	}

	return b
}

func udpBlock() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: "UDP tunnel parameters",
		Attributes: map[string]schema.Attribute{
			"source_port": schema.Int32Attribute{
				MarkdownDescription: "Source L4 port for this UDP egress tunnel (1–65535).",
				Optional:            true, // keep Optional for now; enforce later via common validator
				Validators: []validator.Int32{
					int32validator.Between(1, 65535),
				},
			},
			"destination_port": schema.Int32Attribute{
				MarkdownDescription: "Destination L4 port for this UDP egress tunnel (1–65535).",
				Optional:            true,
				Validators: []validator.Int32{
					int32validator.Between(1, 65535),
				},
			},
		},
	}
}

// ---------------------- Schema ------------------------

// Egress schema
func (r *tunnelOutResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Gigamon Cloud egress tunnel endpoint for a Monitoring Session.",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Alias/name for this egress tunnel.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},

			"monitoring_session_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring Session ID on which to configure this egress tunnel.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"type": schema.StringAttribute{
				MarkdownDescription: "Egress tunnel type (derived from the configured block).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"traffic_direction": schema.StringAttribute{
				MarkdownDescription: "Traffic direction for this tunnel endpoint (out).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"description": schema.StringAttribute{
				MarkdownDescription: "Description for this egress tunnel.",
				Optional:            true,
			},

			"remote_ip": schema.StringAttribute{
				MarkdownDescription: "Remote peer IP address for this egress tunnel.",
				Required:            true,
				Validators: []validator.String{
					ipLiteralValidator{},
				},
			},

			"mtu": schema.Int32Attribute{
				MarkdownDescription: "Egress tunnel MTU in bytes (1280–9600).",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(1500),
				Validators: []validator.Int32{
					int32validator.Between(1280, 9600),
				},
			},

			"ttl": schema.Int32Attribute{
				MarkdownDescription: "Outer IP TTL for this egress tunnel (1–255).",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(64),
				Validators: []validator.Int32{
					int32validator.Between(1, 255),
				},
			},

			"dscp": schema.Int32Attribute{
				MarkdownDescription: "Outer IP DSCP value for this egress tunnel (0–63).",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
				Validators: []validator.Int32{
					int32validator.Between(0, 63),
				},
			},

			"prec": schema.Int32Attribute{
				MarkdownDescription: "Outer IP precedence for this egress tunnel (0–7).",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
				Validators: []validator.Int32{
					int32validator.Between(0, 7),
				},
			},

			"flow_label": schema.Int32Attribute{
				MarkdownDescription: "IPv6 flow label for this egress tunnel (0–1048575).",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
				Validators: []validator.Int32{
					int32validator.Between(0, 1048575),
				},
			},

			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this egress tunnel instance within the Monitoring Session.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},

		Blocks: map[string]schema.Block{
			"l2gre":      l2GreBlock(),
			"vxlan":      vxlanBlock(),
			"tls_pcapng": tlsPcapngOutBlock(),
			"udp":        udpBlock(),
		},
	}
}

// Ingress schema
func (r *tunnelInResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Gigamon Cloud ingress tunnel endpoint for a Monitoring Session.",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Alias/name for this ingress tunnel.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},

			"monitoring_session_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring Session ID on which to configure this ingress tunnel.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"type": schema.StringAttribute{
				MarkdownDescription: "Ingress tunnel type (derived from the configured block).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"traffic_direction": schema.StringAttribute{
				MarkdownDescription: "Traffic direction for this tunnel endpoint (in).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"description": schema.StringAttribute{
				MarkdownDescription: "Description for this ingress tunnel.",
				Optional:            true,
			},

			"remote_ip": schema.StringAttribute{
				MarkdownDescription: "Remote peer IP address for this ingress tunnel.",
				Optional:            true,
				Validators: []validator.String{
					ipLiteralValidator{},
				},
			},

			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this ingress tunnel instance within the Monitoring Session.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},

		Blocks: map[string]schema.Block{
			"l2gre": l2GreBlock(),
			// "udpgre":     udpGreBlock(),
			"vxlan":      vxlanBlock(),
			"geneve":     geneveBlock(),
			"erspan":     erspanBlock(),
			"tls_pcapng": tlsPcapngInBlock(),
		},
	}
}

// ---------------------- Config validators (one-of blocks) --------------------

func (r *tunnelOutResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("l2gre"),
			path.MatchRoot("vxlan"),
			path.MatchRoot("tls_pcapng"),
			path.MatchRoot("udp"),
		),
		tlsKeyAliasOutValidator{},
	}
}

func (r *tunnelInResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("l2gre"),
			// path.MatchRoot("udpgre"),
			path.MatchRoot("vxlan"),
			path.MatchRoot("geneve"),
			path.MatchRoot("erspan"),
			path.MatchRoot("tls_pcapng"),
		),
	}
}

// ---------------------- Configure ---------------------

func (r *tunnelOutResource) Configure(
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
	r.fmClient = fmClient
}

func (r *tunnelInResource) Configure(
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
	r.fmClient = fmClient
}

// ---------- type inference helper ----------

func inferTunnelTypeFromBlocks(
	l2 *L2GreConfig,
	// ug *UdpGreConfig,
	vx *VxlanConfig,
	ge *GeneveConfig,
	er *ErspanConfig,
	tls *TlsPcapngConfig,
	udp *UdpConfig,
) string {
	switch {
	case l2 != nil:
		return "l2gre"
	// case ug != nil:
	// 	return "udpgre"
	case vx != nil:
		return "vxlan"
	case ge != nil:
		return "geneve"
	case er != nil:
		return "erspan"
	case tls != nil:
		return "tlspcapng"
	case udp != nil:
		return "udp"
	default:
		return ""
	}
}

func inferIpVersionFromRemoteIP(remoteIP string) string {
	ip := net.ParseIP(remoteIP)
	if ip == nil {
		return ""
	}
	if ip.To4() != nil {
		return "IPV4"
	}
	return "IPV6"
}

// ---------------------- Validators --------------------

// ipLiteralValidator ensures remote_ip is a valid IPv4 or IPv6 literal.
type ipLiteralValidator struct{}

func (v ipLiteralValidator) Description(ctx context.Context) string {
	return "must be a valid IPv4 or IPv6 address literal"
}

func (v ipLiteralValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v ipLiteralValidator) ValidateString(
	ctx context.Context,
	req validator.StringRequest,
	resp *validator.StringResponse,
) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	s := req.ConfigValue.ValueString()
	if net.ParseIP(s) == nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid IP address",
			fmt.Sprintf("remote_ip %q is not a valid IPv4 or IPv6 literal", s),
		)
	}
}

// tlsKeyAliasOutValidator forbids tls_key_alias on TLS-PCAPNG egress tunnels.
type tlsKeyAliasOutValidator struct{}

func (v tlsKeyAliasOutValidator) Description(ctx context.Context) string {
	return "tls_key_alias is not supported on TLS-PCAPNG egress tunnels"
}

func (v tlsKeyAliasOutValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v tlsKeyAliasOutValidator) ValidateResource(
	ctx context.Context,
	req resource.ValidateConfigRequest,
	resp *resource.ValidateConfigResponse,
) {
	var data TunnelOutModel

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.TlsPcapng != nil &&
		!data.TlsPcapng.TlsKeyAlias.IsNull() &&
		!data.TlsPcapng.TlsKeyAlias.IsUnknown() {

		resp.Diagnostics.AddAttributeError(
			path.Root("tls_pcapng").AtName("tls_key_alias"),
			"tls_key_alias not supported for TLS-PCAPNG egress tunnels",
			"tls_key_alias is only valid for ingress TLS-PCAPNG tunnels (gigamon_tunnel_in). "+
				"Please remove it from gigamon_tunnel_out.",
		)
	}
}

// ---------------------- FMTunnel builders --------------

// Map OUT model to FM
func createFMTunnelFromOut(data *TunnelOutModel) *FMTunnel {
	t := inferTunnelTypeFromBlocks(
		data.L2Gre,
		data.Vxlan,
		nil,
		nil,
		data.TlsPcapng,
		data.Udp,
	)

	// Ensure Computed attribute "type" is known in state
	if t != "" {
		data.Type = types.StringValue(t)
	}

	remoteIP := data.RemoteIP.ValueString()

	fm := &FMTunnel{
		Alias:            data.Alias.ValueString(),
		Description:      data.Description.ValueString(),
		Type:             t,
		TrafficDirection: "out",
		IpVersion:        inferIpVersionFromRemoteIP(remoteIP),
		RemoteIP:         remoteIP,
		Mtu:              data.Mtu.ValueInt32(),
		Ttl:              data.Ttl.ValueInt32(),
		Dscp:             data.Dscp.ValueInt32(),
		Prec:             data.Prec.ValueInt32(),
		FlowLabel:        data.FlowLabel.ValueInt32(),
		AdminState:       "enabled",
	}

	switch t {
	case "l2gre":
		if data.L2Gre != nil {
			fm.Key = data.L2Gre.Key.ValueInt32()
		}

	case "vxlan":
		if data.Vxlan != nil {
			if !data.Vxlan.Vni.IsNull() && !data.Vxlan.Vni.IsUnknown() {
				fm.Vni = data.Vxlan.Vni.ValueInt32()
			}
			if !data.Vxlan.DestinationPort.IsNull() && !data.Vxlan.DestinationPort.IsUnknown() {
				fm.DPort = data.Vxlan.DestinationPort.ValueInt32()
			}
		}

	case "udp":
		if data.Udp != nil {
			if !data.Udp.SourcePort.IsNull() && !data.Udp.SourcePort.IsUnknown() {
				fm.SPort = data.Udp.SourcePort.ValueInt32()
			}
			if !data.Udp.DestinationPort.IsNull() && !data.Udp.DestinationPort.IsUnknown() {
				fm.DPort = data.Udp.DestinationPort.ValueInt32()
			}
		}

	case "tlspcapng":
		if data.TlsPcapng != nil {
			if !data.TlsPcapng.EnableMtls.IsNull() && !data.TlsPcapng.EnableMtls.IsUnknown() {
				if data.TlsPcapng.EnableMtls.ValueBool() {
					fm.Mtls = "enable"
				} else {
					fm.Mtls = "disable"
				}
			}
			fm.Cipher = data.TlsPcapng.TlsCipher.ValueString()
			fm.TlsVersion = data.TlsPcapng.TlsVersion.ValueString()
			fm.SAck = data.TlsPcapng.TlsSAck.ValueString()
			fm.SynRetries = data.TlsPcapng.TlsSynRetries.ValueInt32()
			fm.DelayAck = data.TlsPcapng.TlsDelayAck.ValueString()
			fm.SPort = data.TlsPcapng.SourcePort.ValueInt32()
			fm.DPort = data.TlsPcapng.DestinationPort.ValueInt32()
		}
	}

	return fm
}

// Map IN model to FM
func createFMTunnelFromIn(data *TunnelInModel) *FMTunnel {
	t := inferTunnelTypeFromBlocks(
		data.L2Gre,
		// data.UdpGre,
		data.Vxlan,
		data.Geneve,
		data.Erspan,
		data.TlsPcapng,
		nil,
	)

	// Ensure Computed attribute "type" is known in state
	if t != "" {
		data.Type = types.StringValue(t)
	}

	fm := &FMTunnel{
		Alias:            data.Alias.ValueString(),
		Description:      data.Description.ValueString(),
		Type:             t,
		TrafficDirection: "in",
		AdminState:       "enabled",
	}

	if !data.RemoteIP.IsNull() && !data.RemoteIP.IsUnknown() {
		remoteIP := data.RemoteIP.ValueString()
		fm.RemoteIP = remoteIP
		fm.IpVersion = inferIpVersionFromRemoteIP(remoteIP)
	}

	switch t {
	case "l2gre":
		if data.L2Gre != nil {
			fm.Key = data.L2Gre.Key.ValueInt32()
		}

	// case "udpgre":
	// 	if data.UdpGre != nil {
	// 		fm.Key = data.UdpGre.Key.ValueInt32()
	// 	}

	case "vxlan":
		if data.Vxlan != nil {
			fm.Vni = data.Vxlan.Vni.ValueInt32()
			fm.DPort = data.Vxlan.DestinationPort.ValueInt32()
		}

	case "erspan":
		if data.Erspan != nil {
			fm.FlowId = data.Erspan.FlowId.ValueInt32()
		}

	case "tlspcapng":
		if data.TlsPcapng != nil {
			if !data.TlsPcapng.EnableMtls.IsNull() && !data.TlsPcapng.EnableMtls.IsUnknown() {
				if data.TlsPcapng.EnableMtls.ValueBool() {
					fm.Mtls = "enable"
				} else {
					fm.Mtls = "disable"
				}
			}
			fm.KeyAlias = data.TlsPcapng.TlsKeyAlias.ValueString()
			fm.Cipher = data.TlsPcapng.TlsCipher.ValueString()
			fm.TlsVersion = data.TlsPcapng.TlsVersion.ValueString()
			fm.SAck = data.TlsPcapng.TlsSAck.ValueString()
			fm.SynRetries = data.TlsPcapng.TlsSynRetries.ValueInt32()
			fm.DelayAck = data.TlsPcapng.TlsDelayAck.ValueString()
			fm.SPort = data.TlsPcapng.SourcePort.ValueInt32()
			fm.DPort = data.TlsPcapng.DestinationPort.ValueInt32()
		}

	case "geneve":
		if data.Geneve != nil {
			fm.Vni = data.Geneve.Vni.ValueInt32()
			fm.DPort = data.Geneve.DestinationPort.ValueInt32()
		}
	}

	return fm
}

// updateOutTFStruct copies FM tunnel data into the OUT TF state model.
func updateOutTFStruct(data *TunnelOutModel, fmData *FMTunnel) {
	hadL2Gre := data.L2Gre != nil
	hadVxlan := data.Vxlan != nil
	hadTls := data.TlsPcapng != nil

	data.Alias = types.StringValue(fmData.Alias)

	if fmData.Description != "" {
		data.Description = types.StringValue(fmData.Description)
	}

	data.Type = types.StringValue(fmData.Type)
	data.TrafficDirection = types.StringValue(fmData.TrafficDirection)
	data.RemoteIP = types.StringValue(fmData.RemoteIP)
	data.Mtu = types.Int32Value(fmData.Mtu)
	data.Ttl = types.Int32Value(fmData.Ttl)
	data.Dscp = types.Int32Value(fmData.Dscp)
	data.Prec = types.Int32Value(fmData.Prec)
	data.FlowLabel = types.Int32Value(fmData.FlowLabel)

	data.L2Gre = nil
	data.Vxlan = nil
	data.TlsPcapng = nil

	switch fmData.Type {
	case "l2gre":
		if hadL2Gre || fmData.Key != 0 {
			data.L2Gre = &L2GreConfig{
				Key: types.Int32Value(fmData.Key),
			}
		}

	case "vxlan":
		if hadVxlan || fmData.Vni != 0 || fmData.DPort != 0 {
			cfg := &VxlanConfig{
				Vni: types.Int32Value(fmData.Vni),
			}
			if fmData.DPort != 0 {
				cfg.DestinationPort = types.Int32Value(fmData.DPort)
			}
			data.Vxlan = cfg
		}

	case "tlspcapng":
		nonDefaultTls := fmData.Mtls != "" ||
			fmData.Cipher != "" ||
			fmData.TlsVersion != "" ||
			fmData.SAck != "" ||
			fmData.SynRetries != 0 ||
			fmData.DelayAck != "" ||
			fmData.SPort != 0 ||
			fmData.DPort != 0

		if hadTls || nonDefaultTls {
			cfg := &TlsPcapngConfig{
				TlsCipher:       types.StringValue(fmData.Cipher),
				TlsVersion:      types.StringValue(fmData.TlsVersion),
				TlsSAck:         types.StringValue(fmData.SAck),
				TlsSynRetries:   types.Int32Value(fmData.SynRetries),
				TlsDelayAck:     types.StringValue(fmData.DelayAck),
				SourcePort:      types.Int32Value(fmData.SPort),
				DestinationPort: types.Int32Value(fmData.DPort),
			}
			switch fmData.Mtls {
			case "enable":
				cfg.EnableMtls = types.BoolValue(true)
			case "disable":
				cfg.EnableMtls = types.BoolValue(false)
			default:
				cfg.EnableMtls = types.BoolNull()
			}
			data.TlsPcapng = cfg
		}
	}
}

// updateInTFStruct copies FM tunnel data into the IN TF state model.
func updateInTFStruct(data *TunnelInModel, fmData *FMTunnel) {
	hadL2Gre := data.L2Gre != nil
	// hadUdpGre := data.UdpGre != nil
	hadVxlan := data.Vxlan != nil
	hadGeneve := data.Geneve != nil
	hadErspan := data.Erspan != nil
	hadTls := data.TlsPcapng != nil

	data.Alias = types.StringValue(fmData.Alias)

	if fmData.Description != "" {
		data.Description = types.StringValue(fmData.Description)
	}

	data.Type = types.StringValue(fmData.Type)
	data.TrafficDirection = types.StringValue(fmData.TrafficDirection)

	if fmData.RemoteIP != "" {
		data.RemoteIP = types.StringValue(fmData.RemoteIP)
	}

	data.L2Gre = nil
	// data.UdpGre = nil
	data.Vxlan = nil
	data.Geneve = nil
	data.Erspan = nil
	data.TlsPcapng = nil

	switch fmData.Type {
	case "l2gre":
		if hadL2Gre || fmData.Key != 0 {
			data.L2Gre = &L2GreConfig{
				Key: types.Int32Value(fmData.Key),
			}
		}

	// case "udpgre":
	// 	if hadUdpGre || fmData.Key != 0 {
	// 		data.UdpGre = &UdpGreConfig{
	// 			Key: types.Int32Value(fmData.Key),
	// 		}
	// 	}

	case "vxlan":
		if hadVxlan || fmData.Vni != 0 || fmData.DPort != 0 {
			cfg := &VxlanConfig{
				Vni: types.Int32Value(fmData.Vni),
			}
			if fmData.DPort != 0 {
				cfg.DestinationPort = types.Int32Value(fmData.DPort)
			}
			data.Vxlan = cfg
		}

	case "geneve":
		if hadGeneve || fmData.Vni != 0 {
			data.Geneve = &GeneveConfig{
				Vni:             types.Int32Value(fmData.Vni),
				DestinationPort: types.Int32Value(fmData.DPort),
			}
		}

	case "erspan":
		if hadErspan || fmData.FlowId != 0 {
			data.Erspan = &ErspanConfig{
				FlowId: types.Int32Value(fmData.FlowId),
			}
		}

	case "tlspcapng":
		nonDefaultTls := fmData.Mtls != "" ||
			fmData.KeyAlias != "" ||
			fmData.Cipher != "" ||
			fmData.TlsVersion != "" ||
			fmData.SAck != "" ||
			fmData.SynRetries != 0 ||
			fmData.DelayAck != "" ||
			fmData.SPort != 0 ||
			fmData.DPort != 0

		if hadTls || nonDefaultTls {
			cfg := &TlsPcapngConfig{
				TlsKeyAlias:     types.StringValue(fmData.KeyAlias),
				TlsCipher:       types.StringValue(fmData.Cipher),
				TlsVersion:      types.StringValue(fmData.TlsVersion),
				TlsSAck:         types.StringValue(fmData.SAck),
				TlsSynRetries:   types.Int32Value(fmData.SynRetries),
				TlsDelayAck:     types.StringValue(fmData.DelayAck),
				SourcePort:      types.Int32Value(fmData.SPort),
				DestinationPort: types.Int32Value(fmData.DPort),
			}
			switch fmData.Mtls {
			case "enable":
				cfg.EnableMtls = types.BoolValue(true)
			case "disable":
				cfg.EnableMtls = types.BoolValue(false)
			default:
				cfg.EnableMtls = types.BoolNull()
			}
			data.TlsPcapng = cfg
		}
	}
}

// ---------------------- MS helpers --------------------

func GetMSTunnelData(
	ctx context.Context,
	monitoringSessId, tunnelId, tunnelAlias string,
	tunnelData *FMTunnel,
	fmClient *fmclient.FmClient,
) (bool, error) {

	if tunnelId != "" {
		rawID, err := commonutils.UUIDFromTypedID(tunnelId)
		if err != nil {
			return false, err
		}
		tunnelId = rawID
	}

	fmResp := struct {
		Id      string     `json:"id,omitempty"`
		Tunnels []FMTunnel `json:"tunnels"`
	}{
		Id: monitoringSessId,
	}

	err := UpdateMSData(ctx, monitoringSessId, &fmResp, fmClient)
	if err != nil {
		return false, err
	}

	for _, tnl := range fmResp.Tunnels {
		// Match strictly by FM tunnel ID; alias is mutable and not used for identity.
		if tunnelId != "" && tunnelId == tnl.Id {
			*tunnelData = tnl
			return true, nil
		}
	}

	return false, nil
}

// ---------------------- OUT: CRUD ---------------------

func (r *tunnelOutResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TunnelOutModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.TrafficDirection = types.StringValue("out")

	fmTunnel := createFMTunnelFromOut(&data)

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "tunnel",
				Operation:  "create",
				Tunnel:     fmTunnel,
			},
		},
	}

	id, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		r.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create tunnel in Monitoring Session",
			fmt.Sprintf("tunnel creation failed: %s", err),
		)
		return
	}

	typedID, err := commonutils.MakeTypedID(
		commonutils.ModuleTunnelOut,
		commonutils.Type(fmTunnel.Type),
		id,
	)
	if err != nil {
		return
	}
	data.Id = types.StringValue(typedID)

	// read back from FM so Computed TLS fields become known
	var tunnelData FMTunnel
	ok, err := GetMSTunnelData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		data.Id.ValueString(),
		data.Alias.ValueString(),
		&tunnelData,
		r.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to get tunnel details from Monitoring Session after create",
			fmt.Sprintf("unable to get tunnel details. error is %s", err),
		)
		return
	}
	if !ok {
		resp.Diagnostics.AddError(
			"Unable to get tunnel details from Monitoring Session after create",
			"tunnel not found in Monitoring Session after creation",
		)
		return
	}

	updateOutTFStruct(&data, &tunnelData)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *tunnelOutResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TunnelOutModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tunnelData := FMTunnel{}

	ok, err := GetMSTunnelData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		data.Id.ValueString(),
		data.Alias.ValueString(),
		&tunnelData,
		r.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) && fmErr.ErrorCode() == fmclient.ObjectNotFound {
			// MS or tunnel gone in FM -> drop from state, no hard error
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Unable to get tunnel details from Monitoring Session",
			fmt.Sprintf("unable to get tunnel details. error is %s", err),
		)
		return
	}
	if !ok {
		// MS exists but this tunnel not found in its tunnels[] -> drop from state
		resp.State.RemoveResource(ctx)
		return
	}

	updateOutTFStruct(&data, &tunnelData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *tunnelOutResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan TunnelOutModel
	var state TunnelOutModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.TrafficDirection = types.StringValue("out")

	fmTunnel := createFMTunnelFromOut(&plan)
	rawID, err := commonutils.UUIDFromTypedID(state.Id.ValueString())
	if err != nil {
		return
	}
	fmTunnel.Id = rawID

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "tunnel",
				Operation:  "update",
				Tunnel:     fmTunnel,
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		state.MonitoringSessionId.ValueString(),
		r.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update tunnel in Monitoring Session",
			fmt.Sprintf("tunnel update failed: %s", err),
		)
		return
	}

	plan.Id = state.Id

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tunnelOutResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TunnelOutModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rawID, err := commonutils.UUIDFromTypedID(data.Id.ValueString())
	if err != nil {
		return
	}

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "tunnel",
				Operation:  "delete",
				Tunnel: FMTunnel{
					Id:   rawID,
					Type: data.Type.ValueString(),
				},
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		r.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete tunnel from Monitoring Session",
			fmt.Sprintf("tunnel deletion failed: %s", err),
		)
		return
	}
}

// ---------------------- IN: CRUD ----------------------

func (r *tunnelInResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TunnelInModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.TrafficDirection = types.StringValue("in")

	fmTunnel := createFMTunnelFromIn(&data)

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "tunnel",
				Operation:  "create",
				Tunnel:     fmTunnel,
			},
		},
	}

	id, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		r.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create tunnel in Monitoring Session",
			fmt.Sprintf("tunnel creation failed: %s", err),
		)
		return
	}

	typedID, err := commonutils.MakeTypedID(
		commonutils.ModuleTunnelIn,
		commonutils.Type(fmTunnel.Type),
		id,
	)
	if err != nil {
		return
	}
	data.Id = types.StringValue(typedID)

	// read back from FM so Computed TLS fields become known
	var tunnelData FMTunnel
	ok, err := GetMSTunnelData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		data.Id.ValueString(),
		data.Alias.ValueString(),
		&tunnelData,
		r.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to get tunnel details from Monitoring Session after create",
			fmt.Sprintf("unable to get tunnel details. error is %s", err),
		)
		return
	}
	if !ok {
		resp.Diagnostics.AddError(
			"Unable to get tunnel details from Monitoring Session after create",
			"tunnel not found in Monitoring Session after creation",
		)
		return
	}

	updateInTFStruct(&data, &tunnelData)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *tunnelInResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TunnelInModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tunnelData := FMTunnel{}

	ok, err := GetMSTunnelData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		data.Id.ValueString(),
		data.Alias.ValueString(),
		&tunnelData,
		r.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) && fmErr.ErrorCode() == fmclient.ObjectNotFound {
			// MS or tunnel gone in FM -> drop from state, no hard error
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Unable to get tunnel details from Monitoring Session",
			fmt.Sprintf("unable to get tunnel details. error is %s", err),
		)
		return
	}
	if !ok {
		// MS exists but this tunnel not found in its tunnels[] -> drop from state
		resp.State.RemoveResource(ctx)
		return
	}

	updateInTFStruct(&data, &tunnelData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *tunnelInResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan TunnelInModel
	var state TunnelInModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.TrafficDirection = types.StringValue("in")

	fmTunnel := createFMTunnelFromIn(&plan)
	rawID, err := commonutils.UUIDFromTypedID(state.Id.ValueString())
	if err != nil {
		return
	}
	fmTunnel.Id = rawID

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "tunnel",
				Operation:  "update",
				Tunnel:     fmTunnel,
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		state.MonitoringSessionId.ValueString(),
		r.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update tunnel in Monitoring Session",
			fmt.Sprintf("tunnel update failed: %s", err),
		)
		return
	}

	plan.Id = state.Id

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tunnelInResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TunnelInModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rawID, err := commonutils.UUIDFromTypedID(data.Id.ValueString())
	if err != nil {
		return
	}

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "tunnel",
				Operation:  "delete",
				Tunnel: FMTunnel{
					Id:   rawID,
					Type: data.Type.ValueString(),
				},
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		r.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete tunnel from Monitoring Session",
			fmt.Sprintf("tunnel deletion failed: %s", err),
		)
		return
	}
}
