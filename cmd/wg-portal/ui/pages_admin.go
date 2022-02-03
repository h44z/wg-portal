package ui

import (
	"fmt"
	"net/http"

	"github.com/h44z/wg-portal/internal/persistence"

	"github.com/gin-gonic/gin"
)

func (h *handler) handleAdminIndexGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentSession := h.session.GetData(c)

		interfaces, err := h.backend.GetInterfaces()
		if err != nil {
			h.HandleError(c, http.StatusInternalServerError, err, "failed to load available interfaces")
			return
		}

		var iface *persistence.InterfaceConfig
		if currentSession.InterfaceIdentifier != "" {
			iface, err = h.backend.GetInterface(currentSession.InterfaceIdentifier)
			if err != nil {
				h.HandleError(c, http.StatusInternalServerError, err,
					fmt.Sprintf("failed to load selected interface %s", currentSession.InterfaceIdentifier))
				return
			}
		}

		c.HTML(http.StatusOK, "admin_index.gohtml", gin.H{
			"Route":          c.Request.URL.Path,
			"Alerts":         h.session.GetFlashes(c),
			"Session":        currentSession,
			"Static":         h.getStaticData(),
			"Interface":      iface,
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
			"Interfaces": interfaces,
			"TotalPeers": 12,
		})
	}
}

func (h *handler) handleAdminNewGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentSession := h.session.GetData(c)

		interfaces, err := h.backend.GetInterfaces()
		if err != nil {
			h.HandleError(c, http.StatusInternalServerError, err, "failed to load available interfaces")
			return
		}

		importableInterfaces, err := h.backend.GetImportableInterfaces()
		if err != nil {
			h.HandleError(c, http.StatusInternalServerError, err, "failed to get importable interfaces")
			return
		}

		var iface *persistence.InterfaceConfig
		if currentSession.InterfaceIdentifier != "" {
			iface, err = h.backend.GetInterface(currentSession.InterfaceIdentifier)
			if err != nil {
				h.HandleError(c, http.StatusInternalServerError, err,
					fmt.Sprintf("failed to load selected interface %s", currentSession.InterfaceIdentifier))
				return
			}
		}

		c.HTML(http.StatusOK, "admin_new_interface.gohtml", gin.H{
			"Route":                c.Request.URL.Path,
			"Alerts":               h.session.GetFlashes(c),
			"Session":              currentSession,
			"Static":               h.getStaticData(),
			"Interface":            iface,
			"Interfaces":           interfaces,
			"ImportableInterfaces": importableInterfaces,
		})
	}
}

func (h *handler) handleAdminCreateGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentSession := h.session.GetData(c)

		interfaces, err := h.backend.GetInterfaces()
		if err != nil {
			h.HandleError(c, http.StatusInternalServerError, err, "failed to load available interfaces")
			return
		}

		c.HTML(http.StatusOK, "admin_create_interface.gohtml", gin.H{
			"Route":      c.Request.URL.Path,
			"Alerts":     h.session.GetFlashes(c),
			"Session":    currentSession,
			"Static":     h.getStaticData(),
			"Interface":  nil,
			"Interfaces": interfaces,
		})
	}
}

func (h *handler) handleAdminImportGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentSession := h.session.GetData(c)

		interfaces, err := h.backend.GetInterfaces()
		if err != nil {
			h.HandleError(c, http.StatusInternalServerError, err, "failed to load available interfaces")
			return
		}

		if currentSession.InterfaceIdentifier == "" && len(interfaces) > 0 {
			currentSession.InterfaceIdentifier = interfaces[0].Identifier
			h.session.SetData(c, currentSession)
		}

		var iface *persistence.InterfaceConfig
		if currentSession.InterfaceIdentifier != "" {
			iface, err = h.backend.GetInterface(currentSession.InterfaceIdentifier)
			if err != nil {
				h.HandleError(c, http.StatusInternalServerError, err,
					fmt.Sprintf("failed to load selected interface %s", currentSession.InterfaceIdentifier))
				return
			}
		}

		c.HTML(http.StatusOK, "admin_import_interface.gohtml", gin.H{
			"Route":      c.Request.URL.Path,
			"Alerts":     h.session.GetFlashes(c),
			"Session":    currentSession,
			"Static":     h.getStaticData(),
			"Interface":  iface,
			"Interfaces": interfaces,
		})
	}
}
