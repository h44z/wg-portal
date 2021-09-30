package persistence

type WireGuard interface {
	GetAvailableInterfaces() ([]InterfaceIdentifier, error)

	GetAllInterfaces(interfaceIdentifiers ...InterfaceIdentifier) (map[InterfaceConfig][]PeerConfig, error)
	GetInterface(identifier InterfaceIdentifier) (InterfaceConfig, []PeerConfig, error)

	SaveInterface(cfg InterfaceConfig, peers []PeerConfig) error
	SavePeer(peer PeerConfig, interfaceIdentifier InterfaceIdentifier) error

	DeleteInterface(identifier InterfaceIdentifier) error
	DeletePeer(peer PeerIdentifier, interfaceIdentifier InterfaceIdentifier) error
}
