package wireguard

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
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
	GetUsedIpsPerSubnet(ctx context.Context, subnets []domain.Cidr) (map[domain.Cidr][]domain.Cidr, error)
	GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
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
	cfg *config.Config
	bus EventBus
	db  InterfaceAndPeerDatabaseRepo
	wg  *ControllerManager

	userLockMap      *sync.Map
	interfaceLockMap *sync.Map
}

func NewWireGuardManager(
	cfg *config.Config,
	bus EventBus,
	wg *ControllerManager,
	db InterfaceAndPeerDatabaseRepo,
) (*Manager, error) {
	m := &Manager{
		cfg:              cfg,
		bus:              bus,
		wg:               wg,
		db:               db,
		userLockMap:      &sync.Map{},
		interfaceLockMap: &sync.Map{},
	}

	m.connectToMessageBus()

	return m, nil
}

// StartBackgroundJobs starts background jobs like the expired peers check.
// This method is non-blocking.
func (m Manager) StartBackgroundJobs(ctx context.Context) {
	go m.runExpiredPeersCheck(ctx)
}

func (m Manager) connectToMessageBus() {
	_ = m.bus.Subscribe(app.TopicUserCreated, m.handleUserCreationEvent)
	_ = m.bus.Subscribe(app.TopicAuthLogin, m.handleUserLoginEvent)
	_ = m.bus.Subscribe(app.TopicUserDisabled, m.handleUserDisabledEvent)
	_ = m.bus.Subscribe(app.TopicUserEnabled, m.handleUserEnabledEvent)
	_ = m.bus.Subscribe(app.TopicUserDeleted, m.handleUserDeletedEvent)
	_ = m.bus.Subscribe(app.TopicInterfaceLdapFilterApplied, m.handleInterfaceLdapFilterAppliedEvent)
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

// handleInterfaceLdapFilterAppliedEvent reconciles the peers on one interface
// against the users its LDAP filter currently permits, disabling any that are
// no longer entitled.
//
// It works from the resulting allowed set rather than a diff of what changed,
// so it is idempotent and also repairs drift that occurred while wg-portal was
// not running: a demotion during downtime is invisible to a transition-based
// check but is caught by the next reconcile.
//
// Only peers owned by a user are considered. Peers imported from a device or
// created by an administrator have no owner and are not governed by the filter.
func (m Manager) handleInterfaceLdapFilterAppliedEvent(
	interfaceId domain.InterfaceIdentifier,
	allowedUserIds []domain.UserIdentifier,
) {
	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())

	peers, err := m.db.GetInterfacePeers(ctx, interfaceId)
	if err != nil {
		slog.Error("failed to retrieve peers while reconciling interface access",
			"interface", interfaceId, "error", err)
		return
	}

	allowed := make(map[domain.UserIdentifier]struct{}, len(allowedUserIds))
	for _, id := range allowedUserIds {
		allowed[id] = struct{}{}
	}

	now := time.Now()
	for _, peer := range peers {
		if peer.UserIdentifier == "" {
			continue // not owned by a user, so no filter governs it
		}
		if _, ok := allowed[peer.UserIdentifier]; ok {
			// Entitled again. Undo a revocation this handler performed earlier,
			// so that moving a user back into the group restores their access;
			// peers disabled for any other reason are left to their own owner.
			if peer.DisabledReason != domain.DisabledReasonAccessRevoked {
				continue
			}

			slog.Info("re-enabling peer, user is permitted on this interface again",
				"peer", peer.Identifier, "user", peer.UserIdentifier, "interface", interfaceId)

			peer.Disabled = nil
			peer.DisabledReason = ""

			if _, err := m.UpdatePeer(ctx, &peer); err != nil {
				slog.Error("failed to re-enable peer after interface access was restored",
					"peer", peer.Identifier, "user", peer.UserIdentifier,
					"interface", interfaceId, "error", err)
			}

			continue
		}
		if peer.IsDisabled() {
			continue
		}

		slog.Info("disabling peer, user no longer permitted on this interface",
			"peer", peer.Identifier, "user", peer.UserIdentifier, "interface", interfaceId)

		peer.Disabled = &now
		peer.DisabledReason = domain.DisabledReasonAccessRevoked

		if _, err := m.UpdatePeer(ctx, &peer); err != nil {
			slog.Error("failed to disable peer after interface access was revoked",
				"peer", peer.Identifier, "user", peer.UserIdentifier,
				"interface", interfaceId, "error", err)
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
			slog.Info("peer has expired, disabling", "peer", peer.Identifier)

			peer.Disabled = &now
			peer.DisabledReason = domain.DisabledReasonExpired

			_, err := m.UpdatePeer(ctx, &peer)
			if err != nil {
				slog.Error("failed to update expired peer", "peer", peer.Identifier, "error", err)
			}
		}
	}
}
