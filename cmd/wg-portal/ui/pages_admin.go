package ui

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/h44z/wg-portal/internal/persistence"

	"github.com/gin-gonic/gin"
)

func (h *handler) processMetaRequest(c *gin.Context) bool {
	if newPageStr := c.Query("page"); newPageStr != "" {
		currentSession := h.session.GetData(c)
		newPage, err := strconv.Atoi(newPageStr)
		if err != nil {
			return false
		}
		currentSession.CurrentPage = newPage
		h.session.SetData(c, currentSession)

		return true
	}

	if newPageSizeStr := c.Query("pagesize"); newPageSizeStr != "" {
		currentSession := h.session.GetData(c)
		newPageSize, err := strconv.Atoi(newPageSizeStr)
		if err != nil {
			return false
		}
		if newPageSize < 25 {
			return false
		}
		currentSession.CurrentPage = newPageSize
		h.session.SetData(c, currentSession)

		return true
	}

	if sort := c.Query("sort"); sort != "" {
		currentSession := h.session.GetData(c)
		if currentSession.SortedBy["peers"] != sort {
			currentSession.SortedBy["peers"] = sort
			currentSession.SortDirection["peers"] = "asc"
		} else {
			if currentSession.SortDirection["peers"] == "asc" {
				currentSession.SortDirection["peers"] = "desc"
			} else {
				currentSession.SortDirection["peers"] = "asc"
			}
		}
		h.session.SetData(c, currentSession)

		return true
	}

	if iface := c.Query("iface"); iface != "" {
		currentSession := h.session.GetData(c)
		currentSession.InterfaceIdentifier = persistence.InterfaceIdentifier(iface)
		currentSession.CurrentPage = 1 // reset page
		h.session.SetData(c, currentSession)
		return true
	}

	if search, searching := c.GetQuery("search"); searching {
		currentSession := h.session.GetData(c)
		currentSession.Search["peers"] = search
		currentSession.CurrentPage = 1 // reset page
		h.session.SetData(c, currentSession)

		return true
	}

	return false
}

func (h *handler) handleAdminIndexGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.processMetaRequest(c) {
			c.Redirect(http.StatusSeeOther, c.Request.URL.Path)
			return
		}

		currentSession := h.session.GetData(c)

		interfaces, err := h.backend.GetInterfaces()
		if err != nil {
			h.HandleError(c, http.StatusInternalServerError, err, "failed to load available interfaces")
			return
		}

		var iface *persistence.InterfaceConfig
		var peers []*persistence.PeerConfig
		if currentSession.InterfaceIdentifier != "" {
			iface, err = h.backend.GetInterface(currentSession.InterfaceIdentifier)
			if err != nil {
				h.HandleError(c, http.StatusInternalServerError, err,
					fmt.Sprintf("failed to load selected interface %s", currentSession.InterfaceIdentifier))
				return
			}
			peers, err = h.backend.GetPeers(currentSession.InterfaceIdentifier)
			if err != nil {
				h.HandleError(c, http.StatusInternalServerError, err,
					fmt.Sprintf("failed to load peers for %s", currentSession.InterfaceIdentifier))
				return
			}
		}

		// TODO test peers
		/*iface = &persistence.InterfaceConfig{
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
			DisplayName:                "",
			Type:                       "",
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
		}
		peers = make([]*persistence.PeerConfig, 33)
		for i := 0; i < 33; i++ {
			peers[i] = &persistence.PeerConfig{Identifier: persistence.PeerIdentifier(fmt.Sprintf("%d", i)), DisplayName: fmt.Sprintf("Name%d", i), Interface: &persistence.PeerInterfaceConfig{}}
		}*/
		// TODO end test peers

		peers = sortAndFilterPeers(currentSession, peers)

		activePeers := 0
		for _, p := range peers {
			if p.DisabledAt.Valid { // disabled peer
				continue
			}
			activePeers++
		}

		start := (currentSession.CurrentPage - 1) * currentSession.PageSize
		end := start + currentSession.PageSize
		if end >= len(peers) {
			end = len(peers)
		}
		pagedPeers := peers[start:end]

		c.HTML(http.StatusOK, "admin_index.gohtml", gin.H{
			"Route":               c.Request.URL.Path,
			"Alerts":              h.session.GetFlashes(c),
			"Session":             currentSession,
			"Static":              h.getStaticData(),
			"Interface":           iface,
			"InterfacePeers":      peers,
			"PagedInterfacePeers": pagedPeers,
			"Interfaces":          interfaces,
			"TotalPeers":          len(peers),
			"ActivePeers":         activePeers,
			"Page":                currentSession.CurrentPage,
			"PageSize":            currentSession.PageSize,
			"TotalPages":          int(math.Ceil(float64(len(peers)) / float64(currentSession.PageSize))),
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

func sortAndFilterPeers(session SessionData, peers []*persistence.PeerConfig) []*persistence.PeerConfig {
	filteredPeers := make([]*persistence.PeerConfig, 0, len(peers))
	search := session.Search["peers"]
	for i := range peers {
		if search == "" ||
			strings.Contains(string(peers[i].Identifier), strings.ToLower(search)) ||
			strings.Contains(peers[i].DisplayName, search) ||
			strings.Contains(string(peers[i].UserIdentifier), search) ||
			strings.Contains(peers[i].PublicKey, search) {
			filteredPeers = append(filteredPeers, peers[i])
		}
	}

	sortKey := session.SortedBy["peers"]
	sortDirection := session.SortDirection["peers"]
	sortPeers(sortKey, sortDirection, filteredPeers)

	return filteredPeers
}

func sortPeers(sortKey string, sortDirection string, peers []*persistence.PeerConfig) {
	sort.Slice(peers, func(i, j int) bool {
		var sortValueLeft string
		var sortValueRight string

		switch sortKey {
		case "id":
			sortValueLeft = string(peers[i].Identifier)
			sortValueRight = string(peers[j].Identifier)
		case "pubKey":
			sortValueLeft = peers[i].PublicKey
			sortValueRight = peers[j].PublicKey
		case "displayName":
			sortValueLeft = peers[i].DisplayName
			sortValueRight = peers[j].DisplayName
		case "ip":
			sortValueLeft = peers[i].Interface.AddressStr.GetValue()
			sortValueRight = peers[j].Interface.AddressStr.GetValue()
		case "endpoint":
			sortValueLeft = peers[i].Endpoint.GetValue()
			sortValueRight = peers[j].Endpoint.GetValue()
			/*case "handshake":
			if peers[i].Peer == nil {
				return true
			} else if peers[j].Peer == nil {
				return false
			}
			sortValueLeft = peers[i].Peer.LastHandshakeTime.Format(time.RFC3339)
			sortValueRight = peers[j].Peer.LastHandshakeTime.Format(time.RFC3339)*/
		}

		if sortDirection == "asc" {
			return sortValueLeft < sortValueRight
		} else {
			return sortValueLeft > sortValueRight
		}
	})
}
