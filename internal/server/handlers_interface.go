package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/common"
)

func (s *Server) GetAdminEditInterface(c *gin.Context) {
	device := s.users.GetDevice()
	users := s.users.GetAllUsers()

	currentSession, err := s.setFormInSession(c, device)
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Session error", err.Error())
		return
	}

	c.HTML(http.StatusOK, "admin_edit_interface.html", struct {
		Route   string
		Alerts  []FlashData
		Session SessionData
		Static  StaticData
		Peers   []User
		Device  Device
	}{
		Route:   c.Request.URL.Path,
		Alerts:  s.getFlashes(c),
		Session: currentSession,
		Static:  s.getStaticData(),
		Peers:   users,
		Device:  currentSession.FormData.(Device),
	})
}

func (s *Server) PostAdminEditInterface(c *gin.Context) {
	currentSession := s.getSessionData(c)
	var formDevice Device
	if currentSession.FormData != nil {
		formDevice = currentSession.FormData.(Device)
	}
	if err := c.ShouldBind(&formDevice); err != nil {
		_ = s.updateFormInSession(c, formDevice)
		s.setFlashMessage(c, err.Error(), "danger")
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
	err := s.wg.UpdateDevice(formDevice.DeviceName, formDevice.GetDeviceConfig())
	if err != nil {
		_ = s.updateFormInSession(c, formDevice)
		s.setFlashMessage(c, "Failed to update device in WireGuard: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit?formerr=wg")
		return
	}

	// Update in database
	err = s.users.UpdateDevice(formDevice)
	if err != nil {
		_ = s.updateFormInSession(c, formDevice)
		s.setFlashMessage(c, "Failed to update device in database: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit?formerr=update")
		return
	}

	s.setFlashMessage(c, "Changes applied successfully!", "success")
	s.setFlashMessage(c, "WireGuard must be restarted to apply ip changes.", "warning")
	c.Redirect(http.StatusSeeOther, "/admin/device/edit")
}

func (s *Server) GetInterfaceConfig(c *gin.Context) {
	device := s.users.GetDevice()
	users := s.users.GetActiveUsers()
	cfg, err := device.GetDeviceConfigFile(users)
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
	device := s.users.GetDevice()
	users := s.users.GetAllUsers()

	for _, user := range users {
		user.AllowedIPs = device.AllowedIPs
		user.AllowedIPsStr = device.AllowedIPsStr
		if err := s.users.UpdateUser(user); err != nil {
			s.setFlashMessage(c, err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/device/edit")
		}
	}

	s.setFlashMessage(c, "Allowed ip's updated for all clients.", "success")
	c.Redirect(http.StatusSeeOther, "/admin/device/edit")
	return
}
