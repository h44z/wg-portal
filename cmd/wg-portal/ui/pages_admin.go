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
			"InterfacePeers":      []persistence.PeerConfig{},
			"PagedInterfacePeers": []persistence.PeerConfig{},
			"InterfaceNames":      map[string]string{"wgX": "wgX descr"},
			"TotalPeers":          12,
		})
	}
}
