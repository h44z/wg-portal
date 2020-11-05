package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) GetLogin(c *gin.Context) {
	currentSession := s.getSessionData(c)
	if currentSession.LoggedIn {
		c.Redirect(http.StatusSeeOther, "/") // already logged in
	}

	authError := c.DefaultQuery("err", "")
	errMsg := "Unknown error occurred, try again!"
	switch authError {
	case "missingdata":
		errMsg = "Invalid login data retrieved, please fill out all fields and try again!"
	case "authfail":
		errMsg = "Authentication failed!"
	case "loginreq":
		errMsg = "Login required!"
	}

	c.HTML(http.StatusOK, "login.html", gin.H{
		"error":   authError != "",
		"message": errMsg,
		"static":  s.getStaticData(),
	})
}

func (s *Server) PostLogin(c *gin.Context) {
	currentSession := s.getSessionData(c)
	if currentSession.LoggedIn {
		// already logged in
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	username := strings.ToLower(c.PostForm("username"))
	password := c.PostForm("password")

	// Validate form input
	if strings.Trim(username, " ") == "" || strings.Trim(password, " ") == "" {
		c.Redirect(http.StatusSeeOther, s.config.AuthRoutePrefix+"/login?err=missingdata")
		return
	}

	// Check if user is in cache, avoid unnecessary ldap requests
	if !s.ldapUsers.UserExists(username) {
		c.Redirect(http.StatusSeeOther, s.config.AuthRoutePrefix+"/login?err=authfail")
	}

	// Check if username and password match
	if !s.ldapAuth.CheckLogin(username, password) {
		c.Redirect(http.StatusSeeOther, s.config.AuthRoutePrefix+"/login?err=authfail")
		return
	}

	dn := s.ldapUsers.GetUserDN(username)
	userData := s.ldapUsers.GetUserData(dn)
	sessionData := SessionData{
		LoggedIn:      true,
		IsAdmin:       s.ldapUsers.IsInGroup(username, s.config.AdminLdapGroup),
		UID:           userData.GetUID(),
		UserName:      username,
		Firstname:     userData.Firstname,
		Lastname:      userData.Lastname,
		SortedBy:      "sn",
		SortDirection: "asc",
		Search:        "",
	}

	if err := s.updateSessionData(c, sessionData); err != nil {
		s.HandleError(c, http.StatusInternalServerError, "login error", "failed to save session")
		return
	}
	c.Redirect(http.StatusSeeOther, "/")
}

func (s *Server) GetLogout(c *gin.Context) {
	currentSession := s.getSessionData(c)

	if !currentSession.LoggedIn { // Not logged in
		c.Redirect(http.StatusSeeOther, s.config.LogoutRedirectPath)
		return
	}

	if err := s.destroySessionData(c); err != nil {
		s.HandleError(c, http.StatusInternalServerError, "logout error", "failed to destroy session")
		return
	}
	c.Redirect(http.StatusSeeOther, s.config.LogoutRedirectPath)
}
