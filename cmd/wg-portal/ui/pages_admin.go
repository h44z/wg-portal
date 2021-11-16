package ui

import (
	"net/http"

	"github.com/h44z/wg-portal/internal/persistence"

	"github.com/gin-gonic/gin"
)

func (h *handler) handleAdminIndexGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentSession := h.session.GetData(c)

		c.HTML(http.StatusOK, "admin_index.gohtml", gin.H{
			"Route":   c.Request.URL.Path,
			"Alerts":  h.session.GetFlashes(c),
			"Session": currentSession,
			"Static":  h.getStaticData(),
			"Interface": persistence.InterfaceConfig{
				BaseModel:                  persistence.BaseModel{},
				Identifier:                 "wg0",
				KeyPair:                    persistence.KeyPair{},
				ListenPort:                 0,
				AddressStr:                 "",
				DnsStr:                     "",
				DnsSearchStr:               "",
				Mtu:                        0,
				FirewallMark:               0,
				RoutingTable:               "",
				PreUp:                      "",
				PostUp:                     "",
				PreDown:                    "",
				PostDown:                   "",
				SaveConfig:                 false,
				Enabled:                    false,
				DisplayName:                "wgX descr",
				Type:                       persistence.InterfaceTypeServer,
				DriverType:                 "",
				PeerDefNetworkStr:          "",
				PeerDefDnsStr:              "",
				PeerDefDnsSearchStr:        "",
				PeerDefEndpoint:            "",
				PeerDefAllowedIPsStr:       "",
				PeerDefMtu:                 0,
				PeerDefPersistentKeepalive: 0,
				PeerDefFirewallMark:        0,
				PeerDefRoutingTable:        "",
				PeerDefPreUp:               "",
				PeerDefPostUp:              "",
				PeerDefPreDown:             "",
				PeerDefPostDown:            "",
			},
			"InterfacePeers": []persistence.PeerConfig{},
			"PagedInterfacePeers": []persistence.PeerConfig{
				{
					Endpoint: persistence.StringConfigOption{
						Value:       "vpn.test.net",
						Overridable: false,
					},
					AllowedIPsStr: persistence.StringConfigOption{
						Value:       "10.0.0.0/8,192.168.1.0/24",
						Overridable: false,
					},
					KeyPair: persistence.KeyPair{
						PrivateKey: "privkey",
						PublicKey:  "pubkey",
					},
					PresharedKey: "psk",
					PersistentKeepalive: persistence.IntConfigOption{
						Value:       16,
						Overridable: true,
					},
					DisplayName:    "Display Name",
					Identifier:     "abc123",
					UserIdentifier: "nouser",
					Interface: &persistence.PeerInterfaceConfig{
						Identifier: "wg0",
						Type:       persistence.InterfaceTypeServer,
						PublicKey:  "srvpub",
						AddressStr: persistence.StringConfigOption{
							Value: "10.0.0.1/32,192.168.1.1/32",
						},
						DnsStr:       persistence.StringConfigOption{},
						DnsSearchStr: persistence.StringConfigOption{},
						Mtu:          persistence.IntConfigOption{},
						FirewallMark: persistence.Int32ConfigOption{},
						RoutingTable: persistence.StringConfigOption{},
						PreUp:        persistence.StringConfigOption{},
						PostUp:       persistence.StringConfigOption{},
						PreDown:      persistence.StringConfigOption{},
						PostDown:     persistence.StringConfigOption{},
					},
				},
			},
			"InterfaceNames": map[string]string{"wgX": "wgX descr"},
			"TotalPeers":     12,
		})
	}
}
