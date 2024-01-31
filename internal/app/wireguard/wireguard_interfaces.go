package wireguard

import (
	"context"
	"errors"
	"fmt"
	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/sirupsen/logrus"
	"os"
	"time"
)

func (m Manager) GetImportableInterfaces(ctx context.Context) ([]domain.PhysicalInterface, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	physicalInterfaces, err := m.wg.GetInterfaces(ctx)
	if err != nil {
		return nil, err
	}

	return physicalInterfaces, nil
}

func (m Manager) GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, nil, err
	}

	return m.db.GetInterfaceAndPeers(ctx, id)
}

func (m Manager) GetAllInterfaces(ctx context.Context) ([]domain.Interface, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	return m.db.GetAllInterfaces(ctx)
}

func (m Manager) GetAllInterfacesAndPeers(ctx context.Context) ([]domain.Interface, [][]domain.Peer, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, nil, err
	}

	interfaces, err := m.db.GetAllInterfaces(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to load all interfaces: %w", err)
	}

	allPeers := make([][]domain.Peer, len(interfaces))
	for i, iface := range interfaces {
		peers, err := m.db.GetInterfacePeers(ctx, iface.Identifier)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load peers for interface %s: %w", iface.Identifier, err)
		}
		allPeers[i] = peers
	}

	return interfaces, allPeers, nil
}

func (m Manager) ImportNewInterfaces(ctx context.Context, filter ...domain.InterfaceIdentifier) (int, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return 0, err
	}

	physicalInterfaces, err := m.wg.GetInterfaces(ctx)
	if err != nil {
		return 0, err
	}

	// if no filter is given, exclude already existing interfaces
	var excludedInterfaces []domain.InterfaceIdentifier
	if len(filter) == 0 {
		existingInterfaces, err := m.db.GetAllInterfaces(ctx)
		if err != nil {
			return 0, err
		}
		for _, existingInterface := range existingInterfaces {
			excludedInterfaces = append(excludedInterfaces, existingInterface.Identifier)
		}
	}

	imported := 0
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
			return 0, err
		}

		err = m.importInterface(ctx, &physicalInterface, physicalPeers)
		if err != nil {
			return 0, fmt.Errorf("import of %s failed: %w", physicalInterface.Identifier, err)
		}

		logrus.Infof("imported new interface %s and %d peers", physicalInterface.Identifier, len(physicalPeers))
		imported++
	}

	return imported, nil
}

func (m Manager) ApplyPeerDefaults(ctx context.Context, in *domain.Interface) error {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return err
	}

	existingInterface, err := m.db.GetInterface(ctx, in.Identifier)
	if err != nil {
		return fmt.Errorf("unable to load existing interface %s: %w", in.Identifier, err)
	}

	if err := m.validateInterfaceModifications(ctx, existingInterface, in); err != nil {
		return fmt.Errorf("update not allowed: %w", err)
	}

	peers, err := m.db.GetInterfacePeers(ctx, in.Identifier)
	if err != nil {
		return fmt.Errorf("failed to find peers for interface %s: %w", in.Identifier, err)
	}

	for i := range peers {
		(&peers[i]).ApplyInterfaceDefaults(in)

		_, err := m.UpdatePeer(ctx, &peers[i])
		if err != nil {
			return fmt.Errorf("failed to apply interface defaults to peer %s: %w", peers[i].Identifier, err)
		}
	}

	return nil
}

func (m Manager) RestoreInterfaceState(ctx context.Context, updateDbOnError bool, filter ...domain.InterfaceIdentifier) error {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return err
	}

	interfaces, err := m.db.GetAllInterfaces(ctx)
	if err != nil {
		return err
	}

	for _, iface := range interfaces {
		if len(filter) != 0 && !internal.SliceContains(filter, iface.Identifier) {
			continue // ignore filtered interface
		}

		peers, err := m.db.GetInterfacePeers(ctx, iface.Identifier)
		if err != nil {
			return fmt.Errorf("failed to load peers for %s: %w", iface.Identifier, err)
		}

		_, err = m.wg.GetInterface(ctx, iface.Identifier)
		if err != nil {
			logrus.Debugf("creating missing interface %s...", iface.Identifier)

			// try to create a new interface
			_, err = m.saveInterface(ctx, &iface, peers)
			if err != nil {
				return err
			}
			if err != nil {
				if updateDbOnError {
					// disable interface in database as no physical interface exists
					_ = m.db.SaveInterface(ctx, iface.Identifier, func(in *domain.Interface) (*domain.Interface, error) {
						now := time.Now()
						in.Disabled = &now // set
						in.DisabledReason = domain.DisabledReasonInterfaceMissing
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
			logrus.Debugf("restoring interface state for %s to disabled=%t", iface.Identifier, iface.IsDisabled())

			// try to move interface to stored state
			_, err = m.saveInterface(ctx, &iface, peers)
			if err != nil {
				return err
			}
			if err != nil {
				if updateDbOnError {
					// disable interface in database as no physical interface is available
					_ = m.db.SaveInterface(ctx, iface.Identifier, func(in *domain.Interface) (*domain.Interface, error) {
						if iface.IsDisabled() {
							now := time.Now()
							in.Disabled = &now // set
							in.DisabledReason = domain.DisabledReasonInterfaceMissing
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

	return nil
}

func (m Manager) PrepareInterface(ctx context.Context) (*domain.Interface, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	currentUser := domain.GetUserInfo(ctx)

	kp, err := domain.NewFreshKeypair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate keys: %w", err)
	}

	id, err := m.getNewInterfaceName(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new identifier: %w", err)
	}

	ipv4, ipv6, err := m.getFreshInterfaceIpConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new ip config: %w", err)
	}

	port, err := m.getFreshListenPort(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate new listen port: %w", err)
	}

	ips := []domain.Cidr{ipv4}
	if m.cfg.Advanced.UseIpV6 {
		ips = append(ips, ipv6)
	}
	networks := []domain.Cidr{ipv4.NetworkAddr()}
	if m.cfg.Advanced.UseIpV6 {
		networks = append(networks, ipv6.NetworkAddr())
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
		ListenPort:                 port,
		Addresses:                  ips,
		DnsStr:                     "",
		DnsSearchStr:               "",
		Mtu:                        1420,
		FirewallMark:               0,
		RoutingTable:               "",
		PreUp:                      "",
		PostUp:                     "",
		PreDown:                    "",
		PostDown:                   "",
		SaveConfig:                 m.cfg.Advanced.ConfigStoragePath != "",
		DisplayName:                string(id),
		Type:                       domain.InterfaceTypeServer,
		DriverType:                 "",
		Disabled:                   nil,
		DisabledReason:             "",
		PeerDefNetworkStr:          domain.CidrsToString(networks),
		PeerDefDnsStr:              "",
		PeerDefDnsSearchStr:        "",
		PeerDefEndpoint:            "",
		PeerDefAllowedIPsStr:       domain.CidrsToString(networks),
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

func (m Manager) CreateInterface(ctx context.Context, in *domain.Interface) (*domain.Interface, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	existingInterface, err := m.db.GetInterface(ctx, in.Identifier)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("unable to load existing interface %s: %w", in.Identifier, err)
	}
	if existingInterface != nil {
		return nil, fmt.Errorf("interface %s already exists", in.Identifier)
	}

	if err := m.validateInterfaceCreation(ctx, existingInterface, in); err != nil {
		return nil, fmt.Errorf("creation not allowed: %w", err)
	}

	in, err = m.saveInterface(ctx, in, nil)
	if err != nil {
		return nil, fmt.Errorf("creation failure: %w", err)
	}

	return in, nil
}

func (m Manager) UpdateInterface(ctx context.Context, in *domain.Interface) (*domain.Interface, []domain.Peer, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, nil, err
	}

	existingInterface, existingPeers, err := m.db.GetInterfaceAndPeers(ctx, in.Identifier)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to load existing interface %s: %w", in.Identifier, err)
	}

	if err := m.validateInterfaceModifications(ctx, existingInterface, in); err != nil {
		return nil, nil, fmt.Errorf("update not allowed: %w", err)
	}

	in, err = m.saveInterface(ctx, in, existingPeers)
	if err != nil {
		return nil, nil, fmt.Errorf("update failure: %w", err)
	}

	return in, existingPeers, nil
}

func (m Manager) DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return err
	}

	existingInterface, err := m.db.GetInterface(ctx, id)
	if err != nil {
		return fmt.Errorf("unable to find interface %s: %w", id, err)
	}

	if err := m.validateInterfaceDeletion(ctx, existingInterface); err != nil {
		return fmt.Errorf("deletion not allowed: %w", err)
	}

	now := time.Now()
	existingInterface.Disabled = &now // simulate a disabled interface
	existingInterface.DisabledReason = domain.DisabledReasonDeleted

	physicalInterface, _ := m.wg.GetInterface(ctx, id)

	if err := m.handleInterfacePreSaveHooks(true, existingInterface); err != nil {
		return fmt.Errorf("pre-delete hooks failed: %w", err)
	}

	if err := m.handleInterfacePreSaveActions(existingInterface); err != nil {
		return fmt.Errorf("pre-delete actions failed: %w", err)
	}

	if err := m.deleteInterfacePeers(ctx, id); err != nil {
		return fmt.Errorf("peer deletion failure: %w", err)
	}

	if err := m.wg.DeleteInterface(ctx, id); err != nil {
		return fmt.Errorf("wireguard deletion failure: %w", err)
	}

	if err := m.db.DeleteInterface(ctx, id); err != nil {
		return fmt.Errorf("deletion failure: %w", err)
	}

	fwMark := int(existingInterface.FirewallMark)
	if physicalInterface != nil && fwMark == 0 {
		fwMark = int(physicalInterface.FirewallMark)
	}
	m.bus.Publish(app.TopicRouteRemove, domain.RoutingTableInfo{
		FwMark: fwMark,
		Table:  existingInterface.GetRoutingTable(),
	})

	if err := m.handleInterfacePostSaveHooks(true, existingInterface); err != nil {
		return fmt.Errorf("post-delete hooks failed: %w", err)
	}

	return nil
}

// region helper-functions

func (m Manager) saveInterface(ctx context.Context, iface *domain.Interface, peers []domain.Peer) (*domain.Interface, error) {
	stateChanged := m.hasInterfaceStateChanged(ctx, iface)

	if err := m.handleInterfacePreSaveHooks(stateChanged, iface); err != nil {
		return nil, fmt.Errorf("pre-save hooks failed: %w", err)
	}

	if err := m.handleInterfacePreSaveActions(iface); err != nil {
		return nil, fmt.Errorf("pre-save actions failed: %w", err)
	}

	err := m.db.SaveInterface(ctx, iface.Identifier, func(i *domain.Interface) (*domain.Interface, error) {
		iface.CopyCalculatedAttributes(i)

		err := m.wg.SaveInterface(ctx, iface.Identifier, func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error) {
			domain.MergeToPhysicalInterface(pi, iface)
			return pi, nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to save physical interface %s: %w", iface.Identifier, err)
		}

		return iface, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to save interface: %w", err)
	}

	m.bus.Publish(app.TopicRouteUpdate, "interface updated: "+string(iface.Identifier))
	if iface.IsDisabled() {
		physicalInterface, _ := m.wg.GetInterface(ctx, iface.Identifier)
		fwMark := int(iface.FirewallMark)
		if physicalInterface != nil && fwMark == 0 {
			fwMark = int(physicalInterface.FirewallMark)
		}
		m.bus.Publish(app.TopicRouteRemove, domain.RoutingTableInfo{
			FwMark: fwMark,
			Table:  iface.GetRoutingTable(),
		})
	}

	if err := m.handleInterfacePostSaveHooks(stateChanged, iface); err != nil {
		return nil, fmt.Errorf("post-save hooks failed: %w", err)
	}

	m.bus.Publish(app.TopicInterfaceUpdated, iface)

	return iface, nil
}

func (m Manager) hasInterfaceStateChanged(ctx context.Context, iface *domain.Interface) bool {
	oldInterface, err := m.db.GetInterface(ctx, iface.Identifier)
	if err != nil {
		return false
	}

	return oldInterface.IsDisabled() != iface.IsDisabled()
}

func (m Manager) handleInterfacePreSaveActions(iface *domain.Interface) error {
	if !iface.IsDisabled() {
		if err := m.quick.SetDNS(iface.Identifier, iface.DnsStr, iface.DnsSearchStr); err != nil {
			return fmt.Errorf("failed to update dns settings: %w", err)
		}
	} else {
		if err := m.quick.UnsetDNS(iface.Identifier); err != nil {
			return fmt.Errorf("failed to clear dns settings: %w", err)
		}
	}
	return nil
}

func (m Manager) handleInterfacePreSaveHooks(stateChanged bool, iface *domain.Interface) error {
	if !stateChanged {
		return nil // do nothing if state did not change
	}

	if !iface.IsDisabled() {
		if err := m.quick.ExecuteInterfaceHook(iface.Identifier, iface.PreUp); err != nil {
			return fmt.Errorf("failed to execute pre-up hook: %w", err)
		}
	} else {
		if err := m.quick.ExecuteInterfaceHook(iface.Identifier, iface.PreDown); err != nil {
			return fmt.Errorf("failed to execute pre-down hook: %w", err)
		}
	}
	return nil
}

func (m Manager) handleInterfacePostSaveHooks(stateChanged bool, iface *domain.Interface) error {
	if !stateChanged {
		return nil // do nothing if state did not change
	}

	if !iface.IsDisabled() {
		if err := m.quick.ExecuteInterfaceHook(iface.Identifier, iface.PostUp); err != nil {
			return fmt.Errorf("failed to execute post-up hook: %w", err)
		}
	} else {
		if err := m.quick.ExecuteInterfaceHook(iface.Identifier, iface.PostDown); err != nil {
			return fmt.Errorf("failed to execute post-down hook: %w", err)
		}
	}
	return nil
}

func (m Manager) getNewInterfaceName(ctx context.Context) (domain.InterfaceIdentifier, error) {
	namePrefix := "wg"
	nameSuffix := 0

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

func (m Manager) getFreshInterfaceIpConfig(ctx context.Context) (ipV4, ipV6 domain.Cidr, err error) {
	ips, err := m.db.GetInterfaceIps(ctx)
	if err != nil {
		err = fmt.Errorf("failed to get existing IP addresses: %w", err)
		return
	}

	useV6 := m.cfg.Advanced.UseIpV6
	ipV4, _ = domain.CidrFromString(m.cfg.Advanced.StartCidrV4)
	ipV6, _ = domain.CidrFromString(m.cfg.Advanced.StartCidrV6)

	ipV4 = ipV4.FirstAddr()
	ipV6 = ipV6.FirstAddr()

	netV4 := ipV4.NetworkAddr()
	netV6 := ipV6.NetworkAddr()
	for {
		v4Conflict := false
		v6Conflict := false
		for _, usedIps := range ips {
			for _, usedIp := range usedIps {
				usedNetwork := usedIp.NetworkAddr()
				if netV4 == usedNetwork {
					v4Conflict = true
				}

				if netV6 == usedNetwork {
					v6Conflict = true
				}
			}
		}

		if !v4Conflict && (!useV6 || !v6Conflict) {
			break
		}

		if v4Conflict {
			netV4 = netV4.NextSubnet()
		}

		if v6Conflict && useV6 {
			netV6 = netV6.NextSubnet()
		}

		if !netV4.IsValid() {
			return domain.Cidr{}, domain.Cidr{}, fmt.Errorf("IPv4 space exhausted")
		}

		if useV6 && !netV6.IsValid() {
			return domain.Cidr{}, domain.Cidr{}, fmt.Errorf("IPv6 space exhausted")
		}
	}

	// use first address in network for interface
	ipV4 = netV4.NextAddr()
	ipV6 = netV6.NextAddr()

	return
}

func (m Manager) getFreshListenPort(ctx context.Context) (port int, err error) {
	existingInterfaces, err := m.db.GetAllInterfaces(ctx)
	if err != nil {
		return -1, err
	}

	port = m.cfg.Advanced.StartListenPort

	for {
		conflict := false
		for _, in := range existingInterfaces {
			if in.ListenPort == port {
				conflict = true
				break
			}
		}
		if !conflict {
			break
		}

		port++
	}

	if port > 65535 { // maximum allowed port number (16 bit uint)
		return -1, fmt.Errorf("port space exhausted")
	}

	return
}

func (m Manager) importInterface(ctx context.Context, in *domain.PhysicalInterface, peers []domain.PhysicalPeer) error {
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

func (m Manager) importPeer(ctx context.Context, in *domain.Interface, p *domain.PhysicalPeer) error {
	now := time.Now()
	peer := domain.ConvertPhysicalPeer(p)
	peer.BaseModel = domain.BaseModel{
		CreatedBy: "importer",
		UpdatedBy: "importer",
		CreatedAt: now,
		UpdatedAt: now,
	}

	peer.InterfaceIdentifier = in.Identifier
	peer.EndpointPublicKey = domain.StringConfigOption{Value: in.PublicKey, Overridable: true}
	peer.AllowedIPsStr = domain.StringConfigOption{Value: in.PeerDefAllowedIPsStr, Overridable: true}
	peer.Interface.Addresses = p.AllowedIPs // use allowed IP's as the peer IP's TODO: Should this also match server interface address' prefix length?
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

func (m Manager) deleteInterfacePeers(ctx context.Context, id domain.InterfaceIdentifier) error {
	allPeers, err := m.db.GetInterfacePeers(ctx, id)
	if err != nil {
		return err
	}
	for _, peer := range allPeers {
		err = m.wg.DeletePeer(ctx, id, peer.Identifier)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("wireguard peer deletion failure for %s: %w", peer.Identifier, err)
		}

		err = m.db.DeletePeer(ctx, peer.Identifier)
		if err != nil {
			return fmt.Errorf("peer deletion failure for %s: %w", peer.Identifier, err)
		}
	}

	return nil
}

func (m Manager) validateInterfaceModifications(ctx context.Context, old, new *domain.Interface) error {
	currentUser := domain.GetUserInfo(ctx)

	if !currentUser.IsAdmin {
		return fmt.Errorf("insufficient permissions")
	}

	return nil
}

func (m Manager) validateInterfaceCreation(ctx context.Context, old, new *domain.Interface) error {
	currentUser := domain.GetUserInfo(ctx)

	if new.Identifier == "" {
		return fmt.Errorf("invalid interface identifier")
	}

	if !currentUser.IsAdmin {
		return fmt.Errorf("insufficient permissions")
	}

	return nil
}

func (m Manager) validateInterfaceDeletion(ctx context.Context, del *domain.Interface) error {
	currentUser := domain.GetUserInfo(ctx)

	if !currentUser.IsAdmin {
		return fmt.Errorf("insufficient permissions")
	}

	return nil
}

// endregion helper-functions
