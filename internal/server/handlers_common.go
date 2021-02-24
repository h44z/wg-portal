package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (s *Server) GetHandleError(c *gin.Context, code int, message, details string) {
	c.HTML(code, "error.html", gin.H{
		"Data": gin.H{
			"Code":    strconv.Itoa(code),
			"Message": message,
			"Details": details,
		},
		"Route":   c.Request.URL.Path,
		"Session": GetSessionData(c),
		"Static":  s.getStaticData(),
	})
}

func (s *Server) GetIndex(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", struct {
		Route   string
		Alerts  []FlashData
		Session SessionData
		Static  StaticData
		Device  Device
	}{
		Route:   c.Request.URL.Path,
		Alerts:  GetFlashes(c),
		Session: GetSessionData(c),
		Static:  s.getStaticData(),
		Device:  s.peers.GetDevice(),
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

	device := s.peers.GetDevice()
	users := s.peers.GetFilteredAndSortedPeers(currentSession.SortedBy["peers"], currentSession.SortDirection["peers"], currentSession.Search["peers"])

	c.HTML(http.StatusOK, "admin_index.html", struct {
		Route      string
		Alerts     []FlashData
		Session    SessionData
		Static     StaticData
		Peers      []Peer
		TotalPeers int
		Device     Device
	}{
		Route:      c.Request.URL.Path,
		Alerts:     GetFlashes(c),
		Session:    currentSession,
		Static:     s.getStaticData(),
		Peers:      users,
		TotalPeers: len(s.peers.GetAllPeers()),
		Device:     device,
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

	device := s.peers.GetDevice()
	users := s.peers.GetSortedPeersForEmail(currentSession.SortedBy["userpeers"], currentSession.SortDirection["userpeers"], currentSession.Email)

	c.HTML(http.StatusOK, "user_index.html", struct {
		Route      string
		Alerts     []FlashData
		Session    SessionData
		Static     StaticData
		Peers      []Peer
		TotalPeers int
		Device     Device
	}{
		Route:      c.Request.URL.Path,
		Alerts:     GetFlashes(c),
		Session:    currentSession,
		Static:     s.getStaticData(),
		Peers:      users,
		TotalPeers: len(users),
		Device:     device,
	})
}

func (s *Server) updateFormInSession(c *gin.Context, formData interface{}) error {
	currentSession := GetSessionData(c)
	currentSession.FormData = formData

	if err := UpdateSessionData(c, currentSession); err != nil {
		return err
	}

	return nil
}

func (s *Server) setNewPeerFormInSession(c *gin.Context) (SessionData, error) {
	currentSession := GetSessionData(c)
	// If session does not contain a peer form ignore update
	// If url contains a formerr parameter reset the form
	if currentSession.FormData == nil || c.Query("formerr") == "" {
		user, err := s.PrepareNewPeer()
		if err != nil {
			return currentSession, err
		}
		currentSession.FormData = user
	}

	if err := UpdateSessionData(c, currentSession); err != nil {
		return currentSession, err
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
		return currentSession, err
	}

	return currentSession, nil
}
