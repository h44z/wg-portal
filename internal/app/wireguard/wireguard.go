package wireguard

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	evbus "github.com/vardius/message-bus"

	"github.com/h44z/wg-portal/internal/app"
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

func NewWireGuardManager(
	cfg *config.Config,
	bus evbus.MessageBus,
	wg InterfaceController,
	quick WgQuickController,
	db InterfaceAndPeerDatabaseRepo,
) (*Manager, error) {
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
	_ = m.bus.Subscribe(app.TopicAuthLogin, m.handleUserLoginEvent)
	_ = m.bus.Subscribe(app.TopicUserDisabled, m.handleUserDisabledEvent)
	_ = m.bus.Subscribe(app.TopicUserEnabled, m.handleUserEnabledEvent)
	_ = m.bus.Subscribe(app.TopicUserDeleted, m.handleUserDeletedEvent)
}

func (m Manager) handleUserCreationEvent(user *domain.User) {
	if !m.cfg.Core.CreateDefaultPeerOnCreation {
		return
	}

	logrus.Tracef("handling new user event for %s", user.Identifier)

	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
	err := m.CreateDefaultPeer(ctx, user.Identifier)
	if err != nil {
		logrus.Errorf("failed to create default peer for %s: %v", user.Identifier, err)
		return
	}
}

func (m Manager) handleUserLoginEvent(userId domain.UserIdentifier) {
	if !m.cfg.Core.CreateDefaultPeer {
		return
	}

	userPeers, err := m.db.GetUserPeers(context.Background(), userId)
	if err != nil {
		logrus.Errorf("failed to retrieve existing peers for %s prior to default peer creation: %v", userId, err)
		return
	}

	if len(userPeers) > 0 {
		return // user already has peers, skip creation
	}

	logrus.Tracef("handling new user login for %s", userId)

	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
	err = m.CreateDefaultPeer(ctx, userId)
	if err != nil {
		logrus.Errorf("failed to create default peer for %s: %v", userId, err)
		return
	}
}

func (m Manager) handleUserDisabledEvent(user domain.User) {
	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
	userPeers, err := m.db.GetUserPeers(ctx, user.Identifier)
	if err != nil {
		logrus.Errorf("failed to retrieve peers for disabled user %s: %v", user.Identifier, err)
		return
	}

	for _, peer := range userPeers {
		if peer.IsDisabled() {
			continue // peer is already disabled
		}

		logrus.Debugf("disabling peer %s due to user %s being disabled", peer.Identifier, user.Identifier)

		peer.Disabled = user.Disabled // set to user disabled timestamp
		peer.DisabledReason = domain.DisabledReasonUserDisabled

		_, err := m.UpdatePeer(ctx, &peer)
		if err != nil {
			logrus.Errorf("failed to disable peer %s for disabled user %s: %v",
				peer.Identifier, user.Identifier, err)
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
		logrus.Errorf("failed to retrieve peers for re-enabled user %s: %v", user.Identifier, err)
		return
	}

	for _, peer := range userPeers {
		if !peer.IsDisabled() {
			continue // peer is already active
		}

		if peer.DisabledReason != domain.DisabledReasonUserDisabled {
			continue // peer was disabled for another reason
		}

		logrus.Debugf("enabling peer %s due to user %s being enabled", peer.Identifier, user.Identifier)

		peer.Disabled = nil
		peer.DisabledReason = ""

		_, err := m.UpdatePeer(ctx, &peer)
		if err != nil {
			logrus.Errorf("failed to enable peer %s for enabled user %s: %v",
				peer.Identifier, user.Identifier, err)
		}
	}
	return
}

func (m Manager) handleUserDeletedEvent(user domain.User) {
	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
	userPeers, err := m.db.GetUserPeers(ctx, user.Identifier)
	if err != nil {
		logrus.Errorf("failed to retrieve peers for deleted user %s: %v", user.Identifier, err)
		return
	}

	deletionTime := time.Now()
	for _, peer := range userPeers {
		if peer.IsDisabled() {
			continue // peer is already disabled
		}

		if m.cfg.Core.DeletePeerAfterUserDeleted {
			logrus.Debugf("deleting peer %s due to user %s being deleted", peer.Identifier, user.Identifier)

			if err := m.DeletePeer(ctx, peer.Identifier); err != nil {
				logrus.Errorf("failed to delete peer %s for deleted user %s: %v",
					peer.Identifier, user.Identifier, err)
			}
		} else {
			logrus.Debugf("disabling peer %s due to user %s being deleted", peer.Identifier, user.Identifier)

			peer.UserIdentifier = "" // remove user reference
			peer.Disabled = &deletionTime
			peer.DisabledReason = domain.DisabledReasonUserDeleted

			_, err := m.UpdatePeer(ctx, &peer)
			if err != nil {
				logrus.Errorf("failed to disable peer %s for deleted user %s: %v",
					peer.Identifier, user.Identifier, err)
			}
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
