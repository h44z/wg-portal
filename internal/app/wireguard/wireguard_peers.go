package wireguard

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/sirupsen/logrus"
)

func (m Manager) CreateDefaultPeer(ctx context.Context, userId domain.UserIdentifier) error {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return err
	}

	existingInterfaces, err := m.db.GetAllInterfaces(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch all interfaces: %w", err)
	}

	var newPeers []domain.Peer
	for _, iface := range existingInterfaces {
		if iface.Type != domain.InterfaceTypeServer {
			continue // only create default peers for server interfaces
		}

		peer, err := m.PreparePeer(ctx, iface.Identifier)
		if err != nil {
			return fmt.Errorf("failed to create default peer for interface %s: %w", iface.Identifier, err)
		}

		peer.UserIdentifier = userId
		peer.DisplayName = fmt.Sprintf("Default Peer %s", internal.TruncateString(string(peer.Identifier), 8))
		peer.Notes = fmt.Sprintf("Default peer created for user %s", userId)
		peer.AutomaticallyCreated = true

		newPeers = append(newPeers, *peer)
	}

	for i, peer := range newPeers {
		_, err := m.CreatePeer(ctx, &newPeers[i])
		if err != nil {
			return fmt.Errorf("failed to create default peer %s on interface %s: %w",
				peer.Identifier, peer.InterfaceIdentifier, err)
		}
	}

	logrus.Infof("created %d default peers for user %s", len(newPeers), userId)

	return nil
}

func (m Manager) GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error) {
	if err := domain.ValidateUserAccessRights(ctx, id); err != nil {
		return nil, err
	}

	return m.db.GetUserPeers(ctx, id)
}

func (m Manager) PreparePeer(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Peer, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err // TODO: self provisioning?
	}

	currentUser := domain.GetUserInfo(ctx)

	iface, err := m.db.GetInterface(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("unable to find interface %s: %w", id, err)
	}

	ips, err := m.getFreshPeerIpConfig(ctx, iface)
	if err != nil {
		return nil, fmt.Errorf("unable to get fresh ip addresses: %w", err)
	}

	kp, err := domain.NewFreshKeypair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate keys: %w", err)
	}

	pk, err := domain.NewPreSharedKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate preshared key: %w", err)
	}

	peerMode := domain.InterfaceTypeClient
	if iface.Type == domain.InterfaceTypeClient {
		peerMode = domain.InterfaceTypeServer
	}

	peerId := domain.PeerIdentifier(kp.PublicKey)
	freshPeer := &domain.Peer{
		BaseModel: domain.BaseModel{
			CreatedBy: string(currentUser.Id),
			UpdatedBy: string(currentUser.Id),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Endpoint:            domain.NewStringConfigOption(iface.PeerDefEndpoint, true),
		EndpointPublicKey:   domain.NewStringConfigOption(iface.PublicKey, true),
		AllowedIPsStr:       domain.NewStringConfigOption(iface.PeerDefAllowedIPsStr, true),
		ExtraAllowedIPsStr:  "",
		PresharedKey:        pk,
		PersistentKeepalive: domain.NewIntConfigOption(iface.PeerDefPersistentKeepalive, true),
		DisplayName:         fmt.Sprintf("Peer %s", internal.TruncateString(string(peerId), 8)),
		Identifier:          peerId,
		UserIdentifier:      currentUser.Id,
		InterfaceIdentifier: iface.Identifier,
		Disabled:            nil,
		DisabledReason:      "",
		ExpiresAt:           nil,
		Notes:               "",
		Interface: domain.PeerInterfaceConfig{
			KeyPair:           kp,
			Type:              peerMode,
			Addresses:         ips,
			CheckAliveAddress: "",
			DnsStr:            domain.NewStringConfigOption(iface.PeerDefDnsStr, true),
			DnsSearchStr:      domain.NewStringConfigOption(iface.PeerDefDnsSearchStr, true),
			Mtu:               domain.NewIntConfigOption(iface.PeerDefMtu, true),
			FirewallMark:      domain.NewInt32ConfigOption(iface.PeerDefFirewallMark, true),
			RoutingTable:      domain.NewStringConfigOption(iface.PeerDefRoutingTable, true),
			PreUp:             domain.NewStringConfigOption(iface.PeerDefPreUp, true),
			PostUp:            domain.NewStringConfigOption(iface.PeerDefPostUp, true),
			PreDown:           domain.NewStringConfigOption(iface.PeerDefPreUp, true),
			PostDown:          domain.NewStringConfigOption(iface.PeerDefPostUp, true),
		},
	}

	return freshPeer, nil
}

func (m Manager) GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error) {
	peer, err := m.db.GetPeer(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("unable to find peer %s: %w", id, err)
	}

	if err := domain.ValidateUserAccessRights(ctx, peer.UserIdentifier); err != nil {
		return nil, err
	}

	return peer, nil
}

func (m Manager) CreatePeer(ctx context.Context, peer *domain.Peer) (*domain.Peer, error) {
	if err := domain.ValidateUserAccessRights(ctx, peer.UserIdentifier); err != nil {
		return nil, err
	}

	existingPeer, err := m.db.GetPeer(ctx, peer.Identifier)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("unable to load existing peer %s: %w", peer.Identifier, err)
	}
	if existingPeer != nil {
		return nil, fmt.Errorf("peer %s already exists", peer.Identifier)
	}

	if err := m.validatePeerCreation(ctx, existingPeer, peer); err != nil {
		return nil, fmt.Errorf("creation not allowed: %w", err)
	}

	err = m.savePeers(ctx, peer)
	if err != nil {
		return nil, fmt.Errorf("creation failure: %w", err)
	}

	return peer, nil
}

func (m Manager) CreateMultiplePeers(ctx context.Context, interfaceId domain.InterfaceIdentifier, r *domain.PeerCreationRequest) ([]domain.Peer, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	var newPeers []*domain.Peer

	for _, id := range r.UserIdentifiers {
		freshPeer, err := m.PreparePeer(ctx, interfaceId)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare peer for interface %s: %w", interfaceId, err)
		}

		freshPeer.UserIdentifier = domain.UserIdentifier(id) // use id as user identifier. peers are allowed to have invalid user identifiers
		if r.Suffix != "" {
			freshPeer.DisplayName += " " + r.Suffix
		}

		if err := m.validatePeerCreation(ctx, nil, freshPeer); err != nil {
			return nil, fmt.Errorf("creation not allowed: %w", err)
		}

		newPeers = append(newPeers, freshPeer)
	}

	err := m.savePeers(ctx, newPeers...)
	if err != nil {
		return nil, fmt.Errorf("failed to create new peers: %w", err)
	}

	createdPeers := make([]domain.Peer, len(newPeers))
	for i := range newPeers {
		createdPeers[i] = *newPeers[i]
	}

	return createdPeers, nil
}

func (m Manager) UpdatePeer(ctx context.Context, peer *domain.Peer) (*domain.Peer, error) {
	existingPeer, err := m.db.GetPeer(ctx, peer.Identifier)
	if err != nil {
		return nil, fmt.Errorf("unable to load existing peer %s: %w", peer.Identifier, err)
	}

	if err := domain.ValidateUserAccessRights(ctx, existingPeer.UserIdentifier); err != nil {
		return nil, err
	}

	if err := m.validatePeerModifications(ctx, existingPeer, peer); err != nil {
		return nil, fmt.Errorf("update not allowed: %w", err)
	}

	err = m.savePeers(ctx, peer)
	if err != nil {
		return nil, fmt.Errorf("update failure: %w", err)
	}

	return peer, nil
}

func (m Manager) DeletePeer(ctx context.Context, id domain.PeerIdentifier) error {
	peer, err := m.db.GetPeer(ctx, id)
	if err != nil {
		return fmt.Errorf("unable to find peer %s: %w", id, err)
	}

	if err := domain.ValidateUserAccessRights(ctx, peer.UserIdentifier); err != nil {
		return err
	}

	err = m.wg.DeletePeer(ctx, peer.InterfaceIdentifier, id)
	if err != nil {
		return fmt.Errorf("wireguard failed to delete peer %s: %w", id, err)
	}

	err = m.db.DeletePeer(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete peer %s: %w", id, err)
	}

	// Update routes after peers have changed
	m.bus.Publish(app.TopicRouteUpdate, "peers updated")
	// Update interface after peers have changed
	m.bus.Publish(app.TopicPeerInterfaceUpdated, peer.InterfaceIdentifier)

	return nil
}

func (m Manager) GetPeerStats(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.PeerStatus, error) {
	_, peers, err := m.db.GetInterfaceAndPeers(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch peers for interface %s: %w", id, err)
	}

	peerIds := make([]domain.PeerIdentifier, len(peers))
	for i, peer := range peers {
		if err := domain.ValidateUserAccessRights(ctx, peer.UserIdentifier); err != nil {
			return nil, err
		}

		peerIds[i] = peer.Identifier
	}

	return m.db.GetPeersStats(ctx, peerIds...)
}

func (m Manager) GetUserPeerStats(ctx context.Context, id domain.UserIdentifier) ([]domain.PeerStatus, error) {
	if err := domain.ValidateUserAccessRights(ctx, id); err != nil {
		return nil, err
	}

	peers, err := m.db.GetUserPeers(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch peers for user %s: %w", id, err)
	}

	peerIds := make([]domain.PeerIdentifier, len(peers))
	for i, peer := range peers {
		peerIds[i] = peer.Identifier
	}

	return m.db.GetPeersStats(ctx, peerIds...)
}

// region helper-functions

func (m Manager) savePeers(ctx context.Context, peers ...*domain.Peer) error {
	interfaces := make(map[domain.InterfaceIdentifier]struct{})

	for i := range peers {
		peer := peers[i]
		err := m.db.SavePeer(ctx, peer.Identifier, func(p *domain.Peer) (*domain.Peer, error) {
			peer.CopyCalculatedAttributes(p)

			err := m.wg.SavePeer(ctx, peer.InterfaceIdentifier, peer.Identifier,
				func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error) {
					domain.MergeToPhysicalPeer(pp, peer)
					return pp, nil
				})
			if err != nil {
				return nil, fmt.Errorf("failed to save wireguard peer %s: %w", peer.Identifier, err)
			}

			return peer, nil
		})
		if err != nil {
			return fmt.Errorf("save failure for peer %s: %w", peer.Identifier, err)
		}

		interfaces[peer.InterfaceIdentifier] = struct{}{}
	}

	// Update routes after peers have changed
	if len(interfaces) != 0 {
		m.bus.Publish(app.TopicRouteUpdate, "peers updated")
	}

	for iface := range interfaces {
		m.bus.Publish(app.TopicPeerInterfaceUpdated, iface)
	}

	return nil
}

func (m Manager) getFreshPeerIpConfig(ctx context.Context, iface *domain.Interface) (ips []domain.Cidr, err error) {
	if iface.PeerDefNetworkStr == "" {
		return []domain.Cidr{}, nil // cannot suggest new ip addresses if there is no subnet
	}

	networks, err := domain.CidrsFromString(iface.PeerDefNetworkStr)
	if err != nil {
		err = fmt.Errorf("failed to parse default network address: %w", err)
		return
	}

	existingIps, err := m.db.GetUsedIpsPerSubnet(ctx, networks)
	if err != nil {
		err = fmt.Errorf("failed to get existing IP addresses: %w", err)
		return
	}

	for _, network := range networks {
		ip := network.NextAddr()

		for {
			ipConflict := false
			for _, usedIp := range existingIps[network] {
				if usedIp.Addr == ip.Addr {
					ipConflict = true
					break
				}
			}

			if !ipConflict {
				break
			}

			ip = ip.NextAddr()

			if !ip.IsValid() {
				return nil, fmt.Errorf("ip space on subnet %s is exhausted", network.String())
			}
		}

		ips = append(ips, ip.HostAddr())
	}

	return
}

func (m Manager) validatePeerModifications(ctx context.Context, old, new *domain.Peer) error {
	currentUser := domain.GetUserInfo(ctx)

	if !currentUser.IsAdmin {
		return fmt.Errorf("insufficient permissions")
	}

	return nil
}

func (m Manager) validatePeerCreation(ctx context.Context, old, new *domain.Peer) error {
	currentUser := domain.GetUserInfo(ctx)

	if new.Identifier == "" {
		return fmt.Errorf("invalid peer identifier")
	}

	if !currentUser.IsAdmin {
		return fmt.Errorf("insufficient permissions")
	}

	return nil
}

func (m Manager) validatePeerDeletion(ctx context.Context, del *domain.Peer) error {
	currentUser := domain.GetUserInfo(ctx)

	if !currentUser.IsAdmin {
		return fmt.Errorf("insufficient permissions")
	}

	return nil
}

// endregion helper-functions
