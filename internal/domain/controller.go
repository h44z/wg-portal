package domain

// ControllerType defines the type of controller used to manage interfaces.

const (
	ControllerTypeMikrotik = "mikrotik"
	ControllerTypeLocal    = "wgctrl"
	ControllerTypePfsense  = "pfsense"
	ControllerTypeOpnsense = "opnsense"
)

// Controller extras can be used to store additional information available for specific controllers only.

type MikrotikInterfaceExtras struct {
	Id       string // internal mikrotik ID
	Comment  string
	Disabled bool
}

type MikrotikPeerExtras struct {
	Id              string // internal mikrotik ID
	Name            string
	Comment         string
	IsResponder     bool
	Disabled        bool
	Dynamic         bool
	ClientEndpoint  string
	ClientAddress   string
	ClientDns       string
	ClientKeepalive int
}

type LocalPeerExtras struct {
	Disabled bool
}

type PfsenseInterfaceExtras struct {
	Id       string // internal pfSense ID
	Comment  string
	Disabled bool
}

type PfsensePeerExtras struct {
	Id              string // internal pfSense ID
	Name            string
	Comment         string
	Disabled        bool
	ClientEndpoint  string
	ClientAddress   string
	ClientDns       string
	ClientKeepalive int
}

type OpnsenseInterfaceExtras struct {
	Uuid     string // internal OPNsense UUID of the WireGuard "server" (tunnel)
	Instance string // the wg instance number; OPNsense derives the device name (wg0) from it
	Comment  string
	Disabled bool
}

type OpnsensePeerExtras struct {
	Uuid            string // internal OPNsense UUID of the WireGuard "client" (peer)
	Name            string
	Comment         string
	Disabled        bool
	ClientEndpoint  string
	ClientAddress   string
	ClientDns       string
	ClientKeepalive int
}
