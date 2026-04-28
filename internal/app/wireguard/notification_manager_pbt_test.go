package wireguard

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// ---------------------------------------------------------------------------
// Mock: NotificationMailer
// ---------------------------------------------------------------------------

// mockMailer counts SendExpiryNotification calls and can optionally fail for
// specific peer identifiers.
type mockMailer struct {
	callCount  int64
	failPeerID domain.PeerIdentifier // if set, return error for this peer
}

func (m *mockMailer) SendExpiryNotification(
	_ context.Context,
	peer *domain.Peer,
	_ *domain.User,
	_ int,
) error {
	if peer.Identifier == m.failPeerID {
		return errors.New("mock send failure")
	}
	atomic.AddInt64(&m.callCount, 1)
	return nil
}

func (m *mockMailer) calls() int {
	return int(atomic.LoadInt64(&m.callCount))
}

// ---------------------------------------------------------------------------
// Mock: NotificationManagerDatabaseRepo
// ---------------------------------------------------------------------------

// nmMockDB implements NotificationManagerDatabaseRepo.
// It holds a list of interfaces and a map of peers per interface.
type nmMockDB struct {
	interfaces []domain.Interface
	// peersByIface maps interface identifier → peers
	peersByIface map[domain.InterfaceIdentifier][]domain.Peer
	// users maps user identifier → user
	users map[domain.UserIdentifier]*domain.User
}

func newNMDB() *nmMockDB {
	return &nmMockDB{
		peersByIface: make(map[domain.InterfaceIdentifier][]domain.Peer),
		users:        make(map[domain.UserIdentifier]*domain.User),
	}
}

func (d *nmMockDB) addInterface(iface domain.Interface) {
	d.interfaces = append(d.interfaces, iface)
}

func (d *nmMockDB) addPeer(ifaceID domain.InterfaceIdentifier, peer domain.Peer) {
	d.peersByIface[ifaceID] = append(d.peersByIface[ifaceID], peer)
}

func (d *nmMockDB) addUser(user *domain.User) {
	d.users[user.Identifier] = user
}

func (d *nmMockDB) GetAllInterfaces(_ context.Context) ([]domain.Interface, error) {
	return d.interfaces, nil
}

func (d *nmMockDB) GetInterfacePeers(_ context.Context, id domain.InterfaceIdentifier) ([]domain.Peer, error) {
	return d.peersByIface[id], nil
}

func (d *nmMockDB) GetUser(_ context.Context, id domain.UserIdentifier) (*domain.User, error) {
	u, ok := d.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildNMConfig returns a config with notifications enabled and the given intervals.
func buildNMConfig(enabled bool, intervals []time.Duration, checkInterval time.Duration, retention time.Duration) *config.Config {
	cfg := &config.Config{}
	cfg.Core.Peer.ExpiryNotificationEnabled = enabled
	cfg.Core.Peer.ExpiryNotificationIntervals = intervals
	cfg.Core.Peer.NotificationCleanupAfter = retention
	cfg.Advanced.ExpiryCheckInterval = checkInterval
	return cfg
}

// buildNMPeer creates a peer with the given expiry and optional user link.
func buildNMPeer(id string, expiresAt *time.Time, userID domain.UserIdentifier) domain.Peer {
	now := time.Now()
	return domain.Peer{
		BaseModel: domain.BaseModel{
			CreatedAt: now,
			UpdatedAt: now,
		},
		Identifier:          domain.PeerIdentifier(id),
		InterfaceIdentifier: "wg0",
		ExpiresAt:           expiresAt,
		UserIdentifier:      userID,
		Interface: domain.PeerInterfaceConfig{
			KeyPair: domain.KeyPair{PublicKey: id},
		},
	}
}

// buildNMUser creates a user with the given email.
func buildNMUser(id domain.UserIdentifier, email string) *domain.User {
	return &domain.User{
		Identifier: id,
		Email:      email,
	}
}

// newNMForTest builds a NotificationManager with the given components.
func newNMForTest(
	cfg *config.Config,
	db NotificationManagerDatabaseRepo,
	notifRepo NotificationRepository,
	mailer NotificationMailer,
) *NotificationManager {
	return NewNotificationManager(cfg, db, notifRepo, mailer)
}

// ---------------------------------------------------------------------------
// Property 4: PeerExpiryNotificationEnabled=false → 0 emails sent
// Property 5: PeerExpiryNotificationIntervals=[] → 0 emails sent
// Validates: Requirements 3.4, 3.5
// ---------------------------------------------------------------------------

// Feature: peer-rotation-interval, Property 4/5: notifications disabled or empty intervals → 0 emails
func TestProperty4_NotificationsDisabled_ZeroEmails(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate 0–5 peers with random expiry times
		numPeers := rapid.IntRange(0, 5).Draw(t, "numPeers")

		db := newNMDB()
		iface := domain.Interface{Identifier: "wg0", Type: domain.InterfaceTypeServer}
		db.addInterface(iface)

		for i := 0; i < numPeers; i++ {
			peerID := rapid.StringMatching(`[A-Za-z0-9]{8}`).Draw(t, "peerID")
			offsetHours := rapid.Int64Range(-100, 100).Draw(t, "offsetHours")
			exp := time.Now().Add(time.Duration(offsetHours) * time.Hour)
			userID := domain.UserIdentifier("user-" + peerID)
			peer := buildNMPeer(peerID, &exp, userID)
			db.addPeer("wg0", peer)
			db.addUser(buildNMUser(userID, peerID+"@example.com"))
		}

		notifRepo := newMockNotifRepo()
		mailer := &mockMailer{}

		// PeerExpiryNotificationEnabled = false
		cfg := buildNMConfig(false, []time.Duration{24 * time.Hour}, time.Hour, 720*time.Hour)
		nm := newNMForTest(cfg, db, notifRepo, mailer)

		nm.checkAndNotify(context.Background())

		if mailer.calls() != 0 {
			t.Fatalf("Property 4: expected 0 emails when notifications disabled, got %d", mailer.calls())
		}
	})
}

// Feature: peer-rotation-interval, Property 4/5: notifications disabled or empty intervals → 0 emails
func TestProperty5_EmptyIntervals_ZeroEmails(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numPeers := rapid.IntRange(0, 5).Draw(t, "numPeers")

		db := newNMDB()
		iface := domain.Interface{Identifier: "wg0", Type: domain.InterfaceTypeServer}
		db.addInterface(iface)

		for i := 0; i < numPeers; i++ {
			peerID := rapid.StringMatching(`[A-Za-z0-9]{8}`).Draw(t, "peerID")
			offsetHours := rapid.Int64Range(-100, 100).Draw(t, "offsetHours")
			exp := time.Now().Add(time.Duration(offsetHours) * time.Hour)
			userID := domain.UserIdentifier("user-" + peerID)
			peer := buildNMPeer(peerID, &exp, userID)
			db.addPeer("wg0", peer)
			db.addUser(buildNMUser(userID, peerID+"@example.com"))
		}

		notifRepo := newMockNotifRepo()
		mailer := &mockMailer{}

		// PeerExpiryNotificationIntervals = []
		cfg := buildNMConfig(true, []time.Duration{}, time.Hour, 720*time.Hour)
		nm := newNMForTest(cfg, db, notifRepo, mailer)

		nm.checkAndNotify(context.Background())

		if mailer.calls() != 0 {
			t.Fatalf("Property 5: expected 0 emails when intervals empty, got %d", mailer.calls())
		}
	})
}

// ---------------------------------------------------------------------------
// Property 6: at-most-once per peer per interval
// Validates: Requirements 4.2, 6.1, 6.2
// ---------------------------------------------------------------------------

// Feature: peer-rotation-interval, Property 6: at-most-once notification per peer per interval
func TestProperty6_AtMostOnce_PerPeerPerInterval(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		peerID := rapid.StringMatching(`[A-Za-z0-9]{8}`).Draw(t, "peerID")

		// Pick a notification interval between 1h and 168h.
		intervalHours := rapid.Int64Range(1, 168).Draw(t, "intervalHours")
		notifInterval := time.Duration(intervalHours) * time.Hour

		// Set ExpiresAt = now + notifInterval/2 so the threshold (ExpiresAt - D) is in the past.
		// This means the notification is due, but we pre-seed a record to simulate already sent.
		expiresAt := time.Now().Add(notifInterval / 2)

		userID := domain.UserIdentifier("user-" + peerID)
		peer := buildNMPeer(peerID, &expiresAt, userID)

		db := newNMDB()
		iface := domain.Interface{Identifier: "wg0", Type: domain.InterfaceTypeServer}
		db.addInterface(iface)
		db.addPeer("wg0", peer)
		db.addUser(buildNMUser(userID, peerID+"@example.com"))

		notifRepo := newMockNotifRepo()

		// Pre-seed a notification record for (peer, notifInterval) — simulates already sent.
		existingRec := domain.PeerNotificationRecord{
			PeerIdentifier:  peer.Identifier,
			IntervalSeconds: int64(notifInterval.Seconds()),
			SentAt:          time.Now().Add(-time.Hour),
		}
		notifRepo.records[peer.Identifier] = []domain.PeerNotificationRecord{existingRec}

		mailer := &mockMailer{}
		cfg := buildNMConfig(true, []time.Duration{notifInterval}, time.Hour, 720*time.Hour)
		nm := newNMForTest(cfg, db, notifRepo, mailer)

		nm.checkAndNotify(context.Background())

		if mailer.calls() != 0 {
			t.Fatalf("Property 6: expected 0 additional emails after record already exists, got %d", mailer.calls())
		}
	})
}

// Feature: peer-rotation-interval, Property 6b: missed notification is sent in the next cycle
// Validates: Requirements 4.2 — threshold check ensures missed notifications are not permanently lost
func TestProperty6b_MissedNotification_SentInNextCycle(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		peerID := rapid.StringMatching(`[A-Za-z0-9]{8}`).Draw(t, "peerID")

		// Pick a notification interval between 1h and 168h.
		intervalHours := rapid.Int64Range(1, 168).Draw(t, "intervalHours")
		notifInterval := time.Duration(intervalHours) * time.Hour

		// Set ExpiresAt such that the threshold (ExpiresAt - D) is in the past:
		// ExpiresAt = now + notifInterval/2 → ExpiresAt - D = now - notifInterval/2 < now.
		// This simulates a notification that was due in a prior cycle but was missed.
		expiresAt := time.Now().Add(notifInterval / 2)

		userID := domain.UserIdentifier("user-" + peerID)
		peer := buildNMPeer(peerID, &expiresAt, userID)

		db := newNMDB()
		iface := domain.Interface{Identifier: "wg0", Type: domain.InterfaceTypeServer}
		db.addInterface(iface)
		db.addPeer("wg0", peer)
		db.addUser(buildNMUser(userID, peerID+"@example.com"))

		// No pre-seeded record — simulates the notification was missed in a prior cycle.
		notifRepo := newMockNotifRepo()
		mailer := &mockMailer{}
		cfg := buildNMConfig(true, []time.Duration{notifInterval}, time.Hour, 720*time.Hour)
		nm := newNMForTest(cfg, db, notifRepo, mailer)

		nm.checkAndNotify(context.Background())

		// The threshold is already passed and no record exists → exactly 1 email should be sent.
		if mailer.calls() != 1 {
			t.Fatalf("Property 6b: expected 1 email for missed notification (threshold passed, no record), got %d",
				mailer.calls())
		}
	})
}

// ---------------------------------------------------------------------------
// Property 7: peers without email/user/disabled/nil-expiry are silently skipped
// Property 8: send failure for one peer does not stop remaining peers
// Validates: Requirements 4.3, 4.4, 4.5, 4.7
// ---------------------------------------------------------------------------

// Feature: peer-rotation-interval, Property 7/8: skip conditions and failure isolation
func TestProperty7_SkipConditions_ZeroEmails(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Pick one of the four skip conditions at random.
		skipCase := rapid.IntRange(0, 3).Draw(t, "skipCase")

		db := newNMDB()
		iface := domain.Interface{Identifier: "wg0", Type: domain.InterfaceTypeServer}
		db.addInterface(iface)

		peerID := "peer-skip-test"
		checkInterval := time.Hour
		notifInterval := 24 * time.Hour
		expiresAt := time.Now().Add(notifInterval) // center of window

		switch skipCase {
		case 0:
			// nil ExpiresAt
			peer := buildNMPeer(peerID, nil, "user1")
			db.addPeer("wg0", peer)
			db.addUser(buildNMUser("user1", "user1@example.com"))

		case 1:
			// disabled peer
			now := time.Now()
			peer := buildNMPeer(peerID, &expiresAt, "user1")
			peer.Disabled = &now
			db.addPeer("wg0", peer)
			db.addUser(buildNMUser("user1", "user1@example.com"))

		case 2:
			// no linked user (empty UserIdentifier)
			peer := buildNMPeer(peerID, &expiresAt, "")
			db.addPeer("wg0", peer)

		case 3:
			// user has no email
			peer := buildNMPeer(peerID, &expiresAt, "user-noemail")
			db.addPeer("wg0", peer)
			db.addUser(buildNMUser("user-noemail", "")) // empty email
		}

		notifRepo := newMockNotifRepo()
		mailer := &mockMailer{}
		cfg := buildNMConfig(true, []time.Duration{notifInterval}, checkInterval, 720*time.Hour)
		nm := newNMForTest(cfg, db, notifRepo, mailer)

		// Should not panic and should send 0 emails.
		nm.checkAndNotify(context.Background())

		if mailer.calls() != 0 {
			t.Fatalf("Property 7 (case %d): expected 0 emails for skip condition, got %d", skipCase, mailer.calls())
		}
	})
}

// Feature: peer-rotation-interval, Property 7/8: send failure does not stop remaining peers
func TestProperty8_SendFailure_DoesNotStopRemainingPeers(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate 2–6 peers; the first one will fail to send.
		numPeers := rapid.IntRange(2, 6).Draw(t, "numPeers")

		db := newNMDB()
		iface := domain.Interface{Identifier: "wg0", Type: domain.InterfaceTypeServer}
		db.addInterface(iface)

		checkInterval := time.Hour
		notifInterval := 24 * time.Hour

		var failingPeerID domain.PeerIdentifier
		for i := 0; i < numPeers; i++ {
			peerID := domain.PeerIdentifier("peer-" + string(rune('A'+i)))
			userID := domain.UserIdentifier("user-" + string(rune('A'+i)))
			expiresAt := time.Now().Add(notifInterval) // center of window
			peer := buildNMPeer(string(peerID), &expiresAt, userID)
			db.addPeer("wg0", peer)
			db.addUser(buildNMUser(userID, string(peerID)+"@example.com"))
			if i == 0 {
				failingPeerID = peerID
			}
		}

		notifRepo := newMockNotifRepo()
		mailer := &mockMailer{failPeerID: failingPeerID}
		cfg := buildNMConfig(true, []time.Duration{notifInterval}, checkInterval, 720*time.Hour)
		nm := newNMForTest(cfg, db, notifRepo, mailer)

		nm.checkAndNotify(context.Background())

		// The failing peer sends 0 emails; the remaining (numPeers-1) should each send 1.
		expected := numPeers - 1
		if mailer.calls() != expected {
			t.Fatalf("Property 8: expected %d emails (remaining peers after failure), got %d",
				expected, mailer.calls())
		}
	})
}

// ---------------------------------------------------------------------------
// Property 12: NotificationManager stops on context cancellation
// Validates: Requirements 7.3
// ---------------------------------------------------------------------------

// Feature: peer-rotation-interval, Property 12: NotificationManager stops on context cancellation
func TestProperty12_ContextCancellation_ManagerStops(t *testing.T) {
	// Use a very short check interval so the test is fast.
	checkInterval := 50 * time.Millisecond

	db := newNMDB()
	notifRepo := newMockNotifRepo()
	mailer := &mockMailer{}

	cfg := buildNMConfig(false, []time.Duration{}, checkInterval, 720*time.Hour)
	nm := newNMForTest(cfg, db, notifRepo, mailer)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		nm.Run(ctx)
		close(done)
	}()

	// Cancel the context and assert the goroutine exits within 2 check intervals.
	cancel()

	select {
	case <-done:
		// success
	case <-time.After(2 * checkInterval):
		t.Fatal("Property 12: NotificationManager did not stop within 2 check intervals after context cancellation")
	}
}

// ---------------------------------------------------------------------------
// Property 15: records older than retention period are pruned
// Validates: Requirements 3.7 (notification state hygiene)
// ---------------------------------------------------------------------------

// Feature: peer-rotation-interval, Property 15: records older than retention period are pruned
func TestProperty15_OldRecordsPruned(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a retention period between 1h and 720h.
		retentionHours := rapid.Int64Range(1, 720).Draw(t, "retentionHours")
		retention := time.Duration(retentionHours) * time.Hour

		// Generate 1–5 old records whose SentAt is older than now - retention.
		numOldRecords := rapid.IntRange(1, 5).Draw(t, "numOldRecords")

		db := newNMDB()
		iface := domain.Interface{Identifier: "wg0", Type: domain.InterfaceTypeServer}
		db.addInterface(iface)

		notifRepo := newMockNotifRepo()

		// Insert old records (SentAt = now - retention - 1h, guaranteed stale).
		stalePeerID := domain.PeerIdentifier("stale-peer")
		for i := 0; i < numOldRecords; i++ {
			rec := domain.PeerNotificationRecord{
				ID:              uint(i + 1),
				PeerIdentifier:  stalePeerID,
				IntervalSeconds: int64((i + 1) * 3600),
				SentAt:          time.Now().Add(-retention - time.Hour),
			}
			notifRepo.records[stalePeerID] = append(notifRepo.records[stalePeerID], rec)
		}

		// Verify old records exist before the check.
		if len(notifRepo.records[stalePeerID]) != numOldRecords {
			t.Fatalf("expected %d old records before checkAndNotify, got %d",
				numOldRecords, len(notifRepo.records[stalePeerID]))
		}

		mailer := &mockMailer{}
		// Notifications must be enabled with at least one interval so checkAndNotify
		// doesn't early-return before reaching the pruning step.
		cfg := buildNMConfig(true, []time.Duration{24 * time.Hour}, time.Hour, retention)
		nm := newNMForTest(cfg, db, notifRepo, mailer)

		nm.checkAndNotify(context.Background())

		// All stale records should have been pruned.
		if len(notifRepo.records[stalePeerID]) != 0 {
			t.Fatalf("Property 15: expected 0 records after pruning, got %d",
				len(notifRepo.records[stalePeerID]))
		}
	})
}
