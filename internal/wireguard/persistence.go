package wireguard

// ConfigStore provides an interface for interacting with different configuration storage repositories.
type ConfigStore interface {
	GetAvailableInterfaces() ([]DeviceIdentifier, error)
	GetAllInterfaces(interfaceIdentifiers ...DeviceIdentifier) (map[InterfaceConfig][]PeerConfig, error)
	GetInterface(identifier DeviceIdentifier) (InterfaceConfig, []PeerConfig, error)

	SaveInterface(cfg InterfaceConfig, peers []PeerConfig) error
	SavePeer(peer PeerConfig, interfaceIdentifier DeviceIdentifier) error

	DeleteInterface(identifier DeviceIdentifier) error
	DeletePeer(peer PeerIdentifier, interfaceIdentifier DeviceIdentifier) error
}
