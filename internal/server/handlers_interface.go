package server

import (
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
		"DeviceNames":  s.wg.Cfg.DeviceNames,
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
	formDevice.IPs = common.ParseStringList(formDevice.IPsStr)
	formDevice.DefaultAllowedIPs = common.ParseStringList(formDevice.DefaultAllowedIPsStr)
	formDevice.DNS = common.ParseStringList(formDevice.DNSStr)
	formDevice.IPsStr = common.ListToString(formDevice.IPs)
	formDevice.DefaultAllowedIPsStr = common.ListToString(formDevice.DefaultAllowedIPs)
	formDevice.DNSStr = common.ListToString(formDevice.DNS)

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
		if err := s.wg.SetIPAddress(currentSession.DeviceName, formDevice.IPs); err != nil {
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

func (s *Server) GetApplyGlobalConfig(c *gin.Context) {
	currentSession := GetSessionData(c)
	device := s.peers.GetDevice(currentSession.DeviceName)
	peers := s.peers.GetAllPeers(device.DeviceName)

	for _, peer := range peers {
		peer.AllowedIPs = device.DefaultAllowedIPs
		peer.AllowedIPsStr = device.DefaultAllowedIPsStr
		if err := s.peers.UpdatePeer(peer); err != nil {
			SetFlashMessage(c, err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/device/edit")
		}
	}

	SetFlashMessage(c, "Allowed IP's updated for all clients.", "success")
	c.Redirect(http.StatusSeeOther, "/admin/device/edit")
	return
}
