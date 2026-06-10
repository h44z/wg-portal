package wireguard

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/fedor-git/wg-portal-2/internal/app"
	"github.com/fedor-git/wg-portal-2/internal/config"
	"github.com/fedor-git/wg-portal-2/internal/domain"
)

type StatisticsDatabaseRepo interface {
	GetAllInterfaces(ctx context.Context) ([]domain.Interface, error)
	GetInterfacePeers(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.Peer, error)
	GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error)
	GetAllPeers(ctx context.Context) ([]domain.Peer, error)
	GetAllPeerStatuses(ctx context.Context) ([]domain.PeerStatus, error)
	UpdatePeerStatus(
		ctx context.Context,
		id domain.PeerIdentifier,
		updateFunc func(in *domain.PeerStatus) (*domain.PeerStatus, error),
	) error
	// ClaimPeerStatus claims ownership of a peer status for this node
	// Sets the OwnerNodeId and updates the peer status
	// Returns error if ownership claim fails
	ClaimPeerStatus(
		ctx context.Context,
		id domain.PeerIdentifier,
		ownerNodeId string,
		updateFunc func(in *domain.PeerStatus) (*domain.PeerStatus, error),
	) error
	BatchUpdatePeerStatuses(
		ctx context.Context,
		updates map[domain.PeerIdentifier]func(in *domain.PeerStatus) (*domain.PeerStatus, error),
	) error
	UpdateInterfaceStatus(
		ctx context.Context,
		id domain.InterfaceIdentifier,
		updateFunc func(in *domain.InterfaceStatus) (*domain.InterfaceStatus, error),
	) error
	DeletePeerStatus(ctx context.Context, id domain.PeerIdentifier) error
}

type StatisticsMetricsServer interface {
	UpdateInterfaceMetrics(status domain.InterfaceStatus)
	UpdatePeerMetrics(peer *domain.Peer, status domain.PeerStatus)
	UpdatePeerMetricsValues(peer *domain.Peer, status domain.PeerStatus)
	RegisterPeerMetrics(peer *domain.Peer)
	RemovePeerMetrics(peer *domain.Peer)
	RemovePeerMetricsByID(peerId string)
}

type StatisticsEventBus interface {
	// Subscribe subscribes to a topic
	Subscribe(topic string, fn interface{}) error
	// Publish sends a message to the message bus.
	Publish(topic string, args ...any)
}

type pingJob struct {
	Peer         domain.Peer
	PhysicalPeer domain.PhysicalPeer // Pre-fetched WireGuard peer data to avoid duplicate lookups
	Backend      domain.InterfaceBackend
}

type StatisticsCollector struct {
	cfg *config.Config
	bus StatisticsEventBus

	pingWaitGroup sync.WaitGroup
	pingJobs      chan pingJob

	db StatisticsDatabaseRepo
	wg *ControllerManager
	ms StatisticsMetricsServer

	peerChangeEvent chan domain.PeerIdentifier

	// activeConnectedPeers caches recently active peers to avoid memory bloat when they're deleted
	// Keyed by interface ID, then peer ID
	activeConnectedPeersMu sync.RWMutex
	activeConnectedPeers   map[domain.InterfaceIdentifier]map[domain.PeerIdentifier]bool
}

// NewStatisticsCollector creates a new statistics collector.
func NewStatisticsCollector(
	cfg *config.Config,
	bus StatisticsEventBus,
	db StatisticsDatabaseRepo,
	wg *ControllerManager,
	ms StatisticsMetricsServer,
) (*StatisticsCollector, error) {
	c := &StatisticsCollector{
		cfg: cfg,
		bus: bus,

		db:                   db,
		wg:                   wg,
		ms:                   ms,
		activeConnectedPeers: make(map[domain.InterfaceIdentifier]map[domain.PeerIdentifier]bool),
	}

	c.connectToMessageBus()

	return c, nil
}

// StartBackgroundJobs starts the background jobs for the statistics collector.
// This method is non-blocking and returns immediately after launching background goroutines.
// Background jobs are delayed by 10 seconds to allow database connection pool to stabilize
// and avoid connection storms during node startup in multi-node clusters.
func (c *StatisticsCollector) StartBackgroundJobs(ctx context.Context) {
	// Start background job launcher with delay to allow connection pool stabilization
	go func() {
		// Wait 10 seconds before starting background jobs to allow:
		// 1. Initial database connections to establish
		// 2. Interface state restoration to complete
		// 3. Connection pool to settle
		// 4. Other services to start up
		select {
		case <-ctx.Done():
			return // context cancelled before delay complete
		case <-time.After(10 * time.Second):
		}

		slog.Info("starting background statistics jobs after startup delay")
		// Initialize interface metrics without peers
		c.initializeInterfaceMetrics(ctx)
		c.startPingWorkers(ctx)
		c.startInterfaceDataFetcher(ctx)
		c.startPeerDataFetcher(ctx)
	}()
}

func (c *StatisticsCollector) initializeInterfaceMetrics(ctx context.Context) {
	if !c.cfg.Statistics.CollectPeerData {
		return
	}

	slog.Info("initializing interface metrics at startup (no peers)")

	interfaces, err := c.db.GetAllInterfaces(ctx)
	if err != nil {
		slog.Warn("failed to fetch interfaces for initialization", "error", err)
		return
	}

	for _, iface := range interfaces {
		// Register interface metrics with empty peer metrics
		// Peer metrics will be added dynamically as peers connect
		interfaceStatus := domain.InterfaceStatus{
			InterfaceId: iface.Identifier,
		}
		c.updateInterfaceMetrics(interfaceStatus)
		slog.Debug("initialized interface metrics", "interface", iface.Identifier)
	}

	slog.Info("interface metrics initialization complete", "count", len(interfaces))
}

func (c *StatisticsCollector) startInterfaceDataFetcher(ctx context.Context) {
	if !c.cfg.Statistics.CollectInterfaceData {
		return
	}

	go c.collectInterfaceData(ctx)

	slog.Debug("started interface data fetcher")
}

func (c *StatisticsCollector) collectInterfaceData(ctx context.Context) {
	// Start ticker
	ticker := time.NewTicker(c.cfg.Statistics.DataCollectionInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return // program stopped
		case <-ticker.C:
			interfaces, err := c.db.GetAllInterfaces(ctx)
			if err != nil {
				slog.Warn("failed to fetch all interfaces for data collection", "error", err)
				continue
			}

			for _, in := range interfaces {
				physicalInterface, err := c.wg.GetController(in).GetInterface(ctx, in.Identifier)
				if err != nil {
					slog.Warn("failed to load physical interface for data collection", "interface", in.Identifier,
						"error", err)
					continue
				}
				err = c.db.UpdateInterfaceStatus(ctx, in.Identifier,
					func(i *domain.InterfaceStatus) (*domain.InterfaceStatus, error) {
						i.UpdatedAt = time.Now()
						i.BytesReceived = physicalInterface.BytesDownload
						i.BytesTransmitted = physicalInterface.BytesUpload

						// Update prometheus metrics synchronously to reduce goroutine overhead
						c.updateInterfaceMetrics(*i)

						return i, nil
					})
				if err != nil {
					slog.Warn("failed to update interface status", "interface", in.Identifier, "error", err)
				}
				slog.Debug("updated interface status", "interface", in.Identifier)
			}
		}
	}
}

func (c *StatisticsCollector) startPeerDataFetcher(ctx context.Context) {
	if !c.cfg.Statistics.CollectPeerData {
		return
	}

	go c.collectPeerData(ctx)

	slog.Debug("started peer data fetcher")
}

func (c *StatisticsCollector) collectPeerData(ctx context.Context) {
	ticker := time.NewTicker(c.cfg.Statistics.DataCollectionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return // program stopped
		case <-ticker.C:
			interfaces, err := c.db.GetAllInterfaces(ctx)
			if err != nil {
				slog.Warn("failed to fetch interfaces", "error", err)
				continue
			}

			c.activeConnectedPeersMu.Lock()

			for _, iface := range interfaces {
				// Initialize tracking if needed
				if c.activeConnectedPeers[iface.Identifier] == nil {
					c.activeConnectedPeers[iface.Identifier] = make(map[domain.PeerIdentifier]bool)
				}

				// Get peers from WireGuard kernel and filter for active ones
				// This avoids processing thousands of configured but inactive peers
				wgPeers, err := c.wg.GetController(iface).GetPeers(ctx, iface.Identifier)
				if err != nil {
					slog.Warn("failed to fetch peers from WireGuard", "interface", iface.Identifier, "error", err)
					continue
				}

				// Filter to only recently active peers (handshake within 2 minutes)
				activePeers := c.filterActivePeers(wgPeers)
				slog.Debug("querying WireGuard", "interface", iface.Identifier, "total_peers", len(wgPeers), "active_peers", len(activePeers))

				// Track only the active peers from this cycle (will replace old cache at end)
				currentActivePeers := make(map[domain.PeerIdentifier]bool)

				for _, wgPeer := range activePeers {
					currentActivePeers[wgPeer.Identifier] = true

					// Check if this is a newly connected peer (not in our active cache yet)
					isNewConnection := !c.activeConnectedPeers[iface.Identifier][wgPeer.Identifier]

					// Process this active peer
					// Pass isNewConnection flag so we can claim ownership for new connections
					c.updatePeerFromWireGuard(ctx, iface, wgPeer, isNewConnection)
				}

				// Detect peers that aged out (were active, now inactive)
				// Only check peers we know have been active
				var agedOutPeers []domain.PeerIdentifier
				for oldPeerID := range c.activeConnectedPeers[iface.Identifier] {
					if !currentActivePeers[oldPeerID] {
						// Peer was active, now aged out (handshake > 2 min)
						// Mark as offline to clean up metrics
						agedOutPeers = append(agedOutPeers, oldPeerID)
						c.markPeerDisconnected(ctx, oldPeerID)
						// Remove from active cache
						delete(c.activeConnectedPeers[iface.Identifier], oldPeerID)
					}
				}

				if len(agedOutPeers) > 0 {
					slog.Debug("peers aged out and marked disconnected",
						"interface", iface.Identifier,
						"count", len(agedOutPeers))
					if len(agedOutPeers) <= 5 {
						slog.Debug("aged out peers detail", "peers", agedOutPeers)
					}
				}

				// Update cache with currently active peers
				// These are the only peers we need to track for cleanup next cycle
				c.activeConnectedPeers[iface.Identifier] = currentActivePeers
			}

			c.activeConnectedPeersMu.Unlock()
		}
	}
}

func (c *StatisticsCollector) updatePeerFromWireGuard(ctx context.Context, iface domain.Interface, wgPeer domain.PhysicalPeer, isNewConnection bool) {
	// Get DB peer info to store state
	dbPeer, err := c.db.GetPeer(ctx, wgPeer.Identifier)
	if err != nil || dbPeer == nil {
		slog.Debug("peer in WG but not in DB, skipping", "peer", wgPeer.Identifier)
		return
	}

	var stateChanged bool
	var newStatus domain.PeerStatus
	var lastStatus domain.PeerStatus

	var lastHandshake *time.Time
	if !wgPeer.LastHandshake.IsZero() {
		lastHandshake = &wgPeer.LastHandshake
	}

	// For newly connected peers, claim ownership to ensure this node manages their state
	// This prevents other nodes from overwriting our connection state
	// For existing peers in our cache, use regular update
	if isNewConnection {
		// NEW CONNECTION: Use ClaimPeerStatus to gain ownership
		err = c.db.ClaimPeerStatus(ctx, wgPeer.Identifier, c.cfg.Core.ClusterNodeId,
			func(p *domain.PeerStatus) (*domain.PeerStatus, error) {
				wasConnected := p.IsConnected

				// Update with latest WireGuard data
				p.UpdatedAt = time.Now()
				p.LastHandshake = lastHandshake
				p.Endpoint = wgPeer.Endpoint
				p.BytesReceived = wgPeer.BytesUpload
				p.BytesTransmitted = wgPeer.BytesDownload
				p.LastSessionStart = lastHandshake

				slog.Debug("claiming new peer connection",
					"peer", wgPeer.Identifier,
					"node", c.cfg.Core.ClusterNodeId,
					"bytes_received", p.BytesReceived,
					"bytes_transmitted", p.BytesTransmitted)

				// Force IsPingable=false if ping checks disabled
				if !c.cfg.Statistics.UsePingChecks {
					p.IsPingable = false
				}

				p.CalcConnected()

				// Detect state change
				if wasConnected != p.IsConnected {
					slog.Debug("peer state changed (new connection)",
						"peer", wgPeer.Identifier,
						"was_connected", wasConnected,
						"now_connected", p.IsConnected)
					stateChanged = true
					newStatus = *p
				}

				// Capture current status for metrics update
				lastStatus = *p
				return p, nil
			})
	} else {
		// EXISTING CONNECTION: Use UpdatePeerStatus for regular updates
		err = c.db.UpdatePeerStatus(ctx, wgPeer.Identifier,
			func(p *domain.PeerStatus) (*domain.PeerStatus, error) {
				wasConnected := p.IsConnected

				// Update with latest WireGuard data
				p.UpdatedAt = time.Now()
				p.LastHandshake = lastHandshake
				p.Endpoint = wgPeer.Endpoint
				p.BytesReceived = wgPeer.BytesUpload
				p.BytesTransmitted = wgPeer.BytesDownload

				slog.Debug("updating peer status from WireGuard",
					"peer", wgPeer.Identifier,
					"bytes_received", p.BytesReceived,
					"bytes_transmitted", p.BytesTransmitted)

				// Force IsPingable=false if ping checks disabled
				if !c.cfg.Statistics.UsePingChecks {
					p.IsPingable = false
				}
				// Detect state change
				if wasConnected != p.IsConnected {
					slog.Debug("peer state changed",
						"peer", wgPeer.Identifier,
						"was_connected", wasConnected,
						"now_connected", p.IsConnected)
					stateChanged = true
					newStatus = *p
				}

				// Capture current status for metrics update
				lastStatus = *p
				return p, nil
			})
	}

	if err != nil {
		slog.Warn("failed to update peer status", "peer", wgPeer.Identifier, "error", err)
		return
	}

	// Update metrics:
	// - For connected peers: update every cycle to keep metrics fresh (bytes, handshake, etc.)
	// - For disconnected peers: only update if state changed (to remove expensive RemovePeerMetricsById calls)
	if lastStatus.IsConnected {
		// Connected peer - update metrics every cycle to keep data fresh
		// Pass stateChanged=true to ensure metrics are registered on reconnect
		c.updatePeerMetrics(ctx, lastStatus, stateChanged)
	} else if stateChanged {
		// Disconnected peer - only update metrics if state changed (was connected, now disconnected)
		c.updatePeerMetrics(ctx, lastStatus, true)
	}

	// Publish state change event if:
	// - Connection state changed (connect/disconnect), OR
	// - Peer is currently online (renew TTL on each update cycle)
	if stateChanged || lastStatus.IsConnected {
		c.bus.Publish(app.TopicPeerStateChanged, newStatus, *dbPeer)
	}
}

func (c *StatisticsCollector) markPeerDisconnected(ctx context.Context, peerID domain.PeerIdentifier) {
	// Check if peer still exists - skip if already deleted (e.g., by master node)
	// This prevents trying to mark deleted peers as disconnected on non-master nodes
	_, err := c.db.GetPeer(ctx, peerID)
	if err != nil {
		// Peer was deleted - no need to mark disconnected
		slog.Debug("skipping markPeerDisconnected for deleted peer", "peer", peerID)
		return
	}

	// Capture the updated status and peer for publishing the event
	var updatedStatus domain.PeerStatus

	err = c.db.UpdatePeerStatus(ctx, peerID,
		func(p *domain.PeerStatus) (*domain.PeerStatus, error) {
			// Set peer as disconnected regardless of current state
			// We need to clean up metrics even if peer is already marked false
			// because metrics might still be registered
			wasConnected := p.IsConnected || p.IsPingable

			if wasConnected {
				slog.Info("marking peer as disconnected", "peer", peerID, "was_connected", p.IsConnected, "was_pingable", p.IsPingable)

				// Clear current session bytes when peer goes offline
				p.BytesReceived = 0
				p.BytesTransmitted = 0
			}

			p.IsConnected = false
			p.IsPingable = false
			p.LastHandshake = nil
			p.UpdatedAt = time.Now()

			// Always call updatePeerMetrics to clean up metrics
			// Pass true to indicate state change (from possibly connected to disconnected)
			c.updatePeerMetrics(ctx, *p, true)

			// Capture the updated status for event publishing
			updatedStatus = *p

			return p, nil
		})

	if err != nil {
		slog.Warn("failed to mark peer disconnected", "peer", peerID, "error", err)
		return
	}

	// After marking disconnected, we need to trigger TTL update
	// Get the peer from DB and publish state change event with captured status
	dbPeer, err := c.db.GetPeer(ctx, peerID)
	if err != nil {
		slog.Warn("failed to get peer for TTL update event", "peer", peerID, "error", err)
		return
	}

	// Publish the state change event so handlePeerStateChangeEvent can update the TTL
	// This ensures DefaultUserTTL is applied when peer disconnects
	slog.Debug("publishing peer state change for TTL update", "peer", peerID, "is_connected", updatedStatus.IsConnected)
	if c.bus != nil {
		c.bus.Publish(app.TopicPeerStateChanged, updatedStatus, *dbPeer)
	}
}

// filterActivePeers returns only peers that had a handshake within the last 2 minutes.
// This optimization avoids processing thousands of configured but inactive peers.
// Used by statistics collector to reduce CPU load when many peers are configured but few are active.
func (c *StatisticsCollector) filterActivePeers(allPeers []domain.PhysicalPeer) []domain.PhysicalPeer {
	if len(allPeers) == 0 {
		return allPeers
	}

	now := time.Now()
	twoMinutesAgo := now.Add(-2 * time.Minute)

	activePeers := make([]domain.PhysicalPeer, 0, len(allPeers))

	for _, peer := range allPeers {
		// Keep peer ONLY if handshake is within 2 minutes
		// LastHandshake is the only reliable indicator of active connection
		// (BytesUpload/BytesDownload are cumulative and never reset)
		if !peer.LastHandshake.IsZero() && peer.LastHandshake.After(twoMinutesAgo) {
			activePeers = append(activePeers, peer)
		}
		// Peers with handshake > 2 minutes are considered aged out (inactive)
		// No need to log here - aged out peers are logged in markPeerDisconnected()
	}

	return activePeers
}

func (c *StatisticsCollector) startPingWorkers(ctx context.Context) {
	if !c.cfg.Statistics.UsePingChecks {
		slog.Info("ping checks disabled in configuration")
		return
	}

	if c.pingJobs != nil {
		return // already started
	}

	c.pingWaitGroup = sync.WaitGroup{}
	c.pingWaitGroup.Add(c.cfg.Statistics.PingCheckWorkers)
	c.pingJobs = make(chan pingJob, c.cfg.Statistics.PingCheckWorkers)

	// start workers
	for i := 0; i < c.cfg.Statistics.PingCheckWorkers; i++ {
		go c.pingWorker(ctx)
	}

	slog.Info("ping workers started", "workers", c.cfg.Statistics.PingCheckWorkers)

	// start cleanup goroutine
	go func() {
		c.pingWaitGroup.Wait()

		slog.Debug("stopped ping checks")
	}()

	// Start ping checks with delay to avoid overwhelming database on startup
	// This gives the system time to recover from initial synchronization stress
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
			slog.Info("starting ping checks after 30s startup delay")
			c.enqueuePingChecks(ctx)
		}
	}()

	slog.Info("ping workers started", "workers", c.cfg.Statistics.PingCheckWorkers)
	slog.Debug("scheduled ping checks to start after 30 seconds")
}

func (c *StatisticsCollector) enqueuePingChecks(ctx context.Context) {
	// Start ticker
	ticker := time.NewTicker(c.cfg.Statistics.PingCheckInterval)
	defer ticker.Stop()
	defer close(c.pingJobs)

	for {
		select {
		case <-ctx.Done():
			return // program stopped
		case <-ticker.C:
			interfaces, err := c.db.GetAllInterfaces(ctx)
			if err != nil {
				slog.Warn("failed to fetch all interfaces for ping checks", "error", err)
				continue
			}

			for _, in := range interfaces {
				// OPTIMIZATION: Query WireGuard directly - source of truth for active peers on THIS node
				wireguardPeers, err := c.wg.GetController(in).GetPeers(ctx, in.Identifier)
				if err != nil {
					slog.Warn("failed to fetch WireGuard peers for ping checks", "interface", in.Identifier, "error", err)
					continue
				}

				for _, physicalPeer := range wireguardPeers {
					// Get the DB peer info
					dbPeer, err := c.db.GetPeer(ctx, physicalPeer.Identifier)
					if err != nil || dbPeer == nil {
						slog.Debug("failed to fetch peer from DB for ping check", "peer", physicalPeer.Identifier, "error", err)
						continue
					}

					c.pingJobs <- pingJob{
						Peer:         *dbPeer,
						PhysicalPeer: physicalPeer,
						Backend:      in.Backend,
					}
				}
			}
		}
	}
}

func (c *StatisticsCollector) pingWorker(ctx context.Context) {
	defer c.pingWaitGroup.Done()
	slog.Debug("ping worker started", "node_id", c.cfg.Core.ClusterNodeId)
	for job := range c.pingJobs {
		physicalPeer := job.PhysicalPeer

		var stateChanged bool
		var newStatus domain.PeerStatus

		slog.Debug("processing peer from WireGuard", "peer", physicalPeer.Identifier)

		// Update peer status based on WireGuard data
		err := c.db.UpdatePeerStatus(ctx, physicalPeer.Identifier,
			func(p *domain.PeerStatus) (*domain.PeerStatus, error) {
				wasConnected := p.IsConnected

				// Use WireGuard data directly
				var lastHandshake *time.Time
				if !physicalPeer.LastHandshake.IsZero() {
					lastHandshake = &physicalPeer.LastHandshake
				}

				p.LastHandshake = lastHandshake
				p.Endpoint = physicalPeer.Endpoint
				p.BytesReceived = physicalPeer.BytesUpload
				p.BytesTransmitted = physicalPeer.BytesDownload
				p.UpdatedAt = time.Now()

				// Calculate connected state based on LastHandshake
				p.CalcConnected()

				if wasConnected != p.IsConnected {
					slog.Debug("peer connection state changed",
						"peer", physicalPeer.Identifier,
						"was", wasConnected,
						"now", p.IsConnected)
					stateChanged = true
					newStatus = *p
				}

				// Update metrics
				c.updatePeerMetrics(ctx, *p, stateChanged)

				return p, nil
			})

		if err != nil {
			slog.Warn("failed to update peer status", "peer", physicalPeer.Identifier, "error", err)
		}

		if stateChanged {
			// Publish event if connection state changed
			c.bus.Publish(app.TopicPeerStateChanged, newStatus, job.Peer)
		}

		// Minimal delay
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Millisecond):
		}
	}
}

func (c *StatisticsCollector) isPeerPingable(
	ctx context.Context,
	backend domain.InterfaceBackend,
	peer domain.Peer,
) bool {
	if !c.cfg.Statistics.UsePingChecks {
		return false
	}

	checkAddr := peer.CheckAliveAddress()
	if checkAddr == "" {
		return false
	}

	stats, err := c.wg.GetControllerByName(backend).PingAddresses(ctx, checkAddr)
	if err != nil {
		slog.Debug("failed to ping peer", "peer", peer.Identifier, "error", err)
		return false
	}

	return stats.IsPingable()
}

func (c *StatisticsCollector) updateInterfaceMetrics(status domain.InterfaceStatus) {
	c.ms.UpdateInterfaceMetrics(status)
}

func (c *StatisticsCollector) updatePeerMetrics(ctx context.Context, status domain.PeerStatus, stateChanged bool) {
	// Fetch peer data from the database
	peer, err := c.db.GetPeer(ctx, status.PeerId)
	if err != nil {
		// Peer not found in database - it's orphaned.
		// NOTE: Orphaned peer_status records are protected from recreation by:
		// 1. UpdatePeerStatus checks if peer exists before updating status (line 1432 in database.go)
		// 2. getOrCreatePeerStatus prevents creating records for deleted peers (line 1645 in database.go)
		// 3. CleanupOrphanedPeerMetrics removes stale metrics on startup (line 460 in metrics.go)
		// 4. Callback-based cleanup removes metrics when peers are deleted (DeletePeer, DeletePeerStatus, etc.)
		slog.Debug("skipping metrics update for orphaned peer", "peer", status.PeerId)
		return
	}

	// CRITICAL: Always update metrics first to ensure peer_up reflects current state (0 for offline, 1 for online)
	// This must happen before any metric removal to reflect the actual current state
	c.ms.UpdatePeerMetricsValues(peer, status)
	slog.Debug("updated peer metrics values", "peer", status.PeerId, "is_connected", status.IsConnected, "state_changed", stateChanged)

	// Handle disconnected peers based on configuration
	if !status.IsConnected {
		// If state changed (was connected, now disconnected) and only_export_connected_peers is enabled,
		// ONLY THEN remove metrics for disconnected peers.
		// This avoids expensive RemovePeerMetricsByID calls on every cycle for thousands of offline peers.
		slog.Debug("peer disconnected, checking metric removal",
			"peer", status.PeerId,
			"state_changed", stateChanged,
			"only_export_connected", c.cfg.Statistics.OnlyExportConnectedPeers)

		if stateChanged && c.cfg.Statistics.OnlyExportConnectedPeers {
			c.ms.RemovePeerMetricsByID(string(status.PeerId))
			slog.Info("removed peer metrics due to disconnection", "peer", status.PeerId)
		}
		return
	}

	// For connected peers: ensure metrics are registered and updated
	// Register metrics dynamically when peer connects (for only_export_connected_peers mode)
	c.ms.RegisterPeerMetrics(peer)
	slog.Debug("registered metrics for connected peer", "peer", status.PeerId)
}

func (c *StatisticsCollector) connectToMessageBus() {
	_ = c.bus.Subscribe(app.TopicPeerIdentifierUpdated, c.handlePeerIdentifierChangeEvent)
	_ = c.bus.Subscribe(app.TopicPeerDeleted, c.handlePeerDeleteEvent)
	_ = c.bus.Subscribe(app.TopicPeersExpiredRemoved, c.handlePeersExpiredRemovedLocal)
}

func (c *StatisticsCollector) handlePeerIdentifierChangeEvent(oldIdentifier, newIdentifier domain.PeerIdentifier) {
	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())

	// remove potential left-over status data
	err := c.db.DeletePeerStatus(ctx, oldIdentifier)
	if err != nil {
		slog.Error("failed to delete old peer status for migrated peer", "oldIdentifier", oldIdentifier,
			"newIdentifier", newIdentifier, "error", err)
	}
}

func (c *StatisticsCollector) handlePeerDeleteEvent(peer domain.Peer) {
	// IMMEDIATELY delete peer_status from database for instant cleanup
	// This ensures peer data is cleaned up right away, not waiting for CleanOrphanedStatuses
	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())

	if err := c.db.DeletePeerStatus(ctx, peer.Identifier); err != nil {
		slog.Warn("failed to delete peer_status for deleted peer", "peerIdentifier", peer.Identifier, "error", err)
	} else {
		slog.Debug("deleted peer_status for deleted peer", "peerIdentifier", peer.Identifier)
	}

	// Remove metrics for the deleted peer on THIS node
	c.ms.RemovePeerMetrics(&peer)

	slog.Debug("cleaned up metrics and peer_status for deleted peer on local node", "peerIdentifier", peer.Identifier)
}

// handlePeersExpiredRemovedLocal clears expired peers from activeConnectedPeers cache
// This prevents memory bloat when peers are deleted but remain in the cache
func (c *StatisticsCollector) handlePeersExpiredRemovedLocal(expiredPeerIDs []string) {
	c.activeConnectedPeersMu.Lock()
	defer c.activeConnectedPeersMu.Unlock()

	for _, peerID := range expiredPeerIDs {
		peerId := domain.PeerIdentifier(peerID)

		// Remove from all interfaces' active peer caches
		for ifaceID := range c.activeConnectedPeers {
			if _, exists := c.activeConnectedPeers[ifaceID][peerId]; exists {
				delete(c.activeConnectedPeers[ifaceID], peerId)
				slog.Debug("removed expired peer from activeConnectedPeers cache",
					"interface", ifaceID, "peer", peerId)
			}
		}
	}

	slog.Info("cleared activeConnectedPeers cache for expired peers", "count", len(expiredPeerIDs))
}

// CleanOrphanedStatuses removes peer statuses and metrics for peers that no longer exist in the database.
// This is called after SyncAllPeersFromDB to ensure orphaned statuses are cleaned up.
// OPTIMIZATION: Only run this on the first cluster node to avoid 24x database load
// since all nodes would do the same work and just exhaust the connection pool
func (c *StatisticsCollector) CleanOrphanedStatuses(ctx context.Context) {
	// CRITICAL: Skip cleanup on non-primary nodes to prevent 24 nodes from hammering DB with identical queries
	// Only the node with ClusterNodeId of '1' or containing "node-1" should do this
	// This prevents N+1 query explosion (600+ queries per call) multiplied by 24 nodes = database collapse
	if !c.isPrimaryNode() {
		slog.Debug("CleanOrphanedStatuses: skipping on non-primary node", "node_id", c.cfg.Core.ClusterNodeId)
		return
	}

	slog.Info("CleanOrphanedStatuses: starting cleanup on primary node")

	// Get all peers from database
	dbPeers, err := c.db.GetAllPeers(ctx)
	if err != nil {
		slog.Warn("failed to fetch database peers for orphaned cleanup", "error", err)
		return
	}

	slog.Debug("CleanOrphanedStatuses: found DB peers", "count", len(dbPeers))

	// Create map of valid peer IDs
	validPeerMap := make(map[domain.PeerIdentifier]bool)
	for _, peer := range dbPeers {
		validPeerMap[peer.Identifier] = true
	}

	cleanedCount := 0

	// 1. Check peer_statuses table for orphaned records
	allStatuses, err := c.db.GetAllPeerStatuses(ctx)
	if err != nil {
		slog.Warn("failed to fetch peer statuses for orphaned cleanup", "error", err)
	} else {
		slog.Debug("CleanOrphanedStatuses: found peer statuses", "count", len(allStatuses))

		for _, status := range allStatuses {
			if !validPeerMap[status.PeerId] {
				slog.Info("found orphaned peer status, cleaning up", "peer", status.PeerId)

				// Delete orphaned status from database
				if err := c.db.DeletePeerStatus(ctx, status.PeerId); err != nil {
					slog.Warn("failed to delete orphaned peer status", "peer", status.PeerId, "error", err)
				}

				// Remove orphaned metrics from THIS node's registry
				c.ms.RemovePeerMetricsByID(string(status.PeerId))
				cleanedCount++
			}
		}
	}

	// 2. Also check WireGuard interfaces for peers that shouldn't be there
	// This catches cases where peer was removed but metrics still exist in memory
	interfaces, err := c.db.GetAllInterfaces(ctx)
	if err != nil {
		slog.Warn("failed to fetch interfaces for orphaned cleanup", "error", err)
	} else {
		for _, iface := range interfaces {
			wgPeers, err := c.wg.GetController(iface).GetPeers(ctx, iface.Identifier)
			if err != nil {
				slog.Debug("failed to fetch WireGuard peers for cleanup", "interface", iface.Identifier, "error", err)
				continue
			}

			slog.Debug("CleanOrphanedStatuses: checking WireGuard peers", "interface", iface.Identifier, "count", len(wgPeers))

			// Check each WireGuard peer - if it's not in DB, it's orphaned
			for _, wgPeer := range wgPeers {
				if !validPeerMap[wgPeer.Identifier] {
					slog.Info("found orphaned peer in WireGuard, cleaning up metrics", "peer", wgPeer.Identifier, "interface", iface.Identifier)

					// Remove orphaned metrics (status was already cleaned above or doesn't exist)
					c.ms.RemovePeerMetricsByID(string(wgPeer.Identifier))
					cleanedCount++
				}
			}
		}
	}

	if cleanedCount > 0 {
		slog.Info("cleaned up orphaned peer statuses and metrics", "count", cleanedCount)
	} else {
		slog.Debug("CleanOrphanedStatuses: no orphaned peers found")
	}
}

// isPrimaryNode returns true if this is the primary cleanup node
// Uses ClusterNodeId to determine primary (expected to end with "1")
func (c *StatisticsCollector) isPrimaryNode() bool {
	nodeId := c.cfg.Core.ClusterNodeId
	if nodeId == "" {
		return true // if no cluster ID, assume primary to ensure cleanup happens
	}
	// Node is marked as primary if its ID contains "-1" or ends with "1" suffix
	// This ensures only ONE designated node does the expensive cleanup
	return strings.Contains(nodeId, "-1")
}
