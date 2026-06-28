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

package commonutils

// Modules
const (
	ModuleMonitoringDomain     Module = "monitoringDomain"
	ModuleConnection           Module = "connection"
	ModuleMonitoringSess       Module = "monitoringSession"
	ModuleMap                  Module = "map"
	ModuleApp                  Module = "app"
	ModuleTunnelIn             Module = "tunnelIn"
	ModuleTunnelOut            Module = "tunnelOut"
	ModuleFabric               Module = "fabric"
	ModuleRawEndpoint          Module = "rawEndpoint"
	ModuleEndpointIfaceMapping Module = "endpointIfaceMapping"
)

// Cloud Platforms Types
const (
	// Internal FM APIs for Third Party Orchestration still refer anyCloud
	TypeThirdPartyOrchestration Type = "anyCloud"
	TypeVMWareESXi              Type = "vmwareEsxi"
)

// Map Types
const (
	TypeTrafficMap          Type = "trafficMap"
	TypeInclusionMap        Type = "inclusionMap"
	TypeExclusionMap        Type = "exclusionMap"
	TypeEsxiVMWareSelection Type = "esxiVmwareSelection"
)

// App Types
const (
	TypeDedup           Type = "dedup"
	TypeMasking         Type = "masking"
	TypeSlicing         Type = "slicing"
	TypeHeaderStripping Type = "headerStripping"
	TypeLoadBalancing   Type = "loadBalancing"
	TypeAmx             Type = "amx"
)

// Tunnel Types
const (
	TypeVxLAN     Type = "vxlan"
	TypeL2GRE     Type = "l2gre"
	TypeGeneve    Type = "geneve"
	TypeErspan    Type = "erspan"
	TypeTlsPcapng Type = "tlspcapng"
	TypeUdpTunnel Type = "udp"
)

// Raw Endpoint Types
const (
	TypeRawEndpoint Type = "raw"
)

const (
	TypeEndpointIfaceMapping Type = "mapping"
)
