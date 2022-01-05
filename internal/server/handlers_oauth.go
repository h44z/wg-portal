package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/authentication"
	"github.com/h44z/wg-portal/internal/oauth"
	"github.com/h44z/wg-portal/internal/oauth/oauthproviders"
	"github.com/h44z/wg-portal/internal/oauth/userprofile"
	"github.com/h44z/wg-portal/internal/users"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func getIDFromLoginURL(loginUrl string) (string, string) {
	values := strings.SplitN(loginUrl, "/", 3)

	if values[1] == "oidc" {
		return values[1], fmt.Sprintf("/%s", values[2])
	}

	oauthValues := strings.Split(values[2], "/")

	return values[1], oauthValues[0]
}

func (s *Server) providerFromLoginURL(c *gin.Context, loginUrl string, redirectURL string) (provider oauthproviders.Provider, err error) {
	providerType, providerID := getIDFromLoginURL(loginUrl)

	switch providerType {
	case "oauth":
		provider, err = s.config.OAUTH.NewProviderFromID(oauthproviders.ProviderType(providerID), redirectURL)
	case "oidc":
		provider, err = s.config.OIDC.NewProviderFromID(c.Request.Context(), providerID, redirectURL)
	default:
		err = fmt.Errorf("unknown authentication provider type: %s", providerType)
	}

	return
}

func (s *Server) OAuthLogin(c *gin.Context) {
	redirectURL := fmt.Sprintf("%s/oauth%s", strings.TrimSuffix(s.config.Core.ExternalUrl, "/"), s.config.OAUTH.RedirectURL)

	provider, err := s.providerFromLoginURL(c, c.Request.URL.Path, redirectURL)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=authfail")
		logrus.Errorf("oidc callback login failed for URL %s: %v", c.Request.RequestURI, err)

		return
	}

	// create a new state:
	// store the request remote IP address to validate the state in the callback
	// store the used loginURL to create the right authentication provider later in the callback
	state, err := oauth.GetStateManager(s.ctx).NewState(c.Request.RemoteAddr, c.Request.URL.Path)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=authfail")
	}

	oauth2URL := provider.AuthCodeURL(state)
	c.Redirect(http.StatusSeeOther, oauth2URL)
}

func (s *Server) OAuthCallback(c *gin.Context) {
	redirectURL := fmt.Sprintf("%s/oauth%s", strings.TrimSuffix(s.config.Core.ExternalUrl, "/"), s.config.OAUTH.RedirectURL)
	stateString := c.Request.FormValue("state")
	code := c.Request.FormValue("code")

	// be sure the state is deleted at the end of the callback
	defer oauth.GetStateManager(s.ctx).DeleteState(stateString)

	state, err := oauth.GetStateManager(s.ctx).GetState(stateString)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=authfail")
		logrus.Errorf("oauth callback failed for state %s: %v", stateString, err)

		return
	}

	// check if the returned state is the same we sent before
	if !state.IsValid(c.Request.RemoteAddr) {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=authfail")
		logrus.Errorf("oauth callback failed for state %s: invalid or expired state", stateString)

		return
	}

	// get the right authentication provider, using the loginURL stored in the state
	provider, err := s.providerFromLoginURL(c, state.LoginURL(), redirectURL)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=authfail")
		logrus.Errorf("oauth callback failed for state %s: %v", stateString, err)

		return
	}

	// get the token
	t, err := provider.Exchange(c.Request.Context(), code)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=authfail")
		logrus.Errorf("oauth callback failed: cannot get the token for state %s: %v", stateString, err)

		return
	}

	userInfo, err := provider.UserInfo(c.Request.Context(), provider.TokenSource(s.ctx, t))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=authfail")
		logrus.Errorf("oauth callback failed: cannot get the user info for state %s: %v", stateString, err)

		return
	}

	user, err := s.checkOAuthUser(userInfo, provider.CanCreateUsers())
	if err != nil {
		s.GetHandleError(c, http.StatusInternalServerError, "login error", err.Error())
		return
	}

	// Check if user is authenticated
	if user == nil {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=authfail")
		logrus.Errorf("oauth callback failed for state %s: user not found or disabled", stateString)

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

func (s *Server) checkOAuthUser(profile userprofile.Profile, canCreateUsers bool) (*users.User, error) {
	var user *users.User

	// Check all available auth backends
	for _, provider := range s.auth.GetProvidersForType(authentication.AuthProviderTypeOauth) {
		// try to log in to the given provider
		authEmail, err := provider.Login(&authentication.AuthContext{
			Username: profile.Email,
			Password: "",
		})
		if err != nil {
			continue
		}

		// User doesn't exist, but automatic creation is enabled
		if authEmail == "" && canCreateUsers {
			if err := s.CreateUser(users.User{
				Email:     profile.Email,
				Source:    users.UserSource(provider.GetName()),
				IsAdmin:   false,
				Firstname: profile.FirstName,
				Lastname:  profile.LastName,
			}, s.wg.Cfg.GetDefaultDeviceName()); err != nil {
				return nil, errors.Wrap(err, "failed to update user data")
			}
		}

		user = s.users.GetUser(authEmail)
		if user != nil {
			break // user exists, nothing more to do...
		}
	}

	return user, nil
}
