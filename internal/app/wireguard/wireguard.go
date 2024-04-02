package wireguard

import (
	"context"
	"time"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/sirupsen/logrus"

	evbus "github.com/vardius/message-bus"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type Manager struct {
	cfg *config.Config
	bus evbus.MessageBus

	db    InterfaceAndPeerDatabaseRepo
	wg    InterfaceController
	quick WgQuickController
}

func NewWireGuardManager(cfg *config.Config, bus evbus.MessageBus, wg InterfaceController, quick WgQuickController, db InterfaceAndPeerDatabaseRepo) (*Manager, error) {
	m := &Manager{
		cfg:   cfg,
		bus:   bus,
		wg:    wg,
		db:    db,
		quick: quick,
	}

	m.connectToMessageBus()

	return m, nil
}

func (m Manager) StartBackgroundJobs(ctx context.Context) {
	go m.runExpiredPeersCheck(ctx)
}

func (m Manager) connectToMessageBus() {
	_ = m.bus.Subscribe(app.TopicUserCreated, m.handleUserCreationEvent)
}

func (m Manager) handleUserCreationEvent(user *domain.User) {
	logrus.Errorf("handling new user event for %s", user.Identifier)

	if m.cfg.Core.CreateDefaultPeer && m.cfg.Core.DefaultPeersPerUser > 0 {
		ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
		err := m.CreateDefaultPeer(ctx, user)
		if err != nil {
			logrus.Errorf("failed to create default peer for %s: %v", user.Identifier, err)
			return
		}
	}
}

func (m Manager) runExpiredPeersCheck(ctx context.Context) {
	ctx = domain.SetUserInfo(ctx, domain.SystemAdminContextUserInfo())

	running := true
	for running {
		select {
		case <-ctx.Done():
			running = false
			continue
		case <-time.After(m.cfg.Advanced.ExpiryCheckInterval):
			// select blocks until one of the cases evaluate to true
		}

		interfaces, err := m.db.GetAllInterfaces(ctx)
		if err != nil {
			logrus.Errorf("failed to fetch all interfaces for expiry check: %v", err)
			continue
		}

		for _, iface := range interfaces {
			peers, err := m.db.GetInterfacePeers(ctx, iface.Identifier)
			if err != nil {
				logrus.Errorf("failed to fetch all peers from interface %s for expiry check: %v", iface.Identifier, err)
				continue
			}

			m.checkExpiredPeers(ctx, peers)
		}
	}
}

func (m Manager) checkExpiredPeers(ctx context.Context, peers []domain.Peer) {
	now := time.Now()

	for _, peer := range peers {
		if peer.IsExpired() && !peer.IsDisabled() {
			logrus.Infof("peer %s has expired, disabling...", peer.Identifier)

			peer.Disabled = &now
			peer.DisabledReason = domain.DisabledReasonExpired

			_, err := m.UpdatePeer(ctx, &peer)
			if err != nil {
				logrus.Errorf("failed to update expired peer %s: %v", peer.Identifier, err)
			}
		}
	}
}
