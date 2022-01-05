package server

import (
	"net/http"
	"strings"

	"github.com/pkg/errors"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/authentication"
	"github.com/h44z/wg-portal/internal/users"
	"github.com/sirupsen/logrus"
	csrf "github.com/utrack/gin-csrf"
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
		"error":              authError != "",
		"message":            errMsg,
		"static":             s.getStaticData(),
		"Csrf":               csrf.GetToken(c),
		"socialEnabled":      s.config.OAUTH.IsEnabled() || s.config.OIDC.IsEnabled(),
		"oauthGithubEnabled": s.config.OAUTH.Github.Enabled,
		"oauthGoogleEnabled": s.config.OAUTH.Google.Enabled,
		"oauthGitlabEnabled": s.config.OAUTH.Gitlab.Enabled,
		"oidc":               s.config.OIDC.ToFrontendButtons(),
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

	// Check all available auth backends
	user, err := s.checkAuthentication(username, password)
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "login error", err.Error())
		return
	}

	// Check if user is authenticated
	if user == nil {
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

func (s *Server) checkAuthentication(username, password string) (*users.User, error) {
	var user *users.User

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

		// Login succeeded
		user = s.users.GetUser(authEmail)
		if user != nil {
			break // user exists, nothing more to do...
		}

		// create new user in the database (or reactivate him)
		userData, err := provider.GetUserModel(&authentication.AuthContext{
			Username: username,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to get user model")
		}
		if err := s.CreateUser(users.User{
			Email:     userData.Email,
			Source:    users.UserSource(provider.GetName()),
			IsAdmin:   userData.IsAdmin,
			Firstname: userData.Firstname,
			Lastname:  userData.Lastname,
			Phone:     userData.Phone,
		}, s.wg.Cfg.GetDefaultDeviceName()); err != nil {
			return nil, errors.Wrap(err, "failed to update user data")
		}

		user = s.users.GetUser(authEmail)
		break
	}

	return user, nil
}
