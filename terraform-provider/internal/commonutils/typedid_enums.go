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
)

// Cloud Platforms Types
const (
	TypeAnyCloud   Type = "anyCloud"
	TypeVMWareESXi Type = "vmwareEsxi"
)

// Map Types
const (
	TypeEsxiVMWareSelection Type = "esxiVmwareSelection"
)

// App Types
const (
	TypeDedup Type = "dedup"
)

// Tunnel Types
const (
	TypeVxLAN Type = "vxlan"
	TypeL2GRE Type = "l2gre"
)
