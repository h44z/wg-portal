package server

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/h44z/wg-portal/internal/ldap"

	"github.com/h44z/wg-portal/internal/common"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/gin-gonic/gin"
)

func (s *Server) GetIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"route":   c.Request.URL.Path,
		"session": s.getSessionData(c),
		"static":  s.getStaticData(),
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
		Session: s.getSessionData(c),
		Static:  s.getStaticData(),
		Peers:   users,
		Device:  device,
	})
}

func (s *Server) PostAdminEditInterface(c *gin.Context) {
	var formDevice Device
	if err := c.ShouldBind(&formDevice); err != nil {
		s.setAlert(c, "failed to bind form data: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit")
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
		s.setAlert(c, "failed to update device in WireGuard: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit")
		return
	}

	// Update in database
	err = s.users.UpdateDevice(formDevice)
	if err != nil {
		s.setAlert(c, "failed to update device in database: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit")
		return
	}

	s.setAlert(c, "changes applied successfully", "success")
	c.Redirect(http.StatusSeeOther, "/admin/device/edit")
}

func (s *Server) GetAdminEditPeer(c *gin.Context) {
	device := s.users.GetDevice()
	user := s.users.GetUserByKey(c.Query("pkey"))

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
		Session: s.getSessionData(c),
		Static:  s.getStaticData(),
		Peer:    user,
		Device:  device,
	})
}

func (s *Server) PostAdminEditPeer(c *gin.Context) {
	currentUser := s.users.GetUserByKey(c.Query("pkey"))
	urlEncodedKey := url.QueryEscape(c.Query("pkey"))

	var formUser User
	if err := c.ShouldBind(&formUser); err != nil {
		s.setAlert(c, "failed to bind form data: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey)
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

	// Update WireGuard device
	if formUser.DeactivatedAt == &now {
		err := s.wg.RemovePeer(formUser.PublicKey)
		if err != nil {
			s.setAlert(c, "failed to remove peer in WireGuard: "+err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey)
			return
		}
	} else if formUser.DeactivatedAt == nil && currentUser.Peer != nil {
		err := s.wg.UpdatePeer(formUser.GetPeerConfig())
		if err != nil {
			s.setAlert(c, "failed to update peer in WireGuard: "+err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey)
			return
		}
	} else if formUser.DeactivatedAt == nil && currentUser.Peer == nil {
		err := s.wg.AddPeer(formUser.GetPeerConfig())
		if err != nil {
			s.setAlert(c, "failed to add peer in WireGuard: "+err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey)
			return
		}
	}

	// Update in database
	err := s.users.UpdateUser(formUser)
	if err != nil {
		s.setAlert(c, "failed to update user in database: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey)
		return
	}

	s.setAlert(c, "changes applied successfully", "success")
	c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey)
}

func (s *Server) GetAdminCreatePeer(c *gin.Context) {
	device := s.users.GetDevice()
	user := User{}
	user.AllowedIPsStr = device.AllowedIPsStr
	user.IPsStr = "" // TODO: add a valid ip here
	psk, err := wgtypes.GenerateKey()
	if err != nil {
		s.HandleError(c, http.StatusInternalServerError, "Preshared key generation error", err.Error())
		return
	}
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		s.HandleError(c, http.StatusInternalServerError, "Private key generation error", err.Error())
		return
	}
	user.PresharedKey = psk.String()
	user.PrivateKey = key.String()
	user.PublicKey = key.PublicKey().String()
	user.UID = fmt.Sprintf("u%x", md5.Sum([]byte(user.PublicKey)))

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
		Session: s.getSessionData(c),
		Static:  s.getStaticData(),
		Peer:    user,
		Device:  device,
	})
}

func (s *Server) PostAdminCreatePeer(c *gin.Context) {
	var formUser User
	if err := c.ShouldBind(&formUser); err != nil {
		s.setAlert(c, "failed to bind form data: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/create")
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

	// Update WireGuard device
	if formUser.DeactivatedAt == nil {
		err := s.wg.AddPeer(formUser.GetPeerConfig())
		if err != nil {
			s.setAlert(c, "failed to add peer in WireGuard: "+err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/peer/create")
			return
		}
	}

	// Create in database
	err := s.users.CreateUser(formUser)
	if err != nil {
		s.setAlert(c, "failed to add user in database: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/create")
		return
	}

	s.setAlert(c, "client created successfully", "success")
	c.Redirect(http.StatusSeeOther, "/admin")
}

func (s *Server) GetAdminCreateLdapPeers(c *gin.Context) {
	device := s.users.GetDevice()

	c.HTML(http.StatusOK, "admin_create_clients.html", struct {
		Route   string
		Alerts  AlertData
		Session SessionData
		Static  StaticData
		Users   []*ldap.UserCacheHolderEntry
		Device  Device
	}{
		Route:   c.Request.URL.Path,
		Alerts:  s.getAlertData(c),
		Session: s.getSessionData(c),
		Static:  s.getStaticData(),
		Users:   s.ldapUsers.GetSortedUsers("sn", "asc"),
		Device:  device,
	})
}

func (s *Server) PostAdminCreateLdapPeers(c *gin.Context) {
	email := c.PostForm("email")
	identifier := c.PostForm("identifier")
	if identifier == "" {
		identifier = "Default"
	}
	if email == "" {
		s.setAlert(c, "missing email address", "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/createldap")
		return
	}
	emails := common.ParseStringList(email)
	for i := range emails {
		// TODO: also check email addr for validity?
		if !strings.ContainsRune(emails[i], '@') || s.ldapUsers.GetUserDNByMail(emails[i]) == "" {
			s.setAlert(c, "invalid email address: "+emails[i], "danger")
			c.Redirect(http.StatusSeeOther, "/admin/peer/createldap")
			return
		}
	}

	log.Infof("creating %d ldap peers", len(emails))
	device := s.users.GetDevice()

	for i := range emails {
		ldapUser := s.ldapUsers.GetUserData(s.ldapUsers.GetUserDNByMail(emails[i]))
		user := User{}
		user.AllowedIPsStr = device.AllowedIPsStr
		user.IPsStr = "" // TODO: add a valid ip here
		psk, err := wgtypes.GenerateKey()
		if err != nil {
			s.HandleError(c, http.StatusInternalServerError, "Preshared key generation error", err.Error())
			return
		}
		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			s.HandleError(c, http.StatusInternalServerError, "Private key generation error", err.Error())
			return
		}
		user.PresharedKey = psk.String()
		user.PrivateKey = key.String()
		user.PublicKey = key.PublicKey().String()
		user.UID = fmt.Sprintf("u%x", md5.Sum([]byte(user.PublicKey)))
		user.Email = emails[i]
		user.Identifier = fmt.Sprintf("%s %s (%s)", ldapUser.Firstname, ldapUser.Lastname, identifier)

		// Create wireguard interface
		err = s.wg.AddPeer(user.GetPeerConfig())
		if err != nil {
			s.setAlert(c, "failed to add peer in WireGuard: "+err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/peer/createldap")
			return
		}

		// Create in database
		err = s.users.CreateUser(user)
		if err != nil {
			s.setAlert(c, "failed to add user in database: "+err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/peer/createldap")
			return
		}
	}

	s.setAlert(c, "client(s) created successfully", "success")
	c.Redirect(http.StatusSeeOther, "/admin/peer/createldap")
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
