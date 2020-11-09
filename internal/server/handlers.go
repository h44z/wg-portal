package server

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/h44z/wg-portal/internal/ldap"

	"github.com/h44z/wg-portal/internal/common"

	"github.com/gin-gonic/gin"
)

type LdapCreateForm struct {
	Emails     string `form:"email" binding:"required"`
	Identifier string `form:"identifier" binding:"required,lte=20"`
}

func (s *Server) GetIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", struct {
		Route   string
		Alerts  AlertData
		Session SessionData
		Static  StaticData
		Device  Device
	}{
		Route:   c.Request.URL.Path,
		Alerts:  s.getAlertData(c),
		Session: s.getSessionData(c),
		Static:  s.getStaticData(),
		Device:  s.users.GetDevice(),
	})
}

func (s *Server) HandleError(c *gin.Context, code int, message, details string) {
	// TODO: if json
	//c.JSON(code, gin.H{"error": message, "details": details})

	c.HTML(code, "error.html", gin.H{
		"data": gin.H{
			"Code":    strconv.Itoa(code),
			"Message": message,
			"Details": details,
		},
		"route":   c.Request.URL.Path,
		"session": s.getSessionData(c),
		"static":  s.getStaticData(),
	})
}

func (s *Server) GetAdminIndex(c *gin.Context) {
	device := s.users.GetDevice()
	users := s.users.GetAllUsers()

	c.HTML(http.StatusOK, "admin_index.html", struct {
		Route   string
		Session SessionData
		Static  StaticData
		Peers   []User
		Device  Device
	}{
		Route:   c.Request.URL.Path,
		Session: s.getSessionData(c),
		Static:  s.getStaticData(),
		Peers:   users,
		Device:  device,
	})
}

func (s *Server) GetAdminEditInterface(c *gin.Context) {
	device := s.users.GetDevice()
	users := s.users.GetAllUsers()

	currentSession, err := s.setFormInSession(c, device)
	if err != nil {
		s.HandleError(c, http.StatusInternalServerError, "Session error", err.Error())
		return
	}

	c.HTML(http.StatusOK, "admin_edit_interface.html", struct {
		Route   string
		Alerts  AlertData
		Session SessionData
		Static  StaticData
		Peers   []User
		Device  Device
	}{
		Route:   c.Request.URL.Path,
		Alerts:  s.getAlertData(c),
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
		s.setAlert(c, "failed to bind form data: "+err.Error(), "danger")
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
		s.setAlert(c, "failed to update device in WireGuard: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit?formerr=wg")
		return
	}

	// Update in database
	err = s.users.UpdateDevice(formDevice)
	if err != nil {
		_ = s.updateFormInSession(c, formDevice)
		s.setAlert(c, "failed to update device in database: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit?formerr=update")
		return
	}

	s.setAlert(c, "changes applied successfully", "success")
	c.Redirect(http.StatusSeeOther, "/admin/device/edit")
}

func (s *Server) GetAdminEditPeer(c *gin.Context) {
	device := s.users.GetDevice()
	user := s.users.GetUserByKey(c.Query("pkey"))

	currentSession, err := s.setFormInSession(c, user)
	if err != nil {
		s.HandleError(c, http.StatusInternalServerError, "Session error", err.Error())
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
		s.HandleError(c, http.StatusInternalServerError, "Session error", err.Error())
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
		s.HandleError(c, http.StatusInternalServerError, "Session error", err.Error())
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
		s.HandleError(c, http.StatusInternalServerError, "Deletion error", err.Error())
		return
	}
	s.setAlert(c, "user deleted successfully", "success")
	c.Redirect(http.StatusSeeOther, "/admin")
}

func (s *Server) GetUserQRCode(c *gin.Context) {
	user := s.users.GetUserByKey(c.Query("pkey"))
	png, err := user.GetQRCode()
	if err != nil {
		s.HandleError(c, http.StatusInternalServerError, "QRCode error", err.Error())
		return
	}
	c.Data(http.StatusOK, "image/png", png)
	return
}

func (s *Server) updateFormInSession(c *gin.Context, formData interface{}) error {
	currentSession := s.getSessionData(c)
	currentSession.FormData = formData

	if err := s.updateSessionData(c, currentSession); err != nil {
		return err
	}

	return nil
}

func (s *Server) setNewUserFormInSession(c *gin.Context) (SessionData, error) {
	currentSession := s.getSessionData(c)
	// If session does not contain a user form ignore update
	// If url contains a formerr parameter reset the form
	if currentSession.FormData == nil || c.Query("formerr") == "" {
		user, err := s.PrepareNewUser()
		if err != nil {
			return currentSession, err
		}
		currentSession.FormData = user
	}

	if err := s.updateSessionData(c, currentSession); err != nil {
		return currentSession, err
	}

	return currentSession, nil
}

func (s *Server) setFormInSession(c *gin.Context, formData interface{}) (SessionData, error) {
	currentSession := s.getSessionData(c)
	// If session does not contain a form ignore update
	// If url contains a formerr parameter reset the form
	if currentSession.FormData == nil || c.Query("formerr") == "" {
		currentSession.FormData = formData
	}

	if err := s.updateSessionData(c, currentSession); err != nil {
		return currentSession, err
	}

	return currentSession, nil
}
