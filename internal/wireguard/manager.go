package wireguard

import (
	"io"
	"sync"

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

//
// -- Implementations
//

type PersistentManager struct {
	WgCtrlKeyGenerator
	TemplateHandler

	mux sync.RWMutex // mutex to synchronize access to maps

	// external api clients
	wg lowlevel.WireGuardClient
	nl lowlevel.NetlinkClient

	// persistent backend
	store store

	// internal holder of interface configurations
	interfaces map[persistence.InterfaceIdentifier]persistence.InterfaceConfig
	// internal holder of peer configurations
	peers map[persistence.InterfaceIdentifier]map[persistence.PeerIdentifier]persistence.PeerConfig
}

func NewPersistentManager(wg lowlevel.WireGuardClient, nl lowlevel.NetlinkClient, store store) (*PersistentManager, error) {
	m := &PersistentManager{
		mux: sync.RWMutex{},
		wg:  wg,
		nl:  nl,
	}

	return m, nil
}
