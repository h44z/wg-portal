package server

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

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
	formDevice.IPs = common.ParseIPList(formDevice.IPsStr)
	formDevice.AllowedIPs = common.ParseIPList(formDevice.AllowedIPsStr)
	formDevice.DNS = common.ParseIPList(formDevice.DNSStr)
	formDevice.IPsStr = common.IPListToString(formDevice.IPs)
	formDevice.AllowedIPsStr = common.IPListToString(formDevice.AllowedIPs)
	formDevice.DNSStr = common.IPListToString(formDevice.DNS)

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
	formUser.IPs = common.ParseIPList(formUser.IPsStr)
	formUser.AllowedIPs = common.ParseIPList(formUser.AllowedIPsStr)
	formUser.IPsStr = common.IPListToString(formUser.IPs)
	formUser.AllowedIPsStr = common.IPListToString(formUser.AllowedIPs)

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
	formUser.IPs = common.ParseIPList(formUser.IPsStr)
	formUser.AllowedIPs = common.ParseIPList(formUser.AllowedIPsStr)
	formUser.IPsStr = common.IPListToString(formUser.IPs)
	formUser.AllowedIPsStr = common.IPListToString(formUser.AllowedIPs)

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

	// Update in database
	err := s.users.CreateUser(formUser)
	if err != nil {
		s.setAlert(c, "failed to add user in database: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/create")
		return
	}

	s.setAlert(c, "client created successfully", "success")
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
