package wireguard

import (
	"io"

	"github.com/h44z/wg-portal/tmp/persistence"
)

type KeyGenerator interface {
	GetFreshKeypair() (persistence.KeyPair, error)
	GetPreSharedKey() (persistence.PreSharedKey, error)
}

// InterfaceManager provides methods to create/update/delete physical WireGuard devices.
type InterfaceManager interface {
	GetInterfaces() ([]persistence.InterfaceConfig, error)
	CreateInterface(id persistence.InterfaceIdentifier) error
	DeleteInterface(id persistence.InterfaceIdentifier) error
	UpdateInterface(id persistence.InterfaceIdentifier, cfg persistence.InterfaceConfig) error
}

type ImportableInterface struct {
	persistence.InterfaceConfig
	ImportLocation string
	ImportType     string
}

type ImportManager interface {
	GetImportableInterfaces() (map[ImportableInterface][]persistence.PeerConfig, error)
	ImportInterface(cfg ImportableInterface, peers []persistence.PeerConfig)
}

type ConfigFileGenerator interface {
	GetInterfaceConfig(cfg persistence.InterfaceConfig, peers []persistence.PeerConfig) (io.Reader, error)
	GetPeerConfig(peer persistence.PeerConfig) (io.Reader, error)
}

type PeerManager interface {
	GetPeers(device persistence.InterfaceIdentifier) ([]persistence.PeerConfig, error)
	SavePeers(device persistence.InterfaceIdentifier, peers ...persistence.PeerConfig) error
	RemovePeer(device persistence.InterfaceIdentifier, peer persistence.PeerIdentifier) error
}

type Manager interface {
	KeyGenerator
	InterfaceManager
	PeerManager
	ImportManager
	ConfigFileGenerator
}
