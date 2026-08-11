package wireguard

import (
	"context"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// mockDBWithPeer extends mockDB to allow controlling GetPeer responses.
type mockDBWithPeer struct {
	mockDB
	existingPeer *domain.Peer
}

func (f *mockDBWithPeer) GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error) {
	if f.existingPeer != nil && f.existingPeer.Identifier == id {
		return f.existingPeer, nil
	}
	return nil, domain.ErrNotFound
}

// newAdminManager builds a Manager with admin context and the given config.
func newAdminManager(cfg *config.Config, db *mockDBWithPeer) (Manager, context.Context) {
	ctrlMgr := &ControllerManager{
		controllers: map[domain.InterfaceBackend]backendInstance{
			config.LocalBackendName: {Implementation: &mockController{}},
		},
	}
	m := Manager{
		cfg:       cfg,
		bus:       &mockBus{},
		db:        db,
		wg:        ctrlMgr,
		notifRepo: noopNotificationRepository{},
	}
	ctx := domain.SetUserInfo(context.Background(), &domain.ContextUserInfo{
		Id:      "admin@example.com",
		IsAdmin: true,
	})
	return m, ctx
}

// buildPeer creates a minimal peer with the given public key and CreatedAt.
func buildPeer(pubKey string, createdAt time.Time, expiresAt *time.Time) *domain.Peer {
	return &domain.Peer{
		BaseModel: domain.BaseModel{
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		},
		Identifier:          domain.PeerIdentifier(pubKey),
		InterfaceIdentifier: "wg0",
		ExpiresAt:           expiresAt,
		Interface: domain.PeerInterfaceConfig{
			KeyPair: domain.KeyPair{PublicKey: pubKey},
		},
	}
}

// Feature: peer-rotation-interval, Property 1: interval=0 → ExpiresAt==nil
func TestProperty1_ZeroInterval_NoExpiresAt(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random creation time within a reasonable range
		offsetSec := rapid.Int64Range(-365*24*3600, 365*24*3600).Draw(t, "offsetSec")
		createdAt := time.Now().Add(time.Duration(offsetSec) * time.Second)

		pubKey := rapid.StringMatching(`[A-Za-z0-9+/]{43}=`).Draw(t, "pubKey")

		cfg := &config.Config{}
		cfg.Core.Peer.RotationInterval = 0 // disabled
		cfg.Core.SelfProvisioningAllowed = false

		db := &mockDBWithPeer{
			mockDB: mockDB{
				iface: &domain.Interface{Identifier: "wg0", Type: domain.InterfaceTypeServer},
			},
		}
		m, ctx := newAdminManager(cfg, db)

		peer := buildPeer(pubKey, createdAt, nil)
		out, err := m.CreatePeer(ctx, peer)
		if err != nil {
			t.Fatalf("CreatePeer failed: %v", err)
		}

		if out.ExpiresAt != nil {
			t.Fatalf("expected ExpiresAt to be nil when PeerRotationInterval=0, got %v", out.ExpiresAt)
		}
	})
}

// Feature: peer-rotation-interval, Property 2: ExpiresAt ≈ now + interval
// Regression: ExpiresAt must be based on time.Now(), not peer.CreatedAt (which is zero before DB insert).
func TestProperty2_PositiveInterval_SetsExpiresAt(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a positive rotation interval between 1 hour and 1 year
		intervalHours := rapid.Int64Range(1, 365*24).Draw(t, "intervalHours")
		interval := time.Duration(intervalHours) * time.Hour

		pubKey := rapid.StringMatching(`[A-Za-z0-9+/]{43}=`).Draw(t, "pubKey")

		cfg := &config.Config{}
		cfg.Core.Peer.RotationInterval = interval
		cfg.Core.SelfProvisioningAllowed = false

		db := &mockDBWithPeer{
			mockDB: mockDB{
				iface: &domain.Interface{Identifier: "wg0", Type: domain.InterfaceTypeServer},
			},
		}
		m, ctx := newAdminManager(cfg, db)

		// Deliberately pass a zero CreatedAt to simulate the pre-save state.
		// The bug was: expiresAt = peer.CreatedAt.Add(interval) → zero time + interval.
		peer := buildPeer(pubKey, time.Time{}, nil)

		before := time.Now()
		out, err := m.CreatePeer(ctx, peer)
		after := time.Now()

		if err != nil {
			t.Fatalf("CreatePeer failed: %v", err)
		}

		if out.ExpiresAt == nil {
			t.Fatalf("expected ExpiresAt to be set when PeerRotationInterval=%v, got nil", interval)
		}

		// ExpiresAt must be in [before+interval, after+interval], not near the zero time.
		low := before.Add(interval)
		high := after.Add(interval)
		if out.ExpiresAt.Before(low) || out.ExpiresAt.After(high) {
			t.Fatalf("ExpiresAt=%v is not in expected range [%v, %v]; likely computed from zero CreatedAt instead of time.Now()",
				*out.ExpiresAt, low, high)
		}
	})
}

// Regression test: peer with zero CreatedAt must still get a valid ExpiresAt.
// This directly reproduces the bug where ExpiresAt was set to ~year 0001.
func TestCreatePeer_ZeroCreatedAt_ExpiresAtBasedOnNow(t *testing.T) {
	interval := 24 * time.Hour
	pubKey := "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

	cfg := &config.Config{}
	cfg.Core.Peer.RotationInterval = interval
	cfg.Core.SelfProvisioningAllowed = false

	db := &mockDBWithPeer{
		mockDB: mockDB{
			iface: &domain.Interface{Identifier: "wg0", Type: domain.InterfaceTypeServer},
		},
	}
	m, ctx := newAdminManager(cfg, db)

	peer := buildPeer(pubKey, time.Time{}, nil) // zero CreatedAt, as it is before DB insert

	before := time.Now()
	out, err := m.CreatePeer(ctx, peer)
	after := time.Now()

	if err != nil {
		t.Fatalf("CreatePeer failed: %v", err)
	}
	if out.ExpiresAt == nil {
		t.Fatal("expected ExpiresAt to be non-nil")
	}

	low := before.Add(interval)
	high := after.Add(interval)
	if out.ExpiresAt.Before(low) || out.ExpiresAt.After(high) {
		t.Fatalf("ExpiresAt=%v out of range [%v, %v]; was it computed from zero CreatedAt?",
			*out.ExpiresAt, low, high)
	}
}

// Feature: peer-rotation-interval, Property 3: explicit ExpiresAt is never overwritten
func TestProperty3_ExplicitExpiresAt_NotOverwritten(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a positive rotation interval
		intervalHours := rapid.Int64Range(1, 365*24).Draw(t, "intervalHours")
		interval := time.Duration(intervalHours) * time.Hour

		// Generate a random creation time
		offsetSec := rapid.Int64Range(-365*24*3600, 365*24*3600).Draw(t, "offsetSec")
		createdAt := time.Now().Add(time.Duration(offsetSec) * time.Second)

		// Generate an explicit ExpiresAt (different from what the interval would compute)
		expiryOffsetHours := rapid.Int64Range(1, 10000).Draw(t, "expiryOffsetHours")
		explicitExpiry := createdAt.Add(time.Duration(expiryOffsetHours) * time.Hour)

		pubKey := rapid.StringMatching(`[A-Za-z0-9+/]{43}=`).Draw(t, "pubKey")

		cfg := &config.Config{}
		cfg.Core.Peer.RotationInterval = interval
		cfg.Core.SelfProvisioningAllowed = false

		db := &mockDBWithPeer{
			mockDB: mockDB{
				iface: &domain.Interface{Identifier: "wg0", Type: domain.InterfaceTypeServer},
			},
		}
		m, ctx := newAdminManager(cfg, db)

		peer := buildPeer(pubKey, createdAt, &explicitExpiry)
		out, err := m.CreatePeer(ctx, peer)
		if err != nil {
			t.Fatalf("CreatePeer failed: %v", err)
		}

		if out.ExpiresAt == nil {
			t.Fatalf("expected ExpiresAt to remain set, got nil")
		}

		if !out.ExpiresAt.Equal(explicitExpiry) {
			t.Fatalf("expected ExpiresAt to be preserved as %v, got %v", explicitExpiry, *out.ExpiresAt)
		}
	})
}
