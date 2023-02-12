package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/h44z/wg-portal/internal"
	"github.com/sirupsen/logrus"

	evbus "github.com/vardius/message-bus"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// region local-dependencies

type wireGuardDatabaseRepo interface {
	GetInterface(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, error)
	GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error)
	GetAllInterfaces(ctx context.Context) ([]domain.Interface, error)
	FindInterfaces(ctx context.Context, search string) ([]domain.Interface, error)
	GetInterfaceIps(ctx context.Context) (map[domain.InterfaceIdentifier][]domain.Cidr, error)
	SaveInterface(ctx context.Context, id domain.InterfaceIdentifier, updateFunc func(in *domain.Interface) (*domain.Interface, error)) error
	DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error
	GetInterfacePeers(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.Peer, error)
	FindInterfacePeers(ctx context.Context, id domain.InterfaceIdentifier, search string) ([]domain.Peer, error)
	GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error)
	FindUserPeers(ctx context.Context, id domain.UserIdentifier, search string) ([]domain.Peer, error)
	SavePeer(ctx context.Context, id domain.PeerIdentifier, updateFunc func(in *domain.Peer) (*domain.Peer, error)) error
	DeletePeer(ctx context.Context, id domain.PeerIdentifier) error
}

// endregion local-dependencies

type wireGuardManager struct {
	cfg *config.Config
	bus evbus.MessageBus

	db wireGuardDatabaseRepo
	wg wireGuardRepo
}

func newWireGuardManager(cfg *config.Config, bus evbus.MessageBus, wgRepo wireGuardRepo, db wireGuardDatabaseRepo) (*wireGuardManager, error) {
	m := &wireGuardManager{
		cfg: cfg,
		bus: bus,
		wg:  wgRepo,
		db:  db,
	}

	m.connectToMessageBus()

	return m, nil
}

func (m wireGuardManager) connectToMessageBus() {
	_ = m.bus.Subscribe(TopicUserCreated, m.handleUserCreationEvent)
}

func (m wireGuardManager) handleUserCreationEvent(user *domain.User) {
	logrus.Errorf("Handling new user event for %s", user.Identifier)

	err := m.CreateDefaultPeer(context.Background(), user)
	if err != nil {
		logrus.Errorf("Failed to create default peer")
		return
	}
}

func (m wireGuardManager) GetImportableInterfaces(ctx context.Context) ([]domain.PhysicalInterface, error) {
	physicalInterfaces, err := m.wg.GetInterfaces(ctx)
	if err != nil {
		return nil, err
	}

	return physicalInterfaces, nil
}

func (m wireGuardManager) ImportNewInterfaces(ctx context.Context, filter ...domain.InterfaceIdentifier) error {
	physicalInterfaces, err := m.wg.GetInterfaces(ctx)
	if err != nil {
		return err
	}

	// if no filter is given, exclude already existing interfaces
	var excludedInterfaces []domain.InterfaceIdentifier
	if len(filter) == 0 {
		existingInterfaces, err := m.db.GetAllInterfaces(ctx)
		if err != nil {
			return err
		}
		for _, existingInterface := range existingInterfaces {
			excludedInterfaces = append(excludedInterfaces, existingInterface.Identifier)
		}
	}

	for _, physicalInterface := range physicalInterfaces {
		if internal.SliceContains(excludedInterfaces, physicalInterface.Identifier) {
			continue
		}

		if len(filter) != 0 && !internal.SliceContains(filter, physicalInterface.Identifier) {
			continue
		}

		logrus.Infof("importing new interface %s...", physicalInterface.Identifier)

		physicalPeers, err := m.wg.GetPeers(ctx, physicalInterface.Identifier)
		if err != nil {
			return err
		}

		err = m.importInterface(ctx, &physicalInterface, physicalPeers)
		if err != nil {
			return fmt.Errorf("import of %s failed: %w", physicalInterface.Identifier, err)
		}

		logrus.Infof("imported new interface %s and %d peers", physicalInterface.Identifier, len(physicalPeers))
	}

	return nil
}

func (m wireGuardManager) importInterface(ctx context.Context, in *domain.PhysicalInterface, peers []domain.PhysicalPeer) error {
	now := time.Now()
	iface := domain.ConvertPhysicalInterface(in)
	iface.BaseModel = domain.BaseModel{
		CreatedBy: "importer",
		UpdatedBy: "importer",
		CreatedAt: now,
		UpdatedAt: now,
	}
	iface.PeerDefAllowedIPsStr = iface.AddressStr()

	existingInterface, err := m.db.GetInterface(ctx, iface.Identifier)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	if existingInterface != nil {
		return errors.New("interface already exists")
	}

	err = m.db.SaveInterface(ctx, iface.Identifier, func(_ *domain.Interface) (*domain.Interface, error) {
		return iface, nil
	})
	if err != nil {
		return fmt.Errorf("database save failed: %w", err)
	}

	// import peers
	for _, peer := range peers {
		err = m.importPeer(ctx, iface, &peer)
		if err != nil {
			return fmt.Errorf("import of peer %s failed: %w", peer.Identifier, err)
		}
	}

	return nil
}

func (m wireGuardManager) importPeer(ctx context.Context, in *domain.Interface, p *domain.PhysicalPeer) error {
	now := time.Now()
	peer := domain.ConvertPhysicalPeer(p)
	peer.BaseModel = domain.BaseModel{
		CreatedBy: "importer",
		UpdatedBy: "importer",
		CreatedAt: now,
		UpdatedAt: now,
	}

	peer.InterfaceIdentifier = in.Identifier
	peer.EndpointPublicKey = in.PublicKey
	peer.AllowedIPsStr = domain.StringConfigOption{Value: in.PeerDefAllowedIPsStr, Overridable: true}
	peer.Interface.Addresses = p.AllowedIPs // use allowed IP's as the peer IP's
	peer.Interface.DnsStr = domain.StringConfigOption{Value: in.PeerDefDnsStr, Overridable: true}
	peer.Interface.DnsSearchStr = domain.StringConfigOption{Value: in.PeerDefDnsSearchStr, Overridable: true}
	peer.Interface.Mtu = domain.IntConfigOption{Value: in.PeerDefMtu, Overridable: true}
	peer.Interface.FirewallMark = domain.Int32ConfigOption{Value: in.PeerDefFirewallMark, Overridable: true}
	peer.Interface.RoutingTable = domain.StringConfigOption{Value: in.PeerDefRoutingTable, Overridable: true}
	peer.Interface.PreUp = domain.StringConfigOption{Value: in.PeerDefPreUp, Overridable: true}
	peer.Interface.PostUp = domain.StringConfigOption{Value: in.PeerDefPostUp, Overridable: true}
	peer.Interface.PreDown = domain.StringConfigOption{Value: in.PeerDefPreDown, Overridable: true}
	peer.Interface.PostDown = domain.StringConfigOption{Value: in.PeerDefPostDown, Overridable: true}

	switch in.Type {
	case domain.InterfaceTypeAny:
		peer.Interface.Type = domain.InterfaceTypeAny
		peer.DisplayName = "Autodetected Peer (" + peer.Interface.PublicKey[0:8] + ")"
	case domain.InterfaceTypeClient:
		peer.Interface.Type = domain.InterfaceTypeServer
		peer.DisplayName = "Autodetected Endpoint (" + peer.Interface.PublicKey[0:8] + ")"
	case domain.InterfaceTypeServer:
		peer.Interface.Type = domain.InterfaceTypeClient
		peer.DisplayName = "Autodetected Client (" + peer.Interface.PublicKey[0:8] + ")"
	}

	err := m.db.SavePeer(ctx, peer.Identifier, func(_ *domain.Peer) (*domain.Peer, error) {
		return peer, nil
	})
	if err != nil {
		return fmt.Errorf("database save failed: %w", err)
	}

	return nil
}

func (m wireGuardManager) RestoreInterfaceState(ctx context.Context, updateDbOnError bool, filter ...domain.InterfaceIdentifier) error {
	interfaces, err := m.db.GetAllInterfaces(ctx)
	if err != nil {
		return err
	}

	for _, iface := range interfaces {
		if len(filter) != 0 && !internal.SliceContains(filter, iface.Identifier) {
			continue
		}

		peers, err := m.db.GetInterfacePeers(ctx, iface.Identifier)
		if err != nil {
			return fmt.Errorf("failed to load peers for %s: %w", iface.Identifier, err)
		}

		physicalInterface, err := m.wg.GetInterface(ctx, iface.Identifier)
		if err != nil {
			// try to create a new interface
			err := m.wg.SaveInterface(ctx, iface.Identifier, func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error) {
				domain.MergeToPhysicalInterface(pi, &iface)

				return pi, nil
			})
			if err != nil {
				if updateDbOnError {
					// disable interface in database as no physical interface exists
					_ = m.db.SaveInterface(ctx, iface.Identifier, func(in *domain.Interface) (*domain.Interface, error) {
						now := time.Now()
						in.Disabled = &now // set
						in.DisabledReason = "no physical interface available"
						return in, nil
					})
				}
				return fmt.Errorf("failed to create physical interface %s: %w", iface.Identifier, err)
			}

			// restore peers
			for _, peer := range peers {
				err := m.wg.SavePeer(ctx, iface.Identifier, peer.Identifier, func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error) {
					domain.MergeToPhysicalPeer(pp, &peer)
					return pp, nil
				})
				if err != nil {
					return fmt.Errorf("failed to create physical peer %s: %w", peer.Identifier, err)
				}
			}
		} else {
			if physicalInterface.DeviceUp != !iface.IsDisabled() {
				// try to move interface to stored state
				err := m.wg.SaveInterface(ctx, iface.Identifier, func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error) {
					pi.DeviceUp = !iface.IsDisabled()

					return pi, nil
				})
				if err != nil {
					if updateDbOnError {
						// disable interface in database as no physical interface is available
						_ = m.db.SaveInterface(ctx, iface.Identifier, func(in *domain.Interface) (*domain.Interface, error) {
							if iface.IsDisabled() {
								now := time.Now()
								in.Disabled = &now // set
								in.DisabledReason = "no physical interface active"
							} else {
								in.Disabled = nil
								in.DisabledReason = ""
							}
							return in, nil
						})
					}
					return fmt.Errorf("failed to change physical interface state for %s: %w", iface.Identifier, err)
				}
			}
		}
	}

	return nil
}

func (m wireGuardManager) CreateDefaultPeer(ctx context.Context, user *domain.User) error {
	// TODO: implement
	return nil
}

func (m wireGuardManager) GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error) {
	return m.db.GetInterfaceAndPeers(ctx, id)
}

func (m wireGuardManager) GetAllInterfaces(ctx context.Context) ([]domain.Interface, error) {
	return m.db.GetAllInterfaces(ctx)
}

func (m wireGuardManager) GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error) {
	return m.db.GetUserPeers(ctx, id)
}

func (m wireGuardManager) PrepareInterface(ctx context.Context) (*domain.Interface, error) {
	currentUser := domain.GetUserInfo(ctx)

	kp, err := domain.NewFreshKeypair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate keys: %w", err)
	}

	id, err := m.getNewInterfaceName(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new identifier: %w", err)
	}

	ipv4, ipv6, err := m.getFreshIpConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new ip config: %w", err)
	}

	freshInterface := &domain.Interface{
		BaseModel: domain.BaseModel{
			CreatedBy: string(currentUser.Id),
			UpdatedBy: string(currentUser.Id),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Identifier:                 id,
		KeyPair:                    kp,
		ListenPort:                 0, // TODO
		Addresses:                  []domain.Cidr{ipv4, ipv6},
		DnsStr:                     "",
		DnsSearchStr:               "",
		Mtu:                        1420,
		FirewallMark:               0,
		RoutingTable:               "",
		PreUp:                      "",
		PostUp:                     "",
		PreDown:                    "",
		PostDown:                   "",
		SaveConfig:                 false,
		DisplayName:                string(id),
		Type:                       domain.InterfaceTypeServer,
		DriverType:                 "",
		Disabled:                   nil,
		DisabledReason:             "",
		PeerDefNetworkStr:          "", // TODO
		PeerDefDnsStr:              "", // TODO
		PeerDefDnsSearchStr:        "",
		PeerDefEndpoint:            "",
		PeerDefAllowedIPsStr:       "",
		PeerDefMtu:                 1420,
		PeerDefPersistentKeepalive: 16,
		PeerDefFirewallMark:        0,
		PeerDefRoutingTable:        "",
		PeerDefPreUp:               "",
		PeerDefPostUp:              "",
		PeerDefPreDown:             "",
		PeerDefPostDown:            "",
	}

	return freshInterface, nil
}

func (m wireGuardManager) getNewInterfaceName(ctx context.Context) (domain.InterfaceIdentifier, error) {
	namePrefix := "wg"
	nameSuffix := 1

	existingInterfaces, err := m.db.GetAllInterfaces(ctx)
	if err != nil {
		return "", err
	}
	var name domain.InterfaceIdentifier
	for {
		name = domain.InterfaceIdentifier(fmt.Sprintf("%s%d", namePrefix, nameSuffix))

		conflict := false
		for _, in := range existingInterfaces {
			if in.Identifier == name {
				conflict = true
				break
			}
		}
		if !conflict {
			break
		}

		nameSuffix++
	}

	return name, nil
}

func (m wireGuardManager) getFreshIpConfig(ctx context.Context) (ipV4, ipV6 domain.Cidr, err error) {
	ips, err := m.db.GetInterfaceIps(ctx)
	if err != nil {
		err = fmt.Errorf("failed to get existing IP addresses: %w", err)
		return
	}

	ipV4, _ = domain.CidrFromString("10.6.6.1/24")
	ipV6, _ = domain.CidrFromString("fdfd:d3ad:c0de:1234::1/64")

	for {
		ipV4Conflict := false
		ipV6Conflict := false
		for _, usedIps := range ips {
			for _, ip := range usedIps {
				if ipV4 == ip {
					ipV4Conflict = true
				}

				if ipV6 == ip {
					ipV6Conflict = true
				}
			}
		}

		if !ipV4Conflict && !ipV6Conflict {
			break
		}

		if ipV4Conflict {
			ipV4 = ipV4.NextSubnet()
		}

		if ipV6Conflict {
			ipV6 = ipV6.NextSubnet()
		}
	}

	return
}

func (m wireGuardManager) CreateInterface(ctx context.Context, in *domain.Interface) (*domain.Interface, error) {
	existingInterface, err := m.db.GetInterface(ctx, in.Identifier)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("unable to load existing interface %s: %w", in.Identifier, err)
	}
	if existingInterface != nil {
		return nil, fmt.Errorf("interface %s already exists", in.Identifier)
	}

	if err := m.validateCreation(ctx, existingInterface, in); err != nil {
		return nil, fmt.Errorf("creation not allowed: %w", err)
	}

	err = m.db.SaveInterface(ctx, in.Identifier, func(i *domain.Interface) (*domain.Interface, error) {
		in.CopyCalculatedAttributes(i)

		err = m.wg.SaveInterface(ctx, in.Identifier, func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error) {
			domain.MergeToPhysicalInterface(pi, in)
			return pi, nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create physical interface %s: %w", in.Identifier, err)
		}

		return in, nil
	})
	if err != nil {
		return nil, fmt.Errorf("creation failure: %w", err)
	}

	return in, nil
}

func (m wireGuardManager) UpdateInterface(ctx context.Context, in *domain.Interface) (*domain.Interface, error) {
	existingInterface, err := m.db.GetInterface(ctx, in.Identifier)
	if err != nil {
		return nil, fmt.Errorf("unable to load existing interface %s: %w", in.Identifier, err)
	}

	if err := m.validateModifications(ctx, existingInterface, in); err != nil {
		return nil, fmt.Errorf("update not allowed: %w", err)
	}

	err = m.db.SaveInterface(ctx, in.Identifier, func(i *domain.Interface) (*domain.Interface, error) {
		in.CopyCalculatedAttributes(i)

		err = m.wg.SaveInterface(ctx, in.Identifier, func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error) {
			domain.MergeToPhysicalInterface(pi, in)
			return pi, nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to update physical interface %s: %w", in.Identifier, err)
		}

		return in, nil
	})
	if err != nil {
		return nil, fmt.Errorf("update failure: %w", err)
	}

	return in, nil
}

func (m wireGuardManager) validateModifications(ctx context.Context, old, new *domain.Interface) error {
	currentUser := domain.GetUserInfo(ctx)

	if !currentUser.IsAdmin {
		return fmt.Errorf("insufficient permissions")
	}

	return nil
}

func (m wireGuardManager) validateCreation(ctx context.Context, old, new *domain.Interface) error {
	currentUser := domain.GetUserInfo(ctx)

	if new.Identifier == "" {
		return fmt.Errorf("invalid interface identifier")
	}

	if !currentUser.IsAdmin {
		return fmt.Errorf("insufficient permissions")
	}

	return nil
}
