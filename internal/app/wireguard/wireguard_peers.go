package wireguard

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/fedor-git/wg-portal-2/internal/app"
	"github.com/fedor-git/wg-portal-2/internal/app/audit"
	"github.com/fedor-git/wg-portal-2/internal/domain"
)

// CreateDefaultPeer creates a default peer for the given user on all server interfaces.
func (m Manager) CreateDefaultPeer(ctx context.Context, userId domain.UserIdentifier) error {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return err
	}

	existingInterfaces, err := m.db.GetAllInterfaces(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch all interfaces: %w", err)
	}

	userPeers, err := m.db.GetUserPeers(context.Background(), userId)
	if err != nil {
		return fmt.Errorf("failed to retrieve existing peers prior to default peer creation: %w", err)
	}

	var newPeers []domain.Peer
	for _, iface := range existingInterfaces {
		if iface.Type != domain.InterfaceTypeServer {
			continue // only create default peers for server interfaces
		}

		peerAlreadyCreated := slices.ContainsFunc(userPeers, func(peer domain.Peer) bool {
			return peer.InterfaceIdentifier == iface.Identifier
		})
		if peerAlreadyCreated {
			continue // skip creation if a peer already exists for this interface
		}

		peer, err := m.PreparePeer(ctx, iface.Identifier)
		if err != nil {
			return fmt.Errorf("failed to create default peer for interface %s: %w", iface.Identifier, err)
		}

		peer.UserIdentifier = userId
		peer.Notes = fmt.Sprintf("Default peer created for user %s", userId)
		peer.AutomaticallyCreated = true
		peer.GenerateDisplayName("Default")

		newPeers = append(newPeers, *peer)
	}

	for i, peer := range newPeers {
		_, err := m.CreatePeer(ctx, &newPeers[i])
		if err != nil {
			return fmt.Errorf("failed to create default peer %s on interface %s: %w",
				peer.Identifier, peer.InterfaceIdentifier, err)
		}
	}

	slog.InfoContext(ctx, "created default peers for user",
		"user", userId,
		"count", len(newPeers))

	return nil
}

// GetUserPeers returns all peers for the given user.
func (m Manager) GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error) {
	if err := domain.ValidateUserAccessRights(ctx, id); err != nil {
		return nil, err
	}

	peers, err := m.db.GetUserPeers(ctx, id)
	if err != nil {
		return nil, err
	}

	// For API response: show AllowedIPs from interface config
	// Get all interfaces to map InterfaceIdentifier -> PeerDefAllowedIPsStr
	interfaces, err := m.db.GetAllInterfaces(ctx)
	if err == nil {
		ifaceMap := make(map[domain.InterfaceIdentifier]string)
		for _, iface := range interfaces {
			ifaceMap[iface.Identifier] = iface.PeerDefAllowedIPsStr
		}

		for i := range peers {
			if peers[i].Interface.Type == domain.InterfaceTypeClient {
				if allowedIPs, ok := ifaceMap[peers[i].InterfaceIdentifier]; ok {
					peers[i].AllowedIPsStr.Value = allowedIPs
				}
			}
		}
	}

	return peers, nil
}

// GetPeersByDisplayName retrieves all peers matching the given DisplayName
func (m Manager) GetPeersByDisplayName(ctx context.Context, displayName string) ([]domain.Peer, error) {
	peers, err := m.db.GetPeersByDisplayName(ctx, displayName)
	if err != nil {
		return nil, err
	}

	// For API response: show AllowedIPs from interface config
	// Get all interfaces to map InterfaceIdentifier -> PeerDefAllowedIPsStr
	interfaces, err := m.db.GetAllInterfaces(ctx)
	if err == nil {
		ifaceMap := make(map[domain.InterfaceIdentifier]string)
		for _, iface := range interfaces {
			ifaceMap[iface.Identifier] = iface.PeerDefAllowedIPsStr
		}

		for i := range peers {
			if peers[i].Interface.Type == domain.InterfaceTypeClient {
				if allowedIPs, ok := ifaceMap[peers[i].InterfaceIdentifier]; ok {
					peers[i].AllowedIPsStr.Value = allowedIPs
				}
			}
		}
	}

	return peers, nil
}

// PreparePeer prepares a new peer for the given interface with fresh keys and ip addresses.
func (m Manager) PreparePeer(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Peer, error) {
	if !m.cfg.Core.SelfProvisioningAllowed {
		if err := domain.ValidateAdminAccessRights(ctx); err != nil {
			return nil, err
		}
	}

	currentUser := domain.GetUserInfo(ctx)

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

	// For WireGuard kernel: use client's IP addresses (prevents overlapping AllowedIPs)
	// Convert peer addresses to /32 (IPv4) or /128 (IPv6) host addresses
	hostAddrs := make([]domain.Cidr, len(ips))
	for i, ip := range ips {
		hostAddrs[i] = ip.HostAddr()
	}
	peerAllowedIPs := domain.CidrsToString(hostAddrs)

	freshPeer := &domain.Peer{
		BaseModel: domain.BaseModel{
			CreatedBy: string(currentUser.Id),
			UpdatedBy: string(currentUser.Id),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Endpoint:            domain.NewConfigOption(iface.PeerDefEndpoint, true),
		EndpointPublicKey:   domain.NewConfigOption(iface.PublicKey, true),
		AllowedIPsStr:       domain.NewConfigOption(peerAllowedIPs, true),
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

	// For API response: show AllowedIPs from interface config (e.g., 0.0.0.0/0)
	// instead of actual WireGuard kernel value (client's IP like 192.168.10.2/32)
	iface, err := m.db.GetInterface(ctx, peer.InterfaceIdentifier)
	slog.Debug("GetPeer before override",
		"peer_id", peer.Identifier,
		"peer_type", peer.Interface.Type,
		"allowed_ips_before", peer.AllowedIPsStr.Value,
		"iface_err", err)

	if err == nil && peer.Interface.Type == domain.InterfaceTypeClient {
		slog.Debug("GetPeer overriding AllowedIPsStr",
			"peer_id", peer.Identifier,
			"old_value", peer.AllowedIPsStr.Value,
			"new_value", iface.PeerDefAllowedIPsStr)
		peer.AllowedIPsStr.Value = iface.PeerDefAllowedIPsStr
	}

	slog.Debug("GetPeer after override",
		"peer_id", peer.Identifier,
		"allowed_ips_after", peer.AllowedIPsStr.Value)

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
	}

	sessionUser := domain.GetUserInfo(ctx)

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

	// Retry logic for unique constraint violations on IP addresses
	// Uses sequence-based IP allocation: on conflict, allocate next IP and retry
	// No locks - maximum parallelism
	maxRetries := 5
	var saveErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		saveErr = m.savePeers(ctx, peer)
		if saveErr == nil {
			break // Success
		}

		// Check if this is a unique constraint violation (IP already taken)
		if strings.Contains(saveErr.Error(), "Duplicate entry") ||
			strings.Contains(saveErr.Error(), "UNIQUE constraint") ||
			strings.Contains(saveErr.Error(), "unique constraint") {
			if attempt < maxRetries {
				slog.DebugContext(ctx, "IP allocation conflict - retrying with next IP",
					"peer_id", peer.Identifier,
					"attempt", attempt+1,
					"max_retries", maxRetries)

				// Get fresh IP addresses for all subnets
				iface, ifaceErr := m.db.GetInterface(ctx, peer.InterfaceIdentifier)
				if ifaceErr != nil {
					return nil, fmt.Errorf("failed to get interface after IP conflict: %w", ifaceErr)
				}

				// Allocate next IPs using sequence-based approach (no locks)
				networks, _ := domain.CidrsFromString(iface.PeerDefNetworkStr)
				newIps := make([]domain.Cidr, 0, len(networks))

				for _, network := range networks {
					nextIP, ipErr := m.db.GetNextPeerIPForSubnet(ctx, network)
					if ipErr != nil {
						return nil, fmt.Errorf("failed to allocate IP for subnet %s: %w", network.String(), ipErr)
					}
					newIps = append(newIps, nextIP)
				}

				peer.Interface.Addresses = newIps
				continue
			}
		}

		// For other errors, return immediately
		return nil, fmt.Errorf("creation failure: %w", saveErr)
	}

	if saveErr != nil {
		return nil, fmt.Errorf("creation failure: max retries exceeded: %w", saveErr)
	}

	m.bus.Publish(app.TopicPeerCreated, *peer) // Webhooks receive full peer

	// Publish event-driven sync event with timestamp for delay monitoring
	slog.Info("[PEER_CREATE_SYNC] publishing peer created event for other nodes",
		"peer_id", peer.Identifier,
		"interface", peer.InterfaceIdentifier,
		"timestamp_unix_ns", time.Now().UnixNano())
	m.bus.Publish(app.TopicPeerCreatedSync, peer.Identifier) // Other nodes receive only ID for event-driven sync

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

		// Save with retry logic for unique constraint violations on IP addresses
		// Uses sequence-based IP allocation: on conflict, allocate next IP and retry
		// No locks - maximum parallelism
		maxRetries := 5
		var saveErr error
		for attempt := 0; attempt <= maxRetries; attempt++ {
			saveErr = m.savePeers(ctx, freshPeer)
			if saveErr == nil {
				break // Success
			}

			// Check if this is a unique constraint violation (IP already taken)
			if strings.Contains(saveErr.Error(), "Duplicate entry") ||
				strings.Contains(saveErr.Error(), "UNIQUE constraint") ||
				strings.Contains(saveErr.Error(), "unique constraint") {
				if attempt < maxRetries {
					slog.DebugContext(ctx, "IP allocation conflict in CreateMultiplePeers - retrying with next IP",
						"peer_id", freshPeer.Identifier,
						"user_id", id,
						"attempt", attempt+1,
						"max_retries", maxRetries)

					// Get fresh interface config
					iface, ifaceErr := m.db.GetInterface(ctx, interfaceId)
					if ifaceErr != nil {
						return nil, fmt.Errorf("failed to get interface after IP conflict: %w", ifaceErr)
					}

					// Allocate next IPs (no locks, sequence-based)
					networks, _ := domain.CidrsFromString(iface.PeerDefNetworkStr)
					newIps := make([]domain.Cidr, 0, len(networks))

					for _, network := range networks {
						nextIP, ipErr := m.db.GetNextPeerIPForSubnet(ctx, network)
						if ipErr != nil {
							return nil, fmt.Errorf("failed to allocate IP for subnet %s: %w", network.String(), ipErr)
						}
						newIps = append(newIps, nextIP)
					}

					freshPeer.Interface.Addresses = newIps
					continue
				}
			}

			// For other errors, return immediately
			return nil, fmt.Errorf("failed to create new peer %s: %w", freshPeer.Identifier, saveErr)
		}

		if saveErr != nil {
			return nil, fmt.Errorf("failed to create peer %s: max retries exceeded: %w", freshPeer.Identifier, saveErr)
		}

		createdPeers = append(createdPeers, *freshPeer)

		m.bus.Publish(app.TopicPeerCreated, *freshPeer)
		m.bus.Publish(app.TopicPeerCreatedSync, freshPeer.Identifier)
	}

	return createdPeers, nil
}

// hasMeaningfulChanges checks if peer change is meaningful for cluster sync.
// Returns true only if WireGuard-relevant fields changed, not just timestamps.
// This prevents cluster-wide syncs when only internal timestamps are updated.
func hasMeaningfulChanges(oldPeer, newPeer *domain.Peer) bool {
	// Check WireGuard-relevant fields
	if oldPeer.AllowedIPsStr != newPeer.AllowedIPsStr {
		return true
	}
	if oldPeer.ExtraAllowedIPsStr != newPeer.ExtraAllowedIPsStr {
		return true
	}
	if oldPeer.Endpoint != newPeer.Endpoint {
		return true
	}
	if oldPeer.EndpointPublicKey != newPeer.EndpointPublicKey {
		return true
	}
	if oldPeer.PresharedKey != newPeer.PresharedKey {
		return true
	}
	if oldPeer.PersistentKeepalive != newPeer.PersistentKeepalive {
		return true
	}

	// Check Portal-relevant state changes
	if (oldPeer.Disabled == nil) != (newPeer.Disabled == nil) {
		return true // Disabled state changed
	}
	if oldPeer.Disabled != nil && newPeer.Disabled != nil && !oldPeer.Disabled.Equal(*newPeer.Disabled) {
		return true // Disabled time changed
	}
	if oldPeer.DisabledReason != newPeer.DisabledReason {
		return true
	}
	// NOTE: We do NOT check ExpiresAt for sync
	// Reason: ExpiresAt is a local TTL timer for cleanup, not peer configuration
	// Each node may have different expiration times; sync is not needed
	// TTL changes (peer disconnect/connect) should NOT trigger cluster-wide syncs

	if oldPeer.DisplayName != newPeer.DisplayName {
		return true
	}
	if oldPeer.Notes != newPeer.Notes {
		return true
	}

	// Check Interface settings - compare key fields that matter for WireGuard config
	if oldPeer.Interface.CheckAliveAddress != newPeer.Interface.CheckAliveAddress {
		return true
	}
	if oldPeer.Interface.DnsStr != newPeer.Interface.DnsStr {
		return true
	}
	if oldPeer.Interface.DnsSearchStr != newPeer.Interface.DnsSearchStr {
		return true
	}
	if oldPeer.Interface.Mtu != newPeer.Interface.Mtu {
		return true
	}
	if oldPeer.Interface.FirewallMark != newPeer.Interface.FirewallMark {
		return true
	}
	if oldPeer.Interface.RoutingTable != newPeer.Interface.RoutingTable {
		return true
	}
	if oldPeer.Interface.PreUp != newPeer.Interface.PreUp {
		return true
	}
	if oldPeer.Interface.PostUp != newPeer.Interface.PostUp {
		return true
	}
	if oldPeer.Interface.PreDown != newPeer.Interface.PreDown {
		return true
	}
	if oldPeer.Interface.PostDown != newPeer.Interface.PostDown {
		return true
	}
	// Compare public key
	if oldPeer.Interface.PublicKey != newPeer.Interface.PublicKey {
		return true
	}
	if oldPeer.Interface.Type != newPeer.Interface.Type {
		return true
	}
	// Compare addresses - need custom comparison due to slice
	if len(oldPeer.Interface.Addresses) != len(newPeer.Interface.Addresses) {
		return true
	}
	for i, oldAddr := range oldPeer.Interface.Addresses {
		if i >= len(newPeer.Interface.Addresses) || oldAddr != newPeer.Interface.Addresses[i] {
			return true
		}
	}

	// If we reach here, only internal timestamps changed (UpdatedAt, UpdatedBy)
	return false
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

	m.bus.Publish(app.TopicPeerUpdated, *peer) // Webhooks receive full peer

	// CRITICAL: Only publish sync event if something meaningful changed
	// If only internal timestamps (UpdatedAt, UpdatedBy) changed, don't trigger cluster-wide sync
	if hasMeaningfulChanges(existingPeer, peer) {
		slog.Info("[PEER_UPDATE_SYNC] publishing peer updated event for other nodes",
			"peer_id", peer.Identifier,
			"interface", peer.InterfaceIdentifier,
			"timestamp_unix_ns", time.Now().UnixNano())
		m.bus.Publish(app.TopicPeerUpdatedSync, peer.Identifier) // Other nodes receive only ID for event-driven sync
	} else {
		slog.Debug("[PEER_UPDATE_SYNC] skipping sync event - only internal timestamps changed",
			"peer_id", peer.Identifier,
			"interface", peer.InterfaceIdentifier)
	}

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

	if err := m.validatePeerDeletion(ctx, peer); err != nil {
		return fmt.Errorf("delete not allowed: %w", err)
	}

	iface, err := m.db.GetInterface(ctx, peer.InterfaceIdentifier)
	if err != nil {
		return fmt.Errorf("unable to find interface %s: %w", peer.InterfaceIdentifier, err)
	}

	err = m.wg.GetController(*iface).DeletePeer(ctx, peer.InterfaceIdentifier, id)
	if err != nil {
		return fmt.Errorf("wireguard failed to delete peer %s: %w", id, err)
	}

	err = m.db.DeletePeer(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete peer %s: %w", id, err)
	}

	m.bus.Publish(app.TopicPeerDeleted, *peer) // Webhooks receive full peer

	// Publish event-driven sync event with timestamp for delay monitoring
	slog.Info("[PEER_DELETE_SYNC] publishing peer deleted event for other nodes",
		"peer_id", peer.Identifier,
		"interface", peer.InterfaceIdentifier,
		"timestamp_unix_ns", time.Now().UnixNano())
	m.bus.Publish(app.TopicPeerDeletedSync, peer.Identifier) // Other nodes receive only ID for event-driven sync
	// Update routes after peers have changed
	m.bus.Publish(app.TopicRouteUpdate, "peers updated")
	// Update interface after peers have changed
	// m.bus.Publish(app.TopicPeerInterfaceUpdated, peer.InterfaceIdentifier)

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
	interfaces := make(map[domain.InterfaceIdentifier]struct{})

	for _, peer := range peers {
		iface, err := m.db.GetInterface(ctx, peer.InterfaceIdentifier)
		if err != nil {
			return fmt.Errorf("unable to find interface %s: %w", peer.InterfaceIdentifier, err)
		}

		// Always save the peer to the backend, regardless of disabled/expired state
		// The backend will handle the disabled state appropriately
		err = m.db.SavePeer(ctx, peer.Identifier, func(p *domain.Peer) (*domain.Peer, error) {
			peer.CopyCalculatedAttributes(p)

			err := m.wg.GetController(*iface).SavePeer(ctx, peer.InterfaceIdentifier, peer.Identifier,
				func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error) {
					domain.MergeToPhysicalPeer(pp, peer, m.cfg.Core.ForceClientIPAsAllowedIP)
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

		slog.Debug("savePeers: adding peer to wg0.conf", "iface", peer.InterfaceIdentifier, "peer", peer.Identifier)

		// publish event

		m.bus.Publish(app.TopicAuditPeerChanged, domain.AuditEventWrapper[audit.PeerEvent]{
			Ctx: ctx,
			Event: audit.PeerEvent{
				Action: "save",
				Peer:   *peer,
			},
		})

		interfaces[peer.InterfaceIdentifier] = struct{}{}
	}

	// Update routes after peers have changed
	if len(interfaces) != 0 {
		m.bus.Publish(app.TopicRouteUpdate, "peers updated")
	}

	// NOTE: Do NOT publish TopicPeerInterfaceUpdated here
	// Individual peer saves should NOT trigger full interface resync
	// - For new peers: TopicPeerCreatedSync (single-peer event) handles sync via handlePeerCreatedSyncEvent()
	// - For peer updates: Individual peer updates don't require interface-wide resync
	// - TopicPeerInterfaceUpdated should only be published for interface config changes (address, enabled state, etc)

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

// endregion helper-functions
