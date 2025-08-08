package domain

// ControllerType defines the type of controller used to manage interfaces.

const (
	ControllerTypeMikrotik = "mikrotik"
	ControllerTypeLocal    = "wgctrl"
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
