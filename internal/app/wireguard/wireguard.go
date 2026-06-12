package wireguard

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// region dependencies

type InterfaceAndPeerDatabaseRepo interface {
	NotificationManagerDatabaseRepo // embeds GetAllInterfaces, GetInterfacePeers, GetUser
	GetInterface(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, error)
	GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error)
	GetPeersStats(ctx context.Context, ids ...domain.PeerIdentifier) ([]domain.PeerStatus, error)
	GetInterfaceIps(ctx context.Context) (map[domain.InterfaceIdentifier][]domain.Cidr, error)
	SaveInterface(
		ctx context.Context,
		id domain.InterfaceIdentifier,
		updateFunc func(in *domain.Interface) (*domain.Interface, error),
	) error
	DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error
	GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error)
	SavePeer(
		ctx context.Context,
		id domain.PeerIdentifier,
		updateFunc func(in *domain.Peer) (*domain.Peer, error),
	) error
	DeletePeer(ctx context.Context, id domain.PeerIdentifier) error
	GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error)
	GetUsedIpsPerSubnet(ctx context.Context, subnets []domain.Cidr) (map[domain.Cidr][]domain.Cidr, error)
	GetAllUsers(ctx context.Context) ([]domain.User, error)
}

type WgQuickController interface {
	ExecuteInterfaceHook(ctx context.Context, id domain.InterfaceIdentifier, hookCmd string) error
	SetDNS(ctx context.Context, id domain.InterfaceIdentifier, dnsStr, dnsSearchStr string) error
	UnsetDNS(ctx context.Context, id domain.InterfaceIdentifier, dnsStr, dnsSearchStr string) error
}

type EventBus interface {
	// Publish sends a message to the message bus.
	Publish(topic string, args ...any)
	// Subscribe subscribes to a topic
	Subscribe(topic string, fn interface{}) error
}

// endregion dependencies

type Manager struct {
	cfg       *config.Config
	bus       EventBus
	db        InterfaceAndPeerDatabaseRepo
	wg        *ControllerManager
	notifRepo NotificationRepository
	mailer    NotificationMailer

	userLockMap      *sync.Map
	interfaceLockMap *sync.Map
}

func NewWireGuardManager(
	cfg *config.Config,
	bus EventBus,
	wg *ControllerManager,
	db InterfaceAndPeerDatabaseRepo,
	notifRepo NotificationRepository,
	mailer NotificationMailer,
) (*Manager, error) {
	m := &Manager{
		cfg:              cfg,
		bus:              bus,
		wg:               wg,
		db:               db,
		notifRepo:        notifRepo,
		mailer:           mailer,
		userLockMap:      &sync.Map{},
		interfaceLockMap: &sync.Map{},
	}

	m.connectToMessageBus()

	return m, nil
}

// StartBackgroundJobs starts background jobs like the expired peers check.
// This method is non-blocking.
func (m *Manager) StartBackgroundJobs(ctx context.Context) {
	go m.runExpiredPeersCheck(ctx)

	nm := NewNotificationManager(m.cfg, m.db, m.notifRepo, m.mailer)
	go nm.Run(ctx)
}

func (m Manager) connectToMessageBus() {
	_ = m.bus.Subscribe(app.TopicUserCreated, m.handleUserCreationEvent)
	_ = m.bus.Subscribe(app.TopicAuthLogin, m.handleUserLoginEvent)
	_ = m.bus.Subscribe(app.TopicUserDisabled, m.handleUserDisabledEvent)
	_ = m.bus.Subscribe(app.TopicUserEnabled, m.handleUserEnabledEvent)
	_ = m.bus.Subscribe(app.TopicUserDeleted, m.handleUserDeletedEvent)
	_ = m.bus.Subscribe(app.TopicInterfaceCreated, m.handleInterfaceCreatedEvent)
}

func (m Manager) handleUserCreationEvent(user domain.User) {
	if !m.cfg.Core.CreateDefaultPeerOnUserCreation {
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
	if !m.cfg.Core.CreateDefaultPeerOnLogin {
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

// handleInterfaceCreatedEvent creates default peers for all existing users when a new interface is created.
// This ensures users that already exist (e.g. imported via a prior LDAP sync that had no interface available)
// also receive a default peer for the newly created interface.
func (m Manager) handleInterfaceCreatedEvent(iface domain.Interface) {
	if !m.cfg.Core.CreateDefaultPeerOnUserCreation {
		return
	}

	_, loaded := m.interfaceLockMap.LoadOrStore(iface.Identifier, "create")
	if loaded {
		return // another goroutine is already handling this interface
	}
	defer m.interfaceLockMap.Delete(iface.Identifier)

	slog.Debug("handling new interface event", "interface", iface.Identifier)

	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())

	err := m.CreateDefaultPeers(ctx, iface.Identifier)
	if err != nil {
		slog.Error("failed to create default peers on new interface",
			"interface", iface.Identifier, "error", err)
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
			slog.Error("failed to fetch all interfaces for expiry check", "error", err)
			continue
		}

		for _, iface := range interfaces {
			peers, err := m.db.GetInterfacePeers(ctx, iface.Identifier)
			if err != nil {
				slog.Error("failed to fetch all peers from interface for expiry check",
					"interface", iface.Identifier,
					"error", err)
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
			expiryStr := peer.ExpiresAt.UTC().Format(time.RFC3339)

			// Capture info for potential recreate before the old peer is removed.
			shouldRecreate := m.cfg.Core.Peer.AutoRecreateOnExpiry && peer.UserIdentifier != ""
			oldUser := peer.UserIdentifier
			oldIface := peer.InterfaceIdentifier
			oldDisplayName := peer.DisplayName

			if m.cfg.Core.Peer.ExpiryAction == "delete" {
				slog.Info("peer has expired, deleting",
					"peer", peer.Identifier,
					"expired_at", expiryStr,
					"action", "delete",
				)
				if err := m.DeletePeer(ctx, peer.Identifier); err != nil {
					slog.Error("failed to delete expired peer",
						"peer", peer.Identifier,
						"expired_at", expiryStr,
						"error", err,
					)
					continue // skip recreate on failure
				} else {
					slog.Info("expired peer deleted successfully",
						"peer", peer.Identifier,
						"expired_at", expiryStr,
					)
				}
			} else {
				// default: disable
				slog.Info("peer has expired, disabling",
					"peer", peer.Identifier,
					"expired_at", expiryStr,
					"action", "disable",
				)
				peer.Disabled = &now
				peer.DisabledReason = fmt.Sprintf("expired on %s", expiryStr)
				if _, err := m.UpdatePeer(ctx, &peer); err != nil {
					slog.Error("failed to disable expired peer",
						"peer", peer.Identifier,
						"expired_at", expiryStr,
						"error", err,
					)
					continue // skip recreate on failure
				} else {
					slog.Info("expired peer disabled successfully",
						"peer", peer.Identifier,
						"expired_at", expiryStr,
						"disabled_reason", peer.DisabledReason,
					)
				}
			}

			if shouldRecreate {
				m.recreateExpiredPeer(ctx, oldIface, oldUser, oldDisplayName)
			}
		}

		// Purge disabled expired peers that have been expired longer than PurgeExpiredAfter.
		if m.cfg.Core.Peer.PurgeExpiredAfter > 0 && peer.IsExpired() && peer.IsDisabled() {
			if now.Sub(*peer.ExpiresAt) > m.cfg.Core.Peer.PurgeExpiredAfter {
				slog.Info("purging disabled expired peer",
					"peer", peer.Identifier,
					"expired_at", peer.ExpiresAt.UTC().Format(time.RFC3339),
				)
				if err := m.DeletePeer(ctx, peer.Identifier); err != nil {
					slog.Error("failed to purge expired peer",
						"peer", peer.Identifier,
						"error", err,
					)
				}
			}
		}
	}
}

// recreateExpiredPeer creates a fresh replacement peer for the same user and interface.
func (m Manager) recreateExpiredPeer(
	ctx context.Context,
	ifaceID domain.InterfaceIdentifier,
	userID domain.UserIdentifier,
	displayName string,
) {
	freshPeer, err := m.PreparePeer(ctx, ifaceID)
	if err != nil {
		slog.Error("failed to prepare replacement peer",
			"interface", ifaceID, "user", userID, "error", err)
		return
	}

	freshPeer.UserIdentifier = userID
	freshPeer.DisplayName = displayName
	if suffix := m.cfg.Core.Peer.RecreateOnExpirySuffix; suffix != "" && !strings.HasSuffix(displayName, suffix) {
		freshPeer.DisplayName = displayName + suffix
	}

	if m.cfg.Core.Peer.RotationInterval > 0 {
		expiresAt := time.Now().Add(m.cfg.Core.Peer.RotationInterval)
		freshPeer.ExpiresAt = &expiresAt
	}

	if err := m.savePeers(ctx, freshPeer); err != nil {
		slog.Error("failed to save replacement peer",
			"interface", ifaceID, "user", userID, "error", err)
		return
	}

	m.bus.Publish(app.TopicPeerCreated, *freshPeer)

	slog.Info("replacement peer created for expired peer",
		"new_peer", freshPeer.Identifier,
		"interface", ifaceID,
		"user", userID,
	)
}
