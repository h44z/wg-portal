package wireguard

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/app/audit"
	"github.com/h44z/wg-portal/internal/domain"
)

// CreateDefaultPeer creates a default peer for the given user on all server interfaces.
func (m Manager) CreateDefaultPeer(ctx context.Context, userId domain.UserIdentifier) error {
	if !m.cfg.DefaultPeerCreationEnabled() {
		return nil
	}

	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return err
	}

	existingInterfaces, err := m.db.GetAllInterfaces(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch all interfaces: %w", err)
	}

	user, err := m.db.GetUser(ctx, userId)
	if err != nil {
		return fmt.Errorf("failed to fetch user: %w", err)
	}

	var newPeers []domain.Peer
	for _, iface := range existingInterfaces {
		peer, err := m.prepareDefaultPeer(ctx, &iface, user)
		if err != nil {
			return fmt.Errorf("failed to prepare default peer: %w", err)
		}

		if peer != nil {
			newPeers = append(newPeers, *peer)
		}
	}

	for i, peer := range newPeers {
		_, err := m.CreatePeer(ctx, &newPeers[i])
		if err != nil {
			return fmt.Errorf("failed to create default peer %s on interface %s: %w",
				peer.Identifier, peer.InterfaceIdentifier, err)
		}
	}

	slog.InfoContext(ctx, "created default peers for user", "user", userId, "count", len(newPeers))

	return nil
}

// CreateDefaultPeers creates default peers for all existing users on the given interface.
func (m Manager) CreateDefaultPeers(ctx context.Context, interfaceId domain.InterfaceIdentifier) error {
	if !m.cfg.DefaultPeerCreationEnabled() {
		return nil
	}

	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return err
	}

	iface, err := m.db.GetInterface(ctx, interfaceId)
	if err != nil {
		return fmt.Errorf("failed to fetch interface %s: %w", interfaceId, err)
	}

	if !iface.CreateDefaultPeers() {
		return nil
	}

	users, err := m.db.GetAllUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch all users: %w", err)
	}

	var errs error
	var peerCount int
	for _, user := range users {
		peer, err := m.prepareDefaultPeer(ctx, iface, &user)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to prepare default peer for user %s: %w",
				user.Identifier, err))
			continue
		}
		if peer == nil {
			continue
		}
		_, err = m.CreatePeer(ctx, peer)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to create default peer for user %s: %w",
				user.Identifier, err))
			continue
		}
		peerCount++
	}

	if errs != nil {
		return fmt.Errorf("failed to create default peers for interface %s: %w", interfaceId, errs)
	}

	slog.InfoContext(ctx, "created default peers for interface", "interface", interfaceId, "count", peerCount)

	return nil
}

// GetUserPeers returns all peers for the given user.
func (m Manager) GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error) {
	if err := domain.ValidateUserAccessRights(ctx, id); err != nil {
		return nil, err
	}

	return m.db.GetUserPeers(ctx, id)
}

// PreparePeer prepares a new peer for the given interface with fresh keys and ip addresses.
func (m Manager) PreparePeer(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Peer, error) {
	if !m.cfg.Core.SelfProvisioningAllowed {
		if err := domain.ValidateAdminAccessRights(ctx); err != nil {
			return nil, err
		}
	}

	currentUser := domain.GetUserInfo(ctx)

	if err := m.checkInterfaceAccess(ctx, id); err != nil {
		return nil, err
	}

	iface, err := m.db.GetInterface(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("unable to find interface %s: %w", id, err)
	}

	if m.cfg.Core.SelfProvisioningAllowed && !currentUser.IsAdmin && iface.Type != domain.InterfaceTypeServer {
		return nil, fmt.Errorf("self provisioning is only allowed for server interfaces: %w", domain.ErrNoPermission)
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
		Endpoint:            domain.NewConfigOption(iface.PeerDefEndpoint, true),
		EndpointPublicKey:   domain.NewConfigOption(iface.PublicKey, true),
		AllowedIPsStr:       domain.NewConfigOption(iface.PeerDefAllowedIPsStr, true),
		ExtraAllowedIPsStr:  "",
		PresharedKey:        pk,
		PersistentKeepalive: domain.NewConfigOption(iface.PeerDefPersistentKeepalive, true),
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
			DnsStr:            domain.NewConfigOption(iface.PeerDefDnsStr, true),
			DnsSearchStr:      domain.NewConfigOption(iface.PeerDefDnsSearchStr, true),
			Mtu:               domain.NewConfigOption(iface.PeerDefMtu, true),
			FirewallMark:      domain.NewConfigOption(iface.PeerDefFirewallMark, true),
			RoutingTable:      domain.NewConfigOption(iface.PeerDefRoutingTable, true),
			PreUp:             domain.NewConfigOption(iface.PeerDefPreUp, true),
			PostUp:            domain.NewConfigOption(iface.PeerDefPostUp, true),
			PreDown:           domain.NewConfigOption(iface.PeerDefPreDown, true),
			PostDown:          domain.NewConfigOption(iface.PeerDefPostDown, true),
		},
	}
	freshPeer.GenerateDisplayName("")

	return freshPeer, nil
}

// GetPeer returns the peer with the given identifier.
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

// CreatePeer creates a new peer.
func (m Manager) CreatePeer(ctx context.Context, peer *domain.Peer) (*domain.Peer, error) {
	if !m.cfg.Core.SelfProvisioningAllowed {
		if err := domain.ValidateAdminAccessRights(ctx); err != nil {
			return nil, err
		}
	} else {
		if err := domain.ValidateUserAccessRights(ctx, peer.UserIdentifier); err != nil {
			return nil, err
		}
		if err := m.checkInterfaceAccess(ctx, peer.InterfaceIdentifier); err != nil {
			return nil, err
		}
	}

	sessionUser := domain.GetUserInfo(ctx)

	peer.Identifier = domain.PeerIdentifier(peer.Interface.PublicKey) // ensure that identifier corresponds to the public key

	// Enforce peer limit for non-admin users if LimitAdditionalUserPeers is set
	if m.cfg.Core.SelfProvisioningAllowed && !sessionUser.IsAdmin && m.cfg.Advanced.LimitAdditionalUserPeers > 0 {
		peers, err := m.db.GetUserPeers(ctx, peer.UserIdentifier)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch peers for user %s: %w", peer.UserIdentifier, err)
		}
		// Count enabled peers (disabled IS NULL)
		peerCount := 0
		for _, p := range peers {
			if !p.IsDisabled() {
				peerCount++
			}
		}
		totalAllowedPeers := 1 + m.cfg.Advanced.LimitAdditionalUserPeers // 1 default peer + x additional peers
		if peerCount >= totalAllowedPeers {
			slog.WarnContext(ctx, "peer creation blocked due to limit",
				"user", peer.UserIdentifier,
				"current_count", peerCount,
				"allowed_count", totalAllowedPeers)
			return nil, fmt.Errorf("peer limit reached (%d peers allowed): %w", totalAllowedPeers,
				domain.ErrNoPermission)
		}
	}

	existingPeer, err := m.db.GetPeer(ctx, peer.Identifier)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("unable to load existing peer %s: %w", peer.Identifier, err)
	}
	if existingPeer != nil {
		return nil, fmt.Errorf("peer %s already exists: %w", peer.Identifier, domain.ErrDuplicateEntry)
	}

	// if a peer is self provisioned, ensure that only allowed fields are set from the request
	if !sessionUser.IsAdmin {
		preparedPeer, err := m.PreparePeer(ctx, peer.InterfaceIdentifier)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare peer for interface %s: %w", peer.InterfaceIdentifier, err)
		}

		preparedPeer.OverwriteUserEditableFields(peer, m.cfg)

		peer = preparedPeer
	}

	if err := m.validatePeerCreation(ctx, existingPeer, peer); err != nil {
		return nil, fmt.Errorf("creation not allowed: %w", err)
	}

	// Apply rotation interval if configured and ExpiresAt is not already set
	if m.cfg.Core.Peer.RotationInterval > 0 && peer.ExpiresAt == nil {
		expiresAt := time.Now().Add(m.cfg.Core.Peer.RotationInterval)
		peer.ExpiresAt = &expiresAt
	}

	err = m.savePeers(ctx, peer)
	if err != nil {
		return nil, fmt.Errorf("creation failure: %w", err)
	}

	m.bus.Publish(app.TopicPeerCreated, *peer)

	return peer, nil
}

// CreateMultiplePeers creates multiple new peers for the given user identifiers.
// It calls PreparePeer for each user identifier in the request.
func (m Manager) CreateMultiplePeers(
	ctx context.Context,
	interfaceId domain.InterfaceIdentifier,
	r *domain.PeerCreationRequest,
) ([]domain.Peer, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	createdPeers := make([]domain.Peer, 0, len(r.UserIdentifiers))

	for _, id := range r.UserIdentifiers {
		freshPeer, err := m.PreparePeer(ctx, interfaceId)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare peer for interface %s: %w", interfaceId, err)
		}

		freshPeer.UserIdentifier = domain.UserIdentifier(id) // use id as user identifier. peers are allowed to have invalid user identifiers
		if r.Prefix != "" {
			freshPeer.DisplayName = r.Prefix + " " + freshPeer.DisplayName
		}

		if err := m.validatePeerCreation(ctx, nil, freshPeer); err != nil {
			return nil, fmt.Errorf("creation not allowed: %w", err)
		}

		// Apply rotation interval if configured and ExpiresAt is not already set
		if m.cfg.Core.Peer.RotationInterval > 0 && freshPeer.ExpiresAt == nil {
			expiresAt := time.Now().Add(m.cfg.Core.Peer.RotationInterval)
			freshPeer.ExpiresAt = &expiresAt
		}

		// Save immediately to reserve the assigned IPs so the next prepared peer gets the next free IPs
		if err := m.savePeers(ctx, freshPeer); err != nil {
			return nil, fmt.Errorf("failed to create new peer %s: %w", freshPeer.Identifier, err)
		}

		createdPeers = append(createdPeers, *freshPeer)

		m.bus.Publish(app.TopicPeerCreated, *freshPeer)
	}

	return createdPeers, nil
}

// UpdatePeer updates the given peer.
func (m Manager) UpdatePeer(ctx context.Context, peer *domain.Peer) (*domain.Peer, error) {
	existingPeer, err := m.db.GetPeer(ctx, peer.Identifier)
	if err != nil {
		return nil, fmt.Errorf("unable to load existing peer %s: %w", peer.Identifier, err)
	}

	if err := domain.ValidateUserAccessRights(ctx, existingPeer.UserIdentifier); err != nil {
		return nil, err
	}

	if err := m.checkInterfaceAccess(ctx, existingPeer.InterfaceIdentifier); err != nil {
		return nil, err
	}

	if err := m.validatePeerModifications(ctx, existingPeer, peer); err != nil {
		return nil, fmt.Errorf("update not allowed: %w", err)
	}

	sessionUser := domain.GetUserInfo(ctx)

	// if a peer is self provisioned, ensure that only allowed fields are set from the request
	if !sessionUser.IsAdmin {
		originalPeer, err := m.db.GetPeer(ctx, peer.Identifier)
		if err != nil {
			return nil, fmt.Errorf("unable to load existing peer %s: %w", peer.Identifier, err)
		}
		originalPeer.OverwriteUserEditableFields(peer, m.cfg)

		peer = originalPeer
	}

	// handle peer identifier change (new public key)
	if existingPeer.Identifier != domain.PeerIdentifier(peer.Interface.PublicKey) {
		peer.Identifier = domain.PeerIdentifier(peer.Interface.PublicKey) // set new identifier

		// check for already existing peer with new identifier
		duplicatePeer, err := m.db.GetPeer(ctx, peer.Identifier)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("unable to load existing peer %s: %w", peer.Identifier, err)
		}
		if duplicatePeer != nil {
			return nil, fmt.Errorf("peer %s already exists: %w", peer.Identifier, domain.ErrDuplicateEntry)
		}

		// delete old peer
		err = m.DeletePeer(ctx, existingPeer.Identifier)
		if err != nil {
			return nil, fmt.Errorf("failed to delete old peer %s for %s: %w",
				existingPeer.Identifier, peer.Identifier, err)
		}

		// save new peer
		err = m.savePeers(ctx, peer)
		if err != nil {
			return nil, fmt.Errorf("update failure for re-identified peer %s (was %s): %w",
				peer.Identifier, existingPeer.Identifier, err)
		}

		// publish event
		m.bus.Publish(app.TopicPeerIdentifierUpdated, existingPeer.Identifier, peer.Identifier)
	} else { // normal update
		err = m.savePeers(ctx, peer)
		if err != nil {
			return nil, fmt.Errorf("update failure: %w", err)
		}
	}

	// Clear persisted notifications whenever the new ExpiresAt supersedes the old
	// one, so emails are re-sent relative to the updated expiry
	if peer.ExpiresAt != nil {
		prev := existingPeer.ExpiresAt
		if prev == nil || peer.ExpiresAt.After(*prev) {
			if err := m.notifRepo.DeleteNotificationRecordsForPeer(ctx, peer.Identifier); err != nil {
				slog.WarnContext(ctx, "failed to clear notification records after ExpiresAt change",
					"peer", peer.Identifier,
					"error", err,
				)
			}
		}
	}

	m.bus.Publish(app.TopicPeerUpdated, *peer)

	return peer, nil
}

// DeletePeer deletes the peer with the given identifier.
func (m Manager) DeletePeer(ctx context.Context, id domain.PeerIdentifier) error {
	peer, err := m.db.GetPeer(ctx, id)
	if err != nil {
		return fmt.Errorf("unable to find peer %s: %w", id, err)
	}

	if err := domain.ValidateUserAccessRights(ctx, peer.UserIdentifier); err != nil {
		return err
	}

	if err := m.checkInterfaceAccess(ctx, peer.InterfaceIdentifier); err != nil {
		return err
	}

	if err := m.validatePeerDeletion(ctx, peer); err != nil {
		return fmt.Errorf("delete not allowed: %w", err)
	}

	iface, err := m.db.GetInterface(ctx, peer.InterfaceIdentifier)
	if err != nil {
		return fmt.Errorf("unable to find interface %s: %w", peer.InterfaceIdentifier, err)
	}

	// Delete notification records before removing the peer
	if err := m.notifRepo.DeleteNotificationRecordsForPeer(ctx, id); err != nil {
		slog.WarnContext(ctx, "failed to delete notification records for peer",
			"peer", id,
			"error", err,
		)
	}

	err = m.wg.GetController(*iface).DeletePeer(ctx, peer.InterfaceIdentifier, id)
	if err != nil {
		return fmt.Errorf("wireguard failed to delete peer %s: %w", id, err)
	}

	err = m.db.DeletePeer(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete peer %s: %w", id, err)
	}

	peers, err := m.db.GetInterfacePeers(ctx, iface.Identifier)
	if err != nil {
		return fmt.Errorf("failed to load peers for interface %s: %w", iface.Identifier, err)
	}

	m.bus.Publish(app.TopicPeerDeleted, *peer)
	// Update routes after peers have changed
	m.bus.Publish(app.TopicRouteUpdate, domain.RoutingTableInfo{
		Interface:  *iface,
		AllowedIps: iface.GetAllowedIPs(peers),
		FwMark:     iface.FirewallMark,
		Table:      iface.GetRoutingTable(),
		TableStr:   iface.RoutingTable,
	})
	// Update interface after peers have changed
	m.bus.Publish(app.TopicPeerInterfaceUpdated, peer.InterfaceIdentifier)

	return nil
}

// GetPeerStats returns the status of the peer with the given identifier.
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

// GetUserPeerStats returns the status of all peers for the given user.
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
	interfaces := make(map[domain.InterfaceIdentifier]domain.Interface)

	for _, peer := range peers {
		// get interface from db if it is not yet in the map
		if _, ok := interfaces[peer.InterfaceIdentifier]; !ok {
			iface, err := m.db.GetInterface(ctx, peer.InterfaceIdentifier)
			if err != nil {
				return fmt.Errorf("unable to find interface %s: %w", peer.InterfaceIdentifier, err)
			}
			interfaces[peer.InterfaceIdentifier] = *iface
		}

		iface := interfaces[peer.InterfaceIdentifier]

		// Always save the peer to the backend, regardless of disabled/expired state
		// The backend will handle the disabled state appropriately
		err := m.db.SavePeer(ctx, peer.Identifier, func(p *domain.Peer) (*domain.Peer, error) {
			peer.CopyCalculatedAttributes(p)

			err := m.wg.GetController(iface).SavePeer(ctx, peer.InterfaceIdentifier, peer.Identifier,
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

		// publish event

		m.bus.Publish(app.TopicAuditPeerChanged, domain.AuditEventWrapper[audit.PeerEvent]{
			Ctx: ctx,
			Event: audit.PeerEvent{
				Action: "save",
				Peer:   *peer,
			},
		})
	}

	// Update routes after peers have changed
	for id, iface := range interfaces {
		interfacePeers, err := m.db.GetInterfacePeers(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to re-load peers for interface %s: %w", id, err)
		}

		m.bus.Publish(app.TopicRouteUpdate, domain.RoutingTableInfo{
			Interface:  iface,
			AllowedIps: iface.GetAllowedIPs(interfacePeers),
			FwMark:     iface.FirewallMark,
			Table:      iface.GetRoutingTable(),
			TableStr:   iface.RoutingTable,
		})
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

func (m Manager) validatePeerModifications(ctx context.Context, _, _ *domain.Peer) error {
	currentUser := domain.GetUserInfo(ctx)

	if !currentUser.IsAdmin && !m.cfg.Core.SelfProvisioningAllowed {
		return domain.ErrNoPermission
	}

	return nil
}

func (m Manager) validatePeerCreation(ctx context.Context, _, new *domain.Peer) error {
	currentUser := domain.GetUserInfo(ctx)

	if new.Identifier == "" {
		return fmt.Errorf("invalid peer identifier: %w", domain.ErrInvalidData)
	}

	if !currentUser.IsAdmin && !m.cfg.Core.SelfProvisioningAllowed {
		return domain.ErrNoPermission
	}

	_, err := m.db.GetInterface(ctx, new.InterfaceIdentifier)
	if err != nil {
		return fmt.Errorf("invalid interface: %w", domain.ErrInvalidData)
	}

	return nil
}

func (m Manager) validatePeerDeletion(ctx context.Context, _ *domain.Peer) error {
	currentUser := domain.GetUserInfo(ctx)

	if !currentUser.IsAdmin && !m.cfg.Core.SelfProvisioningAllowed {
		return domain.ErrNoPermission
	}

	return nil
}

func (m Manager) checkInterfaceAccess(ctx context.Context, id domain.InterfaceIdentifier) error {
	user := domain.GetUserInfo(ctx)
	if user.IsAdmin {
		return nil
	}

	iface, err := m.db.GetInterface(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get interface %s: %w", id, err)
	}

	if !iface.IsUserAllowed(user.Id, m.cfg) {
		return fmt.Errorf("user %s is not allowed to access interface %s: %w", user.Id, id, domain.ErrNoPermission)
	}

	return nil
}

func (m Manager) prepareDefaultPeer(ctx context.Context, iface *domain.Interface, user *domain.User) (
	*domain.Peer,
	error,
) {
	if !iface.CreateDefaultPeers() || !user.CreateDefaultPeers() {
		return nil, nil
	}

	userPeers, err := m.db.GetUserPeers(ctx, user.Identifier)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve existing peers prior to default peer creation: %w", err)
	}

	peerAlreadyCreated := slices.ContainsFunc(userPeers, func(peer domain.Peer) bool {
		// Ignore the AutomaticallyCreated flag on the peer.
		// If a user already has a peer for a given interface, no default peer should be created.
		return peer.InterfaceIdentifier == iface.Identifier
	})
	if peerAlreadyCreated {
		return nil, nil // skip creation if a peer already exists for this interface
	}

	peer, err := m.PreparePeer(ctx, iface.Identifier)
	if err != nil {
		return nil, fmt.Errorf("failed to create default peer for interface %s: %w", iface.Identifier, err)
	}

	peer.UserIdentifier = user.Identifier
	peer.Notes = fmt.Sprintf("Default peer created for user %s", user.Identifier)
	peer.AutomaticallyCreated = true
	peer.GenerateDisplayName("Default")

	return peer, nil
}

// endregion helper-functions
