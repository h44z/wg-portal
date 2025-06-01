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
	Name           string
	Comment        string
	IsResponder    bool
	ClientEndpoint string
	ClientAddress  string
	Disabled       bool
}
