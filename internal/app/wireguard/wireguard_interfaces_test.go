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

func TestInferImportedInterfaceType(t *testing.T) {
	tests := []struct {
		name       string
		listenPort int
		peerCount  int
		expected   domain.InterfaceType
	}{
		{
			name:       "no peers stays unknown",
			listenPort: 51820,
			peerCount:  0,
			expected:   domain.InterfaceTypeAny,
		},
		{
			name:       "single peer with listen port is server",
			listenPort: 51820,
			peerCount:  1,
			expected:   domain.InterfaceTypeServer,
		},
		{
			name:       "single peer without listen port stays client",
			listenPort: 0,
			peerCount:  1,
			expected:   domain.InterfaceTypeClient,
		},
		{
			name:       "multiple peers is server",
			listenPort: 0,
			peerCount:  2,
			expected:   domain.InterfaceTypeServer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iface := &domain.Interface{ListenPort: tt.listenPort}
			peers := make([]domain.PhysicalPeer, tt.peerCount)

			assert.Equal(t, tt.expected, inferImportedInterfaceType(iface, peers))
		})
	}
}

func TestImportPeer_AddressMapping(t *testing.T) {
	tests := []struct {
		name                 string
		allowedIPs           []string
		expectedInterface    []string
		expectedExtraAllowed string
	}{
		{
			name:                 "IPv4 host address",
			allowedIPs:           []string{"10.0.0.1/32"},
			expectedInterface:    []string{"10.0.0.1/32"},
			expectedExtraAllowed: "",
		},
		{
			name:                 "IPv6 host address",
			allowedIPs:           []string{"fd00::1/128"},
			expectedInterface:    []string{"fd00::1/128"},
			expectedExtraAllowed: "",
		},
		{
			name:                 "IPv4 network address",
			allowedIPs:           []string{"10.0.1.0/24"},
			expectedInterface:    []string{},
			expectedExtraAllowed: "10.0.1.0/24",
		},
		{
			name:                 "IPv4 normal address with mask",
			allowedIPs:           []string{"10.0.1.5/24"},
			expectedInterface:    []string{"10.0.1.5/24"},
			expectedExtraAllowed: "",
		},
		{
			name: "Mixed addresses",
			allowedIPs: []string{
				"10.0.0.1/32", "192.168.1.0/24", "172.16.0.5/24", "fd00::1/128", "fd00:1::/64",
			},
			expectedInterface:    []string{"10.0.0.1/32", "172.16.0.5/24", "fd00::1/128"},
			expectedExtraAllowed: "192.168.1.0/24,fd00:1::/64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := &mockDB{}
			m := Manager{
				db: db,
			}

			iface := &domain.Interface{
				Identifier: "wg0",
				Type:       domain.InterfaceTypeServer,
			}

			allowedIPs := make([]domain.Cidr, len(tt.allowedIPs))
			for i, s := range tt.allowedIPs {
				cidr, _ := domain.CidrFromString(s)
				allowedIPs[i] = cidr
			}

			p := &domain.PhysicalPeer{
				Identifier: "peer1",
				PublicKey:  "peer1-public-key-is-long-enough",
				AllowedIPs: allowedIPs,
			}

			err := m.importPeer(context.Background(), iface, p)
			assert.NoError(t, err)

			savedPeer := db.savedPeers["peer1"]
			assert.NotNil(t, savedPeer)

			// Check interface addresses
			actualInterface := make([]string, len(savedPeer.Interface.Addresses))
			for i, addr := range savedPeer.Interface.Addresses {
				actualInterface[i] = addr.String()
			}
			assert.ElementsMatch(t, tt.expectedInterface, actualInterface)

			// Check extra allowed IPs
			assert.Equal(t, tt.expectedExtraAllowed, savedPeer.ExtraAllowedIPsStr)
		})
	}
}

func TestInterface_IsUserAllowed(t *testing.T) {
	cfg := &config.Config{
		Auth: config.Auth{
			Ldap: []config.LdapProvider{
				{
					ProviderName: "ldap1",
					InterfaceFilter: map[string]string{
						"wg0": "(memberOf=CN=VPNUsers,...)",
					},
				},
				{
					ProviderName: "ldap2",
					InterfaceFilter: map[string]string{
						"wg0": "(memberOf=CN=OtherVPNUsers,...)",
					},
				},
			},
		},
	}

	tests := []struct {
		name   string
		iface  domain.Interface
		userId domain.UserIdentifier
		expect bool
	}{
		{
			name: "Unrestricted interface",
			iface: domain.Interface{
				Identifier: "wg1",
			},
			userId: "user1",
			expect: true,
		},
		{
			name: "Restricted interface - user allowed",
			iface: domain.Interface{
				Identifier: "wg0",
				LdapAllowedUsers: map[string][]domain.UserIdentifier{
					"ldap1": {"user1"},
				},
			},
			userId: "user1",
			expect: true,
		},
		{
			name: "Restricted interface - user allowed (at least one match)",
			iface: domain.Interface{
				Identifier: "wg0",
				LdapAllowedUsers: map[string][]domain.UserIdentifier{
					"ldap1": {"user2"},
					"ldap2": {"user1"},
				},
			},
			userId: "user1",
			expect: true,
		},
		{
			name: "Restricted interface - user NOT allowed",
			iface: domain.Interface{
				Identifier: "wg0",
				LdapAllowedUsers: map[string][]domain.UserIdentifier{
					"ldap1": {"user2"},
				},
			},
			userId: "user1",
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, tt.iface.IsUserAllowed(tt.userId, cfg, nil))
		})
	}
}

func TestManager_GetUserInterfaces_Filtering(t *testing.T) {
	cfg := &config.Config{}
	cfg.Core.SelfProvisioningAllowed = true
	cfg.Auth.Ldap = []config.LdapProvider{
		{
			ProviderName: "ldap1",
			InterfaceFilter: map[string]string{
				"wg_restricted": "(some-filter)",
			},
		},
	}

	db := &mockDB{
		interfaces: []domain.Interface{
			{Identifier: "wg_public", Type: domain.InterfaceTypeServer},
			{
				Identifier: "wg_restricted",
				Type:       domain.InterfaceTypeServer,
				LdapAllowedUsers: map[string][]domain.UserIdentifier{
					"ldap1": {"allowed_user"},
				},
			},
		},
	}
	m := Manager{
		cfg: cfg,
		db:  db,
	}

	t.Run("Allowed user sees both", func(t *testing.T) {
		ifaces, err := m.GetUserInterfaces(context.Background(), "allowed_user")
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ifaces))
	})

	t.Run("Unallowed user sees only public", func(t *testing.T) {
		ifaces, err := m.GetUserInterfaces(context.Background(), "other_user")
		assert.NoError(t, err)
		assert.Equal(t, 1, len(ifaces))
		if len(ifaces) > 0 {
			assert.Equal(t, domain.InterfaceIdentifier("wg_public"), ifaces[0].Identifier)
		}
	})
}

func memberGrant() []domain.InterfaceAccessGrant {
	return []domain.InterfaceAccessGrant{{
		InterfaceIdentifier: "wg-data", ProviderName: "oidc1", UserIdentifier: "member", EvaluatedAt: time.Now(),
	}}
}

func newOauthFilterConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Core.SelfProvisioningAllowed = true
	cfg.Core.EditableKeys = true
	cfg.Auth.OpenIDConnect = []config.OpenIDConnectProvider{
		{ProviderName: "oidc1", InterfaceFilter: map[string]string{"wg-data": "^vpn-data$"}},
	}
	return cfg
}

func TestManager_GetUserInterfaces_OauthFilter(t *testing.T) {
	db := &mockDB{
		interfaces: []domain.Interface{
			{Identifier: "wg-world", Type: domain.InterfaceTypeServer},
			{Identifier: "wg-data", Type: domain.InterfaceTypeServer},
		},
		users:  []domain.User{{Identifier: "admin", IsAdmin: true}},
		grants: memberGrant(),
	}
	m := Manager{cfg: newOauthFilterConfig(), db: db}

	identifiers := func(ifaces []domain.Interface) []domain.InterfaceIdentifier {
		ids := make([]domain.InterfaceIdentifier, 0, len(ifaces))
		for _, iface := range ifaces {
			ids = append(ids, iface.Identifier)
		}
		return ids
	}

	ifaces, err := m.GetUserInterfaces(context.Background(), "member")
	assert.NoError(t, err)
	assert.ElementsMatch(t, []domain.InterfaceIdentifier{"wg-world", "wg-data"}, identifiers(ifaces))

	ifaces, err = m.GetUserInterfaces(context.Background(), "outsider")
	assert.NoError(t, err)
	assert.ElementsMatch(t, []domain.InterfaceIdentifier{"wg-world"}, identifiers(ifaces))

	ifaces, err = m.GetUserInterfaces(context.Background(), "admin")
	assert.NoError(t, err)
	assert.ElementsMatch(t, []domain.InterfaceIdentifier{"wg-world", "wg-data"}, identifiers(ifaces),
		"admins are not restricted by interface filters")
}

func TestManager_CreatePeer_OauthFilter(t *testing.T) {
	newManager := func() (Manager, *mockDB) {
		db := &mockDB{
			iface:  &domain.Interface{Identifier: "wg-data", Type: domain.InterfaceTypeServer},
			grants: memberGrant(),
		}
		return Manager{
			cfg: newOauthFilterConfig(),
			bus: &mockBus{},
			db:  db,
			wg: &ControllerManager{
				controllers: map[domain.InterfaceBackend]backendInstance{
					config.LocalBackendName: {Implementation: &mockController{}},
				},
			},
		}, db
	}
	newPeer := func(userId domain.UserIdentifier) *domain.Peer {
		return &domain.Peer{
			UserIdentifier:      userId,
			InterfaceIdentifier: "wg-data",
			Interface: domain.PeerInterfaceConfig{
				KeyPair: domain.KeyPair{PublicKey: "PUBLIC_KEY_" + string(userId)},
			},
		}
	}

	tests := []struct {
		name    string
		user    domain.ContextUserInfo
		allowed bool
	}{
		{name: "group member", user: domain.ContextUserInfo{Id: "member"}, allowed: true},
		{name: "not a group member", user: domain.ContextUserInfo{Id: "outsider"}, allowed: false},
		{name: "admin", user: domain.ContextUserInfo{Id: "admin", IsAdmin: true}, allowed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, db := newManager()
			ctx := domain.SetUserInfo(context.Background(), &tt.user)

			_, err := m.CreatePeer(ctx, newPeer(tt.user.Id))
			if tt.allowed {
				assert.NoError(t, err)
				assert.Len(t, db.savedPeers, 1)
			} else {
				assert.ErrorIs(t, err, domain.ErrNoPermission)
				assert.Empty(t, db.savedPeers)
			}
		})
	}
}

func TestManager_PrepareUpdateDeletePeer_OauthFilter(t *testing.T) {
	newManager := func(owner domain.UserIdentifier) Manager {
		peer := domain.Peer{
			Identifier:          "peer-1",
			UserIdentifier:      owner,
			InterfaceIdentifier: "wg-data",
			Interface:           domain.PeerInterfaceConfig{KeyPair: domain.KeyPair{PublicKey: "peer-1"}},
		}
		db := &mockDB{
			iface:  &domain.Interface{Identifier: "wg-data", Type: domain.InterfaceTypeServer},
			grants: memberGrant(),
			peers:  map[domain.PeerIdentifier]domain.Peer{peer.Identifier: peer},
		}
		return Manager{
			cfg: newOauthFilterConfig(),
			bus: &mockBus{},
			db:  db,
			wg: &ControllerManager{
				controllers: map[domain.InterfaceBackend]backendInstance{
					config.LocalBackendName: {Implementation: &mockController{}},
				},
			},
		}
	}

	for _, userId := range []domain.UserIdentifier{"member", "outsider"} {
		t.Run(string(userId), func(t *testing.T) {
			ctx := domain.SetUserInfo(context.Background(), &domain.ContextUserInfo{Id: userId})

			_, updateErr := newManager(userId).UpdatePeer(ctx, &domain.Peer{
				Identifier:  "peer-1",
				DisplayName: "renamed",
				Interface:   domain.PeerInterfaceConfig{KeyPair: domain.KeyPair{PublicKey: "peer-1"}},
			})
			deleteErr := newManager(userId).DeletePeer(ctx, "peer-1")
			_, prepareErr := newManager(userId).PreparePeer(ctx, "wg-data")

			if userId == "outsider" {
				assert.ErrorIs(t, updateErr, domain.ErrNoPermission)
				assert.ErrorIs(t, deleteErr, domain.ErrNoPermission)
				assert.ErrorIs(t, prepareErr, domain.ErrNoPermission)
			} else {
				assert.NoError(t, updateErr)
				assert.NoError(t, deleteErr)
				assert.NoError(t, prepareErr)
			}
		})
	}
}

func newGrantTestManager(cfg *config.Config, db *mockDB) Manager {
	cfg.Core.CreateDefaultPeerOnLogin = true
	return Manager{
		cfg: cfg,
		bus: &mockBus{},
		db:  db,
		wg: &ControllerManager{
			controllers: map[domain.InterfaceBackend]backendInstance{
				config.LocalBackendName: {Implementation: &mockController{}},
			},
		},
	}
}

// Installations without an OAuth/OIDC interface_filter must not depend on the grants table.
func TestManager_NoOauthFilterDoesNotLoadGrants(t *testing.T) {
	cfg := &config.Config{}
	cfg.Core.SelfProvisioningAllowed = true
	cfg.Auth.Ldap = []config.LdapProvider{
		{ProviderName: "ldap1", InterfaceFilter: map[string]string{"wg-data": "(memberOf=CN=VPN)"}},
	}
	db := &mockDB{
		iface: &domain.Interface{
			Identifier:        "wg-data",
			Type:              domain.InterfaceTypeServer,
			CreateDefaultPeer: true,
			LdapAllowedUsers:  map[string][]domain.UserIdentifier{"ldap1": {"member"}},
		},
		grantsErr: assert.AnError,
	}
	m := newGrantTestManager(cfg, db)
	memberCtx := domain.SetUserInfo(context.Background(), &domain.ContextUserInfo{Id: "member"})
	adminCtx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())

	ifaces, err := m.GetUserInterfaces(context.Background(), "member")
	require.NoError(t, err)
	assert.Len(t, ifaces, 1)
	_, err = m.PreparePeer(memberCtx, "wg-data")
	require.NoError(t, err)
	require.NoError(t, m.CreateDefaultPeer(adminCtx, "member"))

	assert.Zero(t, db.grantLoads)
}

// If the grants cannot be loaded, access must be denied rather than granted.
func TestManager_GrantLoadErrorFailsClosed(t *testing.T) {
	db := &mockDB{
		iface:     &domain.Interface{Identifier: "wg-data", Type: domain.InterfaceTypeServer, CreateDefaultPeer: true},
		grants:    memberGrant(),
		grantsErr: assert.AnError,
	}
	m := newGrantTestManager(newOauthFilterConfig(), db)
	memberCtx := domain.SetUserInfo(context.Background(), &domain.ContextUserInfo{Id: "member"})
	adminCtx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())

	_, err := m.GetUserInterfaces(context.Background(), "member")
	assert.ErrorIs(t, err, assert.AnError)
	_, err = m.PreparePeer(memberCtx, "wg-data")
	assert.ErrorIs(t, err, assert.AnError)
	assert.ErrorIs(t, m.CreateDefaultPeer(adminCtx, "member"), assert.AnError)
	assert.Empty(t, db.savedPeers)
}
