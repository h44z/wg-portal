package ui

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/h44z/wg-portal/internal/authentication"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/pkg/errors"
	csrf "github.com/utrack/gin-csrf"
)

func (h *handler) getStaticData() StaticData {
	return StaticData{
		WebsiteTitle: h.config.Core.Title,
		WebsiteLogo:  h.config.Core.LogoUrl,
		CompanyName:  h.config.Core.CompanyName,
		Year:         time.Now().Year(),
		Version:      internal.Version,
	}
}

func (h *handler) handleIndexGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentSession := h.session.GetData(c)

		interfaces, err := h.backend.GetInterfaces()
		if err != nil {
			h.HandleError(c, http.StatusInternalServerError, err, "failed to load available interfaces")
			return
		}

		c.HTML(http.StatusOK, "index.gohtml", gin.H{
			"Route":      c.Request.URL.Path,
			"Alerts":     h.session.GetFlashes(c),
			"Session":    currentSession,
			"Static":     h.getStaticData(),
			"Interface":  nil, // TODO: load interface specified in the session
			"Interfaces": interfaces,
		})
	}
}

func (h *handler) handleErrorGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentSession := h.session.GetData(c)

		var (
			err       = ""
			errorCode = http.StatusNotFound
			details   = ""
			path      = "/"
		)
		if currentSession.Error != nil {
			err = currentSession.Error.Message
			details = currentSession.Error.Details
			errorCode = currentSession.Error.Code
			path = currentSession.Error.Path
		}

		c.HTML(errorCode, "error.gohtml", gin.H{
			"Route":         c.Request.URL.Path,
			"Alerts":        h.session.GetFlashes(c),
			"Session":       currentSession,
			"Static":        h.getStaticData(),
			"ErrorCode":     errorCode,
			"Error":         err,
			"ErrorDetails":  details,
			"PreviousRoute": path,
		})
	}
}

type LoginProviderInfo struct {
	Name template.HTML
	Url  string
}

func (h *handler) handleLogoutGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentSession := h.session.GetData(c)

		if !currentSession.LoggedIn { // Not logged in
			c.Redirect(http.StatusSeeOther, "/")
			return
		}

		h.session.DestroyData(c)
		c.Redirect(http.StatusSeeOther, "/")
	}
}

func (h *handler) handleLoginGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentSession := h.session.GetData(c)
		if currentSession.LoggedIn {
			c.Redirect(http.StatusSeeOther, "/") // already logged in
		}

		authProviders := make([]LoginProviderInfo, 0, len(h.config.Auth.OAuth)+len(h.config.Auth.OpenIDConnect))
		for _, provider := range h.config.Auth.OpenIDConnect {
			providerId := strings.ToLower(provider.ProviderName)
			providerName := provider.DisplayName
			if providerName == "" {
				providerName = provider.ProviderName
			}
			authProviders = append(authProviders, LoginProviderInfo{
				Name: template.HTML(providerName),
				Url:  "/auth/login/" + providerId,
			})
		}
		for _, provider := range h.config.Auth.OAuth {
			providerId := strings.ToLower(provider.ProviderName)
			providerName := provider.DisplayName
			if providerName == "" {
				providerName = provider.ProviderName
			}
			authProviders = append(authProviders, LoginProviderInfo{
				Name: template.HTML(providerName),
				Url:  "/auth/login/" + providerId,
			})
		}

		c.HTML(http.StatusOK, "login.gohtml", gin.H{
			"Alerts":         h.session.GetFlashes(c),
			"Static":         h.getStaticData(),
			"Csrf":           csrf.GetToken(c),
			"LoginProviders": authProviders,
		})
	}
}

func (h *handler) handleLoginPost() gin.HandlerFunc {
	return func(c *gin.Context) {
		currentSession := h.session.GetData(c)
		if currentSession.LoggedIn {
			c.Redirect(http.StatusSeeOther, "/") // already logged in
		}

		username := strings.ToLower(c.PostForm("username"))
		password := c.PostForm("password")

		// Validate form input
		if strings.Trim(username, " ") == "" || strings.Trim(password, " ") == "" {
			h.redirectWithFlash(c, "/auth/login", FlashData{Message: "Please fill out all fields", Type: "danger"})
			return
		}

		user, err := h.passwordAuthentication(persistence.UserIdentifier(username), password)
		if err != nil {
			h.redirectWithFlash(c, "/auth/login", FlashData{Message: "Login failed", Type: "danger"})
			return
		}

		authSession := h.session.DefaultSessionData()
		authSession.LoggedIn = true
		authSession.UserIdentifier = user.Identifier
		authSession.IsAdmin = user.IsAdmin
		authSession.Firstname = user.Firstname
		authSession.Lastname = user.Lastname
		authSession.Email = user.Email

		interfaces, err := h.backend.GetInterfaces()
		if err != nil {
			h.HandleError(c, http.StatusInternalServerError, err, "failed to load available interfaces")
			return
		}
		if len(interfaces) != 0 {
			authSession.InterfaceIdentifier = interfaces[0].Identifier
		}

		h.session.SetData(c, authSession)

		nextUrl := "/"
		if currentSession.DeepLink != "" {
			nextUrl = currentSession.DeepLink
		}

		c.Redirect(http.StatusSeeOther, nextUrl)
	}
}

func (h *handler) handleLoginGetOauth() gin.HandlerFunc {
	return func(c *gin.Context) {

		providerId := c.Param("provider")
		if _, ok := h.oauthAuthenticators[providerId]; !ok {
			h.redirectWithFlash(c, "/auth/login", FlashData{Message: "Invalid login provider", Type: "danger"})
			return
		}

		currentSession := h.session.GetData(c)
		if currentSession.LoggedIn {
			c.Redirect(http.StatusSeeOther, "/") // already logged in
		}

		// Prepare authentication flow, set state cookies
		state, err := randString(16)
		if err != nil {
			h.redirectWithFlash(c, "/auth/login", FlashData{Message: err.Error(), Type: "danger"})
			return
		}
		currentSession.OauthState = state

		authenticator := h.oauthAuthenticators[providerId]

		var authCodeUrl string
		switch authenticator.GetType() {
		case authentication.AuthenticatorTypeOAuth:
			authCodeUrl = authenticator.AuthCodeURL(state)
		case authentication.AuthenticatorTypeOidc:
			nonce, err := randString(16)
			if err != nil {
				h.redirectWithFlash(c, "/auth/login", FlashData{Message: err.Error(), Type: "danger"})
				return
			}
			currentSession.OidcNonce = nonce

			authCodeUrl = authenticator.AuthCodeURL(state, oidc.Nonce(nonce))
		}

		h.session.SetData(c, currentSession)

		c.Redirect(http.StatusFound, authCodeUrl)
	}
}

func (h *handler) handleLoginGetOauthCallback() gin.HandlerFunc {
	return func(c *gin.Context) {
		providerId := c.Param("provider")
		if _, ok := h.oauthAuthenticators[providerId]; !ok {
			h.redirectWithFlash(c, "/auth/login", FlashData{Message: "Invalid login provider", Type: "danger"})
			return
		}

		currentSession := h.session.GetData(c)
		ctx := c.Request.Context()

		if state := c.Query("state"); state != currentSession.OauthState {
			h.redirectWithFlash(c, "/auth/login", FlashData{Message: "Invalid OAuth state", Type: "danger"})
			return
		}

		authenticator := h.oauthAuthenticators[providerId]
		oauthCode := c.Query("code")
		oauth2Token, err := authenticator.Exchange(ctx, oauthCode)
		if err != nil {
			h.redirectWithFlash(c, "/auth/login", FlashData{Message: err.Error(), Type: "danger"})
			return
		}

		rawUserInfo, err := authenticator.GetUserInfo(c.Request.Context(), oauth2Token, currentSession.OidcNonce)
		if err != nil {
			h.redirectWithFlash(c, "/auth/login", FlashData{Message: err.Error(), Type: "danger"})
			return
		}

		userInfo, err := authenticator.ParseUserInfo(rawUserInfo)
		if err != nil {
			h.redirectWithFlash(c, "/auth/login", FlashData{Message: err.Error(), Type: "danger"})
			return
		}

		sessionData, err := h.prepareUserSession(userInfo, providerId)
		if err != nil {
			h.redirectWithFlash(c, "/auth/login", FlashData{Message: err.Error(), Type: "danger"})
			return
		}

		interfaces, err := h.backend.GetInterfaces()
		if err != nil {
			h.HandleError(c, http.StatusInternalServerError, err, "failed to load available interfaces")
			return
		}
		if len(interfaces) != 0 {
			sessionData.InterfaceIdentifier = interfaces[0].Identifier
		}

		h.session.SetData(c, sessionData)

		nextUrl := "/"
		if currentSession.DeepLink != "" {
			nextUrl = currentSession.DeepLink
		}

		c.Redirect(http.StatusSeeOther, nextUrl)
	}
}

func (h *handler) passwordAuthentication(identifier persistence.UserIdentifier, password string) (*persistence.User, error) {
	user, err := h.backend.GetUser(identifier)
	userInDatabase := false
	if err == nil {
		userInDatabase = true
	} else {
		// search user in ldap if registration is enabled
		for _, authenticator := range h.ldapAuthenticators {
			if !authenticator.RegistrationEnabled() {
				continue
			}
			rawUserInfo, err := authenticator.GetUserInfo(context.Background(), identifier)
			if err != nil {
				continue
			}
			userInfo, err := authenticator.ParseUserInfo(rawUserInfo)
			if err != nil {
				continue
			}

			user = &persistence.User{
				Identifier: userInfo.Identifier,
				Email:      userInfo.Email,
				Source:     persistence.UserSourceLdap,
				IsAdmin:    userInfo.IsAdmin,
				Firstname:  userInfo.Firstname,
				Lastname:   userInfo.Lastname,
				Phone:      userInfo.Phone,
				Department: userInfo.Department,
				// TODO: also store pw for registered user?
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			break
		}
	}

	if user == nil {
		return nil, errors.New("user not found")
	}

	switch user.Source {
	case persistence.UserSourceDatabase:
		err = h.backend.PlaintextAuthentication(identifier, password)
	case persistence.UserSourceLdap:
		for _, authenticator := range h.ldapAuthenticators {
			err = authenticator.PlaintextAuthentication(identifier, password)
			if err == nil {
				break // auth succeeded
			}
		}
	default:
		err = errors.New("no authentication backend available")
	}

	if err != nil {
		return nil, errors.WithMessage(err, "failed to authenticate")
	}

	if !userInDatabase {
		if err := h.backend.CreateUser(user); err != nil {
			return nil, errors.WithMessage(err, "failed to create new ldap user")
		}
	}

	return user, nil
}

func (h *handler) getAuthenticatorConfig(id string) (interface{}, error) {
	for i := range h.config.Auth.OpenIDConnect {
		if h.config.Auth.OpenIDConnect[i].ProviderName == id {
			return h.config.Auth.OpenIDConnect[i], nil
		}
	}

	for i := range h.config.Auth.OAuth {
		if h.config.Auth.OAuth[i].ProviderName == id {
			return h.config.Auth.OAuth[i], nil
		}
	}

	return nil, errors.Errorf("no configuration for authenticator id %s", id)
}

func (h *handler) prepareUserSession(userInfo *authentication.AuthenticatorUserInfo, providerId string) (SessionData, error) {
	session := h.session.DefaultSessionData()
	authenticatorCfg, err := h.getAuthenticatorConfig(providerId)
	if err != nil {
		return session, errors.WithMessagef(err, "failed to find auth provider config for %s", providerId)
	}
	registrationEnabled := false
	switch cfg := authenticatorCfg.(type) {
	case authentication.OAuthProvider:
		registrationEnabled = cfg.RegistrationEnabled
	case authentication.OpenIDConnectProvider:
		registrationEnabled = cfg.RegistrationEnabled
	}

	// Search user in backend
	user, err := h.backend.GetUser(userInfo.Identifier)
	switch {
	case err != nil && registrationEnabled:
		user, err = h.registerOauthUser(userInfo)
		if err != nil {
			return session, errors.WithMessage(err, "failed to register user")
		}
	case err != nil:
		return session, errors.WithMessage(err, "registration disabled, cannot create missing user")
	}

	// Set session data for user
	session.LoggedIn = true
	session.UserIdentifier = user.Identifier
	session.IsAdmin = user.IsAdmin
	session.Firstname = user.Firstname
	session.Lastname = user.Lastname
	session.Email = user.Email

	return session, nil
}

func (h *handler) registerOauthUser(userInfo *authentication.AuthenticatorUserInfo) (*persistence.User, error) {
	user := &persistence.User{
		Identifier: userInfo.Identifier,
		Email:      userInfo.Email,
		Source:     persistence.UserSourceOauth,
		IsAdmin:    userInfo.IsAdmin,
		Firstname:  userInfo.Firstname,
		Lastname:   userInfo.Lastname,
		Phone:      userInfo.Phone,
		Department: userInfo.Department,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	err := h.backend.CreateUser(user)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create new user")
	}

	return user, nil
}

func randString(nByte int) (string, error) {
	b := make([]byte, nByte)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (h *handler) redirectWithFlash(c *gin.Context, url string, flash FlashData) {
	h.session.SetFlashes(c, flash)
	c.Redirect(http.StatusSeeOther, url)
}
