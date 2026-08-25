package wireguard

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

func filterTestManager(peers []domain.Peer) (Manager, *mockDB) {
	byId := make(map[domain.PeerIdentifier]*domain.Peer, len(peers))
	for i := range peers {
		byId[peers[i].Identifier] = &peers[i]
	}

	db := &mockDB{
		iface:          &domain.Interface{Identifier: "wg1", Type: domain.InterfaceTypeServer},
		interfacePeers: peers,
		peers:          byId,
	}

	return Manager{
		cfg: &config.Config{},
		bus: &mockBus{},
		db:  db,
		wg: &ControllerManager{
			controllers: map[domain.InterfaceBackend]backendInstance{
				config.LocalBackendName: {Implementation: &mockController{}},
			},
		},
	}, db
}

// savedPeer returns the single peer the manager wrote, if any. The mock keys its
// map by a field UpdatePeer does not carry through, so the count is what matters.
func savedPeer(db *mockDB) *domain.Peer {
	for _, p := range db.savedPeers {
		return p
	}
	return nil
}

func ownedPeer(id domain.PeerIdentifier, owner domain.UserIdentifier) domain.Peer {
	return domain.Peer{
		Identifier:          id,
		InterfaceIdentifier: "wg1",
		UserIdentifier:      owner,
		Interface:           domain.PeerInterfaceConfig{Type: domain.InterfaceTypeServer},
		DisplayName:         string(id),
	}
}

// A user who has dropped out of the filter loses the peer they already hold.
func TestInterfaceLdapFilterApplied_RevokesPeerOfRemovedUser(t *testing.T) {
	m, db := filterTestManager([]domain.Peer{ownedPeer("peer-bob", "bob")})

	m.handleInterfaceLdapFilterAppliedEvent("wg1", []domain.UserIdentifier{"alice"})

	saved := savedPeer(db)
	require.NotNil(t, saved, "bob's peer should have been written")
	assert.True(t, saved.IsDisabled())
	assert.Equal(t, domain.DisabledReasonAccessRevoked, saved.DisabledReason)
}

// Revocation has to be reversible, or a demotion that is later undone leaves the
// peer dead with nothing to bring it back.
func TestInterfaceLdapFilterApplied_RestoresPeerWhenUserReturns(t *testing.T) {
	revoked := time.Now().Add(-time.Hour)
	peer := ownedPeer("peer-bob", "bob")
	peer.Disabled = &revoked
	peer.DisabledReason = domain.DisabledReasonAccessRevoked

	m, db := filterTestManager([]domain.Peer{peer})

	m.handleInterfaceLdapFilterAppliedEvent("wg1", []domain.UserIdentifier{"bob"})

	saved := savedPeer(db)
	require.NotNil(t, saved, "bob's peer should have been written")
	assert.False(t, saved.IsDisabled())
	assert.Empty(t, saved.DisabledReason)
}

// A peer an administrator disabled by hand must not be resurrected just because
// its owner matches the filter.
func TestInterfaceLdapFilterApplied_LeavesPeersDisabledForOtherReasons(t *testing.T) {
	disabled := time.Now().Add(-time.Hour)
	peer := ownedPeer("peer-bob", "bob")
	peer.Disabled = &disabled
	peer.DisabledReason = domain.DisabledReasonUserDisabled

	m, db := filterTestManager([]domain.Peer{peer})

	m.handleInterfaceLdapFilterAppliedEvent("wg1", []domain.UserIdentifier{"bob"})

	assert.Empty(t, db.savedPeers, "a peer disabled for another reason must be left alone")
}

// Imported peers and peers an administrator created have no owner, so no filter
// governs them.
func TestInterfaceLdapFilterApplied_IgnoresPeersWithoutOwner(t *testing.T) {
	m, db := filterTestManager([]domain.Peer{ownedPeer("peer-orphan", "")})

	m.handleInterfaceLdapFilterAppliedEvent("wg1", []domain.UserIdentifier{"alice"})

	assert.Empty(t, db.savedPeers, "an unowned peer must not be revoked")
}
