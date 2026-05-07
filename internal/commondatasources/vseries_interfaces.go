// Copyright (c) Gigamon, Inc.

// vSeries interface mapping data source.
//
// Exposes per-vSeries-node interface maps for all vSeries nodes under a
// given connection:
//
// nodes: {
//   <vseries_node_id> = {
//     name     = <node name>
//     mgmt_ip  = <management IP>
//     platform = <platform, e.g. vmwareEsxi / anyCloud>
//
//     interface_name_to_ipv4 = { <iface> -> [<IPv4>, ...] }
//     interface_name_to_ipv6 = { <iface> -> [<IPv6>, ...] }
//
//     ipv4_to_interface_name = { <IPv4> -> <iface name> }
//     ipv6_to_interface_name = { <IPv6> -> <iface name> }
//
//     interface_name_to_mac  = { <iface> -> <MAC> }
//     mac_to_interface_name  = { <MAC>   -> <iface name> }
//   }
// }
//
// Works across platforms (anyCloud, VMware ESXi, etc.) by selecting the
// appropriate FM REST endpoint based on the typed connection_id and normalizing the response.

package commondatasources

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ datasource.DataSource              = &VSeriesInterfaces{}
	_ datasource.DataSourceWithConfigure = &VSeriesInterfaces{}
)

// NewVSeriesInterfaces returns a new vSeries interfaces data source instance.
func NewVSeriesInterfaces() datasource.DataSource {
	return &VSeriesInterfaces{}
}

// VSeriesInterfaces builds IP ↔ interface-name maps for vSeries nodes.
type VSeriesInterfaces struct {
	fmClient *fmclient.FmClient
}

// vSeriesInterfacesModel is the Terraform model for this data source.
type vSeriesInterfacesModel struct {
	ConnID types.String `tfsdk:"connection_id"`
	// nodes: map[vseries_node_id] -> {
	//   name, mgmt_ip, platform,
	//   interface_name_to_ipv4, interface_name_to_ipv6,
	//   ipv4_to_interface_name, ipv6_to_interface_name,
	//   interface_name_to_mac, mac_to_interface_name
	// }
	Nodes types.Map `tfsdk:"nodes"`
}

// VSeriesInterface is a normalized view of an FM interface JSON entry.
type VSeriesInterface struct {
	Name        string `json:"name"`
	IPAddress   string `json:"ipAddress,omitempty"`
	IPv6Address string `json:"ipv6Address,omitempty"`
	Role        string `json:"role,omitempty"`
	MacAddress  string `json:"macAddress,omitempty"`
}

// VSeriesNode is a normalized view of an FM vSeries node.
type VSeriesNode struct {
	NodeID     string             `json:"nodeId"` // FM nodeId, used as key
	ConnID     string             `json:"connId"`
	Platform   string             `json:"platform"`
	MgmtIP     string             `json:"mgmtIp"`
	Name       string             `json:"name"`
	Interfaces []VSeriesInterface `json:"interfaces"`
}

// Metadata sets the data source type name.
func (d *VSeriesInterfaces) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vseries_interfaces"
}

// Schema defines the Terraform schema for the data source.
func (d *VSeriesInterfaces) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		MarkdownDescription: "Gigamon vSeries interface mappings for all vSeries nodes under a Fabric connection. " +
			"Builds per-node IP ↔ interface-name maps across platforms (anyCloud, VMware ESXi, etc).",

		Attributes: map[string]dsschema.Attribute{
			"connection_id": dsschema.StringAttribute{
				MarkdownDescription: "Fabric Manager connection ID associated with the vSeries nodes.",
				Required:            true,
			},

			"nodes": dsschema.MapNestedAttribute{
				MarkdownDescription: "Per vSeries node interface mappings, keyed by vSeries node ID.",
				Computed:            true,
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"name": dsschema.StringAttribute{
							MarkdownDescription: "vSeries node name.",
							Computed:            true,
						},
						"mgmt_ip": dsschema.StringAttribute{
							MarkdownDescription: "vSeries node management IP.",
							Computed:            true,
						},
						"platform": dsschema.StringAttribute{
							MarkdownDescription: "vSeries node platform (e.g. anyCloud, vmwareEsxi).",
							Computed:            true,
						},

						"interface_name_to_ipv4": dsschema.MapAttribute{
							MarkdownDescription: "Map from interface name to list of IPv4 addresses on that interface for this vSeries node.",
							Computed:            true,
							ElementType:         types.ListType{ElemType: types.StringType},
						},

						"interface_name_to_ipv6": dsschema.MapAttribute{
							MarkdownDescription: "Map from interface name to list of IPv6 addresses on that interface for this vSeries node.",
							Computed:            true,
							ElementType:         types.ListType{ElemType: types.StringType},
						},

						"ipv4_to_interface_name": dsschema.MapAttribute{
							MarkdownDescription: "Map from IPv4 address to interface name for this vSeries node.",
							Computed:            true,
							ElementType:         types.StringType,
						},

						"ipv6_to_interface_name": dsschema.MapAttribute{
							MarkdownDescription: "Map from IPv6 address to interface name for this vSeries node.",
							Computed:            true,
							ElementType:         types.StringType,
						},

						"interface_name_to_mac": dsschema.MapAttribute{
							MarkdownDescription: "Map from interface name to MAC address for this vSeries node.",
							Computed:            true,
							ElementType:         types.StringType,
						},

						"mac_to_interface_name": dsschema.MapAttribute{
							MarkdownDescription: "Map from MAC address to interface name for this vSeries node.",
							Computed:            true,
							ElementType:         types.StringType,
						},
					},
				},
			},
		},
	}
}

// Configure wires in the FM client.
func (d *VSeriesInterfaces) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	fmClient, ok := req.ProviderData.(*fmclient.FmClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *fmclient.FmClient, got: %T. Report the issue to Gigamon", req.ProviderData),
		)
		return
	}
	d.fmClient = fmClient
}

// Read fetches all vSeries nodes under the connection and builds per-node maps.
func (d *VSeriesInterfaces) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.fmClient == nil {
		resp.Diagnostics.AddError(
			"Unconfigured FM client",
			"The provider FM client was not configured. This is a bug in the provider implementation.",
		)
		return
	}

	var data vSeriesInterfacesModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	connID := data.ConnID.ValueString()
	if connID == "" {
		resp.Diagnostics.AddError("Missing connection_id", "connection_id cannot be empty")
		return
	}

	nodes, err := getVSeriesNodesForConn(ctx, connID, d.fmClient)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error retrieving vSeries nodes",
			fmt.Sprintf("Error calling FM for connection_id %q: %v", connID, err),
		)
		return
	}

	nodeAttrTypes := map[string]attr.Type{
		"name":                   types.StringType,
		"mgmt_ip":                types.StringType,
		"platform":               types.StringType,
		"interface_name_to_ipv4": types.MapType{ElemType: types.ListType{ElemType: types.StringType}},
		"interface_name_to_ipv6": types.MapType{ElemType: types.ListType{ElemType: types.StringType}},
		"ipv4_to_interface_name": types.MapType{ElemType: types.StringType},
		"ipv6_to_interface_name": types.MapType{ElemType: types.StringType},
		"interface_name_to_mac":  types.MapType{ElemType: types.StringType},
		"mac_to_interface_name":  types.MapType{ElemType: types.StringType},
	}

	if len(nodes) == 0 {
		emptyNodes, diag := types.MapValue(
			types.ObjectType{AttrTypes: nodeAttrTypes},
			map[string]attr.Value{},
		)
		resp.Diagnostics.Append(diag...)
		if resp.Diagnostics.HasError() {
			return
		}

		data.Nodes = emptyNodes
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Build map[nodeID] -> Object{
	//   name, mgmt_ip, platform,
	//   interface_name_to_ipv4, interface_name_to_ipv6,
	//   ipv4_to_interface_name, ipv6_to_interface_name,
	//   interface_name_to_mac, mac_to_interface_name
	// }

	nodeMap := make(map[string]attr.Value, len(nodes))

	for _, n := range nodes {
		ipv4ToIf, ifToIPv4, ipv6ToIf, ifToIPv6 := buildInterfaceMapsByFamily(n)

		// MAC maps: cover all interfaces that have a MAC, including IP-less DATA interfaces.
		macToIf := make(map[string]attr.Value)
		ifToMac := make(map[string]attr.Value)
		for _, intf := range n.Interfaces {
			if intf.MacAddress == "" {
				continue
			}
			macToIf[intf.MacAddress] = types.StringValue(intf.Name)
			ifToMac[intf.Name] = types.StringValue(intf.MacAddress)
		}

		ipv4ToIfVal, diag := types.MapValue(types.StringType, ipv4ToIf)
		resp.Diagnostics.Append(diag...)
		if resp.Diagnostics.HasError() {
			return
		}

		ipv6ToIfVal, diag := types.MapValue(types.StringType, ipv6ToIf)
		resp.Diagnostics.Append(diag...)
		if resp.Diagnostics.HasError() {
			return
		}

		ifToIPv4Val, diag := types.MapValue(types.ListType{ElemType: types.StringType}, ifToIPv4)
		resp.Diagnostics.Append(diag...)
		if resp.Diagnostics.HasError() {
			return
		}

		ifToIPv6Val, diag := types.MapValue(types.ListType{ElemType: types.StringType}, ifToIPv6)
		resp.Diagnostics.Append(diag...)
		if resp.Diagnostics.HasError() {
			return
		}

		macToIfVal, diag := types.MapValue(types.StringType, macToIf)
		resp.Diagnostics.Append(diag...)
		if resp.Diagnostics.HasError() {
			return
		}

		ifToMacVal, diag := types.MapValue(types.StringType, ifToMac)
		resp.Diagnostics.Append(diag...)
		if resp.Diagnostics.HasError() {
			return
		}

		nodeObj, diag := types.ObjectValue(
			nodeAttrTypes,
			map[string]attr.Value{
				"name":                   types.StringValue(n.Name),
				"mgmt_ip":                types.StringValue(n.MgmtIP),
				"platform":               types.StringValue(n.Platform),
				"interface_name_to_ipv4": ifToIPv4Val,
				"interface_name_to_ipv6": ifToIPv6Val,
				"ipv4_to_interface_name": ipv4ToIfVal,
				"ipv6_to_interface_name": ipv6ToIfVal,
				"interface_name_to_mac":  ifToMacVal,
				"mac_to_interface_name":  macToIfVal,
			},
		)
		resp.Diagnostics.Append(diag...)
		if resp.Diagnostics.HasError() {
			return
		}

		key := n.NodeID
		if key == "" {
			// Fallback if nodeId is not present; use mgmtIp as key
			key = n.MgmtIP
		}
		if key == "" {
			// As a last resort, use name
			key = n.Name
		}
		nodeMap[key] = nodeObj
	}

	nodesVal, diag := types.MapValue(
		types.ObjectType{AttrTypes: nodeAttrTypes},
		nodeMap,
	)
	resp.Diagnostics.Append(diag...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Nodes = nodesVal

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// buildInterfaceMapsByFamily constructs separate IPv4/IPv6 maps:
//
// - ipv4ToIf:  map[IPv4]ifaceName
// - ifToIPv4:  map[ifaceName][]IPv4
// - ipv6ToIf:  map[IPv6]ifaceName
// - ifToIPv6:  map[ifaceName][]IPv6
//
// Includes all interfaces that have at least one IPv4/IPv6 address.
func buildInterfaceMapsByFamily(
	node VSeriesNode,
) (
	map[string]attr.Value, // ipv4ToIf
	map[string]attr.Value, // ifToIPv4
	map[string]attr.Value, // ipv6ToIf
	map[string]attr.Value, // ifToIPv6
) {
	ipv4ToIf := make(map[string]attr.Value)
	ipv6ToIf := make(map[string]attr.Value)
	tmpIfToIPv4 := make(map[string][]string)
	tmpIfToIPv6 := make(map[string][]string)

	for _, intf := range node.Interfaces {
		if intf.IPAddress != "" {
			ipv4ToIf[intf.IPAddress] = types.StringValue(intf.Name)
			tmpIfToIPv4[intf.Name] = append(tmpIfToIPv4[intf.Name], intf.IPAddress)
		}
		if intf.IPv6Address != "" {
			ipv6ToIf[intf.IPv6Address] = types.StringValue(intf.Name)
			tmpIfToIPv6[intf.Name] = append(tmpIfToIPv6[intf.Name], intf.IPv6Address)
		}
	}

	ifToIPv4 := make(map[string]attr.Value, len(tmpIfToIPv4))
	for name, ips := range tmpIfToIPv4 {
		values := make([]attr.Value, len(ips))
		for i, ip := range ips {
			values[i] = types.StringValue(ip)
		}
		listVal, _ := types.ListValue(types.StringType, values)
		ifToIPv4[name] = listVal
	}

	ifToIPv6 := make(map[string]attr.Value, len(tmpIfToIPv6))
	for name, ips := range tmpIfToIPv6 {
		values := make([]attr.Value, len(ips))
		for i, ip := range ips {
			values[i] = types.StringValue(ip)
		}
		listVal, _ := types.ListValue(types.StringType, values)
		ifToIPv6[name] = listVal
	}

	return ipv4ToIf, ifToIPv4, ipv6ToIf, ifToIPv6
}

// -----------------------------------------------------------------------------
// FM client helpers for vSeries nodes
// -----------------------------------------------------------------------------

// fmVSeriesInterface mirrors the relevant FM JSON fields for an interface.
type fmVSeriesInterface struct {
	Name        string `json:"name"`
	IPAddress   string `json:"ipAddress,omitempty"`
	IPv6Address string `json:"ipv6Address,omitempty"`
	Role        string `json:"role,omitempty"`
	MacAddress  string `json:"macAddress,omitempty"`
}

// fmVSeriesNode mirrors the relevant FM JSON fields for a vSeries node.
type fmVSeriesNode struct {
	NodeID     string               `json:"nodeId"`
	ConnID     string               `json:"connId"`
	Platform   string               `json:"platform"`
	MgmtIP     string               `json:"mgmtIp"`
	Name       string               `json:"name"`
	Interfaces []fmVSeriesInterface `json:"interfaces"`
}

// fmVSeriesNodesResponse is the top-level FM response wrapper.
type fmVSeriesNodesResponse struct {
	VSeriesNodes []fmVSeriesNode `json:"vseriesNodes"`
}

// getVSeriesNodesForConn resolves the connection platform from the typed connID
// and calls the appropriate vSeriesNodes FM API.
//
// Typed connection ID format:
//
//	connection::<type>::<uuid>
//
// Supported types (commonutils.Type):
//   - anyCloud   -> api/v1.3/cloud/anyCloud/fabricNodes/vseriesNodes
//   - vmwareEsxi -> api/v1.3/cloud/vmware/fabricDeployment/vseriesNodes
func getVSeriesNodesForConn(
	ctx context.Context,
	connID string,
	fmClient *fmclient.FmClient,
) ([]VSeriesNode, error) {
	// connID is always typed inside the provider.
	platform, err := commonutils.TypeFromTypedID(connID)
	if err != nil {
		return nil, fmt.Errorf("invalid typed connection_id %q: %w", connID, err)
	}

	rawConnID, err := commonutils.UUIDFromTypedID(connID)
	if err != nil {
		return nil, fmt.Errorf("invalid typed connection_id %q: %w", connID, err)
	}

	var (
		path  string
		label string
	)

	switch platform {
	case commonutils.TypeThirdPartyOrchestration:
		// AnyCloud / 3PO style connections across AWS/Azure/GCP/Nutanix/etc.
		path = "api/v1.3/cloud/anyCloud/fabricNodes/vseriesNodes"
		label = "anyCloud"

	case commonutils.TypeVMWareESXi:
		// Native VMware ESXi fabricDeployment connections.
		path = "api/v1.3/cloud/vmware/fabricDeployment/vseriesNodes"
		label = "vmwareEsxi"

	default:
		// If we ever see a new platform type that this data source doesn’t support,
		// fail loud and early rather than guessing.
		return nil, fmt.Errorf("unsupported platform type %q for connection_id %q", platform, connID)
	}

	nodes, err := getVSeriesNodesFromPath(ctx, fmClient, path, rawConnID)
	if err != nil {
		return nil, fmt.Errorf("error calling %s vseriesNodes for connection_id %q: %w", label, connID, err)
	}

	return nodes, nil
}

// getVSeriesNodesFromPath calls a specific FM vseriesNodes endpoint and
// normalizes the response into []VSeriesNode.
func getVSeriesNodesFromPath(
	ctx context.Context,
	fmClient *fmclient.FmClient,
	path string,
	connID string,
) ([]VSeriesNode, error) {
	// Ensure we don't accidentally end up with double slashes in DoRequest.
	path = strings.TrimPrefix(path, "/")

	// Add query parameter directly to the path; DoRequest will add host + leading slash.
	pathWithQuery := fmt.Sprintf("%s?connId=%s", path, connID)

	respBody, err := fmClient.DoRequest(ctx, "GET", pathWithQuery, nil, nil, nil, "")
	if err != nil {
		return nil, err
	}

	var fmResp fmVSeriesNodesResponse
	if err := json.Unmarshal(respBody, &fmResp); err != nil {
		return nil, fmt.Errorf("failed to decode vseriesNodes response for connection_id %q: %w", connID, err)
	}

	if len(fmResp.VSeriesNodes) == 0 {
		return nil, nil
	}

	out := make([]VSeriesNode, len(fmResp.VSeriesNodes))
	for i, n := range fmResp.VSeriesNodes {
		out[i] = VSeriesNode{
			NodeID:   n.NodeID,
			ConnID:   n.ConnID,
			Platform: n.Platform,
			MgmtIP:   n.MgmtIP,
			Name:     n.Name,
		}
		if len(n.Interfaces) > 0 {
			intfs := make([]VSeriesInterface, len(n.Interfaces))
			for j, intf := range n.Interfaces {
				intfs[j] = VSeriesInterface{
					Name:        intf.Name,
					IPAddress:   intf.IPAddress,
					IPv6Address: intf.IPv6Address,
					Role:        intf.Role,
					MacAddress:  intf.MacAddress,
				}
			}
			out[i].Interfaces = intfs
		}
	}

	return out, nil
}
