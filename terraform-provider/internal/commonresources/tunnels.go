// Copyright (c) Gigamon, Inc.

// Implements the Tunnel resources that are common across all environments.

package commonresources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// NewTunnelOut creates a new OUT (egress) tunnel resource.
func NewTunnelOut() resource.Resource {
	return &tunnelResource{
		trafficDirection: "out",
	}
}

// NewTunnelIn creates a new IN (ingress) tunnel resource.
func NewTunnelIn() resource.Resource {
	return &tunnelResource{
		trafficDirection: "in",
	}
}

// tunnelResource manages a tunnel endpoint instance within a Monitoring Session.
// It also creates and deletes the underlying tunnelSpec in FM.
// The same implementation is used for both IN and OUT resources, parameterized
// by trafficDirection.
type tunnelResource struct {
	fmClient         *fmclient.FmClient // FM http client instance
	trafficDirection string             // "in" or "out"
}

// Ensure tunnelResource satisfies the framework interfaces.
var _ resource.Resource = &tunnelResource{}

// TunnelModel describes the Terraform model for the tunnel resource.
type TunnelModel struct {
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`
	Alias               types.String `tfsdk:"alias"`

	// MS-level tunnel instance ID (returned by MS update API).
	Id types.String `tfsdk:"id"`

	// Tunnel spec ID in /cloud/tunnelSpecs (created by this resource).
	TunnelSpecId types.String `tfsdk:"tunnel_spec_id"`

	// Internal flag: true if this resource created the tunnelSpec.
	OwnedTunnelSpec types.Bool `tfsdk:"owned_tunnel_spec"`

	// Type and direction
	Type             types.String `tfsdk:"type"`              // l2gre, vxlan, erspan, udpgre, udp, tlspcapng, geneve
	TrafficDirection types.String `tfsdk:"traffic_direction"` // in, out

	// Common fields
	Description  types.String `tfsdk:"description"`
	IpVersion    types.String `tfsdk:"ip_version"`     // IPV4, IPV6
	RemoteIP     types.String `tfsdk:"remote_ip"`      // peer IP (mainly for out)
	Mtu          types.Int32  `tfsdk:"mtu"`            // bytes
	Ttl          types.Int32  `tfsdk:"ttl"`            // hops
	Dscp         types.Int32  `tfsdk:"dscp"`           // 0–63
	Prec         types.Int32  `tfsdk:"prec"`           // 0–7
	FlowLabel    types.Int32  `tfsdk:"flow_label"`     // IPv6 flow label
	DataSubnetId types.String `tfsdk:"data_subnet_id"` // V Series data subnet id

	// Type-specific, non-TLS
	Key     types.Int32 `tfsdk:"key"`    // L2GRE/UDPGRE key
	Vni     types.Int32 `tfsdk:"vni"`    // VXLAN / Geneve NID
	SPort   types.Int32 `tfsdk:"s_port"` // VXLAN/UDPGRE/UDP/TLS source L4 port
	DPort   types.Int32 `tfsdk:"d_port"` // VXLAN/UDPGRE/UDP/TLS/Geneve dest L4 port
	FlowId  types.Int32 `tfsdk:"flow_id"`
	Multi   types.Bool  `tfsdk:"multi_tunnel"`
	NumTuns types.Int32 `tfsdk:"num_tunnels"`

	// TLS-PCAPNG (tlspcapng / TcpTunnel) specific
	TlsMtls          types.String `tfsdk:"tls_mtls"`           // "enable" / "disable"
	TlsKeyStoreAlias types.String `tfsdk:"tls_keystore_alias"` // keyStoreAlias
	TlsKeyAlias      types.String `tfsdk:"tls_key_alias"`      // keyAlias
	TlsCipher        types.String `tfsdk:"tls_cipher"`         // cipher
	TlsVersion       types.String `tfsdk:"tls_version"`        // tlsVersion
	TlsSAck          types.String `tfsdk:"tls_sack"`           // "enable" / "disable"
	TlsKeepAlive     types.Int32  `tfsdk:"tls_keepalive_timer"`
	TlsSynRetries    types.Int32  `tfsdk:"tls_syn_retries"`
	TlsDelayAck      types.String `tfsdk:"tls_delay_ack"` // "enable" / "disable"
	TlsFlowId        types.Int32  `tfsdk:"tls_flow_id"`   // optional TLS flow id (maps to flowId as well)
}

// FMTunnel is the FM representation of a tunnel/tunnelSpec.
// Only configuration fields are modeled here (no health/status fields).
type FMTunnel struct {
	Type             string `json:"type,omitempty"`
	Id               string `json:"id,omitempty"`
	Alias            string `json:"alias,omitempty"`
	Description      string `json:"description,omitempty"`
	TrafficDirection string `json:"trafficDirection,omitempty"`
	IpVersion        string `json:"ipVersion,omitempty"`
	AdminState       string `json:"adminState,omitempty"`

	RemoteIP     string `json:"remoteIP,omitempty"`
	Mtu          int32  `json:"mtu,omitempty"`
	Ttl          int32  `json:"ttl,omitempty"`
	Dscp         int32  `json:"dscp,omitempty"`
	Prec         int32  `json:"prec,omitempty"`
	FlowLabel    int32  `json:"flowLabel,omitempty"`
	DataSubnetId string `json:"dataSubnetId,omitempty"`

	// Type-specific (non-TLS)
	Key     int32 `json:"key,omitempty"`   // L2GRE/UDPGRE key
	Vni     int32 `json:"vni,omitempty"`   // VXLAN / Geneve VNI
	SPort   int32 `json:"sport,omitempty"` // UDP/UDPGRE/VXLAN/TLS source L4 port
	DPort   int32 `json:"dport,omitempty"` // UDP/UDPGRE/VXLAN/TLS/Geneve dest L4 port
	FlowId  int32 `json:"flowId,omitempty"`
	Multi   bool  `json:"multiTunnel,omitempty"`
	NumTuns int32 `json:"numTunnels,omitempty"`

	// TLS-PCAPNG (TcpTunnel) specific
	Mtls          string `json:"mtls,omitempty"`          // "enable"/"disable"
	KeyStoreAlias string `json:"keyStoreAlias,omitempty"` // keyStoreAlias
	KeyAlias      string `json:"keyAlias,omitempty"`      // keyAlias
	Cipher        string `json:"cipher,omitempty"`        // cipher
	TlsVersion    string `json:"tlsVersion,omitempty"`    // tlsVersion
	SAck          string `json:"sAck,omitempty"`          // "enable"/"disable"
	KeepAlive     int32  `json:"keepAliveTimer,omitempty"`
	SynRetries    int32  `json:"synRetries,omitempty"`
	DelayAck      string `json:"delayAck,omitempty"` // "enable"/"disable"
}

// Metadata sets the resource type name.
func (r *tunnelResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	suffix := "_tunnel_out"
	if r.trafficDirection == "in" {
		suffix = "_tunnel_in"
	}
	resp.TypeName = req.ProviderTypeName + suffix
}

// Schema defines the Terraform schema for the tunnel resource.
func (r *tunnelResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Gigamon Cloud Tunnel endpoint for a Monitoring Session. " +
			"This resource creates a tunnelSpec in FM and then attaches it to the specified Monitoring Session.",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Alias/name for this tunnel (also used as tunnelSpec alias).",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},

			"monitoring_session_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring Session ID on which to configure this tunnel.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"type": schema.StringAttribute{
				MarkdownDescription: "Tunnel type.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("l2gre", "vxlan", "erspan", "udpgre", "udp", "tlspcapng", "geneve"),
				},
			},

			// Computed-only and fixed per resource (in/out).
			"traffic_direction": schema.StringAttribute{
				MarkdownDescription: "Traffic direction for this tunnel endpoint (in or out).",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"description": schema.StringAttribute{
				MarkdownDescription: "Optional description for this tunnel.",
				Optional:            true,
			},

			"ip_version": schema.StringAttribute{
				MarkdownDescription: "IP version used for the tunnel outer header (IPV4 or IPV6).",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("IPV4"),
				Validators: []validator.String{
					stringvalidator.OneOf("IPV4", "IPV6"),
				},
			},

			"remote_ip": schema.StringAttribute{
				MarkdownDescription: "Tunnel remote IP address. For 'out' tunnels, this is typically required by FM.",
				Optional:            true,
			},

			"mtu": schema.Int32Attribute{
				MarkdownDescription: "Tunnel MTU in bytes (1280–9600). Only applicable if traffic direction is 'out'.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(1500),
			},

			"ttl": schema.Int32Attribute{
				MarkdownDescription: "Outer IP TTL (1–255). Only applicable if traffic direction is 'out'.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(64),
			},

			"dscp": schema.Int32Attribute{
				MarkdownDescription: "Outer IP DSCP value (0–63). Only applicable if traffic direction is 'out'.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
			},

			"prec": schema.Int32Attribute{
				MarkdownDescription: "IP precedence (0–7). Only applicable if traffic direction is 'out'.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
			},

			"flow_label": schema.Int32Attribute{
				MarkdownDescription: "IPv6 flow label (0–1048575). Only applicable if traffic direction is 'out'.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
			},

			"data_subnet_id": schema.StringAttribute{
				MarkdownDescription: "V Series Node data subnet ID used as tunnel interface (dataSubnetId).",
				Optional:            true,
			},

			"key": schema.Int32Attribute{
				MarkdownDescription: "L2GRE/UDPGRE key (1–4294967295). Only applicable for L2GRE and UDPGRE.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
			},

			"vni": schema.Int32Attribute{
				MarkdownDescription: "VXLAN/GENEVE Network Identifier (1–16777215). Only applicable for VXLAN and Geneve.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
			},

			"s_port": schema.Int32Attribute{
				MarkdownDescription: "Source L4 port (1–65535) for VXLAN, UDPGRE, UDP, and TLS-PCAPNG tunnels.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
			},

			"d_port": schema.Int32Attribute{
				MarkdownDescription: "Destination L4 port for VXLAN, UDPGRE, UDP, TLS-PCAPNG, and Geneve tunnels.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
			},

			"flow_id": schema.Int32Attribute{
				MarkdownDescription: "ERSPAN Flow ID (1–1023). Only applicable for ERSPAN tunnels.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
			},

			"multi_tunnel": schema.BoolAttribute{
				MarkdownDescription: "Enable VXLAN multi-tunnel. Only applicable for VXLAN tunnels.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},

			"num_tunnels": schema.Int32Attribute{
				MarkdownDescription: "Number of VXLAN tunnels to create when multi_tunnel is enabled (1–16). VXLAN only.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
			},

			// TLS-PCAPNG fields (optional)
			"tls_mtls": schema.StringAttribute{
				MarkdownDescription: "mTLS state for TLS-PCAPNG tunnels (enable/disable).",
				Optional:            true,
			},
			"tls_keystore_alias": schema.StringAttribute{
				MarkdownDescription: "Keystore alias for TLS-PCAPNG tunnels.",
				Optional:            true,
			},
			"tls_key_alias": schema.StringAttribute{
				MarkdownDescription: "Key alias for TLS-PCAPNG tunnels.",
				Optional:            true,
			},
			"tls_cipher": schema.StringAttribute{
				MarkdownDescription: "Cipher suite label for TLS-PCAPNG tunnels.",
				Optional:            true,
			},
			"tls_version": schema.StringAttribute{
				MarkdownDescription: "TLS version label for TLS-PCAPNG tunnels (e.g., TLS1.3).",
				Optional:            true,
			},
			"tls_sack": schema.StringAttribute{
				MarkdownDescription: "Selective ACK state for TLS-PCAPNG tunnels (enable/disable).",
				Optional:            true,
			},
			"tls_keepalive_timer": schema.Int32Attribute{
				MarkdownDescription: "Keep-alive timer for TLS-PCAPNG tunnels (seconds).",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
			},
			"tls_syn_retries": schema.Int32Attribute{
				MarkdownDescription: "SYN retries for TLS-PCAPNG tunnels (1–6).",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
			},
			"tls_delay_ack": schema.StringAttribute{
				MarkdownDescription: "Delay ACK state for TLS-PCAPNG tunnels (enable/disable).",
				Optional:            true,
			},
			"tls_flow_id": schema.Int32Attribute{
				MarkdownDescription: "TLS-PCAPNG flow id (1–1023).",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
			},

			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this tunnel instance within the Monitoring Session (used for links, deletion).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"tunnel_spec_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of the underlying tunnelSpec created in FM.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"owned_tunnel_spec": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Internal flag indicating whether this resource created the underlying tunnelSpec (true).",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Configure initializes the FM client.
func (r *tunnelResource) Configure(
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

// createFMSpecStruct converts TF model to an FM tunnelSpec struct.
func (r *tunnelResource) createFMSpecStruct(data *TunnelModel) *FMTunnel {
	fm := &FMTunnel{
		Alias:            data.Alias.ValueString(),
		Description:      data.Description.ValueString(),
		Type:             data.Type.ValueString(),
		TrafficDirection: data.TrafficDirection.ValueString(),
		IpVersion:        data.IpVersion.ValueString(),
		RemoteIP:         data.RemoteIP.ValueString(),
		Mtu:              data.Mtu.ValueInt32(),
		Ttl:              data.Ttl.ValueInt32(),
		Dscp:             data.Dscp.ValueInt32(),
		Prec:             data.Prec.ValueInt32(),
		FlowLabel:        data.FlowLabel.ValueInt32(),
		DataSubnetId:     data.DataSubnetId.ValueString(),
		AdminState:       "enabled",
	}

	t := data.Type.ValueString()

	switch t {
	case "l2gre":
		fm.Key = data.Key.ValueInt32()

	case "udpgre":
		fm.Key = data.Key.ValueInt32()
		fm.SPort = data.SPort.ValueInt32()
		fm.DPort = data.DPort.ValueInt32()

	case "vxlan":
		fm.Vni = data.Vni.ValueInt32()
		fm.SPort = data.SPort.ValueInt32()
		fm.DPort = data.DPort.ValueInt32()

		if !data.Multi.IsNull() && data.Multi.ValueBool() {
			fm.Multi = true
			if !data.NumTuns.IsNull() && data.NumTuns.ValueInt32() > 0 {
				fm.NumTuns = data.NumTuns.ValueInt32()
			}
		}

	case "erspan":
		fm.FlowId = data.FlowId.ValueInt32()

	case "udp":
		fm.SPort = data.SPort.ValueInt32()
		fm.DPort = data.DPort.ValueInt32()

	case "tlspcapng":
		fm.SPort = data.SPort.ValueInt32()
		fm.DPort = data.DPort.ValueInt32()
		fm.Mtls = data.TlsMtls.ValueString()
		fm.KeyStoreAlias = data.TlsKeyStoreAlias.ValueString()
		fm.KeyAlias = data.TlsKeyAlias.ValueString()
		fm.Cipher = data.TlsCipher.ValueString()
		fm.TlsVersion = data.TlsVersion.ValueString()
		fm.SAck = data.TlsSAck.ValueString()
		fm.KeepAlive = data.TlsKeepAlive.ValueInt32()
		fm.SynRetries = data.TlsSynRetries.ValueInt32()
		fm.DelayAck = data.TlsDelayAck.ValueString()

		if v := data.TlsFlowId.ValueInt32(); v != 0 {
			fm.FlowId = v
		}

	case "geneve":
		fm.DPort = data.DPort.ValueInt32()
		fm.Vni = data.Vni.ValueInt32()
	}

	return fm
}

// createFMTunnelInstanceStruct converts TF model + tunnelSpecId to FM tunnel instance struct.
func (r *tunnelResource) createFMTunnelInstanceStruct(data *TunnelModel) *FMTunnel {
	fm := &FMTunnel{
		Id:               data.TunnelSpecId.ValueString(), // will be overridden to instance Id in Update
		Alias:            data.Alias.ValueString(),
		Description:      data.Description.ValueString(),
		Type:             data.Type.ValueString(),
		TrafficDirection: data.TrafficDirection.ValueString(),
		IpVersion:        data.IpVersion.ValueString(),
		RemoteIP:         data.RemoteIP.ValueString(),
		Mtu:              data.Mtu.ValueInt32(),
		Ttl:              data.Ttl.ValueInt32(),
		Dscp:             data.Dscp.ValueInt32(),
		Prec:             data.Prec.ValueInt32(),
		FlowLabel:        data.FlowLabel.ValueInt32(),
		DataSubnetId:     data.DataSubnetId.ValueString(),
		AdminState:       "enabled",
	}

	t := data.Type.ValueString()

	switch t {
	case "l2gre":
		fm.Key = data.Key.ValueInt32()

	case "udpgre":
		fm.Key = data.Key.ValueInt32()
		fm.SPort = data.SPort.ValueInt32()
		fm.DPort = data.DPort.ValueInt32()

	case "vxlan":
		fm.Vni = data.Vni.ValueInt32()
		fm.SPort = data.SPort.ValueInt32()
		fm.DPort = data.DPort.ValueInt32()
		if !data.Multi.IsNull() && data.Multi.ValueBool() {
			fm.Multi = true
			if !data.NumTuns.IsNull() && data.NumTuns.ValueInt32() > 0 {
				fm.NumTuns = data.NumTuns.ValueInt32()
			}
		}

	case "erspan":
		fm.FlowId = data.FlowId.ValueInt32()

	case "udp":
		fm.SPort = data.SPort.ValueInt32()
		fm.DPort = data.DPort.ValueInt32()

	case "tlspcapng":
		fm.SPort = data.SPort.ValueInt32()
		fm.DPort = data.DPort.ValueInt32()
		fm.Mtls = data.TlsMtls.ValueString()
		fm.KeyStoreAlias = data.TlsKeyStoreAlias.ValueString()
		fm.KeyAlias = data.TlsKeyAlias.ValueString()
		fm.Cipher = data.TlsCipher.ValueString()
		fm.TlsVersion = data.TlsVersion.ValueString()
		fm.SAck = data.TlsSAck.ValueString()
		fm.KeepAlive = data.TlsKeepAlive.ValueInt32()
		fm.SynRetries = data.TlsSynRetries.ValueInt32()
		fm.DelayAck = data.TlsDelayAck.ValueString()
		if v := data.TlsFlowId.ValueInt32(); v != 0 {
			fm.FlowId = v
		}

	case "geneve":
		fm.DPort = data.DPort.ValueInt32()
		fm.Vni = data.Vni.ValueInt32()
	}

	return fm
}

// updateTFStruct copies FM tunnel data into the TF state model.
func (r *tunnelResource) updateTFStruct(data *TunnelModel, fmData *FMTunnel) {
	data.Alias = types.StringValue(fmData.Alias)
	data.Description = types.StringValue(fmData.Description)
	data.Type = types.StringValue(fmData.Type)
	data.TrafficDirection = types.StringValue(fmData.TrafficDirection)
	data.IpVersion = types.StringValue(fmData.IpVersion)
	data.RemoteIP = types.StringValue(fmData.RemoteIP)
	data.Mtu = types.Int32Value(fmData.Mtu)
	data.Ttl = types.Int32Value(fmData.Ttl)
	data.Dscp = types.Int32Value(fmData.Dscp)
	data.Prec = types.Int32Value(fmData.Prec)
	data.FlowLabel = types.Int32Value(fmData.FlowLabel)
	data.DataSubnetId = types.StringValue(fmData.DataSubnetId)

	data.Key = types.Int32Value(fmData.Key)
	data.Vni = types.Int32Value(fmData.Vni)
	data.SPort = types.Int32Value(fmData.SPort)
	data.DPort = types.Int32Value(fmData.DPort)
	data.FlowId = types.Int32Value(fmData.FlowId)
	data.Multi = types.BoolValue(fmData.Multi)
	data.NumTuns = types.Int32Value(fmData.NumTuns)

	data.TlsMtls = types.StringValue(fmData.Mtls)
	data.TlsKeyStoreAlias = types.StringValue(fmData.KeyStoreAlias)
	data.TlsKeyAlias = types.StringValue(fmData.KeyAlias)
	data.TlsCipher = types.StringValue(fmData.Cipher)
	data.TlsVersion = types.StringValue(fmData.TlsVersion)
	data.TlsSAck = types.StringValue(fmData.SAck)
	data.TlsKeepAlive = types.Int32Value(fmData.KeepAlive)
	data.TlsSynRetries = types.Int32Value(fmData.SynRetries)
	data.TlsDelayAck = types.StringValue(fmData.DelayAck)
	data.TlsFlowId = types.Int32Value(fmData.FlowId)

	if fmData.Id != "" {
		data.Id = types.StringValue(fmData.Id)
	}
}

// createTunnelSpec creates a tunnelSpec in FM and returns its ID.
func (r *tunnelResource) createTunnelSpec(ctx context.Context, data *TunnelModel) (string, error) {
	fmSpec := r.createFMSpecStruct(data)

	jsonData, err := json.Marshal(fmSpec)
	if err != nil {
		return "", fmt.Errorf("unable to convert tunnelSpec struct to JSON: %w", err)
	}

	respData, err := r.fmClient.DoRequest(
		ctx,
		"POST",
		"api/v1.3/cloud/tunnelSpecs",
		nil,
		nil,
		bytes.NewBuffer(jsonData),
		"application/json",
	)
	if err != nil {
		return "", err
	}

	var fmResp FMTunnel
	if err := json.Unmarshal(respData, &fmResp); err != nil {
		return "", fmt.Errorf("unable to parse tunnelSpec create response: %s error: %w", string(respData), err)
	}

	if fmResp.Id == "" {
		return "", fmt.Errorf("tunnelSpec create response did not contain an id: %s", string(respData))
	}

	return fmResp.Id, nil
}

// deleteTunnelSpec deletes a tunnelSpec in FM.
func (r *tunnelResource) deleteTunnelSpec(ctx context.Context, specId string) error {
	if specId == "" {
		return nil
	}

	_, err := r.fmClient.DoRequest(
		ctx,
		"DELETE",
		fmt.Sprintf("api/v1.3/cloud/tunnelSpecs/%s", specId),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf("unable to delete tunnelSpec %s: %w", specId, err)
	}
	return nil
}

// getTunnelSpecByAlias fetches an existing tunnelSpec by alias by listing all
// tunnelSpecs and filtering client-side.
func (r *tunnelResource) getTunnelSpecByAlias(ctx context.Context, alias string) (*FMTunnel, error) {
	if alias == "" {
		return nil, fmt.Errorf("empty alias provided to getTunnelSpecByAlias")
	}

	respData, err := r.fmClient.DoRequest(
		ctx,
		"GET",
		"api/v1.3/cloud/tunnelSpecs",
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, err
	}

	var list struct {
		TunnelSpecs []FMTunnel `json:"tunnelSpecs"`
	}
	if err := json.Unmarshal(respData, &list); err != nil {
		return nil, fmt.Errorf("unable to parse tunnelSpecs list response: %s error: %w", string(respData), err)
	}

	for _, ts := range list.TunnelSpecs {
		if ts.Alias == alias {
			return &ts, nil
		}
	}

	return nil, fmt.Errorf("tunnelSpec with alias %q not found", alias)
}

// specsEqual compares the relevant configuration fields of two tunnelSpecs.
func specsEqual(a, b *FMTunnel) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Type == b.Type &&
		a.TrafficDirection == b.TrafficDirection &&
		a.IpVersion == b.IpVersion &&
		a.Alias == b.Alias &&
		a.Description == b.Description &&
		a.RemoteIP == b.RemoteIP &&
		a.Mtu == b.Mtu &&
		a.Ttl == b.Ttl &&
		a.Dscp == b.Dscp &&
		a.Prec == b.Prec &&
		a.FlowLabel == b.FlowLabel &&
		a.DataSubnetId == b.DataSubnetId &&
		a.Key == b.Key &&
		a.Vni == b.Vni &&
		a.SPort == b.SPort &&
		a.DPort == b.DPort &&
		a.FlowId == b.FlowId &&
		a.Multi == b.Multi &&
		a.NumTuns == b.NumTuns &&
		a.Mtls == b.Mtls &&
		a.KeyStoreAlias == b.KeyStoreAlias &&
		a.KeyAlias == b.KeyAlias &&
		a.Cipher == b.Cipher &&
		a.TlsVersion == b.TlsVersion &&
		a.SAck == b.SAck &&
		a.KeepAlive == b.KeepAlive &&
		a.SynRetries == b.SynRetries &&
		a.DelayAck == b.DelayAck
}

// ---------- New helper functions for config equality ----------

func equalString(a, b types.String) bool {
	if a.IsUnknown() || b.IsUnknown() {
		return true
	}
	if a.IsNull() && b.IsNull() {
		return true
	}
	if a.IsNull() != b.IsNull() {
		return false
	}
	return a.ValueString() == b.ValueString()
}

func equalInt32(a, b types.Int32) bool {
	if a.IsUnknown() || b.IsUnknown() {
		return true
	}
	if a.IsNull() && b.IsNull() {
		return true
	}
	if a.IsNull() != b.IsNull() {
		return false
	}
	return a.ValueInt32() == b.ValueInt32()
}

func equalBool(a, b types.Bool) bool {
	if a.IsUnknown() || b.IsUnknown() {
		return true
	}
	if a.IsNull() && b.IsNull() {
		return true
	}
	if a.IsNull() != b.IsNull() {
		return false
	}
	return a.ValueBool() == b.ValueBool()
}

// tunnelConfigEqual checks whether all user-configurable fields are identical
// between plan and state. It intentionally ignores computed-only fields like
// id, tunnel_spec_id, owned_tunnel_spec, and traffic_direction.
func tunnelConfigEqual(plan, state *TunnelModel) bool {
	// Alias (user-set)
	if !equalString(plan.Alias, state.Alias) {
		return false
	}

	// Required, user-set
	if !equalString(plan.Type, state.Type) {
		return false
	}
	if !equalString(plan.Description, state.Description) {
		return false
	}
	if !equalString(plan.IpVersion, state.IpVersion) {
		return false
	}
	if !equalString(plan.RemoteIP, state.RemoteIP) {
		return false
	}

	// Common numeric fields
	if !equalInt32(plan.Mtu, state.Mtu) ||
		!equalInt32(plan.Ttl, state.Ttl) ||
		!equalInt32(plan.Dscp, state.Dscp) ||
		!equalInt32(plan.Prec, state.Prec) ||
		!equalInt32(plan.FlowLabel, state.FlowLabel) {
		return false
	}

	if !equalString(plan.DataSubnetId, state.DataSubnetId) {
		return false
	}

	// Type-specific non-TLS fields
	if !equalInt32(plan.Key, state.Key) ||
		!equalInt32(plan.Vni, state.Vni) ||
		!equalInt32(plan.SPort, state.SPort) ||
		!equalInt32(plan.DPort, state.DPort) ||
		!equalInt32(plan.FlowId, state.FlowId) ||
		!equalBool(plan.Multi, state.Multi) ||
		!equalInt32(plan.NumTuns, state.NumTuns) {
		return false
	}

	// TLS-PCAPNG fields
	if !equalString(plan.TlsMtls, state.TlsMtls) ||
		!equalString(plan.TlsKeyStoreAlias, state.TlsKeyStoreAlias) ||
		!equalString(plan.TlsKeyAlias, state.TlsKeyAlias) ||
		!equalString(plan.TlsCipher, state.TlsCipher) ||
		!equalString(plan.TlsVersion, state.TlsVersion) ||
		!equalString(plan.TlsSAck, state.TlsSAck) ||
		!equalInt32(plan.TlsKeepAlive, state.TlsKeepAlive) ||
		!equalInt32(plan.TlsSynRetries, state.TlsSynRetries) ||
		!equalString(plan.TlsDelayAck, state.TlsDelayAck) ||
		!equalInt32(plan.TlsFlowId, state.TlsFlowId) {
		return false
	}

	return true
}

// ---------- Error parsers ----------

// parseAliasAlreadyExistsError detects "Tunnel [name] already exists" errors and extracts the name.
func parseAliasAlreadyExistsError(err error) (alias string, ok bool) {
	if err == nil {
		return "", false
	}
	msg := err.Error()
	if !strings.Contains(msg, "Tunnel [") || !strings.Contains(msg, "already exists") {
		return "", false
	}
	start := strings.Index(msg, "Tunnel [")
	if start == -1 {
		return "", true
	}
	start += len("Tunnel [")
	end := strings.Index(msg[start:], "]")
	if end == -1 {
		return "", true
	}
	return msg[start : start+end], true
}

// parseSameConfigExistsError detects "Tunnel with same configuration already exists [name]" errors.
func parseSameConfigExistsError(err error) (existingAlias string, ok bool) {
	if err == nil {
		return "", false
	}
	msg := err.Error()
	if !strings.Contains(msg, "Tunnel with same configuration already exists") {
		return "", false
	}
	start := strings.Index(msg, "Tunnel with same configuration already exists [")
	if start == -1 {
		return "", true
	}
	start += len("Tunnel with same configuration already exists [")
	end := strings.Index(msg[start:], "]")
	if end == -1 {
		return "", true
	}
	return msg[start : start+end], true
}

// ensureTunnelSpec creates or reuses a tunnelSpec in FM and returns (specId, owned, error).
func (r *tunnelResource) ensureTunnelSpec(ctx context.Context, data *TunnelModel) (string, bool, error) {
	specId, err := r.createTunnelSpec(ctx, data)
	if err == nil {
		// Created successfully; we own this spec.
		return specId, true, nil
	}

	// Different name, same configuration case.
	if existingAlias, ok := parseSameConfigExistsError(err); ok {
		if existingAlias != "" {
			return "", false, fmt.Errorf(
				"tunnelSpec with the same configuration already exists in FM as %q. "+
					"Either change the tunnel configuration in Terraform, or set alias = %q "+
					"so that existing tunnelSpec is used.",
				existingAlias, existingAlias,
			)
		}
		return "", false, fmt.Errorf(
			"tunnelSpec with the same configuration already exists in FM. " +
				"Either change the tunnel configuration in Terraform, or set alias to the " +
				"existing tunnel's name so that tunnelSpec is used.",
		)
	}

	// Same name (alias) exists; config may or may not match.
	if aliasFromErr, ok := parseAliasAlreadyExistsError(err); ok {
		desired := r.createFMSpecStruct(data)

		existing, getErr := r.getTunnelSpecByAlias(ctx, desired.Alias)
		if getErr != nil {
			return "", false, fmt.Errorf(
				"tunnelSpec alias %q already exists in FM but could not be read: %w",
				desired.Alias, getErr,
			)
		}

		if specsEqual(desired, existing) {
			// Same name + same configuration: reuse existing tunnelSpec.
			return existing.Id, false, nil
		}

		// Same name + different configuration: user must change alias/import.
		return "", false, fmt.Errorf(
			"tunnelSpec alias %q already exists in FM with a different configuration. "+
				"Terraform cannot change or reuse it safely. Change the alias in your Terraform "+
				"configuration, or import the existing tunnel before managing it via Terraform.",
			aliasFromErr,
		)
	}

	// All other errors: propagate as generic failure.
	return "", false, fmt.Errorf("unable to create tunnelSpec: %w", err)
}

// Create configures a new tunnelSpec and attaches it to the monitoring session.
func (r *tunnelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TunnelModel

	// Read Terraform plan data into the model.
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Force the direction for this resource.
	data.TrafficDirection = types.StringValue(r.trafficDirection)

	// Create or reuse tunnelSpec in FM.
	specId, owned, err := r.ensureTunnelSpec(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create tunnelSpec",
			fmt.Sprintf("tunnelSpec creation failed: %s", err),
		)
		return
	}
	data.TunnelSpecId = types.StringValue(specId)
	data.OwnedTunnelSpec = types.BoolValue(owned)

	// Attach tunnel to Monitoring Session using MS update API.
	fmTunnel := r.createFMTunnelInstanceStruct(&data)

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
		// Best-effort cleanup of tunnelSpec if we own it.
		if owned {
			_ = r.deleteTunnelSpec(ctx, specId)
		}
		return
	}

	data.Id = types.StringValue(id)

	// Deploy the MS if it is not already deployed.
	err = deployIfNeeded(ctx, r.fmClient, data.MonitoringSessionId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to deploy Monitoring Session after tunnel creation",
			fmt.Sprintf("unable to deploy Monitoring Session. error is %s", err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// GetMSTunnelData fetches the tunnel instance details from the Monitoring Session.
func GetMSTunnelData(
	ctx context.Context,
	monitoringSessId, tunnelId, tunnelAlias string,
	tunnelData *FMTunnel,
	fmClient *fmclient.FmClient,
) (bool, error) {

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
		if (tunnelId == "" || tunnelId == tnl.Id) &&
			(tunnelAlias == "" || tunnelAlias == tnl.Alias) {
			*tunnelData = tnl
			return true, nil
		}
	}

	return false, nil
}

// Read refreshes the tunnel state from FM.
func (r *tunnelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TunnelModel

	// Read Terraform prior state data into the model.
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
		resp.Diagnostics.AddError(
			"Unable to get tunnel details from Monitoring Session",
			fmt.Sprintf("unable to get tunnel details. error is %s", err),
		)
		return
	}
	if !ok {
		// Tunnel no longer exists in FM; remove from state.
		resp.State.RemoveResource(ctx)
		return
	}

	r.updateTFStruct(&data, &tunnelData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update updates the existing tunnel instance in place with the new configuration.
// Replacement is handled by RequiresReplace plan modifiers on schema attributes.
func (r *tunnelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan TunnelModel
	var state TunnelModel

	// Read desired configuration (plan) and current state.
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If no user-visible configuration has changed, do nothing.
	if tunnelConfigEqual(&plan, &state) {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	// Force the direction for this resource.
	plan.TrafficDirection = types.StringValue(r.trafficDirection)

	// Build an FM tunnel instance from the plan, but use the existing instance ID.
	fmTunnel := r.createFMTunnelInstanceStruct(&plan)
	// Id must be the MS tunnel instance ID, not the tunnelSpec ID, for in-place update.
	fmTunnel.Id = state.Id.ValueString()

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "tunnel",
				Operation:  "update",
				Tunnel:     fmTunnel,
			},
		},
	}

	_, err := commonutils.UpdateMonSess(
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

	// Keep the same instance ID and tunnelSpec info; we updated in place.
	plan.Id = state.Id
	plan.TunnelSpecId = state.TunnelSpecId
	plan.OwnedTunnelSpec = state.OwnedTunnelSpec

	// Deploy the MS if needed.
	err = deployIfNeeded(ctx, r.fmClient, plan.MonitoringSessionId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to deploy Monitoring Session after tunnel update",
			fmt.Sprintf("unable to deploy Monitoring Session. error is %s", err),
		)
		return
	}

	// Save new state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes the tunnel instance from the Monitoring Session and deletes
// its tunnelSpec only if this resource owns it.
func (r *tunnelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TunnelModel

	// Read Terraform prior state data into the model.
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the tunnel instance from the Monitoring Session.
	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "tunnel",
				Operation:  "delete",
				Tunnel: FMTunnel{
					Id:   data.Id.ValueString(),
					Type: data.Type.ValueString(),
				},
			},
		},
	}

	_, err := commonutils.UpdateMonSess(
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
		// Even if instance delete fails, do not attempt spec delete.
		return
	}

	// Best-effort delete of the tunnelSpec we created, but only if we own it.
	specId := data.TunnelSpecId.ValueString()
	owned := true
	if !data.OwnedTunnelSpec.IsNull() && !data.OwnedTunnelSpec.IsUnknown() {
		owned = data.OwnedTunnelSpec.ValueBool()
	}

	if owned && specId != "" {
		_ = r.deleteTunnelSpec(ctx, specId)
	}
}
