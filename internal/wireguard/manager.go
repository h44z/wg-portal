package wireguard

import (
	"io"

	"github.com/pkg/errors"

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

	ApplyDefaultConfigs(device persistence.InterfaceIdentifier) error
}

//
// -- Implementations
//

type PersistentManager struct {
	wgCtrlKeyGenerator
	*templateHandler
	*wgCtrlManager
}

func NewPersistentManager(wg lowlevel.WireGuardClient, nl lowlevel.NetlinkClient, store store) (*PersistentManager, error) {
	wgManager, err := newWgCtrlManager(wg, nl, store)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize WireGuard manager")
	}

	tplManager, err := newTemplateHandler()
	if err != nil {
		return nil, errors.WithMessage(err, "failed to initialize template manager")
	}

	m := &PersistentManager{
		wgCtrlKeyGenerator: wgCtrlKeyGenerator{},
		wgCtrlManager:      wgManager,
		templateHandler:    tplManager,
	}

	return m, nil
}

func (p *PersistentManager) ApplyDefaultConfigs(device persistence.InterfaceIdentifier) error {
	// TODO: implement
	return nil
}
