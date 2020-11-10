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
		"Session": s.getSessionData(c),
		"Static":  s.getStaticData(),
	})
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

func (s *Server) GetAdminIndex(c *gin.Context) {
	currentSession := s.getSessionData(c)

	sort := c.Query("sort")
	if sort != "" {
		if currentSession.SortedBy != sort {
			currentSession.SortedBy = sort
			currentSession.SortDirection = "asc"
		} else {
			if currentSession.SortDirection == "asc" {
				currentSession.SortDirection = "desc"
			} else {
				currentSession.SortDirection = "asc"
			}
		}

		if err := s.updateSessionData(c, currentSession); err != nil {
			s.GetHandleError(c, http.StatusInternalServerError, "sort error", "failed to save session")
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin")
		return
	}

	search, searching := c.GetQuery("search")
	if searching {
		currentSession.Search = search

		if err := s.updateSessionData(c, currentSession); err != nil {
			s.GetHandleError(c, http.StatusInternalServerError, "search error", "failed to save session")
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin")
		return
	}

	device := s.users.GetDevice()
	users := s.users.GetFilteredAndSortedUsers(currentSession.SortedBy, currentSession.SortDirection, currentSession.Search)

	c.HTML(http.StatusOK, "admin_index.html", struct {
		Route        string
		Alerts       AlertData
		Session      SessionData
		Static       StaticData
		Peers        []User
		TotalPeers   int
		Device       Device
		LdapDisabled bool
	}{
		Route:        c.Request.URL.Path,
		Alerts:       s.getAlertData(c),
		Session:      currentSession,
		Static:       s.getStaticData(),
		Peers:        users,
		TotalPeers:   len(s.users.GetAllUsers()),
		Device:       device,
		LdapDisabled: s.ldapDisabled,
	})
}

func (s *Server) GetUserIndex(c *gin.Context) {
	currentSession := s.getSessionData(c)

	sort := c.Query("sort")
	if sort != "" {
		if currentSession.SortedBy != sort {
			currentSession.SortedBy = sort
			currentSession.SortDirection = "asc"
		} else {
			if currentSession.SortDirection == "asc" {
				currentSession.SortDirection = "desc"
			} else {
				currentSession.SortDirection = "asc"
			}
		}

		if err := s.updateSessionData(c, currentSession); err != nil {
			s.GetHandleError(c, http.StatusInternalServerError, "sort error", "failed to save session")
			return
		}
		c.Redirect(http.StatusSeeOther, "/admin")
		return
	}

	device := s.users.GetDevice()
	users := s.users.GetSortedUsersForEmail(currentSession.SortedBy, currentSession.SortDirection, currentSession.Email)

	c.HTML(http.StatusOK, "user_index.html", struct {
		Route      string
		Alerts     AlertData
		Session    SessionData
		Static     StaticData
		Peers      []User
		TotalPeers int
		Device     Device
	}{
		Route:      c.Request.URL.Path,
		Alerts:     s.getAlertData(c),
		Session:    currentSession,
		Static:     s.getStaticData(),
		Peers:      users,
		TotalPeers: len(users),
		Device:     device,
	})
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
