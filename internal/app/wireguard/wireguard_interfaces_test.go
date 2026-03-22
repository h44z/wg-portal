package wireguard

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

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
				KeyPair:    domain.KeyPair{PublicKey: "peer1-public-key-is-long-enough"},
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

func (f *mockDB) GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error) {
	return &domain.User{
		Identifier: id,
		IsAdmin:    false,
	}, nil
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
			assert.Equal(t, tt.expect, tt.iface.IsUserAllowed(tt.userId, cfg))
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
