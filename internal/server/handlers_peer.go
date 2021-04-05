package server

import (
	"bytes"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/common"
	"github.com/h44z/wg-portal/internal/users"
	"github.com/h44z/wg-portal/internal/wireguard"
	"github.com/sirupsen/logrus"
	"github.com/tatsushid/go-fastping"
	csrf "github.com/utrack/gin-csrf"
)

type LdapCreateForm struct {
	Emails     string `form:"email" binding:"required"`
	Identifier string `form:"identifier" binding:"required,lte=20"`
}

func (s *Server) GetAdminEditPeer(c *gin.Context) {
	peer := s.peers.GetPeerByKey(c.Query("pkey"))

	currentSession, err := s.setFormInSession(c, peer)
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Session error", err.Error())
		return
	}

	c.HTML(http.StatusOK, "admin_edit_client.html", gin.H{
		"Route":        c.Request.URL.Path,
		"Alerts":       GetFlashes(c),
		"Session":      currentSession,
		"Static":       s.getStaticData(),
		"Peer":         currentSession.FormData.(wireguard.Peer),
		"EditableKeys": s.config.Core.EditableKeys,
		"Device":       s.peers.GetDevice(currentSession.DeviceName),
		"DeviceNames":  s.GetDeviceNames(),
		"AdminEmail":   s.config.Core.AdminUser,
		"Csrf":         csrf.GetToken(c),
	})
}

func (s *Server) PostAdminEditPeer(c *gin.Context) {
	currentPeer := s.peers.GetPeerByKey(c.Query("pkey"))
	urlEncodedKey := url.QueryEscape(c.Query("pkey"))

	currentSession := GetSessionData(c)
	var formPeer wireguard.Peer
	if currentSession.FormData != nil {
		formPeer = currentSession.FormData.(wireguard.Peer)
	}
	if err := c.ShouldBind(&formPeer); err != nil {
		_ = s.updateFormInSession(c, formPeer)
		SetFlashMessage(c, "failed to bind form data: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey+"&formerr=bind")
		return
	}

	// Clean list input
	formPeer.IPsStr = common.ListToString(common.ParseStringList(formPeer.IPsStr))
	formPeer.AllowedIPsStr = common.ListToString(common.ParseStringList(formPeer.AllowedIPsStr))

	disabled := c.PostForm("isdisabled") != ""
	now := time.Now()
	if disabled && currentPeer.DeactivatedAt == nil {
		formPeer.DeactivatedAt = &now
	} else if !disabled {
		formPeer.DeactivatedAt = nil
	}

	// Update in database
	if err := s.UpdatePeer(formPeer, now); err != nil {
		_ = s.updateFormInSession(c, formPeer)
		SetFlashMessage(c, "failed to update user: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey+"&formerr=update")
		return
	}

	SetFlashMessage(c, "changes applied successfully", "success")
	c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey)
}

func (s *Server) GetAdminCreatePeer(c *gin.Context) {
	currentSession, err := s.setNewPeerFormInSession(c)
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Session error", err.Error())
		return
	}
	c.HTML(http.StatusOK, "admin_edit_client.html", gin.H{
		"Route":        c.Request.URL.Path,
		"Alerts":       GetFlashes(c),
		"Session":      currentSession,
		"Static":       s.getStaticData(),
		"Peer":         currentSession.FormData.(wireguard.Peer),
		"EditableKeys": s.config.Core.EditableKeys,
		"Device":       s.peers.GetDevice(currentSession.DeviceName),
		"DeviceNames":  s.GetDeviceNames(),
		"AdminEmail":   s.config.Core.AdminUser,
		"Csrf":         csrf.GetToken(c),
	})
}

func (s *Server) PostAdminCreatePeer(c *gin.Context) {
	currentSession := GetSessionData(c)
	var formPeer wireguard.Peer
	if currentSession.FormData != nil {
		formPeer = currentSession.FormData.(wireguard.Peer)
	}
	if err := c.ShouldBind(&formPeer); err != nil {
		_ = s.updateFormInSession(c, formPeer)
		SetFlashMessage(c, "failed to bind form data: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/create?formerr=bind")
		return
	}

	// Clean list input
	formPeer.IPsStr = common.ListToString(common.ParseStringList(formPeer.IPsStr))
	formPeer.AllowedIPsStr = common.ListToString(common.ParseStringList(formPeer.AllowedIPsStr))

	disabled := c.PostForm("isdisabled") != ""
	now := time.Now()
	if disabled {
		formPeer.DeactivatedAt = &now
	}

	if err := s.CreatePeer(currentSession.DeviceName, formPeer); err != nil {
		_ = s.updateFormInSession(c, formPeer)
		SetFlashMessage(c, "failed to add user: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/create?formerr=create")
		return
	}

	SetFlashMessage(c, "client created successfully", "success")
	c.Redirect(http.StatusSeeOther, "/admin")
}

func (s *Server) GetAdminCreateLdapPeers(c *gin.Context) {
	currentSession, err := s.setFormInSession(c, LdapCreateForm{Identifier: "Default"})
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Session error", err.Error())
		return
	}

	c.HTML(http.StatusOK, "admin_create_clients.html", gin.H{
		"Route":       c.Request.URL.Path,
		"Alerts":      GetFlashes(c),
		"Session":     currentSession,
		"Static":      s.getStaticData(),
		"Users":       s.users.GetFilteredAndSortedUsers("lastname", "asc", ""),
		"FormData":    currentSession.FormData.(LdapCreateForm),
		"Device":      s.peers.GetDevice(currentSession.DeviceName),
		"DeviceNames": s.GetDeviceNames(),
		"Csrf":        csrf.GetToken(c),
	})
}

func (s *Server) PostAdminCreateLdapPeers(c *gin.Context) {
	currentSession := GetSessionData(c)
	var formData LdapCreateForm
	if currentSession.FormData != nil {
		formData = currentSession.FormData.(LdapCreateForm)
	}
	if err := c.ShouldBind(&formData); err != nil {
		_ = s.updateFormInSession(c, formData)
		SetFlashMessage(c, "failed to bind form data: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/createldap?formerr=bind")
		return
	}

	emails := common.ParseStringList(formData.Emails)
	for i := range emails {
		// TODO: also check email addr for validity?
		if !strings.ContainsRune(emails[i], '@') {
			_ = s.updateFormInSession(c, formData)
			SetFlashMessage(c, "invalid email address: "+emails[i], "danger")
			c.Redirect(http.StatusSeeOther, "/admin/peer/createldap?formerr=mail")
			return
		}
	}

	logrus.Infof("creating %d ldap peers", len(emails))

	for i := range emails {
		if err := s.CreatePeerByEmail(currentSession.DeviceName, emails[i], formData.Identifier, false); err != nil {
			_ = s.updateFormInSession(c, formData)
			SetFlashMessage(c, "failed to add user: "+err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/peer/createldap?formerr=create")
			return
		}
	}

	SetFlashMessage(c, "client(s) created successfully", "success")
	c.Redirect(http.StatusSeeOther, "/admin/peer/createldap")
}

func (s *Server) GetAdminDeletePeer(c *gin.Context) {
	currentPeer := s.peers.GetPeerByKey(c.Query("pkey"))
	if err := s.DeletePeer(currentPeer); err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Deletion error", err.Error())
		return
	}
	SetFlashMessage(c, "peer deleted successfully", "success")
	c.Redirect(http.StatusSeeOther, "/admin")
}

func (s *Server) GetPeerQRCode(c *gin.Context) {
	peer := s.peers.GetPeerByKey(c.Query("pkey"))
	currentSession := GetSessionData(c)
	if !currentSession.IsAdmin && peer.Email != currentSession.Email {
		s.GetHandleError(c, http.StatusUnauthorized, "No permissions", "You don't have permissions to view this resource!")
		return
	}

	png, err := peer.GetQRCode()
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "QRCode error", err.Error())
		return
	}
	c.Data(http.StatusOK, "image/png", png)
	return
}

func (s *Server) GetPeerConfig(c *gin.Context) {
	peer := s.peers.GetPeerByKey(c.Query("pkey"))
	currentSession := GetSessionData(c)
	if !currentSession.IsAdmin && peer.Email != currentSession.Email {
		s.GetHandleError(c, http.StatusUnauthorized, "No permissions", "You don't have permissions to view this resource!")
		return
	}

	cfg, err := peer.GetConfigFile(s.peers.GetDevice(currentSession.DeviceName))
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "ConfigFile error", err.Error())
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+peer.GetConfigFileName())
	c.Data(http.StatusOK, "application/config", cfg)
	return
}

func (s *Server) GetPeerConfigMail(c *gin.Context) {
	peer := s.peers.GetPeerByKey(c.Query("pkey"))
	currentSession := GetSessionData(c)
	if !currentSession.IsAdmin && peer.Email != currentSession.Email {
		s.GetHandleError(c, http.StatusUnauthorized, "No permissions", "You don't have permissions to view this resource!")
		return
	}

	user := s.users.GetUser(peer.Email)

	cfg, err := peer.GetConfigFile(s.peers.GetDevice(currentSession.DeviceName))
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "ConfigFile error", err.Error())
		return
	}
	png, err := peer.GetQRCode()
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "QRCode error", err.Error())
		return
	}
	// Apply mail template
	var tplBuff bytes.Buffer
	if err := s.mailTpl.Execute(&tplBuff, struct {
		Peer          wireguard.Peer
		User          *users.User
		QrcodePngName string
		PortalUrl     string
	}{
		Peer:          peer,
		User:          user,
		QrcodePngName: "wireguard-config.png",
		PortalUrl:     s.config.Core.ExternalUrl,
	}); err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Template error", err.Error())
		return
	}

	// Send mail
	attachments := []common.MailAttachment{
		{
			Name:        peer.GetConfigFileName(),
			ContentType: "application/config",
			Data:        bytes.NewReader(cfg),
		},
		{
			Name:        "wireguard-config.png",
			ContentType: "image/png",
			Data:        bytes.NewReader(png),
		},
	}

	if err := common.SendEmailWithAttachments(s.config.Email, s.config.Core.MailFrom, "", "WireGuard VPN Configuration",
		"Your mail client does not support HTML. Please find the configuration attached to this mail.", tplBuff.String(),
		[]string{peer.Email}, attachments); err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Email error", err.Error())
		return
	}

	SetFlashMessage(c, "mail sent successfully", "success")
	if strings.HasPrefix(c.Request.URL.Path, "/user") {
		c.Redirect(http.StatusSeeOther, "/user/profile")
	} else {
		c.Redirect(http.StatusSeeOther, "/admin")
	}
}

func (s *Server) GetPeerStatus(c *gin.Context) {
	peer := s.peers.GetPeerByKey(c.Query("pkey"))
	currentSession := GetSessionData(c)
	if !currentSession.IsAdmin && peer.Email != currentSession.Email {
		s.GetHandleError(c, http.StatusUnauthorized, "No permissions", "You don't have permissions to view this resource!")
		return
	}

	if peer.Peer == nil { // no peer means disabled
		c.JSON(http.StatusOK, false)
		return
	}

	isOnline := false
	ping := make(chan bool)
	defer close(ping)
	for _, cidr := range peer.GetIPAddresses() {
		ip, _, _ := net.ParseCIDR(cidr)
		var ra *net.IPAddr
		if common.IsIPv6(ip.String()) {
			ra, _ = net.ResolveIPAddr("ip6:ipv6-icmp", ip.String())
		} else {

			ra, _ = net.ResolveIPAddr("ip4:icmp", ip.String())
		}

		p := fastping.NewPinger()
		p.AddIPAddr(ra)
		p.OnRecv = func(addr *net.IPAddr, rtt time.Duration) {
			ping <- true
			p.Stop()
		}
		p.OnIdle = func() {
			ping <- false
			p.Stop()
		}
		p.MaxRTT = 500 * time.Millisecond
		p.RunLoop()

		if <-ping {
			isOnline = true
			break
		}
	}

	c.JSON(http.StatusOK, isOnline)
	return
}
