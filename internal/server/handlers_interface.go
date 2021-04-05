package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/common"
	"github.com/h44z/wg-portal/internal/wireguard"
	csrf "github.com/utrack/gin-csrf"
)

func (s *Server) GetAdminEditInterface(c *gin.Context) {
	currentSession := GetSessionData(c)
	device := s.peers.GetDevice(currentSession.DeviceName)
	currentSession, err := s.setFormInSession(c, device)
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Session error", err.Error())
		return
	}

	c.HTML(http.StatusOK, "admin_edit_interface.html", gin.H{
		"Route":        c.Request.URL.Path,
		"Alerts":       GetFlashes(c),
		"Session":      currentSession,
		"Static":       s.getStaticData(),
		"Device":       currentSession.FormData.(wireguard.Device),
		"EditableKeys": s.config.Core.EditableKeys,
		"DeviceNames":  s.GetDeviceNames(),
		"Csrf":         csrf.GetToken(c),
	})
}

func (s *Server) PostAdminEditInterface(c *gin.Context) {
	currentSession := GetSessionData(c)
	var formDevice wireguard.Device
	if currentSession.FormData != nil {
		formDevice = currentSession.FormData.(wireguard.Device)
	}
	if err := c.ShouldBind(&formDevice); err != nil {
		_ = s.updateFormInSession(c, formDevice)
		SetFlashMessage(c, err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit?formerr=bind")
		return
	}
	// Clean list input
	formDevice.IPsStr = common.ListToString(common.ParseStringList(formDevice.IPsStr))
	formDevice.DefaultAllowedIPsStr = common.ListToString(common.ParseStringList(formDevice.DefaultAllowedIPsStr))
	formDevice.DNSStr = common.ListToString(common.ParseStringList(formDevice.DNSStr))

	// Clean interface parameters based on interface type
	switch formDevice.Type {
	case wireguard.DeviceTypeClient:
		formDevice.ListenPort = 0
		formDevice.DefaultEndpoint = ""
		formDevice.DefaultAllowedIPsStr = ""
		formDevice.DefaultPersistentKeepalive = 0
		formDevice.SaveConfig = false
	case wireguard.DeviceTypeServer:
	}

	// Update WireGuard device
	err := s.wg.UpdateDevice(formDevice.DeviceName, formDevice.GetConfig())
	if err != nil {
		_ = s.updateFormInSession(c, formDevice)
		SetFlashMessage(c, "Failed to update device in WireGuard: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit?formerr=wg")
		return
	}

	// Update in database
	err = s.peers.UpdateDevice(formDevice)
	if err != nil {
		_ = s.updateFormInSession(c, formDevice)
		SetFlashMessage(c, "Failed to update device in database: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit?formerr=update")
		return
	}

	// Update WireGuard config file
	err = s.WriteWireGuardConfigFile(currentSession.DeviceName)
	if err != nil {
		_ = s.updateFormInSession(c, formDevice)
		SetFlashMessage(c, "Failed to update WireGuard config-file: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit?formerr=update")
		return
	}

	// Update interface IP address
	if s.config.WG.ManageIPAddresses {
		if err := s.wg.SetIPAddress(currentSession.DeviceName, formDevice.GetIPAddresses()); err != nil {
			_ = s.updateFormInSession(c, formDevice)
			SetFlashMessage(c, "Failed to update ip address: "+err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/device/edit?formerr=update")
		}
		if err := s.wg.SetMTU(currentSession.DeviceName, formDevice.Mtu); err != nil {
			_ = s.updateFormInSession(c, formDevice)
			SetFlashMessage(c, "Failed to update MTU: "+err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/device/edit?formerr=update")
		}
	}

	SetFlashMessage(c, "Changes applied successfully!", "success")
	if !s.config.WG.ManageIPAddresses {
		SetFlashMessage(c, "WireGuard must be restarted to apply ip changes.", "warning")
	}
	c.Redirect(http.StatusSeeOther, "/admin/device/edit")
}

func (s *Server) GetInterfaceConfig(c *gin.Context) {
	currentSession := GetSessionData(c)
	device := s.peers.GetDevice(currentSession.DeviceName)
	peers := s.peers.GetActivePeers(device.DeviceName)
	cfg, err := device.GetConfigFile(peers)
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "ConfigFile error", err.Error())
		return
	}

	filename := strings.ToLower(device.DeviceName) + ".conf"

	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Data(http.StatusOK, "application/config", cfg)
	return
}

func (s *Server) GetSaveConfig(c *gin.Context) {
	currentSession := GetSessionData(c)

	err := s.WriteWireGuardConfigFile(currentSession.DeviceName)
	if err != nil {
		SetFlashMessage(c, "Failed to save WireGuard config-file: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/")
		return
	}

	SetFlashMessage(c, "Updated WireGuard config-file", "success")
	c.Redirect(http.StatusSeeOther, "/admin/")
	return
}

func (s *Server) GetApplyGlobalConfig(c *gin.Context) {
	currentSession := GetSessionData(c)
	device := s.peers.GetDevice(currentSession.DeviceName)
	peers := s.peers.GetAllPeers(device.DeviceName)

	if device.Type == wireguard.DeviceTypeClient {
		SetFlashMessage(c, "Cannot apply global configuration while interface is in client mode.", "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit")
		return
	}

	updateCounter := 0
	for _, peer := range peers {
		if peer.IgnoreGlobalSettings {
			continue
		}

		peer.AllowedIPsStr = device.DefaultAllowedIPsStr
		peer.Endpoint = device.DefaultEndpoint
		peer.PersistentKeepalive = device.DefaultPersistentKeepalive
		peer.DNSStr = device.DNSStr
		peer.Mtu = device.Mtu

		if err := s.peers.UpdatePeer(peer); err != nil {
			SetFlashMessage(c, err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/device/edit")
			return
		}
		updateCounter++
	}

	SetFlashMessage(c, fmt.Sprintf("Global configuration updated for %d clients.", updateCounter), "success")
	c.Redirect(http.StatusSeeOther, "/admin/device/edit")
	return
}
