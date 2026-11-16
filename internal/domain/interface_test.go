package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/h44z/wg-portal/internal/config"
)

func TestInterface_IsDisabledReturnsTrueWhenDisabled(t *testing.T) {
	iface := &Interface{}
	assert.False(t, iface.IsDisabled())

	now := time.Now()
	iface.Disabled = &now
	assert.True(t, iface.IsDisabled())
}

func TestInterface_AddressStrReturnsCorrectString(t *testing.T) {
	iface := &Interface{
		Addresses: []Cidr{
			{Cidr: "192.168.1.1/24", Addr: "192.168.1.1", NetLength: 24},
			{Cidr: "10.0.0.1/24", Addr: "10.0.0.1", NetLength: 24},
		},
	}
	expected := "192.168.1.1/24,10.0.0.1/24"
	assert.Equal(t, expected, iface.AddressStr())
}

func TestInterface_GetConfigFileNameReturnsCorrectFileName(t *testing.T) {
	iface := &Interface{Identifier: "wg0"}
	expected := "wg0.conf"
	assert.Equal(t, expected, iface.GetConfigFileName())

	iface.Identifier = "wg0@123"
	expected = "wg0123.conf"
	assert.Equal(t, expected, iface.GetConfigFileName())
}

func TestInterface_GetAllowedIPsReturnsCorrectCidrsServerMode(t *testing.T) {
	peer1 := Peer{
		AllowedIPsStr: ConfigOption[string]{Value: "192.168.2.2/32"},
		Interface: PeerInterfaceConfig{
			Addresses: []Cidr{
				{Cidr: "192.168.1.2/32", Addr: "192.168.1.2", NetLength: 32},
			},
		},
	}
	peer2 := Peer{
		AllowedIPsStr:      ConfigOption[string]{Value: "10.0.2.2/32"},
		ExtraAllowedIPsStr: "10.20.2.2/32",
		Interface: PeerInterfaceConfig{
			Addresses: []Cidr{
				{Cidr: "10.0.0.2/32", Addr: "10.0.0.2", NetLength: 32},
			},
		},
	}
	iface := &Interface{Type: InterfaceTypeServer}
	expected := []Cidr{
		{Cidr: "192.168.1.2/32", Addr: "192.168.1.2", NetLength: 32},
		{Cidr: "10.0.0.2/32", Addr: "10.0.0.2", NetLength: 32},
		{Cidr: "10.20.2.2/32", Addr: "10.20.2.2", NetLength: 32},
	}
	assert.Equal(t, expected, iface.GetAllowedIPs([]Peer{peer1, peer2}))
}

func TestInterface_GetAllowedIPsReturnsCorrectCidrsClientMode(t *testing.T) {
	peer1 := Peer{
		AllowedIPsStr: ConfigOption[string]{Value: "192.168.2.2/32"},
		Interface: PeerInterfaceConfig{
			Addresses: []Cidr{
				{Cidr: "192.168.1.2/32", Addr: "192.168.1.2", NetLength: 32},
			},
		},
	}
	peer2 := Peer{
		AllowedIPsStr:      ConfigOption[string]{Value: "10.0.2.2/32"},
		ExtraAllowedIPsStr: "10.20.2.2/32",
		Interface: PeerInterfaceConfig{
			Addresses: []Cidr{
				{Cidr: "10.0.0.2/32", Addr: "10.0.0.2", NetLength: 32},
			},
		},
	}
	iface := &Interface{Type: InterfaceTypeClient}
	expected := []Cidr{
		{Cidr: "192.168.2.2/32", Addr: "192.168.2.2", NetLength: 32},
		{Cidr: "10.0.2.2/32", Addr: "10.0.2.2", NetLength: 32},
	}
	assert.Equal(t, expected, iface.GetAllowedIPs([]Peer{peer1, peer2}))
}

func TestInterface_ManageRoutingTableReturnsCorrectValue(t *testing.T) {
	iface := &Interface{RoutingTable: "off"}
	assert.False(t, iface.ManageRoutingTable())

	iface.RoutingTable = "100"
	assert.True(t, iface.ManageRoutingTable())

	iface = &Interface{RoutingTable: "off", Backend: config.LocalBackendName}
	assert.False(t, iface.ManageRoutingTable())

	iface.RoutingTable = "100"
	assert.True(t, iface.ManageRoutingTable())

	iface = &Interface{RoutingTable: "off", Backend: "mikrotik-xxx"}
	assert.False(t, iface.ManageRoutingTable())

	iface.RoutingTable = "100"
	assert.True(t, iface.ManageRoutingTable())
}

func TestInterface_GetRoutingTableReturnsCorrectValue(t *testing.T) {
	iface := &Interface{RoutingTable: "", Backend: config.LocalBackendName}
	assert.Equal(t, 0, iface.GetRoutingTable())

	iface.RoutingTable = "off"
	assert.Equal(t, -1, iface.GetRoutingTable())

	iface.RoutingTable = "0x64"
	assert.Equal(t, 100, iface.GetRoutingTable())

	iface.RoutingTable = "200"
	assert.Equal(t, 200, iface.GetRoutingTable())
}

func TestInterface_GetRoutingTableNonLocal(t *testing.T) {
	iface := &Interface{RoutingTable: "off", Backend: "something different"}
	assert.Equal(t, -1, iface.GetRoutingTable())

	iface.RoutingTable = "0"
	assert.Equal(t, 0, iface.GetRoutingTable())

	iface.RoutingTable = "100"
	assert.Equal(t, 0, iface.GetRoutingTable())

	iface.RoutingTable = "abc"
	assert.Equal(t, 0, iface.GetRoutingTable())
}

func TestInterface_CreateDefaultPeers(t *testing.T) {
	iface := &Interface{}
	assert.False(t, iface.CreateDefaultPeers())

	iface.CreateDefaultPeer = true
	assert.False(t, iface.CreateDefaultPeers()) // still wrong type

	iface2 := &Interface{Type: InterfaceTypeServer}
	assert.False(t, iface2.CreateDefaultPeers()) // CreateDefaultPeer flag is false

	iface2.CreateDefaultPeer = true
	assert.True(t, iface2.CreateDefaultPeers())

	iface3 := &Interface{Type: InterfaceTypeClient}
	assert.False(t, iface3.CreateDefaultPeers())

	iface3.CreateDefaultPeer = true
	assert.False(t, iface3.CreateDefaultPeers())

	iface4 := &Interface{Type: InterfaceTypeAny}
	assert.False(t, iface4.CreateDefaultPeers())

	iface4.CreateDefaultPeer = true
	assert.False(t, iface4.CreateDefaultPeers())
}

func grantsFor(iface InterfaceIdentifier, provider string, at time.Time, users ...UserIdentifier) []InterfaceAccessGrant {
	grants := make([]InterfaceAccessGrant, 0, len(users))
	for _, u := range users {
		grants = append(grants, InterfaceAccessGrant{
			InterfaceIdentifier: iface, ProviderName: provider, UserIdentifier: u, EvaluatedAt: at,
		})
	}
	return grants
}

func TestInterface_IsUserAllowed_OauthFilter(t *testing.T) {
	cfg := &config.Config{}
	cfg.Auth.OpenIDConnect = []config.OpenIDConnectProvider{
		{ProviderName: "oidc1", InterfaceFilter: map[string]string{"wg-data": "^vpn-data$"}},
	}
	cfg.Auth.OAuth = []config.OAuthProvider{
		{ProviderName: "oauth1", InterfaceFilter: map[string]string{"wg-gh": "^ops$"}},
	}
	cfg.Auth.Ldap = []config.LdapProvider{
		{ProviderName: "ldap1", InterfaceFilter: map[string]string{
			"wg-data": "(memberOf=CN=VPN)",
			"wg-ldap": "(memberOf=CN=VPN)",
		}},
		{ProviderName: "ldap-old"}, // no longer filters any interface
	}
	now := time.Now()

	tests := []struct {
		name   string
		iface  Interface
		grants []InterfaceAccessGrant
		userId UserIdentifier
		expect bool
	}{
		{
			name:   "no filter on the interface",
			iface:  Interface{Identifier: "wg-world"},
			userId: "alice",
			expect: true,
		},
		{
			name:   "oidc - user granted",
			iface:  Interface{Identifier: "wg-data"},
			grants: grantsFor("wg-data", "oidc1", now, "alice"),
			userId: "alice",
			expect: true,
		},
		{
			name:   "oidc - grant of another user",
			iface:  Interface{Identifier: "wg-data"},
			grants: grantsFor("wg-data", "oidc1", now, "alice"),
			userId: "bob",
			expect: false,
		},
		{
			name:   "oidc - grant for another interface",
			iface:  Interface{Identifier: "wg-data"},
			grants: grantsFor("wg-gh", "oidc1", now, "alice"),
			userId: "alice",
			expect: false,
		},
		{
			name:   "oidc - no grant denies",
			iface:  Interface{Identifier: "wg-data"},
			userId: "alice",
			expect: false,
		},
		{
			name:   "plain oauth - user granted",
			iface:  Interface{Identifier: "wg-gh"},
			grants: grantsFor("wg-gh", "oauth1", now, "alice"),
			userId: "alice",
			expect: true,
		},
		{
			name:   "plain oauth - interface is restricted",
			iface:  Interface{Identifier: "wg-gh"},
			userId: "alice",
			expect: false,
		},
		{
			name: "ldap and oidc on the same interface - ldap list grants",
			iface: Interface{Identifier: "wg-data", LdapAllowedUsers: map[string][]UserIdentifier{
				"ldap1": {"carol"},
			}},
			userId: "carol",
			expect: true,
		},
		{
			name: "ldap and oidc on the same interface - oidc grant grants",
			iface: Interface{Identifier: "wg-data", LdapAllowedUsers: map[string][]UserIdentifier{
				"ldap1": {"carol"},
			}},
			grants: grantsFor("wg-data", "oidc1", now, "alice"),
			userId: "alice",
			expect: true,
		},
		{
			name:   "ldap-only interface ignores oidc grants",
			iface:  Interface{Identifier: "wg-ldap"},
			grants: grantsFor("wg-ldap", "oidc1", now, "alice"),
			userId: "alice",
			expect: false,
		},
		{
			name:   "grant of a provider without a filter for the interface is ignored",
			iface:  Interface{Identifier: "wg-data"},
			grants: grantsFor("wg-data", "oauth1", now, "alice"),
			userId: "alice",
			expect: false,
		},
		{
			name: "ldap list of a provider without a filter for the interface is ignored",
			iface: Interface{Identifier: "wg-data", LdapAllowedUsers: map[string][]UserIdentifier{
				"ldap-old": {"alice"},
			}},
			userId: "alice",
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, tt.iface.IsUserAllowed(tt.userId, cfg, tt.grants))
		})
	}
}

func TestInterface_IsUserAllowed_OauthMaxAge(t *testing.T) {
	short := 10 * time.Minute
	noExpiry := time.Duration(0)
	cfg := &config.Config{}
	cfg.Auth.OpenIDConnect = []config.OpenIDConnectProvider{
		{ProviderName: "default", InterfaceFilter: map[string]string{"wg-default": "^g$"}},
		{ProviderName: "short", InterfaceFilter: map[string]string{"wg-short": "^g$"}, InterfaceFilterMaxAge: &short},
		{ProviderName: "never", InterfaceFilter: map[string]string{"wg-never": "^g$"}, InterfaceFilterMaxAge: &noExpiry},
	}
	cfg.Auth.OAuth = []config.OAuthProvider{
		{ProviderName: "plain", InterfaceFilter: map[string]string{"wg-plain": "^g$"}, InterfaceFilterMaxAge: &short},
	}
	now := time.Now()

	tests := []struct {
		name     string
		iface    InterfaceIdentifier
		provider string
		at       time.Time
		expect   bool
	}{
		{name: "fresh grant", iface: "wg-short", provider: "short", at: now.Add(-time.Minute), expect: true},
		{name: "expired grant", iface: "wg-short", provider: "short", at: now.Add(-11 * time.Minute)},
		{name: "grant exactly max age old", iface: "wg-short", provider: "short", at: now.Add(-short), expect: true},
		{name: "plain oauth - expired grant", iface: "wg-plain", provider: "plain", at: now.Add(-11 * time.Minute)},
		{
			name: "default max age - within", iface: "wg-default", provider: "default",
			at: now.Add(-config.DefaultInterfaceFilterMaxAge + time.Minute), expect: true,
		},
		{
			name: "default max age - expired", iface: "wg-default", provider: "default",
			at: now.Add(-config.DefaultInterfaceFilterMaxAge - time.Minute),
		},
		{name: "grant from the future is invalid", iface: "wg-short", provider: "short", at: now.Add(2 * time.Minute)},
		{
			name: "small clock skew is tolerated", iface: "wg-short", provider: "short",
			at: now.Add(30 * time.Second), expect: true,
		},
		{name: "max age 0 never expires", iface: "wg-never", provider: "never", at: now.AddDate(-1, 0, 0), expect: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iface := Interface{Identifier: tt.iface}
			grants := grantsFor(tt.iface, tt.provider, tt.at, "alice")
			assert.Equal(t, tt.expect, iface.isUserAllowedAt("alice", cfg, grants, now))
		})
	}
}

func TestInterface_CopyCalculatedAttributesKeepsLdapAllowedUsers(t *testing.T) {
	stored := &Interface{
		Identifier:       "wg0",
		LdapAllowedUsers: map[string][]UserIdentifier{"ldap1": {"carol"}},
	}
	edited := &Interface{Identifier: "wg0", DisplayName: "edited in the UI"}

	edited.CopyCalculatedAttributes(stored)

	assert.Equal(t, stored.LdapAllowedUsers, edited.LdapAllowedUsers)
}
