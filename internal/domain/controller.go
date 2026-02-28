package domain

// ControllerType defines the type of controller used to manage interfaces.

const (
	ControllerTypeMikrotik = "mikrotik"
	ControllerTypeLocal    = "wgctrl"
	ControllerTypePfsense  = "pfsense"
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
