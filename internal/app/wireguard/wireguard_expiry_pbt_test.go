package wireguard

import (
	"context"
	"regexp"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// expiryMockDB extends mockDB to support full peer lifecycle needed by checkExpiredPeers.
// It stores peers by identifier and tracks DeletePeer calls.
type expiryMockDB struct {
	mockDB
	peers         map[domain.PeerIdentifier]*domain.Peer
	deletedPeers  map[domain.PeerIdentifier]bool
	savedPeerData map[domain.PeerIdentifier]*domain.Peer // tracks what was saved via SavePeer
}

func newExpiryMockDB(iface *domain.Interface) *expiryMockDB {
	return &expiryMockDB{
		mockDB: mockDB{
			iface: iface,
		},
		peers:         make(map[domain.PeerIdentifier]*domain.Peer),
		deletedPeers:  make(map[domain.PeerIdentifier]bool),
		savedPeerData: make(map[domain.PeerIdentifier]*domain.Peer),
	}
}

func (f *expiryMockDB) addPeer(p *domain.Peer) {
	f.peers[p.Identifier] = p
}

func (f *expiryMockDB) GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error) {
	if p, ok := f.peers[id]; ok {
		return p, nil
	}
	return nil, domain.ErrNotFound
}

func (f *expiryMockDB) SavePeer(
	ctx context.Context,
	id domain.PeerIdentifier,
	updateFunc func(in *domain.Peer) (*domain.Peer, error),
) error {
	existing := f.peers[id]
	if existing == nil {
		existing = &domain.Peer{Identifier: id}
	}
	updated, err := updateFunc(existing)
	if err != nil {
		return err
	}
	f.peers[updated.Identifier] = updated
	f.savedPeerData[updated.Identifier] = updated
	return nil
}

func (f *expiryMockDB) DeletePeer(ctx context.Context, id domain.PeerIdentifier) error {
	delete(f.peers, id)
	f.deletedPeers[id] = true
	return nil
}

// buildExpiredPeer creates a peer that is already expired (ExpiresAt in the past) and not disabled.
func buildExpiredPeer(pubKey string, expiresAt time.Time) *domain.Peer {
	createdAt := expiresAt.Add(-24 * time.Hour)
	return &domain.Peer{
		BaseModel: domain.BaseModel{
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		},
		Identifier:          domain.PeerIdentifier(pubKey),
		InterfaceIdentifier: "wg0",
		ExpiresAt:           &expiresAt,
		Interface: domain.PeerInterfaceConfig{
			KeyPair: domain.KeyPair{PublicKey: pubKey},
		},
	}
}

// newExpiryManager builds a Manager with admin context configured for expiry action.
func newExpiryManager(expiryAction string, db *expiryMockDB) (Manager, context.Context) {
	ctrlMgr := &ControllerManager{
		controllers: map[domain.InterfaceBackend]backendInstance{
			config.LocalBackendName: {Implementation: &mockController{}},
		},
	}
	cfg := &config.Config{}
	cfg.Core.Peer.ExpiryAction = expiryAction

	m := Manager{
		cfg:       cfg,
		bus:       &mockBus{},
		db:        db,
		wg:        ctrlMgr,
		notifRepo: newMockNotifRepo(),
	}
	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
	return m, ctx
}

// rfc3339Regex matches an RFC3339 timestamp embedded in a string.
var rfc3339Regex = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`)

// Feature: peer-rotation-interval, Property 13: action=disable → peer disabled with RFC3339 reason
// Validates: Requirements 1.8
func TestProperty13_DisableAction_PeerDisabledWithRFC3339Reason(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a past expiry time (between 1 second and 365 days ago)
		expiredSecsAgo := rapid.Int64Range(1, 365*24*3600).Draw(t, "expiredSecsAgo")
		expiresAt := time.Now().Add(-time.Duration(expiredSecsAgo) * time.Second)

		pubKey := rapid.StringMatching(`[A-Za-z0-9+/]{43}=`).Draw(t, "pubKey")

		iface := &domain.Interface{Identifier: "wg0", Type: domain.InterfaceTypeServer}
		db := newExpiryMockDB(iface)

		peer := buildExpiredPeer(pubKey, expiresAt)
		db.addPeer(peer)

		m, ctx := newExpiryManager("disable", db)

		// Run checkExpiredPeers with the expired peer
		m.checkExpiredPeers(ctx, []domain.Peer{*peer})

		// Assert: peer was saved (updated) with Disabled != nil
		saved, ok := db.savedPeerData[peer.Identifier]
		if !ok {
			t.Fatalf("expected peer %s to be updated via SavePeer, but it was not", peer.Identifier)
		}

		if saved.Disabled == nil {
			t.Fatalf("expected peer.Disabled to be set after disable action, got nil")
		}

		// Assert: DisabledReason contains an RFC3339 timestamp
		if !rfc3339Regex.MatchString(saved.DisabledReason) {
			t.Fatalf("expected DisabledReason to contain RFC3339 timestamp, got %q", saved.DisabledReason)
		}
	})
}

// Feature: peer-rotation-interval, Property 14: action=delete → peer removed from storage
func TestProperty14_DeleteAction_PeerRemovedFromStorage(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a past expiry time (between 1 second and 365 days ago)
		expiredSecsAgo := rapid.Int64Range(1, 365*24*3600).Draw(t, "expiredSecsAgo")
		expiresAt := time.Now().Add(-time.Duration(expiredSecsAgo) * time.Second)

		pubKey := rapid.StringMatching(`[A-Za-z0-9+/]{43}=`).Draw(t, "pubKey")

		iface := &domain.Interface{Identifier: "wg0", Type: domain.InterfaceTypeServer}
		db := newExpiryMockDB(iface)

		peer := buildExpiredPeer(pubKey, expiresAt)
		db.addPeer(peer)

		m, ctx := newExpiryManager("delete", db)

		// Run checkExpiredPeers with the expired peer
		m.checkExpiredPeers(ctx, []domain.Peer{*peer})

		// Assert: peer was deleted from storage
		if !db.deletedPeers[peer.Identifier] {
			t.Fatalf("expected peer %s to be deleted from storage, but DeletePeer was not called", peer.Identifier)
		}

		if _, stillExists := db.peers[peer.Identifier]; stillExists {
			t.Fatalf("expected peer %s to no longer exist in storage after delete action", peer.Identifier)
		}
	})
}
