package wireguard

import (
	"context"
	"sync"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"github.com/sirupsen/logrus"
	evbus "github.com/vardius/message-bus"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type StatisticsCollector struct {
	cfg *config.Config
	bus evbus.MessageBus

	pingWaitGroup sync.WaitGroup
	pingJobs      chan domain.Peer

	db StatisticsDatabaseRepo
	wg InterfaceController
	ms MetricsServer
}

func NewStatisticsCollector(
	cfg *config.Config,
	bus evbus.MessageBus,
	db StatisticsDatabaseRepo,
	wg InterfaceController,
	ms MetricsServer,
) (*StatisticsCollector, error) {
	c := &StatisticsCollector{
		cfg: cfg,
		bus: bus,

		db: db,
		wg: wg,
		ms: ms,
	}

	c.connectToMessageBus()

	return c, nil
}

func (c *StatisticsCollector) StartBackgroundJobs(ctx context.Context) {
	c.startPingWorkers(ctx)
	c.startInterfaceDataFetcher(ctx)
	c.startPeerDataFetcher(ctx)
}

func (c *StatisticsCollector) startInterfaceDataFetcher(ctx context.Context) {
	if !c.cfg.Statistics.CollectInterfaceData {
		return
	}

	go c.collectInterfaceData(ctx)

	logrus.Tracef("started interface data fetcher")
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
				logrus.Warnf("failed to fetch all interfaces for data collection: %v", err)
				continue
			}

			for _, in := range interfaces {
				physicalInterface, err := c.wg.GetInterface(ctx, in.Identifier)
				if err != nil {
					logrus.Warnf("failed to load physical interface %s for data collection: %v", in.Identifier, err)
					continue
				}
				err = c.db.UpdateInterfaceStatus(ctx, in.Identifier,
					func(i *domain.InterfaceStatus) (*domain.InterfaceStatus, error) {
						i.UpdatedAt = time.Now()
						i.BytesReceived = physicalInterface.BytesDownload
						i.BytesTransmitted = physicalInterface.BytesUpload

						// Update prometheus metrics
						go c.updateInterfaceMetrics(*i)

						return i, nil
					})
				if err != nil {
					logrus.Warnf("failed to update interface status for %s: %v", in.Identifier, err)
				}
				logrus.Tracef("updated interface status for %s", in.Identifier)
			}
		}
	}
}

func (c *StatisticsCollector) startPeerDataFetcher(ctx context.Context) {
	if !c.cfg.Statistics.CollectPeerData {
		return
	}

	go c.collectPeerData(ctx)

	logrus.Tracef("started peer data fetcher")
}

func (c *StatisticsCollector) collectPeerData(ctx context.Context) {
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
				logrus.Warnf("failed to fetch all interfaces for peer data collection: %v", err)
				continue
			}

			for _, in := range interfaces {
				peers, err := c.wg.GetPeers(ctx, in.Identifier)
				if err != nil {
					logrus.Warnf("failed to fetch peers for data collection (interface %s): %v", in.Identifier, err)
					continue
				}
				for _, peer := range peers {
					err = c.db.UpdatePeerStatus(ctx, peer.Identifier,
						func(p *domain.PeerStatus) (*domain.PeerStatus, error) {
							var lastHandshake *time.Time
							if !peer.LastHandshake.IsZero() {
								lastHandshake = &peer.LastHandshake
							}

							// calculate if session was restarted
							p.UpdatedAt = time.Now()
							p.LastSessionStart = getSessionStartTime(*p, peer.BytesUpload, peer.BytesDownload,
								lastHandshake)
							p.BytesReceived = peer.BytesUpload      // store bytes that where uploaded from the peer and received by the server
							p.BytesTransmitted = peer.BytesDownload // store bytes that where received from the peer and sent by the server
							p.Endpoint = peer.Endpoint
							p.LastHandshake = lastHandshake

							// Update prometheus metrics
							go c.updatePeerMetrics(ctx, *p)

							return p, nil
						})
					if err != nil {
						logrus.Warnf("failed to update peer status for %s: %v", peer.Identifier, err)
					} else {
						logrus.Tracef("updated peer status for %s", peer.Identifier)
					}
				}
			}
		}
	}
}

func getSessionStartTime(
	oldStats domain.PeerStatus,
	newReceived, newTransmitted uint64,
	latestHandshake *time.Time,
) *time.Time {
	if latestHandshake == nil {
		return nil // currently not connected
	}

	oldestHandshakeTime := time.Now().Add(-2 * time.Minute) // if a handshake is older than 2 minutes, the peer is no longer connected
	switch {
	// old session was never initiated
	case oldStats.BytesReceived == 0 && oldStats.BytesTransmitted == 0 && (newReceived > 0 || newTransmitted > 0):
		return latestHandshake
	// session never received bytes -> first receive
	case oldStats.BytesReceived == 0 && newReceived > 0 && (oldStats.LastHandshake == nil || oldStats.LastHandshake.Before(oldestHandshakeTime)):
		return latestHandshake
	// session never transmitted bytes -> first transmit
	case oldStats.BytesTransmitted == 0 && newTransmitted > 0 && (oldStats.LastSessionStart == nil || oldStats.LastHandshake.Before(oldestHandshakeTime)):
		return latestHandshake
	// session restarted as newer send or transmit counts are lower
	case (newReceived != 0 && newReceived < oldStats.BytesReceived) || (newTransmitted != 0 && newTransmitted < oldStats.BytesTransmitted):
		return latestHandshake
	// session initiated (but some bytes were already transmitted
	case oldStats.LastSessionStart == nil && (newReceived > oldStats.BytesReceived || newTransmitted > oldStats.BytesTransmitted):
		return latestHandshake
	default:
		return oldStats.LastSessionStart
	}
}

func (c *StatisticsCollector) startPingWorkers(ctx context.Context) {
	if !c.cfg.Statistics.UsePingChecks {
		return
	}

	if c.pingJobs != nil {
		return // already started
	}

	c.pingWaitGroup = sync.WaitGroup{}
	c.pingWaitGroup.Add(c.cfg.Statistics.PingCheckWorkers)
	c.pingJobs = make(chan domain.Peer, c.cfg.Statistics.PingCheckWorkers)

	// start workers
	for i := 0; i < c.cfg.Statistics.PingCheckWorkers; i++ {
		go c.pingWorker(ctx)
	}

	// start cleanup goroutine
	go func() {
		c.pingWaitGroup.Wait()

		logrus.Tracef("stopped ping checks")
	}()

	go c.enqueuePingChecks(ctx)

	logrus.Tracef("started ping checks")
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
				logrus.Warnf("failed to fetch all interfaces for ping checks: %v", err)
				continue
			}

			for _, in := range interfaces {
				peers, err := c.db.GetInterfacePeers(ctx, in.Identifier)
				if err != nil {
					logrus.Warnf("failed to fetch peers for ping checks (interface %s): %v", in.Identifier, err)
					continue
				}
				for _, peer := range peers {
					c.pingJobs <- peer
				}
			}
		}
	}
}

func (c *StatisticsCollector) pingWorker(ctx context.Context) {
	defer c.pingWaitGroup.Done()
	for peer := range c.pingJobs {
		peerPingable := c.isPeerPingable(ctx, peer)
		logrus.Tracef("peer %s pingable: %t", peer.Identifier, peerPingable)

		now := time.Now()
		err := c.db.UpdatePeerStatus(ctx, peer.Identifier,
			func(p *domain.PeerStatus) (*domain.PeerStatus, error) {
				if peerPingable {
					p.IsPingable = true
					p.LastPing = &now
				} else {
					p.IsPingable = false
					p.LastPing = nil
				}

				// Update prometheus metrics
				go c.updatePeerMetrics(ctx, *p)

				return p, nil
			})
		if err != nil {
			logrus.Warnf("failed to update peer ping status for %s: %v", peer.Identifier, err)
		} else {
			logrus.Tracef("updated peer ping status for %s", peer.Identifier)
		}
	}
}

func (c *StatisticsCollector) isPeerPingable(ctx context.Context, peer domain.Peer) bool {
	if !c.cfg.Statistics.UsePingChecks {
		return false
	}

	checkAddr := peer.CheckAliveAddress()
	if checkAddr == "" {
		return false
	}

	pinger, err := probing.NewPinger(checkAddr)
	if err != nil {
		logrus.Tracef("failed to instatiate pinger for %s (%s): %v", peer.Identifier, checkAddr, err)
		return false
	}

	checkCount := 1
	pinger.SetPrivileged(!c.cfg.Statistics.PingUnprivileged)
	pinger.Count = checkCount
	pinger.Timeout = 2 * time.Second
	err = pinger.RunWithContext(ctx) // Blocks until finished.
	if err != nil {
		logrus.Tracef("pinger for peer %s (%s) exited unexpectedly: %v", peer.Identifier, checkAddr, err)
		return false
	}
	stats := pinger.Statistics()
	return stats.PacketsRecv == checkCount
}

func (c *StatisticsCollector) updateInterfaceMetrics(status domain.InterfaceStatus) {
	c.ms.UpdateInterfaceMetrics(status)
}

func (c *StatisticsCollector) updatePeerMetrics(ctx context.Context, status domain.PeerStatus) {
	// Fetch peer data from the database
	peer, err := c.db.GetPeer(ctx, status.PeerId)
	if err != nil {
		logrus.Warnf("failed to fetch peer data for metrics %s: %v", status.PeerId, err)
		return
	}
	c.ms.UpdatePeerMetrics(peer, status)
}

func (c *StatisticsCollector) connectToMessageBus() {
	_ = c.bus.Subscribe(app.TopicPeerIdentifierUpdated, c.handlePeerIdentifierChangeEvent)
}

func (c *StatisticsCollector) handlePeerIdentifierChangeEvent(oldIdentifier, newIdentifier domain.PeerIdentifier) {
	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())

	// remove potential left-over status data
	err := c.db.DeletePeerStatus(ctx, oldIdentifier)
	if err != nil {
		logrus.Errorf("failed to delete old peer status for migrated peer, %s -> %s: %v",
			oldIdentifier, newIdentifier, err)
	}
}
