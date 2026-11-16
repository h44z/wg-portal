package wireguard

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// --- Test mocks ---

type mockBus struct{}

func (f *mockBus) Publish(topic string, args ...any)    {}
func (f *mockBus) Subscribe(topic string, fn any) error { return nil }

type mockController struct{}

func (f *mockController) GetId() domain.InterfaceBackend { return "local" }
func (f *mockController) GetInterfaces(_ context.Context) ([]domain.PhysicalInterface, error) {
	return nil, nil
}
func (f *mockController) GetInterface(_ context.Context, id domain.InterfaceIdentifier) (
	*domain.PhysicalInterface,
	error,
) {
	return &domain.PhysicalInterface{Identifier: id}, nil
}
func (f *mockController) GetPeers(_ context.Context, _ domain.InterfaceIdentifier) ([]domain.PhysicalPeer, error) {
	return nil, nil
}
func (f *mockController) SaveInterface(
	_ context.Context,
	_ domain.InterfaceIdentifier,
	updateFunc func(pi *domain.PhysicalInterface) (*domain.PhysicalInterface, error),
) error {
	_, _ = updateFunc(&domain.PhysicalInterface{})
	return nil
}
func (f *mockController) DeleteInterface(_ context.Context, _ domain.InterfaceIdentifier) error {
	return nil
}
func (f *mockController) SavePeer(
	_ context.Context,
	_ domain.InterfaceIdentifier,
	_ domain.PeerIdentifier,
	updateFunc func(pp *domain.PhysicalPeer) (*domain.PhysicalPeer, error),
) error {
	_, _ = updateFunc(&domain.PhysicalPeer{})
	return nil
}
func (f *mockController) DeletePeer(_ context.Context, _ domain.InterfaceIdentifier, _ domain.PeerIdentifier) error {
	return nil
}
func (f *mockController) PingAddresses(_ context.Context, _ string) (*domain.PingerResult, error) {
	return nil, nil
}

type mockDB struct {
	savedPeers map[domain.PeerIdentifier]*domain.Peer
	iface      *domain.Interface
	interfaces []domain.Interface
	users      []domain.User
	grants     []domain.InterfaceAccessGrant
	grantsErr  error
	grantLoads int
	peers      map[domain.PeerIdentifier]domain.Peer
}

func (f *mockDB) GetInterface(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, error) {
	if f.iface != nil && f.iface.Identifier == id {
		return f.iface, nil
	}
	return &domain.Interface{Identifier: id}, nil
}
func (f *mockDB) GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (
	*domain.Interface,
	[]domain.Peer,
	error,
) {
	return f.iface, nil, nil
}
func (f *mockDB) GetPeersStats(ctx context.Context, ids ...domain.PeerIdentifier) ([]domain.PeerStatus, error) {
	return nil, nil
}
func (f *mockDB) GetAllInterfaces(ctx context.Context) ([]domain.Interface, error) {
	if f.interfaces != nil {
		return f.interfaces, nil
	}
	if f.iface != nil {
		return []domain.Interface{*f.iface}, nil
	}
	return nil, nil
}
func (f *mockDB) GetInterfaceIps(ctx context.Context) (map[domain.InterfaceIdentifier][]domain.Cidr, error) {
	return nil, nil
}
func (f *mockDB) SaveInterface(
	ctx context.Context,
	id domain.InterfaceIdentifier,
	updateFunc func(in *domain.Interface) (*domain.Interface, error),
) error {
	if f.iface == nil {
		f.iface = &domain.Interface{Identifier: id}
	}
	var err error
	f.iface, err = updateFunc(f.iface)
	return err
}
func (f *mockDB) DeleteInterface(ctx context.Context, id domain.InterfaceIdentifier) error {
	return nil
}
func (f *mockDB) GetInterfacePeers(ctx context.Context, id domain.InterfaceIdentifier) ([]domain.Peer, error) {
	return nil, nil
}
func (f *mockDB) GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error) {
	return nil, nil
}
func (f *mockDB) SavePeer(
	ctx context.Context,
	id domain.PeerIdentifier,
	updateFunc func(in *domain.Peer) (*domain.Peer, error),
) error {
	if f.savedPeers == nil {
		f.savedPeers = make(map[domain.PeerIdentifier]*domain.Peer)
	}
	existing := f.savedPeers[id]
	if existing == nil {
		existing = &domain.Peer{Identifier: id}
	}
	updated, err := updateFunc(existing)
	if err != nil {
		return err
	}
	f.savedPeers[updated.Identifier] = updated
	return nil
}
func (f *mockDB) DeletePeer(ctx context.Context, id domain.PeerIdentifier) error { return nil }
func (f *mockDB) GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error) {
	if peer, ok := f.peers[id]; ok {
		return &peer, nil
	}
	return nil, domain.ErrNotFound
}
func (f *mockDB) GetUsedIpsPerSubnet(ctx context.Context, subnets []domain.Cidr) (
	map[domain.Cidr][]domain.Cidr,
	error,
) {
	return map[domain.Cidr][]domain.Cidr{}, nil
}
func (f *mockDB) GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error) {
	for i := range f.users {
		if f.users[i].Identifier == id {
			return &f.users[i], nil
		}
	}
	return &domain.User{
		Identifier: id,
		IsAdmin:    false,
	}, nil
}
func (f *mockDB) GetAllUsers(ctx context.Context) ([]domain.User, error) {
	return f.users, nil
}
func (f *mockDB) GetUserInterfaceAccessGrants(
	ctx context.Context,
	userId domain.UserIdentifier,
) ([]domain.InterfaceAccessGrant, error) {
	f.grantLoads++
	if f.grantsErr != nil {
		return nil, f.grantsErr
	}
	var grants []domain.InterfaceAccessGrant
	for _, grant := range f.grants {
		if grant.UserIdentifier == userId {
			grants = append(grants, grant)
		}
	}
	return grants, nil
}

// --- Test ---

func TestCreatePeer_SetsIdentifier_FromPublicKey(t *testing.T) {
	// Arrange
	cfg := &config.Config{}
	cfg.Core.SelfProvisioningAllowed = true
	cfg.Core.EditableKeys = true
	cfg.Advanced.LimitAdditionalUserPeers = 0

	bus := &mockBus{}

	// Prepare a controller manager with our mock controller
	ctrlMgr := &ControllerManager{
		controllers: map[domain.InterfaceBackend]backendInstance{
			config.LocalBackendName: {Implementation: &mockController{}},
		},
	}

	db := &mockDB{iface: &domain.Interface{Identifier: "wg0", Type: domain.InterfaceTypeServer}}

	m := Manager{
		cfg: cfg,
		bus: bus,
		db:  db,
		wg:  ctrlMgr,
	}

	userId := domain.UserIdentifier("user@example.com")
	ctx := domain.SetUserInfo(context.Background(), &domain.ContextUserInfo{Id: userId, IsAdmin: false})

	pubKey := "TEST_PUBLIC_KEY_ABC123"

	input := &domain.Peer{
		Identifier:          "should_be_overwritten",
		UserIdentifier:      userId,
		InterfaceIdentifier: domain.InterfaceIdentifier("wg0"),
		Interface: domain.PeerInterfaceConfig{
			KeyPair: domain.KeyPair{PublicKey: pubKey},
		},
	}

	// Act
	out, err := m.CreatePeer(ctx, input)

	// Assert
	if err != nil {
		t.Fatalf("CreatePeer returned error: %v", err)
	}

	expectedId := domain.PeerIdentifier(pubKey)
	if out.Identifier != expectedId {
		t.Fatalf("expected Identifier to be set from public key %q, got %q", expectedId, out.Identifier)
	}

	// Ensure the saved peer in DB also has the expected identifier
	if db.savedPeers[expectedId] == nil {
		t.Fatalf("expected peer with identifier %q to be saved in DB", expectedId)
	}
}

// TestCreatePeer_KeepsBrowserGeneratedKeyWithoutPrivateKey covers the browser-side key generation flow of
// frontend/src/components/UserPeerEditModal.vue: the client only sends its public key, so the server must
// neither store a private key nor replace the key pair with a server-generated one.
func TestCreatePeer_KeepsBrowserGeneratedKeyWithoutPrivateKey(t *testing.T) {
	cfg := &config.Config{}
	cfg.Core.SelfProvisioningAllowed = true
	cfg.Core.EditableKeys = true

	ctrlMgr := &ControllerManager{
		controllers: map[domain.InterfaceBackend]backendInstance{
			config.LocalBackendName: {Implementation: &mockController{}},
		},
	}

	db := &mockDB{iface: &domain.Interface{Identifier: "wg0", Type: domain.InterfaceTypeServer}}

	m := Manager{
		cfg: cfg,
		bus: &mockBus{},
		db:  db,
		wg:  ctrlMgr,
	}

	userId := domain.UserIdentifier("user@example.com")
	ctx := domain.SetUserInfo(context.Background(), &domain.ContextUserInfo{Id: userId, IsAdmin: false})

	pubKey := "BROWSER_GENERATED_PUBLIC_KEY"

	input := &domain.Peer{
		UserIdentifier:      userId,
		InterfaceIdentifier: domain.InterfaceIdentifier("wg0"),
		Interface: domain.PeerInterfaceConfig{
			KeyPair: domain.KeyPair{PublicKey: pubKey},
		},
	}

	out, err := m.CreatePeer(ctx, input)
	if err != nil {
		t.Fatalf("CreatePeer returned error: %v", err)
	}

	saved := db.savedPeers[domain.PeerIdentifier(pubKey)]
	if saved == nil {
		t.Fatalf("expected peer with identifier %q to be saved in DB", pubKey)
	}
	for name, peer := range map[string]*domain.Peer{"returned": out, "saved": saved} {
		if peer.Interface.PublicKey != pubKey {
			t.Fatalf("expected %s peer to keep public key %q, got %q", name, pubKey, peer.Interface.PublicKey)
		}
		if peer.Interface.PrivateKey != "" {
			t.Fatalf("expected %s peer to have no private key, got %q", name, peer.Interface.PrivateKey)
		}
	}
}

func TestCreateDefaultPeer_RespectsInterfaceFlag(t *testing.T) {
	// Arrange
	cfg := &config.Config{}
	cfg.Core.CreateDefaultPeerOnLogin = true

	bus := &mockBus{}
	ctrlMgr := &ControllerManager{
		controllers: map[domain.InterfaceBackend]backendInstance{
			config.LocalBackendName: {Implementation: &mockController{}},
		},
	}

	db := &mockDB{
		iface: &domain.Interface{
			Identifier:        "wg0",
			Type:              domain.InterfaceTypeServer,
			CreateDefaultPeer: false, // Flag is disabled!
		},
	}

	m := Manager{
		cfg: cfg,
		bus: bus,
		db:  db,
		wg:  ctrlMgr,
	}

	userId := domain.UserIdentifier("user@example.com")
	ctx := domain.SetUserInfo(context.Background(), &domain.ContextUserInfo{Id: userId, IsAdmin: true})

	// Act
	err := m.CreateDefaultPeer(ctx, userId)

	// Assert
	if err != nil {
		t.Fatalf("CreateDefaultPeer returned error: %v", err)
	}

	if len(db.savedPeers) != 0 {
		t.Fatalf("expected no peers to be created because interface flag is false, but got %d", len(db.savedPeers))
	}

	// Now enable the flag and try again
	db.iface.CreateDefaultPeer = true
	err = m.CreateDefaultPeer(ctx, userId)

	if err != nil {
		t.Fatalf("CreateDefaultPeer returned error after enabling flag: %v", err)
	}

	if len(db.savedPeers) != 1 {
		t.Fatalf("expected 1 peer to be created because interface flag is true, but got %d", len(db.savedPeers))
	}
}

func TestDefaultPeers_RespectInterfaceFilter(t *testing.T) {
	ldapCfg := &config.Config{}
	ldapCfg.Auth.Ldap = []config.LdapProvider{
		{ProviderName: "ldap1", InterfaceFilter: map[string]string{"wg-data": "(memberOf=CN=VPN)"}},
	}
	oidcCfg := &config.Config{}
	oidcCfg.Auth.OpenIDConnect = []config.OpenIDConnectProvider{
		{ProviderName: "oidc1", InterfaceFilter: map[string]string{"wg-data": "^vpn-data$"}},
	}

	filters := []struct {
		name   string
		cfg    *config.Config
		iface  domain.Interface
		grants []domain.InterfaceAccessGrant
	}{
		{
			name: "ldap",
			cfg:  ldapCfg,
			iface: domain.Interface{LdapAllowedUsers: map[string][]domain.UserIdentifier{
				"ldap1": {"member"},
			}},
		},
		{
			name: "oidc",
			cfg:  oidcCfg,
			grants: []domain.InterfaceAccessGrant{{
				InterfaceIdentifier: "wg-data", ProviderName: "oidc1", UserIdentifier: "member", EvaluatedAt: time.Now(),
			}},
		},
	}
	users := []domain.User{
		{Identifier: "member"},
		{Identifier: "outsider"},
		{Identifier: "admin", IsAdmin: true},
	}

	newManager := func(cfg *config.Config, iface domain.Interface, grants []domain.InterfaceAccessGrant) (
		Manager,
		*mockDB,
	) {
		cfg.Core.CreateDefaultPeerOnLogin = true
		iface.Identifier = "wg-data"
		iface.Type = domain.InterfaceTypeServer
		iface.CreateDefaultPeer = true
		db := &mockDB{iface: &iface, users: users, grants: grants}
		return Manager{
			cfg: cfg,
			bus: &mockBus{},
			db:  db,
			wg: &ControllerManager{
				controllers: map[domain.InterfaceBackend]backendInstance{
					config.LocalBackendName: {Implementation: &mockController{}},
				},
			},
		}, db
	}
	peerOwners := func(db *mockDB) []domain.UserIdentifier {
		var owners []domain.UserIdentifier
		for _, peer := range db.savedPeers {
			owners = append(owners, peer.UserIdentifier)
		}
		return owners
	}
	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())

	for _, filter := range filters {
		t.Run(filter.name+"/per user", func(t *testing.T) {
			for _, user := range users {
				m, db := newManager(filter.cfg, filter.iface, filter.grants)
				require.NoError(t, m.CreateDefaultPeer(ctx, user.Identifier))
				if user.Identifier == "outsider" {
					assert.Empty(t, db.savedPeers, "a filtered-out user must not get a default peer")
				} else {
					assert.Equal(t, []domain.UserIdentifier{user.Identifier}, peerOwners(db))
				}
			}
		})
		t.Run(filter.name+"/per interface", func(t *testing.T) {
			m, db := newManager(filter.cfg, filter.iface, filter.grants)
			require.NoError(t, m.CreateDefaultPeers(ctx, "wg-data"))
			assert.ElementsMatch(t, []domain.UserIdentifier{"member", "admin"}, peerOwners(db))
		})
	}
}
