package wireguard

import (
	"io"

	"github.com/h44z/wg-portal/internal/lowlevel"

	"github.com/h44z/wg-portal/internal/persistence"
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
	ImportInterface(cfg ImportableInterface, peers []persistence.PeerConfig) error
}

type ConfigFileGenerator interface {
	GetInterfaceConfig(cfg persistence.InterfaceConfig, peers []persistence.PeerConfig) (io.Reader, error)
	GetPeerConfig(peer persistence.PeerConfig) (io.Reader, error)
}

type PeerManager interface {
	GetPeers(device persistence.InterfaceIdentifier) ([]persistence.PeerConfig, error)
	SavePeers(peers ...persistence.PeerConfig) error
	RemovePeer(peer persistence.PeerIdentifier) error
}

type Manager interface {
	KeyGenerator
	InterfaceManager
	PeerManager
	ImportManager
	ConfigFileGenerator
}

//
// -- Implementations
//

type PersistentManager struct {
	WgCtrlKeyGenerator
	TemplateHandler
	WgCtrlManager
}

func NewPersistentManager(wg lowlevel.WireGuardClient, nl lowlevel.NetlinkClient, store store) (*PersistentManager, error) {
	m := &PersistentManager{}

	return m, nil
}
