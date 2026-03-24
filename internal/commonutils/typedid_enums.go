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
