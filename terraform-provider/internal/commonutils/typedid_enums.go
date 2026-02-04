package commonutils

// Modules
const (
	ModuleMonitoringDomain Module = "monitoringDomain"
	ModuleConnection       Module = "connection"
	ModuleMonitoringSess   Module = "monitoringSession"
	ModuleTunnelIn         Module = "tunnelIn"
	ModuleTunnelOut        Module = "tunnelOur"
)

// Cloud Platforms Types
const (
	TypeAnyCloud Type = "anyCloud"
	TypeESXi     Type = "vmwareEsxi"
)

// Tunnel Types
const (
	TypeVxLAN Type = "vxlan"
	TypeL2GRE Type = "l2gre"
)
