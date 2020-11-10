package server

import (
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/common"
	"github.com/h44z/wg-portal/internal/ldap"
	log "github.com/sirupsen/logrus"
)

type LdapCreateForm struct {
	Emails     string `form:"email" binding:"required"`
	Identifier string `form:"identifier" binding:"required,lte=20"`
}

func (s *Server) GetAdminEditPeer(c *gin.Context) {
	device := s.users.GetDevice()
	user := s.users.GetUserByKey(c.Query("pkey"))

	currentSession, err := s.setFormInSession(c, user)
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Session error", err.Error())
		return
	}

	c.HTML(http.StatusOK, "admin_edit_client.html", struct {
		Route   string
		Alerts  AlertData
		Session SessionData
		Static  StaticData
		Peer    User
		Device  Device
	}{
		Route:   c.Request.URL.Path,
		Alerts:  s.getAlertData(c),
		Session: currentSession,
		Static:  s.getStaticData(),
		Peer:    currentSession.FormData.(User),
		Device:  device,
	})
}

func (s *Server) PostAdminEditPeer(c *gin.Context) {
	currentUser := s.users.GetUserByKey(c.Query("pkey"))
	urlEncodedKey := url.QueryEscape(c.Query("pkey"))

	currentSession := s.getSessionData(c)
	var formUser User
	if currentSession.FormData != nil {
		formUser = currentSession.FormData.(User)
	}
	if err := c.ShouldBind(&formUser); err != nil {
		_ = s.updateFormInSession(c, formUser)
		s.setAlert(c, "failed to bind form data: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey+"&formerr=bind")
		return
	}

	// Clean list input
	formUser.IPs = common.ParseStringList(formUser.IPsStr)
	formUser.AllowedIPs = common.ParseStringList(formUser.AllowedIPsStr)
	formUser.IPsStr = common.ListToString(formUser.IPs)
	formUser.AllowedIPsStr = common.ListToString(formUser.AllowedIPs)

	disabled := c.PostForm("isdisabled") != ""
	now := time.Now()
	if disabled && currentUser.DeactivatedAt == nil {
		formUser.DeactivatedAt = &now
	} else if !disabled {
		formUser.DeactivatedAt = nil
	}

	// Update in database
	if err := s.UpdateUser(formUser, now); err != nil {
		_ = s.updateFormInSession(c, formUser)
		s.setAlert(c, "failed to update user: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey+"&formerr=update")
		return
	}

	s.setAlert(c, "changes applied successfully", "success")
	c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey)
}

func (s *Server) GetAdminCreatePeer(c *gin.Context) {
	device := s.users.GetDevice()

	currentSession, err := s.setNewUserFormInSession(c)
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Session error", err.Error())
		return
	}
	c.HTML(http.StatusOK, "admin_edit_client.html", struct {
		Route   string
		Alerts  AlertData
		Session SessionData
		Static  StaticData
		Peer    User
		Device  Device
	}{
		Route:   c.Request.URL.Path,
		Alerts:  s.getAlertData(c),
		Session: currentSession,
		Static:  s.getStaticData(),
		Peer:    currentSession.FormData.(User),
		Device:  device,
	})
}

func (s *Server) PostAdminCreatePeer(c *gin.Context) {
	currentSession := s.getSessionData(c)
	var formUser User
	if currentSession.FormData != nil {
		formUser = currentSession.FormData.(User)
	}
	if err := c.ShouldBind(&formUser); err != nil {
		_ = s.updateFormInSession(c, formUser)
		s.setAlert(c, "failed to bind form data: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/create?formerr=bind")
		return
	}

	// Clean list input
	formUser.IPs = common.ParseStringList(formUser.IPsStr)
	formUser.AllowedIPs = common.ParseStringList(formUser.AllowedIPsStr)
	formUser.IPsStr = common.ListToString(formUser.IPs)
	formUser.AllowedIPsStr = common.ListToString(formUser.AllowedIPs)

	disabled := c.PostForm("isdisabled") != ""
	now := time.Now()
	if disabled {
		formUser.DeactivatedAt = &now
	}

	if err := s.CreateUser(formUser); err != nil {
		_ = s.updateFormInSession(c, formUser)
		s.setAlert(c, "failed to add user: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/create?formerr=create")
		return
	}

	s.setAlert(c, "client created successfully", "success")
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
		Alerts   AlertData
		Session  SessionData
		Static   StaticData
		Users    []*ldap.UserCacheHolderEntry
		FormData LdapCreateForm
		Device   Device
	}{
		Route:    c.Request.URL.Path,
		Alerts:   s.getAlertData(c),
		Session:  currentSession,
		Static:   s.getStaticData(),
		Users:    s.ldapUsers.GetSortedUsers("sn", "asc"),
		FormData: currentSession.FormData.(LdapCreateForm),
		Device:   s.users.GetDevice(),
	})
}

func (s *Server) PostAdminCreateLdapPeers(c *gin.Context) {
	currentSession := s.getSessionData(c)
	var formData LdapCreateForm
	if currentSession.FormData != nil {
		formData = currentSession.FormData.(LdapCreateForm)
	}
	if err := c.ShouldBind(&formData); err != nil {
		_ = s.updateFormInSession(c, formData)
		s.setAlert(c, "failed to bind form data: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/createldap?formerr=bind")
		return
	}

	emails := common.ParseStringList(formData.Emails)
	for i := range emails {
		// TODO: also check email addr for validity?
		if !strings.ContainsRune(emails[i], '@') || s.ldapUsers.GetUserDNByMail(emails[i]) == "" {
			_ = s.updateFormInSession(c, formData)
			s.setAlert(c, "invalid email address: "+emails[i], "danger")
			c.Redirect(http.StatusSeeOther, "/admin/peer/createldap?formerr=mail")
			return
		}
	}

	log.Infof("creating %d ldap peers", len(emails))

	for i := range emails {
		if err := s.CreateUserByEmail(emails[i], formData.Identifier, false); err != nil {
			_ = s.updateFormInSession(c, formData)
			s.setAlert(c, "failed to add user: "+err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/peer/createldap?formerr=create")
			return
		}
	}

	s.setAlert(c, "client(s) created successfully", "success")
	c.Redirect(http.StatusSeeOther, "/admin/peer/createldap")
}

func (s *Server) GetAdminDeletePeer(c *gin.Context) {
	currentUser := s.users.GetUserByKey(c.Query("pkey"))
	if err := s.DeleteUser(currentUser); err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "Deletion error", err.Error())
		return
	}
	s.setAlert(c, "user deleted successfully", "success")
	c.Redirect(http.StatusSeeOther, "/admin")
}

func (s *Server) GetPeerQRCode(c *gin.Context) {
	user := s.users.GetUserByKey(c.Query("pkey"))
	currentSession := s.getSessionData(c)
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
	user := s.users.GetUserByKey(c.Query("pkey"))
	currentSession := s.getSessionData(c)
	if !currentSession.IsAdmin && user.Email != currentSession.Email {
		s.GetHandleError(c, http.StatusUnauthorized, "No permissions", "You don't have permissions to view this resource!")
		return
	}

	cfg, err := user.GetClientConfigFile(s.users.GetDevice())
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "ConfigFile error", err.Error())
		return
	}

	c.Header("Content-Disposition", "attachment; filename="+user.GetConfigFileName())
	c.Data(http.StatusOK, "application/config", cfg)
	return
}

func (s *Server) GetPeerConfigMail(c *gin.Context) {
	user := s.users.GetUserByKey(c.Query("pkey"))
	currentSession := s.getSessionData(c)
	if !currentSession.IsAdmin && user.Email != currentSession.Email {
		s.GetHandleError(c, http.StatusUnauthorized, "No permissions", "You don't have permissions to view this resource!")
		return
	}

	cfg, err := user.GetClientConfigFile(s.users.GetDevice())
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
		Client        User
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

	s.setAlert(c, "mail sent successfully", "success")
	c.Redirect(http.StatusSeeOther, "/admin")
}
