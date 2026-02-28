package wireguard

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

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
