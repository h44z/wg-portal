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
	"github.com/sirupsen/logrus"
	"github.com/tatsushid/go-fastping"
)

type LdapCreateForm struct {
	Emails     string `form:"email" binding:"required"`
	Identifier string `form:"identifier" binding:"required,lte=20"`
}

func (s *Server) GetAdminEditPeer(c *gin.Context) {
	device := s.peers.GetDevice()
	peer := s.peers.GetPeerByKey(c.Query("pkey"))

	currentSession, err := s.setFormInSession(c, peer)
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Session error", err.Error())
		return
	}

	c.HTML(http.StatusOK, "admin_edit_client.html", struct {
		Route        string
		Alerts       []FlashData
		Session      SessionData
		Static       StaticData
		Peer         Peer
		Device       Device
		EditableKeys bool
	}{
		Route:        c.Request.URL.Path,
		Alerts:       GetFlashes(c),
		Session:      currentSession,
		Static:       s.getStaticData(),
		Peer:         currentSession.FormData.(Peer),
		Device:       device,
		EditableKeys: s.config.Core.EditableKeys,
	})
}

func (s *Server) PostAdminEditPeer(c *gin.Context) {
	currentPeer := s.peers.GetPeerByKey(c.Query("pkey"))
	urlEncodedKey := url.QueryEscape(c.Query("pkey"))

	currentSession := GetSessionData(c)
	var formPeer Peer
	if currentSession.FormData != nil {
		formPeer = currentSession.FormData.(Peer)
	}
	if err := c.ShouldBind(&formPeer); err != nil {
		_ = s.updateFormInSession(c, formPeer)
		SetFlashMessage(c, "failed to bind form data: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey+"&formerr=bind")
		return
	}

	// Clean list input
	formPeer.IPs = common.ParseStringList(formPeer.IPsStr)
	formPeer.AllowedIPs = common.ParseStringList(formPeer.AllowedIPsStr)
	formPeer.IPsStr = common.ListToString(formPeer.IPs)
	formPeer.AllowedIPsStr = common.ListToString(formPeer.AllowedIPs)

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
	device := s.peers.GetDevice()

	currentSession, err := s.setNewPeerFormInSession(c)
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Session error", err.Error())
		return
	}
	c.HTML(http.StatusOK, "admin_edit_client.html", struct {
		Route        string
		Alerts       []FlashData
		Session      SessionData
		Static       StaticData
		Peer         Peer
		Device       Device
		EditableKeys bool
	}{
		Route:        c.Request.URL.Path,
		Alerts:       GetFlashes(c),
		Session:      currentSession,
		Static:       s.getStaticData(),
		Peer:         currentSession.FormData.(Peer),
		Device:       device,
		EditableKeys: s.config.Core.EditableKeys,
	})
}

func (s *Server) PostAdminCreatePeer(c *gin.Context) {
	currentSession := GetSessionData(c)
	var formPeer Peer
	if currentSession.FormData != nil {
		formPeer = currentSession.FormData.(Peer)
	}
	if err := c.ShouldBind(&formPeer); err != nil {
		_ = s.updateFormInSession(c, formPeer)
		SetFlashMessage(c, "failed to bind form data: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/create?formerr=bind")
		return
	}

	// Clean list input
	formPeer.IPs = common.ParseStringList(formPeer.IPsStr)
	formPeer.AllowedIPs = common.ParseStringList(formPeer.AllowedIPsStr)
	formPeer.IPsStr = common.ListToString(formPeer.IPs)
	formPeer.AllowedIPsStr = common.ListToString(formPeer.AllowedIPs)

	disabled := c.PostForm("isdisabled") != ""
	now := time.Now()
	if disabled {
		formPeer.DeactivatedAt = &now
	}

	if err := s.CreatePeer(formPeer); err != nil {
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

	c.HTML(http.StatusOK, "admin_create_clients.html", struct {
		Route    string
		Alerts   []FlashData
		Session  SessionData
		Static   StaticData
		Users    []users.User
		FormData LdapCreateForm
		Device   Device
	}{
		Route:    c.Request.URL.Path,
		Alerts:   GetFlashes(c),
		Session:  currentSession,
		Static:   s.getStaticData(),
		Users:    s.users.GetFilteredAndSortedUsers("lastname", "asc", ""),
		FormData: currentSession.FormData.(LdapCreateForm),
		Device:   s.peers.GetDevice(),
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
		if !strings.ContainsRune(emails[i], '@') || s.users.GetUser(emails[i]) == nil {
			_ = s.updateFormInSession(c, formData)
			SetFlashMessage(c, "invalid email address: "+emails[i], "danger")
			c.Redirect(http.StatusSeeOther, "/admin/peer/createldap?formerr=mail")
			return
		}
	}

	logrus.Infof("creating %d ldap peers", len(emails))

	for i := range emails {
		if err := s.CreatePeerByEmail(emails[i], formData.Identifier, false); err != nil {
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
	currentUser := s.peers.GetPeerByKey(c.Query("pkey"))
	if err := s.DeletePeer(currentUser); err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Deletion error", err.Error())
		return
	}
	SetFlashMessage(c, "user deleted successfully", "success")
	c.Redirect(http.StatusSeeOther, "/admin")
}

func (s *Server) GetPeerQRCode(c *gin.Context) {
	user := s.peers.GetPeerByKey(c.Query("pkey"))
	currentSession := GetSessionData(c)
	if !currentSession.IsAdmin && user.Email != currentSession.Email {
		s.GetHandleError(c, http.StatusUnauthorized, "No permissions", "You don't have permissions to view this resource!")
		return
	}

	png, err := user.GetQRCode()
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "QRCode error", err.Error())
		return
	}
	c.Data(http.StatusOK, "image/png", png)
	return
}

func (s *Server) GetPeerConfig(c *gin.Context) {
	user := s.peers.GetPeerByKey(c.Query("pkey"))
	currentSession := GetSessionData(c)
	if !currentSession.IsAdmin && user.Email != currentSession.Email {
		s.GetHandleError(c, http.StatusUnauthorized, "No permissions", "You don't have permissions to view this resource!")
		return
	}

	cfg, err := user.GetConfigFile(s.peers.GetDevice())
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "ConfigFile error", err.Error())
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+user.GetConfigFileName())
	c.Data(http.StatusOK, "application/config", cfg)
	return
}

func (s *Server) GetPeerConfigMail(c *gin.Context) {
	user := s.peers.GetPeerByKey(c.Query("pkey"))
	currentSession := GetSessionData(c)
	if !currentSession.IsAdmin && user.Email != currentSession.Email {
		s.GetHandleError(c, http.StatusUnauthorized, "No permissions", "You don't have permissions to view this resource!")
		return
	}

	cfg, err := user.GetConfigFile(s.peers.GetDevice())
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "ConfigFile error", err.Error())
		return
	}
	png, err := user.GetQRCode()
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "QRCode error", err.Error())
		return
	}
	// Apply mail template
	var tplBuff bytes.Buffer
	if err := s.mailTpl.Execute(&tplBuff, struct {
		Client        Peer
		QrcodePngName string
		PortalUrl     string
	}{
		Client:        user,
		QrcodePngName: "wireguard-config.png",
		PortalUrl:     s.config.Core.ExternalUrl,
	}); err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Template error", err.Error())
		return
	}

	// Send mail
	attachments := []common.MailAttachment{
		{
			Name:        user.GetConfigFileName(),
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
		[]string{user.Email}, attachments); err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Email error", err.Error())
		return
	}

	SetFlashMessage(c, "mail sent successfully", "success")
	c.Redirect(http.StatusSeeOther, "/admin")
}

func (s *Server) GetPeerStatus(c *gin.Context) {
	user := s.peers.GetPeerByKey(c.Query("pkey"))
	currentSession := GetSessionData(c)
	if !currentSession.IsAdmin && user.Email != currentSession.Email {
		s.GetHandleError(c, http.StatusUnauthorized, "No permissions", "You don't have permissions to view this resource!")
		return
	}

	if user.Peer == nil { // no peer means disabled
		c.JSON(http.StatusOK, false)
		return
	}

	isOnline := false
	ping := make(chan bool)
	defer close(ping)
	for _, cidr := range user.IPs {
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
