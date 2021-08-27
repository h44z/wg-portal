package wireguard

type FileBackend struct {
	ConfigurationPath string
}

func (f FileBackend) SaveInterface(cfg InterfaceConfig, peers []PeerConfig) error {
	panic("implement me")
}

func (f FileBackend) SavePeer(peer PeerConfig, cfg InterfaceConfig) error {
	panic("implement me")
}

func (f FileBackend) DeleteInterface(cfg InterfaceConfig, peers []PeerConfig) error {
	panic("implement me")
}

func (f FileBackend) DeletePeer(peer PeerConfig, cfg InterfaceConfig) error {
	panic("implement me")
}

func (f FileBackend) Load(identifier DeviceIdentifier) (InterfaceConfig, []PeerConfig, error) {
	panic("implement me")
}

func (f FileBackend) LoadAll() (map[InterfaceConfig][]PeerConfig, error) {
	panic("implement me")
}
