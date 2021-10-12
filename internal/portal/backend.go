package portal

import (
	"github.com/h44z/wg-portal/internal/lowlevel"
	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/h44z/wg-portal/internal/user"
	"github.com/h44z/wg-portal/internal/wireguard"
	"github.com/pkg/errors"
	"golang.zx2c4.com/wireguard/wgctrl"
)

// type alias
type UserManager = user.Manager
type WireGuardManager = wireguard.Manager

type Backend struct {
	UserManager
	WireGuardManager
}

func NewBackend(db *persistence.Database) (*Backend, error) {
	wg, err := wgctrl.New()
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get wgctrl handle")
	}

	nl := &lowlevel.NetlinkManager{}

	wgm, err := wireguard.NewPersistentManager(wg, nl, db)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to setup WireGuard manager")
	}

	um, err := user.NewPersistentManager(db)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to setup user manager")
	}

	b := &Backend{
		UserManager:      um,
		WireGuardManager: wgm,
	}

	return b, nil
}

func (b *Backend) ImportInterface(identifier persistence.InterfaceIdentifier) error {
	importable, err := b.GetImportableInterfaces()
	if err != nil {
		return errors.WithMessage(err, "failed to get importable interfaces")
	}

	var interfaceConfig *wireguard.ImportableInterface
	var peers []*persistence.PeerConfig
	for cfg, peerList := range importable {
		if cfg.Identifier == identifier {
			interfaceConfig = cfg
			peers = peerList
			break
		}
	}

	if interfaceConfig == nil {
		return errors.New("the given interface is not importable")
	}

	err = b.WireGuardManager.ImportInterface(interfaceConfig, peers)
	if err != nil {
		return errors.WithMessagef(err, "failed to import interface")
	}

	return nil
}
