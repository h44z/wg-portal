package core

import (
	"github.com/h44z/wg-portal/internal/lowlevel"
	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/h44z/wg-portal/internal/user"
	"github.com/h44z/wg-portal/internal/wireguard"
	"github.com/pkg/errors"
	"golang.zx2c4.com/wireguard/wgctrl"
)

// Backend combines the user manager and WireGuard manager. It also provides some additional functions.
type Backend interface {
	user.Manager
	wireguard.Manager

	ImportInterfaceById(identifier persistence.InterfaceIdentifier) error
	PrepareFreshPeer(identifier persistence.InterfaceIdentifier) (*persistence.PeerConfig, error)
	GetPeersForUser(identifier persistence.UserIdentifier) ([]*persistence.PeerConfig, error)
}

// type alias
type UserManager = user.Manager
type WireGuardManager = wireguard.Manager

type PersistentBackend struct {
	UserManager
	WireGuardManager
}

func NewPersistentBackend(db *persistence.Database) (*PersistentBackend, error) {
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

	b := &PersistentBackend{
		UserManager:      um,
		WireGuardManager: wgm,
	}

	return b, nil
}

// ImportInterfaceById imports an interface. The given interface identifier must be available as importable interface.
func (b *PersistentBackend) ImportInterfaceById(identifier persistence.InterfaceIdentifier) error {
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

// PrepareFreshPeer creates a new persistence.PeerConfig with prefilled keys and IP addresses.
func (b *PersistentBackend) PrepareFreshPeer(identifier persistence.InterfaceIdentifier) (*persistence.PeerConfig, error) {
	return nil, nil // TODO: implement
}

// GetPeersForUser returns all peers for the given user.
func (b *PersistentBackend) GetPeersForUser(identifier persistence.UserIdentifier) ([]*persistence.PeerConfig, error) {
	return nil, nil // TODO: implement
}
