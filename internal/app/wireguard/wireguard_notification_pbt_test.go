package wireguard

import (
	"context"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// mockNotifRepo is a mock NotificationRepository that tracks calls to
// DeleteNotificationRecordsForPeer and stores seeded records in memory.
type mockNotifRepo struct {
	// records maps peerID → list of notification records (simulates DB state)
	records map[domain.PeerIdentifier][]domain.PeerNotificationRecord
	// deletedPeers tracks which peer IDs had DeleteNotificationRecordsForPeer called
	deletedPeers map[domain.PeerIdentifier]int // count of calls per peer
}

func newMockNotifRepo() *mockNotifRepo {
	return &mockNotifRepo{
		records:      make(map[domain.PeerIdentifier][]domain.PeerNotificationRecord),
		deletedPeers: make(map[domain.PeerIdentifier]int),
	}
}

func (r *mockNotifRepo) seedRecords(peerID domain.PeerIdentifier, count int) {
	recs := make([]domain.PeerNotificationRecord, count)
	for i := range recs {
		recs[i] = domain.PeerNotificationRecord{
			ID:              uint(i + 1),
			PeerIdentifier:  peerID,
			IntervalSeconds: int64((i + 1) * 3600),
			SentAt:          time.Now().Add(-time.Duration(i+1) * time.Hour),
		}
	}
	r.records[peerID] = recs
}

func (r *mockNotifRepo) SaveNotificationRecord(ctx context.Context, rec domain.PeerNotificationRecord) error {
	r.records[rec.PeerIdentifier] = append(r.records[rec.PeerIdentifier], rec)
	return nil
}

func (r *mockNotifRepo) GetNotificationRecords(ctx context.Context, peerID domain.PeerIdentifier) ([]domain.PeerNotificationRecord, error) {
	return r.records[peerID], nil
}

func (r *mockNotifRepo) DeleteNotificationRecordsForPeer(ctx context.Context, peerID domain.PeerIdentifier) error {
	r.deletedPeers[peerID]++
	delete(r.records, peerID)
	return nil
}

func (r *mockNotifRepo) DeleteNotificationRecordsBefore(ctx context.Context, cutoff time.Time) error {
	for peerID, recs := range r.records {
		var kept []domain.PeerNotificationRecord
		for _, rec := range recs {
			if !rec.SentAt.Before(cutoff) {
				kept = append(kept, rec)
			}
		}
		r.records[peerID] = kept
	}
	return nil
}

// mockDBForNotif extends mockDB with full peer lifecycle support needed for UpdatePeer/DeletePeer.
type mockDBForNotif struct {
	mockDB
	peers map[domain.PeerIdentifier]*domain.Peer
}

func newMockDBForNotif(iface *domain.Interface) *mockDBForNotif {
	return &mockDBForNotif{
		mockDB: mockDB{iface: iface},
		peers:  make(map[domain.PeerIdentifier]*domain.Peer),
	}
}

func (d *mockDBForNotif) addPeer(p *domain.Peer) {
	d.peers[p.Identifier] = p
}

func (d *mockDBForNotif) GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error) {
	if p, ok := d.peers[id]; ok {
		// return a copy to avoid mutation issues
		cp := *p
		return &cp, nil
	}
	return nil, domain.ErrNotFound
}

func (d *mockDBForNotif) SavePeer(
	ctx context.Context,
	id domain.PeerIdentifier,
	updateFunc func(in *domain.Peer) (*domain.Peer, error),
) error {
	existing := d.peers[id]
	if existing == nil {
		existing = &domain.Peer{Identifier: id}
	}
	cp := *existing
	updated, err := updateFunc(&cp)
	if err != nil {
		return err
	}
	d.peers[updated.Identifier] = updated
	return nil
}

func (d *mockDBForNotif) DeletePeer(ctx context.Context, id domain.PeerIdentifier) error {
	delete(d.peers, id)
	return nil
}

func (d *mockDBForNotif) GetInterfacePeers(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.Peer, error) {
	var result []domain.Peer
	for _, p := range d.peers {
		if p.InterfaceIdentifier == id {
			result = append(result, *p)
		}
	}
	return result, nil
}

// newNotifTestManager builds a Manager with admin context, a mock notifRepo, and the given DB.
func newNotifTestManager(db *mockDBForNotif, notifRepo *mockNotifRepo) (Manager, context.Context) {
	ctrlMgr := &ControllerManager{
		controllers: map[domain.InterfaceBackend]backendInstance{
			config.LocalBackendName: {Implementation: &mockController{}},
		},
	}
	cfg := &config.Config{}
	m := Manager{
		cfg:       cfg,
		bus:       &mockBus{},
		db:        db,
		wg:        ctrlMgr,
		notifRepo: notifRepo,
	}
	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
	return m, ctx
}

// buildNotifPeer creates a minimal peer with the given public key and ExpiresAt.
func buildNotifPeer(pubKey string, expiresAt *time.Time) *domain.Peer {
	now := time.Now()
	return &domain.Peer{
		BaseModel: domain.BaseModel{
			CreatedAt: now,
			UpdatedAt: now,
		},
		Identifier:          domain.PeerIdentifier(pubKey),
		InterfaceIdentifier: "wg0",
		ExpiresAt:           expiresAt,
		Interface: domain.PeerInterfaceConfig{
			KeyPair: domain.KeyPair{PublicKey: pubKey},
		},
	}
}

// Feature: peer-rotation-interval, Property 10: records cleared when ExpiresAt extended
func TestProperty10_UpdatePeer_ExpiresAtExtended_ClearsNotificationRecords(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		pubKey := rapid.StringMatching(`[A-Za-z0-9+/]{43}=`).Draw(t, "pubKey")

		// Generate an original ExpiresAt in the future (1h to 30 days from now)
		origOffsetHours := rapid.Int64Range(1, 30*24).Draw(t, "origOffsetHours")
		origExpiresAt := time.Now().Add(time.Duration(origOffsetHours) * time.Hour)

		// Generate a new ExpiresAt strictly later than the original (1h to 365 days after original)
		extendHours := rapid.Int64Range(1, 365*24).Draw(t, "extendHours")
		newExpiresAt := origExpiresAt.Add(time.Duration(extendHours) * time.Hour)

		// Generate a number of pre-existing notification records (1 to 5)
		recordCount := rapid.IntRange(1, 5).Draw(t, "recordCount")

		iface := &domain.Interface{Identifier: "wg0", Type: domain.InterfaceTypeServer}
		db := newMockDBForNotif(iface)
		notifRepo := newMockNotifRepo()

		// Seed the peer with an existing ExpiresAt
		peer := buildNotifPeer(pubKey, &origExpiresAt)
		db.addPeer(peer)

		// Pre-seed notification records for this peer
		notifRepo.seedRecords(peer.Identifier, recordCount)

		// Verify records exist before the update
		if len(notifRepo.records[peer.Identifier]) == 0 {
			t.Fatalf("expected pre-seeded notification records to exist before UpdatePeer")
		}

		m, ctx := newNotifTestManager(db, notifRepo)

		// Call UpdatePeer with a later ExpiresAt
		updatedPeer := buildNotifPeer(pubKey, &newExpiresAt)
		_, err := m.UpdatePeer(ctx, updatedPeer)
		if err != nil {
			t.Fatalf("UpdatePeer failed: %v", err)
		}

		// Assert: DeleteNotificationRecordsForPeer was called for this peer
		if notifRepo.deletedPeers[peer.Identifier] == 0 {
			t.Fatalf("expected DeleteNotificationRecordsForPeer to be called for peer %s after ExpiresAt extension, but it was not",
				peer.Identifier)
		}

		// Assert: records are now empty for this peer
		if len(notifRepo.records[peer.Identifier]) != 0 {
			t.Fatalf("expected notification records to be empty after ExpiresAt extension, got %d records",
				len(notifRepo.records[peer.Identifier]))
		}
	})
}

// Feature: peer-rotation-interval, Property 11: records deleted when peer deleted
func TestProperty11_DeletePeer_ClearsNotificationRecords(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		pubKey := rapid.StringMatching(`[A-Za-z0-9+/]{43}=`).Draw(t, "pubKey")

		// Generate an ExpiresAt in the future (optional, peer may or may not have one)
		hasExpiry := rapid.Bool().Draw(t, "hasExpiry")
		var expiresAt *time.Time
		if hasExpiry {
			offsetHours := rapid.Int64Range(1, 365*24).Draw(t, "offsetHours")
			t2 := time.Now().Add(time.Duration(offsetHours) * time.Hour)
			expiresAt = &t2
		}

		// Generate a number of pre-existing notification records (1 to 5)
		recordCount := rapid.IntRange(1, 5).Draw(t, "recordCount")

		iface := &domain.Interface{Identifier: "wg0", Type: domain.InterfaceTypeServer}
		db := newMockDBForNotif(iface)
		notifRepo := newMockNotifRepo()

		// Seed the peer
		peer := buildNotifPeer(pubKey, expiresAt)
		db.addPeer(peer)

		// Pre-seed notification records for this peer
		notifRepo.seedRecords(peer.Identifier, recordCount)

		// Verify records exist before deletion
		if len(notifRepo.records[peer.Identifier]) == 0 {
			t.Fatalf("expected pre-seeded notification records to exist before DeletePeer")
		}

		m, ctx := newNotifTestManager(db, notifRepo)

		// Call DeletePeer
		err := m.DeletePeer(ctx, peer.Identifier)
		if err != nil {
			t.Fatalf("DeletePeer failed: %v", err)
		}

		// Assert: DeleteNotificationRecordsForPeer was called for this peer
		if notifRepo.deletedPeers[peer.Identifier] == 0 {
			t.Fatalf("expected DeleteNotificationRecordsForPeer to be called for peer %s after DeletePeer, but it was not",
				peer.Identifier)
		}

		// Assert: records are now empty for this peer
		if len(notifRepo.records[peer.Identifier]) != 0 {
			t.Fatalf("expected notification records to be empty after DeletePeer, got %d records",
				len(notifRepo.records[peer.Identifier]))
		}
	})
}

// Feature: peer-rotation-interval, Property 10b: records cleared when ExpiresAt newly set (prev==nil)
func TestProperty10b_UpdatePeer_ExpiresAtNewlySet_ClearsNotificationRecords(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		pubKey := rapid.StringMatching(`[A-Za-z0-9+/]{43}=`).Draw(t, "pubKey")

		// Peer starts with no ExpiresAt (nil)
		// Generate a new ExpiresAt in the future
		offsetHours := rapid.Int64Range(1, 365*24).Draw(t, "offsetHours")
		newExpiresAt := time.Now().Add(time.Duration(offsetHours) * time.Hour)

		// Generate a number of pre-existing notification records (1 to 5)
		recordCount := rapid.IntRange(1, 5).Draw(t, "recordCount")

		iface := &domain.Interface{Identifier: "wg0", Type: domain.InterfaceTypeServer}
		db := newMockDBForNotif(iface)
		notifRepo := newMockNotifRepo()

		// Seed the peer with nil ExpiresAt
		peer := buildNotifPeer(pubKey, nil)
		db.addPeer(peer)

		// Pre-seed notification records for this peer
		notifRepo.seedRecords(peer.Identifier, recordCount)

		if len(notifRepo.records[peer.Identifier]) == 0 {
			t.Fatalf("expected pre-seeded notification records to exist before UpdatePeer")
		}

		m, ctx := newNotifTestManager(db, notifRepo)

		// Call UpdatePeer with a newly assigned ExpiresAt (prev was nil)
		updatedPeer := buildNotifPeer(pubKey, &newExpiresAt)
		_, err := m.UpdatePeer(ctx, updatedPeer)
		if err != nil {
			t.Fatalf("UpdatePeer failed: %v", err)
		}

		// Assert: DeleteNotificationRecordsForPeer was called
		if notifRepo.deletedPeers[peer.Identifier] == 0 {
			t.Fatalf("expected DeleteNotificationRecordsForPeer to be called for peer %s when ExpiresAt newly set, but it was not",
				peer.Identifier)
		}

		// Assert: records are now empty
		if len(notifRepo.records[peer.Identifier]) != 0 {
			t.Fatalf("expected notification records to be empty after ExpiresAt newly set, got %d records",
				len(notifRepo.records[peer.Identifier]))
		}
	})
}
