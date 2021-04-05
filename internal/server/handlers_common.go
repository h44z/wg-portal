package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/common"
	"github.com/h44z/wg-portal/internal/users"
	"github.com/pkg/errors"
)

func (s *Server) GetHandleError(c *gin.Context, code int, message, details string) {
	currentSession := GetSessionData(c)

	c.HTML(code, "error.html", gin.H{
		"Data": gin.H{
			"Code":    strconv.Itoa(code),
			"Message": message,
			"Details": details,
		},
		"Route":       c.Request.URL.Path,
		"Session":     GetSessionData(c),
		"Static":      s.getStaticData(),
		"Device":      s.peers.GetDevice(currentSession.DeviceName),
		"DeviceNames": s.GetDeviceNames(),
	})
}

func (s *Server) GetIndex(c *gin.Context) {
	currentSession := GetSessionData(c)

	c.HTML(http.StatusOK, "index.html", gin.H{
		"Route":       c.Request.URL.Path,
		"Alerts":      GetFlashes(c),
		"Session":     currentSession,
		"Static":      s.getStaticData(),
		"Device":      s.peers.GetDevice(currentSession.DeviceName),
		"DeviceNames": s.GetDeviceNames(),
	})
}

func (s *Server) GetAdminIndex(c *gin.Context) {
	currentSession := GetSessionData(c)

	sort := c.Query("sort")
	if sort != "" {
		if currentSession.SortedBy["peers"] != sort {
			currentSession.SortedBy["peers"] = sort
			currentSession.SortDirection["peers"] = "asc"
		} else {
			if currentSession.SortDirection["peers"] == "asc" {
				currentSession.SortDirection["peers"] = "desc"
			} else {
				currentSession.SortDirection["peers"] = "asc"
			}
		}

		if err := UpdateSessionData(c, currentSession); err != nil {
			s.GetHandleError(c, http.StatusInternalServerError, "sort error", "failed to save session")
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin/")
		return
	}

	search, searching := c.GetQuery("search")
	if searching {
		currentSession.Search["peers"] = search

		if err := UpdateSessionData(c, currentSession); err != nil {
			s.GetHandleError(c, http.StatusInternalServerError, "search error", "failed to save session")
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin/")
		return
	}

	deviceName := c.Query("device")
	if deviceName != "" {
		if !common.ListContains(s.wg.Cfg.DeviceNames, deviceName) {
			s.GetHandleError(c, http.StatusInternalServerError, "device selection error", "no such device")
			return
		}
		currentSession.DeviceName = deviceName

		if err := UpdateSessionData(c, currentSession); err != nil {
			s.GetHandleError(c, http.StatusInternalServerError, "device selection error", "failed to save session")
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin/")
		return
	}

	device := s.peers.GetDevice(currentSession.DeviceName)
	users := s.peers.GetFilteredAndSortedPeers(currentSession.DeviceName, currentSession.SortedBy["peers"], currentSession.SortDirection["peers"], currentSession.Search["peers"])

	c.HTML(http.StatusOK, "admin_index.html", gin.H{
		"Route":       c.Request.URL.Path,
		"Alerts":      GetFlashes(c),
		"Session":     currentSession,
		"Static":      s.getStaticData(),
		"Peers":       users,
		"TotalPeers":  len(s.peers.GetAllPeers(currentSession.DeviceName)),
		"Users":       s.users.GetUsers(),
		"Device":      device,
		"DeviceNames": s.GetDeviceNames(),
	})
}

func (s *Server) GetUserIndex(c *gin.Context) {
	currentSession := GetSessionData(c)

	sort := c.Query("sort")
	if sort != "" {
		if currentSession.SortedBy["userpeers"] != sort {
			currentSession.SortedBy["userpeers"] = sort
			currentSession.SortDirection["userpeers"] = "asc"
		} else {
			if currentSession.SortDirection["userpeers"] == "asc" {
				currentSession.SortDirection["userpeers"] = "desc"
			} else {
				currentSession.SortDirection["userpeers"] = "asc"
			}
		}

		if err := UpdateSessionData(c, currentSession); err != nil {
			s.GetHandleError(c, http.StatusInternalServerError, "sort error", "failed to save session")
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin")
		return
	}

	peers := s.peers.GetSortedPeersForEmail(currentSession.SortedBy["userpeers"], currentSession.SortDirection["userpeers"], currentSession.Email)

	c.HTML(http.StatusOK, "user_index.html", gin.H{
		"Route":       c.Request.URL.Path,
		"Alerts":      GetFlashes(c),
		"Session":     currentSession,
		"Static":      s.getStaticData(),
		"Peers":       peers,
		"TotalPeers":  len(peers),
		"Users":       []users.User{*s.users.GetUser(currentSession.Email)},
		"Device":      s.peers.GetDevice(currentSession.DeviceName),
		"DeviceNames": s.GetDeviceNames(),
	})
}

func (s *Server) updateFormInSession(c *gin.Context, formData interface{}) error {
	currentSession := GetSessionData(c)
	currentSession.FormData = formData

	if err := UpdateSessionData(c, currentSession); err != nil {
		return errors.WithMessage(err, "failed to update form in session")
	}

	return nil
}

func (s *Server) setNewPeerFormInSession(c *gin.Context) (SessionData, error) {
	currentSession := GetSessionData(c)

	// If session does not contain a peer form ignore update
	// If url contains a formerr parameter reset the form
	if currentSession.FormData == nil || c.Query("formerr") == "" {
		user, err := s.PrepareNewPeer(currentSession.DeviceName)
		if err != nil {
			return currentSession, errors.WithMessage(err, "failed to prepare new peer")
		}
		currentSession.FormData = user
	}

	if err := UpdateSessionData(c, currentSession); err != nil {
		return currentSession, errors.WithMessage(err, "failed to update peer form in session")
	}

	return currentSession, nil
}

func (s *Server) setFormInSession(c *gin.Context, formData interface{}) (SessionData, error) {
	currentSession := GetSessionData(c)
	// If session does not contain a form ignore update
	// If url contains a formerr parameter reset the form
	if currentSession.FormData == nil || c.Query("formerr") == "" {
		currentSession.FormData = formData
	}

	if err := UpdateSessionData(c, currentSession); err != nil {
		return currentSession, errors.WithMessage(err, "failed to set form in session")
	}

	return currentSession, nil
}
