package server

import (
	"net/http"
	"strings"

	csrf "github.com/utrack/gin-csrf"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/authentication"
	"github.com/h44z/wg-portal/internal/users"
	"github.com/sirupsen/logrus"
)

func (s *Server) GetLogin(c *gin.Context) {
	currentSession := GetSessionData(c)
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
		"Csrf":    csrf.GetToken(c),
	})
}

func (s *Server) PostLogin(c *gin.Context) {
	currentSession := GetSessionData(c)
	if currentSession.LoggedIn {
		// already logged in
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	username := strings.ToLower(c.PostForm("username"))
	password := c.PostForm("password")

	// Validate form input
	if strings.Trim(username, " ") == "" || strings.Trim(password, " ") == "" {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=missingdata")
		return
	}

	// Check user database for an matching entry
	var loginProvider authentication.AuthProvider
	email := ""
	user := s.users.GetUser(username) // retrieve active candidate user from db
	if user != nil {                  // existing user
		loginProvider = s.auth.GetProvider(string(user.Source))
		if loginProvider == nil {
			s.GetHandleError(c, http.StatusInternalServerError, "login error", "login provider unavailable")
			return
		}
		authEmail, err := loginProvider.Login(&authentication.AuthContext{
			Username: username,
			Password: password,
		})
		if err == nil {
			email = authEmail
		}
	} else { // possible new user
		// Check all available auth backends
		for _, provider := range s.auth.GetProvidersForType(authentication.AuthProviderTypePassword) {
			// try to log in to the given provider
			authEmail, err := provider.Login(&authentication.AuthContext{
				Username: username,
				Password: password,
			})
			if err != nil {
				continue
			}

			email = authEmail
			loginProvider = provider

			// create new user in the database (or reactivate him)
			userData, err := loginProvider.GetUserModel(&authentication.AuthContext{
				Username: email,
			})
			if err != nil {
				s.GetHandleError(c, http.StatusInternalServerError, "login error", err.Error())
				return
			}
			if err := s.CreateUser(users.User{
				Email:     userData.Email,
				Source:    users.UserSource(loginProvider.GetName()),
				IsAdmin:   userData.IsAdmin,
				Firstname: userData.Firstname,
				Lastname:  userData.Lastname,
				Phone:     userData.Phone,
			}, s.wg.Cfg.GetDefaultDeviceName()); err != nil {
				s.GetHandleError(c, http.StatusInternalServerError, "login error", "failed to update user data")
				return
			}

			user = s.users.GetUser(username)
			break
		}
	}

	// Check if user is authenticated
	if email == "" || loginProvider == nil || user == nil {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=authfail")
		return
	}

	// Set authenticated session
	sessionData := GetSessionData(c)
	sessionData.LoggedIn = true
	sessionData.IsAdmin = user.IsAdmin
	sessionData.Email = user.Email
	sessionData.Firstname = user.Firstname
	sessionData.Lastname = user.Lastname
	sessionData.DeviceName = s.wg.Cfg.DeviceNames[0]

	// Check if user already has a peer setup, if not create one
	if err := s.CreateUserDefaultPeer(user.Email, s.wg.Cfg.GetDefaultDeviceName()); err != nil {
		// Not a fatal error, just log it...
		logrus.Errorf("failed to automatically create vpn peer for %s: %v", sessionData.Email, err)
	}

	if err := UpdateSessionData(c, sessionData); err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "login error", "failed to save session")
		return
	}
	c.Redirect(http.StatusSeeOther, "/")
}

func (s *Server) GetLogout(c *gin.Context) {
	currentSession := GetSessionData(c)

	if !currentSession.LoggedIn { // Not logged in
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	if err := DestroySessionData(c); err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "logout error", "failed to destroy session")
		return
	}
	c.Redirect(http.StatusSeeOther, "/")
}
