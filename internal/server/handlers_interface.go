package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/common"
)

func (s *Server) GetAdminEditInterface(c *gin.Context) {
	device := s.peers.GetDevice()
	users := s.peers.GetAllPeers()

	currentSession, err := s.setFormInSession(c, device)
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Session error", err.Error())
		return
	}

	c.HTML(http.StatusOK, "admin_edit_interface.html", struct {
		Route        string
		Alerts       []FlashData
		Session      SessionData
		Static       StaticData
		Peers        []Peer
		Device       Device
		EditableKeys bool
	}{
		Route:        c.Request.URL.Path,
		Alerts:       GetFlashes(c),
		Session:      currentSession,
		Static:       s.getStaticData(),
		Peers:        users,
		Device:       currentSession.FormData.(Device),
		EditableKeys: s.config.Core.EditableKeys,
	})
}

func (s *Server) PostAdminEditInterface(c *gin.Context) {
	currentSession := GetSessionData(c)
	var formDevice Device
	if currentSession.FormData != nil {
		formDevice = currentSession.FormData.(Device)
	}
	if err := c.ShouldBind(&formDevice); err != nil {
		_ = s.updateFormInSession(c, formDevice)
		SetFlashMessage(c, err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit?formerr=bind")
		return
	}
	// Clean list input
	formDevice.IPs = common.ParseStringList(formDevice.IPsStr)
	formDevice.AllowedIPs = common.ParseStringList(formDevice.AllowedIPsStr)
	formDevice.DNS = common.ParseStringList(formDevice.DNSStr)
	formDevice.IPsStr = common.ListToString(formDevice.IPs)
	formDevice.AllowedIPsStr = common.ListToString(formDevice.AllowedIPs)
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
	err = s.WriteWireGuardConfigFile()
	if err != nil {
		_ = s.updateFormInSession(c, formDevice)
		SetFlashMessage(c, "Failed to update WireGuard config-file: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit?formerr=update")
		return
	}

	// Update interface IP address
	if s.config.WG.ManageIPAddresses {
		if err := s.wg.SetIPAddress(formDevice.IPs); err != nil {
			_ = s.updateFormInSession(c, formDevice)
			SetFlashMessage(c, "Failed to update ip address: "+err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/device/edit?formerr=update")
		}
		if err := s.wg.SetMTU(formDevice.Mtu); err != nil {
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
	device := s.peers.GetDevice()
	users := s.peers.GetActivePeers()
	cfg, err := device.GetConfigFile(users)
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
	device := s.peers.GetDevice()
	users := s.peers.GetAllPeers()

	for _, user := range users {
		user.AllowedIPs = device.AllowedIPs
		user.AllowedIPsStr = device.AllowedIPsStr
		if err := s.peers.UpdatePeer(user); err != nil {
			SetFlashMessage(c, err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/device/edit")
		}
	}

	SetFlashMessage(c, "Allowed IP's updated for all clients.", "success")
	c.Redirect(http.StatusSeeOther, "/admin/device/edit")
	return
}
