package server

import (
	"net/http"
	"strconv"

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
	dev, err := s.wg.GetDeviceInfo()
	if err != nil {
		s.HandleError(c, http.StatusInternalServerError, "WireGuard error", err.Error())
		return
	}
	peers, err := s.wg.GetPeerList()
	if err != nil {
		s.HandleError(c, http.StatusInternalServerError, "WireGuard error", err.Error())
		return
	}

	device := s.users.GetDevice()
	device.Interface = dev

	users := make([]User, len(peers))
	for i, peer := range peers {
		users[i] = s.users.GetOrCreateUserForPeer(peer)
	}
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

func (s *Server) GetUserQRCode(c *gin.Context) {
	user := s.users.GetUser(c.Param("pkey"))
	png, err := user.GetQRCode()
	if err != nil {
		s.HandleError(c, http.StatusInternalServerError, "QRCode error", err.Error())
		return
	}
	c.Data(http.StatusOK, "image/png", png)
	return
}
