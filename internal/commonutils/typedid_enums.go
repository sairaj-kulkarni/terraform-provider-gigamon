package commonutils

// Modules
const (
	ModuleMonitoringDomain Module = "monitoringDomain"
	ModuleConnection       Module = "connection"
	ModuleMonitoringSess   Module = "monitoringSession"
	ModuleMap              Module = "map"
	ModuleApp              Module = "app"
	ModuleTunnelIn         Module = "tunnelIn"
	ModuleTunnelOut        Module = "tunnelOut"
	ModuleFabric           Module = "fabric"
)

// Cloud Platforms Types
const (
	TypeAnyCloud   Type = "anyCloud"
	TypeVMWareESXi Type = "vmwareEsxi"
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
