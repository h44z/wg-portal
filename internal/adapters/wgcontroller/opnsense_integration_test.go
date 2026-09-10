//go:build integration

package wgcontroller

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// These tests run against a real OPNsense instance. They are skipped unless the
// connection details are supplied:
//
//	WG_PORTAL_OPNSENSE_URL=https://10.177.0.1 \
//	WG_PORTAL_OPNSENSE_KEY=... \
//	WG_PORTAL_OPNSENSE_SECRET=... \
//	WG_PORTAL_OPNSENSE_INTERFACE=wg0 \
//	go test -tags integration ./internal/adapters/wgcontroller/ -run Opnsense -v
//
// The tests create and remove their own peer; they do not modify the tunnel or
// any peer they did not create.

func opnsenseTestController(t *testing.T) (*OpnsenseController, domain.InterfaceIdentifier) {
	t.Helper()

	apiUrl := os.Getenv("WG_PORTAL_OPNSENSE_URL")
	apiKey := os.Getenv("WG_PORTAL_OPNSENSE_KEY")
	apiSecret := os.Getenv("WG_PORTAL_OPNSENSE_SECRET")
	iface := os.Getenv("WG_PORTAL_OPNSENSE_INTERFACE")

	if apiUrl == "" || apiKey == "" || apiSecret == "" {
		t.Skip("set WG_PORTAL_OPNSENSE_URL/KEY/SECRET to run OPNsense integration tests")
	}
	if iface == "" {
		iface = "wg0"
	}

	controller, err := NewOpnsenseController(&config.Config{}, &config.BackendOpnsense{
		BackendBase:  config.BackendBase{Id: "opnsense-test"},
		ApiUrl:       apiUrl,
		ApiKey:       apiKey,
		ApiSecret:    apiSecret,
		ApiVerifyTls: false,
		ApiTimeout:   30 * time.Second,
	})
	require.NoError(t, err)

	return controller, domain.InterfaceIdentifier(iface)
}

func TestOpnsenseGetInterfaces(t *testing.T) {
	controller, iface := opnsenseTestController(t)
	ctx := context.Background()

	interfaces, err := controller.GetInterfaces(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, interfaces, "expected at least one WireGuard tunnel on the firewall")

	var found *domain.PhysicalInterface
	for i := range interfaces {
		if interfaces[i].Identifier == iface {
			found = &interfaces[i]
			break
		}
	}
	require.NotNil(t, found, "tunnel %s not found in %v", iface, interfaces)

	// The identifier must be the device name so that an imported interface
	// looks the same as one managed by the local backend.
	assert.Equal(t, iface, found.Identifier)
	assert.NotEmpty(t, found.KeyPair.PublicKey, "public key should be populated")
	assert.NotEmpty(t, found.Addresses, "tunnel addresses should be parsed")
	assert.Greater(t, found.ListenPort, 0, "listen port should be populated")
	assert.Equal(t, domain.ControllerTypeOpnsense, found.ImportSource)

	extras, ok := found.GetExtras().(domain.OpnsenseInterfaceExtras)
	require.True(t, ok, "extras should be OpnsenseInterfaceExtras")
	assert.NotEmpty(t, extras.Uuid, "the OPNsense UUID must be carried in extras")

	// GetInterface must agree with GetInterfaces.
	single, err := controller.GetInterface(ctx, iface)
	require.NoError(t, err)
	assert.Equal(t, found.Identifier, single.Identifier)
	assert.Equal(t, found.KeyPair.PublicKey, single.KeyPair.PublicKey)
}

func TestOpnsenseGetPeers(t *testing.T) {
	controller, iface := opnsenseTestController(t)
	ctx := context.Background()

	peers, err := controller.GetPeers(ctx, iface)
	require.NoError(t, err)
	require.NotEmpty(t, peers, "expected at least one peer on %s", iface)

	for _, peer := range peers {
		assert.NotEmpty(t, peer.Identifier, "peer identifier (public key) must be set")
		assert.Equal(t, string(peer.Identifier), peer.KeyPair.PublicKey)
		assert.Equal(t, domain.ControllerTypeOpnsense, peer.ImportSource)

		extras, ok := peer.GetExtras().(domain.OpnsensePeerExtras)
		require.True(t, ok)
		assert.NotEmpty(t, extras.Uuid)
	}
}

// A connected peer should report handshake and transfer counters, which the
// pfSense backend cannot supply. This asserts the service/show plumbing works;
// it tolerates the case where no peer is currently connected.
func TestOpnsensePeerStatistics(t *testing.T) {
	controller, iface := opnsenseTestController(t)
	ctx := context.Background()

	peers, err := controller.GetPeers(ctx, iface)
	require.NoError(t, err)

	var withHandshake int
	for _, peer := range peers {
		if !peer.LastHandshake.IsZero() {
			withHandshake++
			assert.Greater(t, peer.BytesDownload+peer.BytesUpload, uint64(0),
				"a peer that has handshaken should have moved some bytes")
		}
	}
	t.Logf("%d of %d peers have completed a handshake", withHandshake, len(peers))
}

// Full create -> read -> update -> delete cycle for a peer, which is the path
// wg-portal exercises when an LDAP user gains or loses VPN access.
func TestOpnsenseSaveAndDeletePeer(t *testing.T) {
	controller, iface := opnsenseTestController(t)
	ctx := context.Background()

	// A syntactically valid, deterministic key that will not collide with the
	// testbed's real peers.
	const testKey = "TEsT0000integrationTESTkey0000000000000000k="
	peerId := domain.PeerIdentifier(testKey)

	peersBefore, err := controller.GetPeers(ctx, iface)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := controller.DeletePeer(context.Background(), iface, peerId); err != nil {
			t.Logf("cleanup: failed to delete test peer: %v", err)
		}
	})

	allowed, err := domain.CidrsFromString("10.99.0.240/32")
	require.NoError(t, err)

	err = controller.SavePeer(ctx, iface, peerId, func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error) {
		pp.AllowedIPs = allowed
		pp.PersistentKeepalive = 25
		pp.SetExtras(domain.OpnsensePeerExtras{
			Name:     "wg-portal-integration-test",
			Disabled: false,
		})
		return pp, nil
	})
	require.NoError(t, err, "creating a peer should succeed")

	// Read it back.
	peersAfter, err := controller.GetPeers(ctx, iface)
	require.NoError(t, err)
	assert.Len(t, peersAfter, len(peersBefore)+1, "exactly one peer should have been added")

	var created *domain.PhysicalPeer
	for i := range peersAfter {
		if peersAfter[i].Identifier == peerId {
			created = &peersAfter[i]
			break
		}
	}
	require.NotNil(t, created, "the created peer should be readable back")
	assert.Equal(t, "10.99.0.240/32", domain.CidrsToString(created.AllowedIPs))
	assert.Equal(t, 25, created.PersistentKeepalive)

	createdExtras := created.GetExtras().(domain.OpnsensePeerExtras)
	assert.Equal(t, "wg-portal-integration-test", createdExtras.Name)
	assert.False(t, createdExtras.Disabled)

	// Update it: a second SavePeer must modify in place, not create a duplicate.
	updatedAllowed, err := domain.CidrsFromString("10.99.0.241/32")
	require.NoError(t, err)

	err = controller.SavePeer(ctx, iface, peerId, func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error) {
		pp.AllowedIPs = updatedAllowed
		extras := pp.GetExtras().(domain.OpnsensePeerExtras)
		extras.Disabled = true
		pp.SetExtras(extras)
		return pp, nil
	})
	require.NoError(t, err)

	peersUpdated, err := controller.GetPeers(ctx, iface)
	require.NoError(t, err)
	assert.Len(t, peersUpdated, len(peersBefore)+1, "update must not create a duplicate peer")

	// Look the peer up explicitly rather than asserting inside a filter loop:
	// a loop that matches nothing would run zero assertions and pass.
	var updated *domain.PhysicalPeer
	for i := range peersUpdated {
		if peersUpdated[i].Identifier == peerId {
			updated = &peersUpdated[i]
			break
		}
	}
	require.NotNil(t, updated, "the updated peer must still be present")
	assert.Equal(t, "10.99.0.241/32", domain.CidrsToString(updated.AllowedIPs))
	assert.True(t, updated.GetExtras().(domain.OpnsensePeerExtras).Disabled,
		"the peer should now be disabled")

	// Delete it.
	require.NoError(t, controller.DeletePeer(ctx, iface, peerId))

	peersFinal, err := controller.GetPeers(ctx, iface)
	require.NoError(t, err)
	assert.Len(t, peersFinal, len(peersBefore), "peer count should return to its original value")
	for _, peer := range peersFinal {
		assert.NotEqual(t, peerId, peer.Identifier, "the test peer should be gone")
	}
}

// Deleting a peer that does not exist must be a no-op rather than an error, so
// that a sync which runs twice does not fail the second time.
func TestOpnsenseDeleteUnknownPeerIsNoOp(t *testing.T) {
	controller, iface := opnsenseTestController(t)

	err := controller.DeletePeer(context.Background(), iface,
		domain.PeerIdentifier("d2dwb3J0YWwtaW50ZWdyYXRpb24tdGVzdC1ub2tleSE="))
	assert.NoError(t, err)
}

// Full lifecycle for a tunnel. This creates a *new* tunnel on a spare instance
// so the testbed's primary tunnel and its live peer are left untouched.
func TestOpnsenseSaveAndDeleteInterface(t *testing.T) {
	controller, _ := opnsenseTestController(t)
	ctx := context.Background()

	spare := domain.InterfaceIdentifier(os.Getenv("WG_PORTAL_OPNSENSE_SPARE_INTERFACE"))
	if spare == "" {
		spare = "wg9"
	}

	// This test creates a tunnel and then deletes it. If a tunnel of that name
	// already exists it belongs to someone else: SaveInterface would silently
	// adopt and reconfigure it, and the cleanup below would then delete it.
	// Refuse rather than destroy a tunnel we did not create.
	if existing, err := controller.GetInterface(ctx, spare); err == nil && existing != nil {
		t.Skipf("refusing to run: %s already exists on this firewall; "+
			"set WG_PORTAL_OPNSENSE_SPARE_INTERFACE to an unused wgN name", spare)
	}

	t.Cleanup(func() {
		if err := controller.DeleteInterface(context.Background(), spare); err != nil {
			t.Logf("cleanup: failed to delete test interface: %v", err)
		}
	})

	before, err := controller.GetInterfaces(ctx)
	require.NoError(t, err)

	addresses, err := domain.CidrsFromString("10.98.0.1/24")
	require.NoError(t, err)

	err = controller.SaveInterface(ctx, spare, func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error) {
		pi.Addresses = addresses
		pi.ListenPort = 51899
		pi.Mtu = 1420
		pi.KeyPair = domain.KeyPair{
			// Placeholders, not a real keypair: OPNsense only checks that these
			// are 32-byte base64, and the tunnel is deleted without ever being
			// brought up, so there is nothing for real key material to protect.
			PrivateKey: "d2dwb3J0YWwtdGVzdGJlZC1wcml2a2V5LUVYQU1QTCE=",
			PublicKey:  "d2dwb3J0YWwtdGVzdGJlZC1wdWJrZXktRVhBTVBMRSE=",
		}
		extras := pi.GetExtras().(domain.OpnsenseInterfaceExtras)
		extras.Comment = "wg-portal-integration-test"
		pi.SetExtras(extras)
		return pi, nil
	})
	require.NoError(t, err, "creating a tunnel should succeed")

	created, err := controller.GetInterface(ctx, spare)
	require.NoError(t, err, "the created tunnel should be readable back")
	assert.Equal(t, spare, created.Identifier,
		"the identifier must be the derived device name, not the free-text name")
	assert.Equal(t, 51899, created.ListenPort)
	assert.Equal(t, "10.98.0.1/24", domain.CidrsToString(created.Addresses))

	createdExtras := created.GetExtras().(domain.OpnsenseInterfaceExtras)
	assert.NotEmpty(t, createdExtras.Uuid)
	assert.Equal(t, "wg-portal-integration-test", createdExtras.Comment)

	after, err := controller.GetInterfaces(ctx)
	require.NoError(t, err)
	assert.Len(t, after, len(before)+1, "exactly one tunnel should have been added")

	// Update in place: a second SaveInterface must not create a duplicate.
	updatedAddresses, err := domain.CidrsFromString("10.98.1.1/24")
	require.NoError(t, err)

	err = controller.SaveInterface(ctx, spare, func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error) {
		pi.Addresses = updatedAddresses
		pi.ListenPort = 51898
		return pi, nil
	})
	require.NoError(t, err)

	updated, err := controller.GetInterface(ctx, spare)
	require.NoError(t, err)
	assert.Equal(t, "10.98.1.1/24", domain.CidrsToString(updated.Addresses))
	assert.Equal(t, 51898, updated.ListenPort)
	assert.Equal(t, createdExtras.Uuid, updated.GetExtras().(domain.OpnsenseInterfaceExtras).Uuid,
		"update must reuse the existing UUID rather than creating a second tunnel")

	afterUpdate, err := controller.GetInterfaces(ctx)
	require.NoError(t, err)
	assert.Len(t, afterUpdate, len(before)+1, "update must not create a duplicate tunnel")

	require.NoError(t, controller.DeleteInterface(ctx, spare))

	final, err := controller.GetInterfaces(ctx)
	require.NoError(t, err)
	assert.Len(t, final, len(before), "tunnel count should return to its original value")
}

// Deleting a tunnel that does not exist must be a no-op.
func TestOpnsenseDeleteUnknownInterfaceIsNoOp(t *testing.T) {
	controller, _ := opnsenseTestController(t)
	assert.NoError(t, controller.DeleteInterface(context.Background(), "wg99"))
}

// An OPNsense "client" is a single record that can be attached to several
// tunnels at once. Removing a peer from one tunnel must detach it from that
// tunnel only -- deleting the record would silently remove it from every other
// tunnel too. This is the branch that only runs for multi-tunnel peers, so it
// needs a peer deliberately attached to two.
func TestOpnsenseDeletePeerDetachesRatherThanDeleting(t *testing.T) {
	controller, primary := opnsenseTestController(t)
	ctx := context.Background()

	second := domain.InterfaceIdentifier(os.Getenv("WG_PORTAL_OPNSENSE_SECOND_INTERFACE"))
	if second == "" {
		t.Skip("set WG_PORTAL_OPNSENSE_SECOND_INTERFACE to a second existing tunnel to run this test")
	}

	const testKey = "bXVsdGl0dW5uZWwtZGV0YWNoLXRlc3Qta2V5ISEhISE="
	peerId := domain.PeerIdentifier(testKey)

	t.Cleanup(func() {
		_ = controller.DeletePeer(context.Background(), primary, peerId)
		_ = controller.DeletePeer(context.Background(), second, peerId)
	})

	allowed, err := domain.CidrsFromString("10.99.0.250/32")
	require.NoError(t, err)

	// Attach to both tunnels. SavePeer merges the server list, so saving twice
	// leaves the peer on both.
	for _, iface := range []domain.InterfaceIdentifier{primary, second} {
		err = controller.SavePeer(ctx, iface, peerId, func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error) {
			pp.AllowedIPs = allowed
			extras, _ := pp.GetExtras().(domain.OpnsensePeerExtras)
			extras.Name = "wg-portal-multitunnel-test"
			pp.SetExtras(extras)
			return pp, nil
		})
		require.NoError(t, err, "attaching to %s should succeed", iface)
	}

	onPrimary, err := controller.GetPeers(ctx, primary)
	require.NoError(t, err)
	onSecond, err := controller.GetPeers(ctx, second)
	require.NoError(t, err)
	require.True(t, containsPeer(onPrimary, peerId), "peer should be on %s", primary)
	require.True(t, containsPeer(onSecond, peerId), "peer should be on %s", second)

	// Remove from the primary only.
	require.NoError(t, controller.DeletePeer(ctx, primary, peerId))

	onPrimary, err = controller.GetPeers(ctx, primary)
	require.NoError(t, err)
	onSecond, err = controller.GetPeers(ctx, second)
	require.NoError(t, err)

	assert.False(t, containsPeer(onPrimary, peerId), "peer must be gone from %s", primary)
	assert.True(t, containsPeer(onSecond, peerId),
		"peer must SURVIVE on %s: deleting from one tunnel must not detach it from others", second)

	// Removing it from the last tunnel deletes the record outright.
	require.NoError(t, controller.DeletePeer(ctx, second, peerId))
	onSecond, err = controller.GetPeers(ctx, second)
	require.NoError(t, err)
	assert.False(t, containsPeer(onSecond, peerId), "peer must be gone from %s once unreferenced", second)
}

func containsPeer(peers []domain.PhysicalPeer, id domain.PeerIdentifier) bool {
	for _, p := range peers {
		if p.Identifier == id {
			return true
		}
	}
	return false
}

// IPv6 handling across the whole round-trip: a dual-stack tunnel, a peer with
// both families in its allowed addresses, and a bracketed IPv6 endpoint.
//
// The endpoint is the interesting part. OPNsense stores host and port in
// separate fields, so an IPv6 literal has to be bracketed when they are joined
// back together -- otherwise "fd00::1" and "51820" become "fd00::1:51820",
// which still parses as an address with the port silently absorbed.
func TestOpnsenseIPv6RoundTrip(t *testing.T) {
	controller, _ := opnsenseTestController(t)
	ctx := context.Background()

	spare := domain.InterfaceIdentifier(os.Getenv("WG_PORTAL_OPNSENSE_SPARE_INTERFACE"))
	if spare == "" {
		spare = "wg9"
	}
	if existing, err := controller.GetInterface(ctx, spare); err == nil && existing != nil {
		t.Skipf("refusing to run: %s already exists on this firewall", spare)
	}
	t.Cleanup(func() { _ = controller.DeleteInterface(context.Background(), spare) })

	dualStack, err := domain.CidrsFromString("10.97.0.1/24,fd00:97::1/64")
	require.NoError(t, err)

	err = controller.SaveInterface(ctx, spare, func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error) {
		pi.Addresses = dualStack
		pi.ListenPort = 51897
		pi.KeyPair = domain.KeyPair{
			PrivateKey: "d2dwb3J0YWwtdGVzdGJlZC1wcml2a2V5LUVYQU1QTCE=",
			PublicKey:  "d2dwb3J0YWwtdGVzdGJlZC1wdWJrZXktRVhBTVBMRSE=",
		}
		extras := pi.GetExtras().(domain.OpnsenseInterfaceExtras)
		extras.Comment = "wg-portal-ipv6-test"
		pi.SetExtras(extras)
		return pi, nil
	})
	require.NoError(t, err, "a dual-stack tunnel should be accepted")

	created, err := controller.GetInterface(ctx, spare)
	require.NoError(t, err)
	assert.Equal(t, "10.97.0.1/24,fd00:97::1/64", domain.CidrsToString(created.Addresses),
		"both address families must survive the round-trip, in order")

	// A peer with dual-stack allowed addresses and an IPv6 endpoint.
	const peerKey = "C63a3Ddezps4AI4Gg5nzMGA978Cx/ASfQ9BnWdgAhlM="
	peerId := domain.PeerIdentifier(peerKey)
	t.Cleanup(func() { _ = controller.DeletePeer(context.Background(), spare, peerId) })

	allowed, err := domain.CidrsFromString("10.97.0.5/32,fd00:97::5/128")
	require.NoError(t, err)

	err = controller.SavePeer(ctx, spare, peerId, func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error) {
		pp.AllowedIPs = allowed
		pp.Endpoint = "[2001:db8::1]:51820"
		pp.PersistentKeepalive = 25
		extras, _ := pp.GetExtras().(domain.OpnsensePeerExtras)
		extras.Name = "wg-portal-ipv6-peer"
		pp.SetExtras(extras)
		return pp, nil
	})
	require.NoError(t, err, "a peer with IPv6 allowed addresses and endpoint should be accepted")

	peers, err := controller.GetPeers(ctx, spare)
	require.NoError(t, err)

	var peer *domain.PhysicalPeer
	for i := range peers {
		if peers[i].Identifier == peerId {
			peer = &peers[i]
			break
		}
	}
	require.NotNil(t, peer, "the IPv6 peer should be readable back")
	assert.Equal(t, "10.97.0.5/32,fd00:97::5/128", domain.CidrsToString(peer.AllowedIPs))
	assert.Equal(t, "[2001:db8::1]:51820", peer.Endpoint,
		"the IPv6 endpoint must round-trip with its brackets and port intact")
}
