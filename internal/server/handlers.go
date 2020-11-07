package server

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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
	device := s.users.GetDevice()
	var err error

	device.ListenPort, err = strconv.Atoi(c.PostForm("port"))
	if err != nil {
		s.setAlert(c, "invalid port: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit")
		return
	}

	ipField := c.PostForm("ip")
	ips := strings.Split(ipField, ",")
	validatedIPs := make([]string, 0, len(ips))
	for i := range ips {
		ips[i] = strings.TrimSpace(ips[i])
		if ips[i] != "" {
			validatedIPs = append(validatedIPs, ips[i])
		}
	}
	if len(validatedIPs) == 0 {
		s.setAlert(c, "invalid ip address", "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit")
		return
	}
	device.IPs = validatedIPs

	device.Endpoint = c.PostForm("endpoint")

	dnsField := c.PostForm("dns")
	dns := strings.Split(dnsField, ",")
	validatedDNS := make([]string, 0, len(dns))
	for i := range dns {
		dns[i] = strings.TrimSpace(dns[i])
		if dns[i] != "" {
			validatedDNS = append(validatedDNS, dns[i])
		}
	}
	device.DNS = validatedDNS

	allowedIPField := c.PostForm("allowedip")
	allowedIP := strings.Split(allowedIPField, ",")
	validatedAllowedIP := make([]string, 0, len(allowedIP))
	for i := range allowedIP {
		allowedIP[i] = strings.TrimSpace(allowedIP[i])
		if allowedIP[i] != "" {
			validatedAllowedIP = append(validatedAllowedIP, allowedIP[i])
		}
	}
	device.AllowedIPs = validatedAllowedIP

	device.Mtu, err = strconv.Atoi(c.PostForm("mtu"))
	if err != nil {
		s.setAlert(c, "invalid MTU: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit")
		return
	}

	device.PersistentKeepalive, err = strconv.Atoi(c.PostForm("keepalive"))
	if err != nil {
		s.setAlert(c, "invalid PersistentKeepalive: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit")
		return
	}

	// Update WireGuard device
	err = s.wg.UpdateDevice(device.DeviceName, device.GetDeviceConfig())
	if err != nil {
		s.setAlert(c, "failed to update device in WireGuard: "+err.Error(), "danger")
		c.Redirect(http.StatusSeeOther, "/admin/device/edit")
		return
	}

	// Update in database
	err = s.users.UpdateDevice(device)
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
	user := s.users.GetUserByKey(c.Query("pkey"))
	urlEncodedKey := url.QueryEscape(c.Query("pkey"))
	var err error

	user.Identifier = c.PostForm("identifier")
	if user.Identifier == "" {
		s.setAlert(c, "invalid identifier, must not be empty", "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey)
		return
	}

	user.Email = c.PostForm("mail")
	if user.Email == "" {
		s.setAlert(c, "invalid email, must not be empty", "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey)
		return
	}

	ipField := c.PostForm("ip")
	ips := strings.Split(ipField, ",")
	validatedIPs := make([]string, 0, len(ips))
	for i := range ips {
		ips[i] = strings.TrimSpace(ips[i])
		if ips[i] != "" {
			validatedIPs = append(validatedIPs, ips[i])
		}
	}
	if len(validatedIPs) == 0 {
		s.setAlert(c, "invalid ip address", "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey)
		return
	}
	user.IPs = validatedIPs

	allowedIPField := c.PostForm("allowedip")
	allowedIP := strings.Split(allowedIPField, ",")
	validatedAllowedIP := make([]string, 0, len(allowedIP))
	for i := range allowedIP {
		allowedIP[i] = strings.TrimSpace(allowedIP[i])
		if allowedIP[i] != "" {
			validatedAllowedIP = append(validatedAllowedIP, allowedIP[i])
		}
	}
	user.AllowedIPs = validatedAllowedIP

	user.IgnorePersistentKeepalive = c.PostForm("ignorekeepalive") != ""
	disabled := c.PostForm("isdisabled") != ""
	now := time.Now()
	if disabled && user.DeactivatedAt == nil {
		user.DeactivatedAt = &now
	} else if !disabled {
		user.DeactivatedAt = nil
	}

	// Update WireGuard device
	if user.DeactivatedAt == &now {
		err = s.wg.RemovePeer(user.PublicKey)
		if err != nil {
			s.setAlert(c, "failed to remove peer in WireGuard: "+err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey)
			return
		}
	} else if user.DeactivatedAt == nil && user.Peer != nil {
		err = s.wg.UpdatePeer(user.GetPeerConfig())
		if err != nil {
			s.setAlert(c, "failed to update peer in WireGuard: "+err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey)
			return
		}
	} else if user.DeactivatedAt == nil && user.Peer == nil {
		err = s.wg.AddPeer(user.GetPeerConfig())
		if err != nil {
			s.setAlert(c, "failed to add peer in WireGuard: "+err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/peer/edit?pkey="+urlEncodedKey)
			return
		}
	}

	// Update in database
	err = s.users.UpdateUser(user)
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
	user := User{}
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		s.HandleError(c, http.StatusInternalServerError, "Private key generation error", err.Error())
		return
	}
	user.PrivateKey = key.String()
	user.PublicKey = key.PublicKey().String()

	user.Identifier = c.PostForm("identifier")
	if user.Identifier == "" {
		s.setAlert(c, "invalid identifier, must not be empty", "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/create")
		return
	}

	user.Email = c.PostForm("mail")
	if user.Email == "" {
		s.setAlert(c, "invalid email, must not be empty", "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/create")
		return
	}

	ipField := c.PostForm("ip")
	ips := strings.Split(ipField, ",")
	validatedIPs := make([]string, 0, len(ips))
	for i := range ips {
		ips[i] = strings.TrimSpace(ips[i])
		if ips[i] != "" {
			validatedIPs = append(validatedIPs, ips[i])
		}
	}
	if len(validatedIPs) == 0 {
		s.setAlert(c, "invalid ip address", "danger")
		c.Redirect(http.StatusSeeOther, "/admin/peer/create")
		return
	}
	user.IPs = validatedIPs

	allowedIPField := c.PostForm("allowedip")
	allowedIP := strings.Split(allowedIPField, ",")
	validatedAllowedIP := make([]string, 0, len(allowedIP))
	for i := range allowedIP {
		allowedIP[i] = strings.TrimSpace(allowedIP[i])
		if allowedIP[i] != "" {
			validatedAllowedIP = append(validatedAllowedIP, allowedIP[i])
		}
	}
	user.AllowedIPs = validatedAllowedIP

	user.IgnorePersistentKeepalive = c.PostForm("ignorekeepalive") != ""
	disabled := c.PostForm("isdisabled") != ""
	now := time.Now()
	if disabled && user.DeactivatedAt == nil {
		user.DeactivatedAt = &now
	} else if !disabled {
		user.DeactivatedAt = nil
	}

	// Update WireGuard device
	if user.DeactivatedAt == nil {
		err = s.wg.AddPeer(user.GetPeerConfig())
		if err != nil {
			s.setAlert(c, "failed to add peer in WireGuard: "+err.Error(), "danger")
			c.Redirect(http.StatusSeeOther, "/admin/peer/create")
			return
		}
	}

	// Update in database
	err = s.users.CreateUser(user)
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
