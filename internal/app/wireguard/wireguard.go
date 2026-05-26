package wireguard

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/fedor-git/wg-portal-2/internal/app"
	"github.com/fedor-git/wg-portal-2/internal/config"
	"github.com/fedor-git/wg-portal-2/internal/domain"
	// no need to import wireguard here; StatisticsCollector is in the same package
)

// region dependencies

type InterfaceAndPeerDatabaseRepo interface {
	GetInterface(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, error)
	GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error)
	GetPeersStats(ctx context.Context, ids ...domain.PeerIdentifier) ([]domain.PeerStatus, error)
	GetAllInterfaces(ctx context.Context) ([]domain.Interface, error)
	GetInterfaceIps(ctx context.Context) (map[domain.InterfaceIdentifier][]domain.Cidr, error)
	SaveInterface(
		ctx context.Context,
		id domain.InterfaceIdentifier,
		updateFunc func(in *domain.Interface) (*domain.Interface, error),
	) error
	DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error
	GetInterfacePeers(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.Peer, error)
	GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error)
	SavePeer(
		ctx context.Context,
		id domain.PeerIdentifier,
		updateFunc func(in *domain.Peer) (*domain.Peer, error),
	) error
	DeletePeer(ctx context.Context, id domain.PeerIdentifier) error
	GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error)
	GetPeersByDisplayName(ctx context.Context, displayName string) ([]domain.Peer, error)
	GetUsedIpsPerSubnet(ctx context.Context, subnets []domain.Cidr) (map[domain.Cidr][]domain.Cidr, error)
	GetNextPeerIPForSubnet(ctx context.Context, subnet domain.Cidr) (domain.Cidr, error)
	SyncAllPeersFromDB(ctx context.Context) (int, error) // Synchronize all peers from the database

	// Event-driven sync methods
	FindAndDeleteExpiredPeersWithLock(ctx context.Context, nodeID string) ([]string, error) // Returns IDs of deleted peers
	GetExpiredPeers(ctx context.Context) ([]domain.Peer, error)                             // Find expired peers
}

type WgQuickController interface {
	ExecuteInterfaceHook(id domain.InterfaceIdentifier, hookCmd string) error
	SetDNS(id domain.InterfaceIdentifier, dnsStr, dnsSearchStr string) error
	UnsetDNS(id domain.InterfaceIdentifier) error
}

type EventBus interface {
	// Publish sends a message to the message bus.
	Publish(topic string, args ...any)
	// Subscribe subscribes to a topic
	Subscribe(topic string, fn interface{}) error
}

// endregion dependencies

type Manager struct {
	cfg             *config.Config
	bus             EventBus
	db              InterfaceAndPeerDatabaseRepo
	wg              *ControllerManager
	quick           WgQuickController
	statsCollector  *StatisticsCollector
	userLockMap     *sync.Map
	startupComplete chan struct{} // Signals when startup peer loading complete
}

func NewWireGuardManager(
	cfg *config.Config,
	bus EventBus,
	wg *ControllerManager,
	quick WgQuickController,
	db InterfaceAndPeerDatabaseRepo,
	statsCollector *StatisticsCollector,
) (*Manager, error) {
	m := &Manager{
		cfg:             cfg,
		bus:             bus,
		wg:              wg,
		db:              db,
		quick:           quick,
		statsCollector:  statsCollector,
		userLockMap:     &sync.Map{},
		startupComplete: make(chan struct{}),
	}

	m.connectToMessageBus()

	return m, nil
}

// StartBackgroundJobs starts background jobs.
// Event-driven sync (via TopicPeerCreatedSync, TopicPeerUpdatedSync, TopicPeerDeletedSync)
// handles peer synchronization across nodes - NO periodic full syncs needed.
func (m Manager) StartBackgroundJobs(ctx context.Context) {
	// Peer loading on startup is now handled by RestoreInterfaceState in main.go (if SyncOnStartup enabled)
	// This function just signals completion to unblock fanout events

	// Metrics registration will be handled by statistics collector initialization (10 sec delay)
	// or can be added explicitly here if needed for immediate metric availability

	// Signal that manager is ready - this unblocks fanout from sending events
	close(m.startupComplete)
}

// Periodic full sync completely removed:
// - Instead use event-driven sync: when peer is created/updated/deleted,
//   only that specific peer is synced across nodes
// - No runExpiredPeersCheck() - peers expire based on database TTL,
//   master node handles cleanup when it processes sync events
// - No initializePeerTTL() - use database triggers for TTL management

// IsStartupComplete returns true if startup peer loading finished.
func (m Manager) IsStartupComplete() bool {
	select {
	case <-m.startupComplete:
		return true
	default:
		return false
	}
}

// StartExpiredPeersCheckAfterServer starts the expired peers cleanup loop
// This MUST be called AFTER the web server is fully initialized and ready to accept requests
// Called from main() to ensure cluster coordination is ready before deleting peers
func (m Manager) StartExpiredPeersCheckAfterServer(ctx context.Context) {
	// Start expiry check loop ONLY on master node
	// Non-master nodes skip this entirely to avoid unnecessary loops
	if m.cfg.Core.Master {
		go m.runExpiredPeersCheck(ctx)
	}
}

// registerAllPeerMetricsAtStartup registers metrics for all loaded peers at startup (ONCE)
// Called after SyncAllPeersFromDB() to ensure all peers have their metrics initialized
// This happens only ONCE on startup, preventing expensive re-registration during stats collection
func (m Manager) registerAllPeerMetricsAtStartup(ctx context.Context) error {
	if m.statsCollector == nil || m.statsCollector.ms == nil {
		return nil // No metrics server, nothing to register
	}

	// If only_export_connected_peers is enabled, skip initial registration
	// Metrics will be registered dynamically when peers connect
	if m.cfg.Statistics.OnlyExportConnectedPeers {
		slog.Info("[METRICS_STARTUP] skipping initial peer metrics registration - only_export_connected_peers enabled, metrics will be registered dynamically")
		return nil
	}

	// Get all interfaces
	interfaces, err := m.db.GetAllInterfaces(ctx)
	if err != nil {
		return fmt.Errorf("failed to get interfaces for metrics registration: %w", err)
	}

	registeredCount := 0
	for _, iface := range interfaces {
		// Get all peers for this interface
		peers, err := m.db.GetInterfacePeers(ctx, iface.Identifier)
		if err != nil {
			slog.Warn("[METRICS_STARTUP] failed to get interface peers for metrics",
				"interface", iface.Identifier, "error", err)
			continue
		}

		// Register metrics for each peer
		for _, peer := range peers {
			if !peer.IsDisabled() {
				m.statsCollector.ms.RegisterPeerMetrics(&peer)
				registeredCount++
			}
		}
	}

	slog.Info("[METRICS_STARTUP] registered peer metrics at startup",
		"peers", registeredCount)
	return nil
}

// ConnectToMessageBus subscribes to event bus topics for peer synchronization
// Result:
// - No more CPU spike from GetAllPeers() contention (666 peers on every sync)
// - No more debounce delays - sync happens immediately on API create/update/delete
// - Fanout only handles interface topology changes, not peer changes
func (m Manager) connectToMessageBus() {
	_ = m.bus.Subscribe(app.TopicUserCreated, m.handleUserCreationEvent)
	_ = m.bus.Subscribe(app.TopicAuthLogin, m.handleUserLoginEvent)
	_ = m.bus.Subscribe(app.TopicUserDisabled, m.handleUserDisabledEvent)
	_ = m.bus.Subscribe(app.TopicUserEnabled, m.handleUserEnabledEvent)
	_ = m.bus.Subscribe(app.TopicPeerStateChanged, m.handlePeerStateChangeEvent)
	_ = m.bus.Subscribe(app.TopicUserDeleted, m.handleUserDeletedEvent)
	_ = m.bus.Subscribe(app.TopicPeerInterfaceUpdated, m.handlePeerInterfaceUpdatedEvent)
	_ = m.bus.Subscribe(app.TopicPeersExpiredRemoved, m.handlePeersExpiredRemovedEvent)

	// Event-driven peer synchronization across nodes
	_ = m.bus.Subscribe(app.TopicPeerCreatedSync, m.handlePeerCreatedSyncEvent)
	_ = m.bus.Subscribe(app.TopicPeerUpdatedSync, m.handlePeerUpdatedSyncEvent)
	_ = m.bus.Subscribe(app.TopicPeerDeletedSync, m.handlePeerDeletedSyncEvent)

	// Local peer sync events from HTTP endpoint (breaks fanout feedback loop)
	_ = m.bus.Subscribe(app.TopicPeerSyncedLocal, m.handlePeerSyncedLocalEvent)
}

func (m Manager) handleUserCreationEvent(user domain.User) {
	if !m.cfg.Core.CreateDefaultPeerOnCreation {
		return
	}

	_, loaded := m.userLockMap.LoadOrStore(user.Identifier, "create")
	if loaded {
		return // another goroutine is already handling this user
	}
	defer m.userLockMap.Delete(user.Identifier)

	slog.Debug("handling new user event", "user", user.Identifier)

	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
	err := m.CreateDefaultPeer(ctx, user.Identifier)
	if err != nil {
		slog.Error("failed to create default peer", "user", user.Identifier, "error", err)
		return
	}
}

func (m Manager) handleUserLoginEvent(userId domain.UserIdentifier) {
	if !m.cfg.Core.CreateDefaultPeer {
		return
	}

	_, loaded := m.userLockMap.LoadOrStore(userId, "login")
	if loaded {
		return // another goroutine is already handling this user
	}
	defer m.userLockMap.Delete(userId)

	userPeers, err := m.db.GetUserPeers(context.Background(), userId)
	if err != nil {
		slog.Error("failed to retrieve existing peers prior to default peer creation",
			"user", userId,
			"error", err)
		return
	}

	if len(userPeers) > 0 {
		return // user already has peers, skip creation
	}

	slog.Debug("handling new user login", "user", userId)

	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
	err = m.CreateDefaultPeer(ctx, userId)
	if err != nil {
		slog.Error("failed to create default peer", "user", userId, "error", err)
		return
	}
}

func (m Manager) handleUserDisabledEvent(user domain.User) {
	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
	userPeers, err := m.db.GetUserPeers(ctx, user.Identifier)
	if err != nil {
		slog.Error("failed to retrieve peers for disabled user",
			"user", user.Identifier,
			"error", err)
		return
	}

	for _, peer := range userPeers {
		if peer.IsDisabled() {
			continue // peer is already disabled
		}

		slog.Debug("disabling peer due to user being disabled",
			"peer", peer.Identifier,
			"user", user.Identifier)

		peer.Disabled = user.Disabled // set to user disabled timestamp
		peer.DisabledReason = domain.DisabledReasonUserDisabled

		_, err := m.UpdatePeer(ctx, &peer)
		if err != nil {
			slog.Error("failed to disable peer for disabled user",
				"peer", peer.Identifier,
				"user", user.Identifier,
				"error", err)
		}
	}
}

func (m Manager) handleUserEnabledEvent(user domain.User) {
	if !m.cfg.Core.ReEnablePeerAfterUserEnable {
		return
	}

	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
	userPeers, err := m.db.GetUserPeers(ctx, user.Identifier)
	if err != nil {
		slog.Error("failed to retrieve peers for re-enabled user",
			"user", user.Identifier,
			"error", err)
		return
	}

	for _, peer := range userPeers {
		if !peer.IsDisabled() {
			continue // peer is already active
		}

		if peer.DisabledReason != domain.DisabledReasonUserDisabled {
			continue // peer was disabled for another reason
		}

		slog.Debug("enabling peer due to user being enabled",
			"peer", peer.Identifier,
			"user", user.Identifier)

		peer.Disabled = nil
		peer.DisabledReason = ""

		_, err := m.UpdatePeer(ctx, &peer)
		if err != nil {
			slog.Error("failed to enable peer for enabled user",
				"peer", peer.Identifier,
				"user", user.Identifier,
				"error", err)
		}
	}
	return
}

func (m Manager) handleUserDeletedEvent(user domain.User) {
	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
	userPeers, err := m.db.GetUserPeers(ctx, user.Identifier)
	if err != nil {
		slog.Error("failed to retrieve peers for deleted user",
			"user", user.Identifier,
			"error", err)
		return
	}

	deletionTime := time.Now()
	for _, peer := range userPeers {
		if peer.IsDisabled() {
			continue // peer is already disabled
		}

		if m.cfg.Core.DeletePeerAfterUserDeleted {
			slog.Debug("deleting peer due to user being deleted",
				"peer", peer.Identifier,
				"user", user.Identifier)

			if err := m.DeletePeer(ctx, peer.Identifier); err != nil {
				slog.Error("failed to delete peer for deleted user",
					"peer", peer.Identifier,
					"user", user.Identifier,
					"error", err)
			}
		} else {
			slog.Debug("disabling peer due to user being deleted",
				"peer", peer.Identifier,
				"user", user.Identifier)

			peer.UserIdentifier = "" // remove user reference
			peer.Disabled = &deletionTime
			peer.DisabledReason = domain.DisabledReasonUserDeleted

			_, err := m.UpdatePeer(ctx, &peer)
			if err != nil {
				slog.Error("failed to disable peer for deleted user",
					"peer", peer.Identifier,
					"user", user.Identifier,
					"error", err)
			}
		}
	}
}

func (m Manager) runExpiredPeersCheck(ctx context.Context) {
	// OPTIMIZATION: Only run expiry check on MASTER node
	// Non-master nodes skip this entire loop to avoid unnecessary CPU/checks
	// Master node has exclusive database lock for peer deletion
	if !m.cfg.Core.Master {
		slog.Debug("[EXPIRE_CLEANUP] this node is not master, skipping expiry check loop")
		return
	}

	// Get nodeID from config (hostname fallback to default)
	nodeID := m.cfg.Core.ClusterNodeId
	if nodeID == "" {
		nodeID = "unknown-node"
	}

	slog.Info("[EXPIRE_CLEANUP] starting expiry check loop on master node", "node_id", nodeID, "interval", m.cfg.Advanced.ExpiryCheckInterval)

	// Run expiry check immediately on startup with background context
	// Use Background() for startup check to avoid context cancellation during first check
	dbCtx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
	expiredPeerIDs, err := m.db.FindAndDeleteExpiredPeersWithLock(dbCtx, nodeID)
	if err != nil {
		// Under high load during startup, defer cleanup to first periodic check
		if strings.Contains(err.Error(), "lock") || strings.Contains(err.Error(), "timeout") {
			slog.WarnContext(dbCtx, "[EXPIRE_CLEANUP] high load at startup - deferring expired peer cleanup",
				"node_id", nodeID, "reason", err.Error())
		} else {
			slog.ErrorContext(dbCtx, "[EXPIRE_CLEANUP] failed to find and delete expired peers at startup",
				"node_id", nodeID, "error", err)
		}
	} else if len(expiredPeerIDs) > 0 {
		slog.Info("[EXPIRE_CLEANUP] found and deleted expired peers at startup", "count", len(expiredPeerIDs), "node_id", nodeID)
		// Publish batch event for all nodes to handle cleanup locally
		m.bus.Publish(app.TopicPeersExpiredRemoved, expiredPeerIDs)
		// Publish individual delete-sync events so other systems get notified (same as API delete)
		// Use small batches to avoid event bus queue overflow (100 items)
		for i, peerID := range expiredPeerIDs {
			m.bus.Publish(app.TopicPeerDeletedSync, domain.PeerIdentifier(peerID))
			// Every 50 events, add small delay to avoid overwhelming event queue
			if (i+1)%50 == 0 {
				time.Sleep(10 * time.Millisecond)
			}
		}
	}

	// Use time.Ticker instead of time.After() to prevent memory leak from unused timers
	// time.After() creates new timers on every iteration that aren't garbage collected
	ticker := time.NewTicker(m.cfg.Advanced.ExpiryCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("[EXPIRE_CLEANUP] stopping expiry check loop", "node_id", nodeID)
			return
		case <-ticker.C:
			// Timer fired, proceed with cleanup
		}

		// Attempt to delete expired peers with lock (only this master node does this)
		// Other nodes don't even run this function
		// Use Background() for periodic checks to avoid cancellation mid-query
		dbCtx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
		expiredPeerIDs, err := m.db.FindAndDeleteExpiredPeersWithLock(dbCtx, nodeID)
		if err != nil {
			// Under high load, deletion might fail due to lock contention
			// Log as warning (not error) and gracefully defer to next cycle
			if strings.Contains(err.Error(), "lock") || strings.Contains(err.Error(), "timeout") {
				slog.WarnContext(dbCtx, "[EXPIRE_CLEANUP] high load detected - deferring expired peer cleanup to next cycle",
					"node_id", nodeID, "reason", err.Error())
			} else {
				slog.ErrorContext(dbCtx, "[EXPIRE_CLEANUP] failed to find and delete expired peers",
					"node_id", nodeID, "error", err)
			}
			continue // Retry on next ticker cycle
		}

		if len(expiredPeerIDs) > 0 {
			slog.Info("[EXPIRE_CLEANUP] found and deleted expired peers",
				"count", len(expiredPeerIDs), "node_id", nodeID)

			// Publish batch event for all nodes to handle cleanup locally
			m.bus.Publish(app.TopicPeersExpiredRemoved, expiredPeerIDs)
			// Publish individual delete-sync events so other systems get notified (same as API delete)
			// Use small batches to avoid event bus queue overflow (100 items)
			for i, peerID := range expiredPeerIDs {
				m.bus.Publish(app.TopicPeerDeletedSync, domain.PeerIdentifier(peerID))
				// Every 50 events, add small delay to avoid overwhelming event queue
				if (i+1)%50 == 0 {
					time.Sleep(10 * time.Millisecond)
				}
			}
		}
	}
}

func (m Manager) checkExpiredPeers(ctx context.Context, peers []domain.Peer) {
	now := time.Now()

	for _, peer := range peers {
		if peer.IsExpired() && !peer.IsDisabled() {
			slog.Info("peer has expired, processing", "peer", peer.Identifier)

			if m.cfg.Core.DeleteExpiredPeers {
				slog.Info("deleting expired peer", "peer", peer.Identifier)
				if err := m.DeletePeer(ctx, peer.Identifier); err != nil {
					slog.Error("failed to delete expired peer", "peer", peer.Identifier, "error", err)
				}
			} else {
				slog.Info("disabling expired peer", "peer", peer.Identifier)
				peer.Disabled = &now
				peer.DisabledReason = domain.DisabledReasonExpired

				_, err := m.UpdatePeer(ctx, &peer)
				if err != nil {
					slog.Error("failed to update expired peer", "peer", peer.Identifier, "error", err)
				}
			}

			// Trigger interface synchronization
			m.bus.Publish(app.TopicPeerInterfaceUpdated, peer.InterfaceIdentifier)
		}
	}
}

func (m Manager) ClearPeers(ctx context.Context, iface domain.InterfaceIdentifier) error {
	return m.clearPeers(ctx, iface)
}

// handlePeerStateChangeEvent handles peer connection state changes and updates TTL accordingly
func (m Manager) handlePeerStateChangeEvent(peerStatus domain.PeerStatus, peer domain.Peer) {
	// Skip TTL updates if delete_expired_peers is disabled
	if !m.cfg.Core.DeleteExpiredPeers {
		slog.Debug("skipping TTL update - delete_expired_peers is disabled", "peer", peer.Identifier)
		return
	}

	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())

	slog.Debug("peer state change event received", "peer", peer.Identifier, "connected", peerStatus.IsConnected)

	// Parse the default user TTL from config
	ttlDuration, err := config.ParseDurationWithDays(m.cfg.Core.DefaultUserTTL)
	if err != nil {
		slog.Error("failed to parse default user TTL", "error", err)
		return
	}

	// Skip TTL update if TTL is locked (explicitly set)
	if peer.TTLLocked {
		slog.Debug("skipping TTL update - TTL is locked for peer", "peer", peer.Identifier)
		return
	}

	// Also skip if peer has an explicit future expiration date already set
	// This covers cases where:
	// 1. TTLLocked wasn't set during peer creation (legacy peers)
	// 2. User explicitly set ExpiresAt via API update without setting TTLLocked
	if peer.ExpiresAt != nil && peer.ExpiresAt.After(time.Now().Add(1*time.Hour)) {
		slog.Debug("skipping TTL update - peer has explicit future expiration date",
			"peer", peer.Identifier,
			"expires_at", peer.ExpiresAt.Format(time.RFC3339))
		return
	}

	// Always update TTL when peer state changes
	// - If peer is ONLINE: renews TTL (peer is active, defer deletion)
	// - If peer is OFFLINE: sets TTL timer (countdown to removal)
	expiryTime := time.Now().Add(ttlDuration)

	logAction := "setting TTL"
	if peerStatus.IsConnected {
		logAction = "renewing TTL (peer online)"
	} else {
		logAction = "setting TTL countdown (peer offline)"
	}

	slog.Info("updating peer TTL", "peer", peer.Identifier, "action", logAction, "expires_at", expiryTime.Format(time.RFC3339))

	// Update peer with new TTL
	updatedPeer := peer
	updatedPeer.ExpiresAt = &expiryTime

	_, err = m.UpdatePeer(ctx, &updatedPeer)
	if err != nil {
		slog.Error("failed to update peer TTL", "peer", peer.Identifier, "error", err)
	}
}

// initializePeerTTL initializes TTL for peers based on their current connection state
func (m Manager) initializePeerTTL(ctx context.Context) {
	// Skip TTL initialization if delete_expired_peers is disabled
	if !m.cfg.Core.DeleteExpiredPeers {
		slog.Debug("skipping peer TTL initialization - delete_expired_peers is disabled")
		return
	}

	ctx = domain.SetUserInfo(ctx, domain.SystemAdminContextUserInfo())
	slog.Debug("initializing peer TTL based on connection states")

	// Parse the default user TTL from config
	ttlDuration, err := config.ParseDurationWithDays(m.cfg.Core.DefaultUserTTL)
	if err != nil {
		slog.Error("failed to parse default user TTL during initialization", "error", err)
		return
	}

	// Get all interfaces
	interfaces, err := m.db.GetAllInterfaces(ctx)
	if err != nil {
		slog.Error("failed to get all interfaces for TTL initialization", "error", err)
		return
	}

	// Process peers from all interfaces
	for _, iface := range interfaces {
		_, peers, err := m.db.GetInterfaceAndPeers(ctx, iface.Identifier)
		if err != nil {
			slog.Error("failed to get peers for interface", "interface", iface.Identifier, "error", err)
			continue
		}

		for _, peer := range peers {
			if peer.IsDisabled() {
				continue // skip disabled peers
			}

			// Get peer status to check connection state
			peerStats, err := m.db.GetPeersStats(ctx, peer.Identifier)
			if err != nil || len(peerStats) == 0 {
				continue
			}

			peerStatus := peerStats[0]

			// Only update TTL for disconnected peers without expiration or with expired TTL
			if !peerStatus.IsConnected && (peer.ExpiresAt == nil || peer.IsExpired()) {
				expiryTime := time.Now().Add(ttlDuration)
				updatedPeer := peer
				updatedPeer.ExpiresAt = &expiryTime

				_, err = m.UpdatePeer(ctx, &updatedPeer)
				if err != nil {
					slog.Error("failed to initialize peer TTL", "peer", peer.Identifier, "error", err)
				} else {
					slog.Info("initialized TTL for disconnected peer", "peer", peer.Identifier, "expires_at", expiryTime.Format(time.RFC3339))
				}
			}
		}
	}
}

func (m Manager) handlePeerInterfaceUpdatedEvent(interfaceId domain.InterfaceIdentifier) {
	// Prevent panic from crashing the entire node in multi-node cluster
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic recovered in handlePeerInterfaceUpdatedEvent",
				"interface", interfaceId,
				"panic", r,
				"stack", "see logs above")
		}
	}()

	ctx := context.Background()

	slog.Debug("handling peer interface updated event for WireGuard sync", "interface", interfaceId)

	// Get current peers from database
	dbPeers, err := m.db.GetInterfacePeers(ctx, interfaceId)
	if err != nil {
		slog.Error("failed to get interface peers from DB", "interface", interfaceId, "error", err)
		return
	}

	slog.Debug("WireGuard sync found peers in DB", "interface", interfaceId, "count", len(dbPeers))
	for _, peer := range dbPeers {
		slog.Debug("DB peer", "interface", interfaceId, "peer", peer.Identifier, "disabled", peer.IsDisabled())
	}

	// Get local controller
	localController := m.wg.GetControllerByName(config.LocalBackendName)
	if localController == nil {
		slog.Error("local interface controller not found")
		return
	}

	// Get current peers from WireGuard device
	wgPeers, err := localController.GetPeers(ctx, interfaceId)
	if err != nil {
		slog.Error("failed to get peers from WireGuard device", "interface", interfaceId, "error", err)
		return
	}

	slog.Debug("WireGuard sync found peers in WG device", "interface", interfaceId, "count", len(wgPeers))
	for _, peer := range wgPeers {
		slog.Debug("WG peer", "interface", interfaceId, "peer", peer.Identifier)
	}

	// Create maps for easier comparison
	dbPeerMap := make(map[domain.PeerIdentifier]domain.Peer)
	for _, peer := range dbPeers {
		dbPeerMap[peer.Identifier] = peer
	}

	wgPeerMap := make(map[domain.PeerIdentifier]domain.PhysicalPeer)
	for _, peer := range wgPeers {
		wgPeerMap[peer.Identifier] = peer
	}

	// Remove peers that exist in WireGuard but not in DB
	for wgPeerID := range wgPeerMap {
		if _, exists := dbPeerMap[wgPeerID]; !exists {
			slog.Debug("removing peer from WireGuard device", "interface", interfaceId, "peer", wgPeerID)
			err := localController.DeletePeer(ctx, interfaceId, wgPeerID)
			if err != nil {
				slog.Error("failed to remove peer from WireGuard device", "interface", interfaceId, "peer", wgPeerID, "error", err)
			}
		}
	}

	// Add or update peers that exist in DB but not in WireGuard or are different
	for _, dbPeer := range dbPeers {
		if dbPeer.IsDisabled() {
			// If peer is disabled in DB, make sure it's removed from WireGuard
			if _, exists := wgPeerMap[dbPeer.Identifier]; exists {
				slog.Debug("removing disabled peer from WireGuard device", "interface", interfaceId, "peer", dbPeer.Identifier)
				err := localController.DeletePeer(ctx, interfaceId, dbPeer.Identifier)
				if err != nil {
					slog.Error("failed to remove disabled peer from WireGuard device", "interface", interfaceId, "peer", dbPeer.Identifier, "error", err)
				}
			}
			continue
		}

		// Peer is enabled, make sure it exists in WireGuard with correct configuration
		slog.Debug("syncing peer in WireGuard device", "interface", interfaceId, "peer", dbPeer.Identifier)
		err := localController.SavePeer(ctx, interfaceId, dbPeer.Identifier, func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error) {
			// Use MergeToPhysicalPeer to properly convert domain.Peer to PhysicalPeer
			// This respects the ForceClientIPAsAllowedIP config setting
			domain.MergeToPhysicalPeer(pp, &dbPeer, m.cfg.Core.ForceClientIPAsAllowedIP)
			return pp, nil
		})
		if err != nil {
			slog.Error("failed to sync peer in WireGuard device", "interface", interfaceId, "peer", dbPeer.Identifier, "error", err)
		}
	}

	slog.Debug("completed WireGuard interface sync", "interface", interfaceId)
}

// handlePeersExpiredRemovedEvent обробляє событие про видалені протухлі peer'
func (m Manager) handlePeersExpiredRemovedEvent(expiredPeerIDs []string) {
	ctx := context.Background()

	slog.Info("[EXPIRE_CLEANUP] handling expired peers removed event - START",
		"count", len(expiredPeerIDs), "node_id", m.cfg.Core.ClusterNodeId)

	if len(expiredPeerIDs) == 0 {
		slog.Info("[EXPIRE_CLEANUP] no expired peers to process")
		return
	}

	// For each deleted peer - remove it locally from WireGuard
	interfaces, err := m.db.GetAllInterfaces(ctx)
	if err != nil {
		slog.Error("[EXPIRE_CLEANUP] failed to get all interfaces", "error", err)
		return
	}

	slog.Info("[EXPIRE_CLEANUP] got interfaces for cleanup",
		"count", len(interfaces), "peers_to_delete", len(expiredPeerIDs))

	deletedFromWg := 0
	failedFromWg := 0

	for _, iface := range interfaces {
		localController := m.wg.GetControllerByName(config.LocalBackendName)
		if localController == nil {
			slog.Warn("[EXPIRE_CLEANUP] local controller not available", "interface", iface.Identifier)
			continue
		}

		// Performing bulk deletion
		for _, peerID := range expiredPeerIDs {
			peerIdent := domain.PeerIdentifier(peerID)

			// Delete from WireGuard
			if err := localController.DeletePeer(ctx, iface.Identifier, peerIdent); err != nil {
				failedFromWg++
				slog.Debug("[EXPIRE_CLEANUP] peer not in WireGuard or already deleted",
					"interface", iface.Identifier, "peer_id", peerID, "error", err)
			} else {
				deletedFromWg++
				slog.Debug("[EXPIRE_CLEANUP] removed expired peer from WireGuard",
					"interface", iface.Identifier, "peer_id", peerID)
			}
		}
	}

	slog.Info("[EXPIRE_CLEANUP] finished cleaning up expired peers locally - COMPLETED",
		"node_id", m.cfg.Core.ClusterNodeId,
		"count", len(expiredPeerIDs),
		"deleted_from_wg", deletedFromWg,
		"failed_from_wg", failedFromWg)
}

// handlePeerCreatedSyncEvent syncs a newly created peer to local WireGuard
// Called on all nodes when peer:created:sync event is published (contains only peerID)
func (m Manager) handlePeerCreatedSyncEvent(peerID domain.PeerIdentifier) {
	ctx := context.Background()
	startProcessTime := time.Now()
	slog.Info("[PEER_SYNC] handling created peer - START",
		"peer_id", peerID,
		"processing_start_unix_ns", startProcessTime.UnixNano())

	// Get peer from database
	dbStartTime := time.Now()
	peer, err := m.db.GetPeer(ctx, peerID)
	dbDuration := time.Since(dbStartTime)
	if err != nil {
		slog.Error("[PEER_SYNC] failed to get peer from database", "peer_id", peerID, "error", err)
		return
	}
	slog.Info("[PEER_SYNC] GetPeer completed",
		"peer_id", peerID,
		"interface", peer.InterfaceIdentifier,
		"db_query_ms", dbDuration.Milliseconds())

	// Get the WireGuard controller for the interface
	localController := m.wg.GetControllerByName(config.LocalBackendName)
	if localController == nil {
		slog.Warn("[PEER_SYNC] local WireGuard controller not available")
		return
	}

	// Add peer to WireGuard using SavePeer
	// SavePeer handles duplicate peers internally by using UpdateOnly flag
	// Removed GetPeers() check as it was causing excessive CPU usage on large clusters
	savePeerStartTime := time.Now()
	if err := localController.SavePeer(ctx, peer.InterfaceIdentifier, peer.Identifier, func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error) {
		domain.MergeToPhysicalPeer(pp, peer, m.cfg.Core.ForceClientIPAsAllowedIP)
		return pp, nil
	}); err != nil {
		slog.Error("[PEER_SYNC] failed to add peer to WireGuard", "peer_id", peerID, "error", err)
		return
	}
	savePeerDuration := time.Since(savePeerStartTime)
	slog.Info("[PEER_SYNC] SavePeer completed",
		"peer_id", peerID,
		"interface", peer.InterfaceIdentifier,
		"save_peer_ms", savePeerDuration.Milliseconds())

	totalDuration := time.Since(startProcessTime)
	slog.Info("[PEER_SYNC] successfully added peer to WireGuard - COMPLETED",
		"peer_id", peerID,
		"interface", peer.InterfaceIdentifier,
		"total_processing_ms", totalDuration.Milliseconds(),
		"total_processing_us", totalDuration.Microseconds(),
		"breakdown_ms", map[string]interface{}{
			"db_query":  dbDuration.Milliseconds(),
			"save_peer": savePeerDuration.Milliseconds(),
		})
}

// handlePeerUpdatedSyncEvent syncs an updated peer to local WireGuard
// Called on all nodes when peer:updated:sync event is published (contains only peerID)
func (m Manager) handlePeerUpdatedSyncEvent(peerID domain.PeerIdentifier) {
	ctx := context.Background()
	startProcessTime := time.Now()
	slog.Info("[PEER_SYNC] handling updated peer - START",
		"peer_id", peerID,
		"processing_start_unix_ns", startProcessTime.UnixNano())

	// Get peer from database
	dbStartTime := time.Now()
	peer, err := m.db.GetPeer(ctx, peerID)
	dbDuration := time.Since(dbStartTime)
	if err != nil {
		slog.Error("[PEER_SYNC] failed to get peer from database", "peer_id", peerID, "error", err)
		return
	}
	slog.Info("[PEER_SYNC] GetPeer completed",
		"peer_id", peerID,
		"interface", peer.InterfaceIdentifier,
		"db_query_ms", dbDuration.Milliseconds())

	// Get the WireGuard controller for the interface
	localController := m.wg.GetControllerByName(config.LocalBackendName)
	if localController == nil {
		slog.Warn("[PEER_SYNC] local WireGuard controller not available")
		return
	}

	// Update peer in WireGuard using SavePeer
	savePeerStartTime := time.Now()
	if err := localController.SavePeer(ctx, peer.InterfaceIdentifier, peer.Identifier, func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error) {
		domain.MergeToPhysicalPeer(pp, peer, m.cfg.Core.ForceClientIPAsAllowedIP)
		return pp, nil
	}); err != nil {
		slog.Error("[PEER_SYNC] failed to update peer in WireGuard", "peer_id", peerID, "error", err)
		return
	}
	savePeerDuration := time.Since(savePeerStartTime)
	slog.Info("[PEER_SYNC] SavePeer completed",
		"peer_id", peerID,
		"interface", peer.InterfaceIdentifier,
		"save_peer_ms", savePeerDuration.Milliseconds())

	totalDuration := time.Since(startProcessTime)
	slog.Info("[PEER_SYNC] successfully updated peer in WireGuard - COMPLETED",
		"peer_id", peerID,
		"interface", peer.InterfaceIdentifier,
		"total_processing_ms", totalDuration.Milliseconds(),
		"total_processing_us", totalDuration.Microseconds(),
		"breakdown_ms", map[string]interface{}{
			"db_query":  dbDuration.Milliseconds(),
			"save_peer": savePeerDuration.Milliseconds(),
		})
}

// handlePeerDeletedSyncEvent removes a deleted peer from local WireGuard
// Called on all nodes when peer:deleted:sync event is published (contains only peerID)
func (m Manager) handlePeerDeletedSyncEvent(peerID domain.PeerIdentifier) {
	ctx := context.Background()
	startProcessTime := time.Now()
	slog.Info("[PEER_SYNC] handling deleted peer - START",
		"peer_id", peerID,
		"processing_start_unix_ns", startProcessTime.UnixNano())

	// Get all interfaces to check peer in all of them
	dbStartTime := time.Now()
	interfaces, err := m.db.GetAllInterfaces(ctx)
	dbDuration := time.Since(dbStartTime)
	if err != nil {
		slog.Error("[PEER_SYNC] failed to get interfaces",
			"db_query_duration_ms", dbDuration.Milliseconds(),
			"error", err)
		return
	}

	slog.Info("[PEER_SYNC] interfaces fetched from database",
		"interfaces_count", len(interfaces),
		"db_query_duration_ms", dbDuration.Milliseconds())

	// Get the WireGuard controller
	localController := m.wg.GetControllerByName(config.LocalBackendName)
	if localController == nil {
		slog.Warn("[PEER_SYNC] local WireGuard controller not available")
		return
	}

	// Remove peer from WireGuard on all interfaces
	totalDeleteDuration := time.Duration(0)
	for _, iface := range interfaces {
		deleteStartTime := time.Now()
		if err := localController.DeletePeer(ctx, iface.Identifier, peerID); err != nil {
			slog.Debug("[PEER_SYNC] peer not in WireGuard or already deleted",
				"interface", iface.Identifier, "peer_id", peerID)
		} else {
			deleteDuration := time.Since(deleteStartTime)
			totalDeleteDuration += deleteDuration
			slog.Info("[PEER_SYNC] successfully removed peer from WireGuard",
				"interface", iface.Identifier, "peer_id", peerID,
				"delete_duration_ms", deleteDuration.Milliseconds())
		}
	}

	totalDuration := time.Since(startProcessTime)
	slog.Info("[PEER_SYNC] deleted peer from all interfaces - COMPLETED",
		"peer_id", peerID,
		"interfaces_checked", len(interfaces),
		"total_processing_ms", totalDuration.Milliseconds(),
		"total_processing_us", totalDuration.Microseconds(),
		"breakdown_ms", map[string]interface{}{
			"db_query":          dbDuration.Milliseconds(),
			"delete_operations": totalDeleteDuration.Milliseconds(),
		})
}

// handlePeerSyncedLocalEvent syncs a peer received from HTTP fanout to local WireGuard
// Published by HTTP endpoint to break fanout feedback loop
// Only called locally (fanout does NOT subscribe to this event)
func (m Manager) handlePeerSyncedLocalEvent(peerID domain.PeerIdentifier) {
	ctx := context.Background()
	startProcessTime := time.Now()
	slog.Info("[PEER_SYNC_LOCAL] handling synced peer - START",
		"peer_id", peerID,
		"processing_start_unix_ns", startProcessTime.UnixNano())

	// Get peer from database
	dbStartTime := time.Now()
	peer, err := m.db.GetPeer(ctx, peerID)
	dbDuration := time.Since(dbStartTime)
	if err != nil {
		// Peer not found could mean:
		// 1. Deletion event (peer was synced as deleted)
		// 2. Database replication lag
		// Either way, try to delete from WireGuard
		slog.Warn("[PEER_SYNC_LOCAL] peer not found in database (may be deleted)", "peer_id", peerID, "db_query_ms", dbDuration.Milliseconds())

		// Get interfaces and delete peer from all of them
		interfaces, err := m.db.GetAllInterfaces(ctx)
		if err == nil {
			localController := m.wg.GetControllerByName(config.LocalBackendName)
			if localController != nil {
				for _, iface := range interfaces {
					_ = localController.DeletePeer(ctx, iface.Identifier, peerID)
				}
			}
		}
		slog.Info("[PEER_SYNC_LOCAL] completed deletion cleanup - COMPLETED", "peer_id", peerID)
		return
	}
	slog.Info("[PEER_SYNC_LOCAL] GetPeer completed",
		"peer_id", peerID,
		"interface", peer.InterfaceIdentifier,
		"db_query_ms", dbDuration.Milliseconds())

	// Get the WireGuard controller for the interface
	localController := m.wg.GetControllerByName(config.LocalBackendName)
	if localController == nil {
		slog.Warn("[PEER_SYNC_LOCAL] local WireGuard controller not available")
		return
	}

	// Add peer to WireGuard using SavePeer
	savePeerStartTime := time.Now()
	if err := localController.SavePeer(ctx, peer.InterfaceIdentifier, peer.Identifier, func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error) {
		domain.MergeToPhysicalPeer(pp, peer, m.cfg.Core.ForceClientIPAsAllowedIP)
		return pp, nil
	}); err != nil {
		slog.Error("[PEER_SYNC_LOCAL] failed to add peer to WireGuard", "peer_id", peerID, "error", err)
		return
	}
	savePeerDuration := time.Since(savePeerStartTime)
	slog.Info("[PEER_SYNC_LOCAL] SavePeer completed",
		"peer_id", peerID,
		"interface", peer.InterfaceIdentifier,
		"save_peer_ms", savePeerDuration.Milliseconds())

	totalDuration := time.Since(startProcessTime)
	slog.Info("[PEER_SYNC_LOCAL] successfully synced peer to WireGuard - COMPLETED",
		"peer_id", peerID,
		"interface", peer.InterfaceIdentifier,
		"total_processing_ms", totalDuration.Milliseconds(),
		"total_processing_us", totalDuration.Microseconds(),
		"breakdown_ms", map[string]interface{}{
			"db_query":  dbDuration.Milliseconds(),
			"save_peer": savePeerDuration.Milliseconds(),
		})
}
