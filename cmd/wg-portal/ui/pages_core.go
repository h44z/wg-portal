package ui

import (
	"crypto/rand"
	"encoding/base64"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/h44z/wg-portal/internal/persistence"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal"
	csrf "github.com/utrack/gin-csrf"
)

func (h *Handler) getStaticData() StaticData {
	return StaticData{
		WebsiteTitle: h.config.Core.Title,
		WebsiteLogo:  h.config.Core.LogoUrl,
		CompanyName:  h.config.Core.CompanyName,
		Year:         time.Now().Year(),
		Version:      internal.Version,
	}
}

func (h *Handler) GetIndex(c *gin.Context) {
	currentSession := GetSessionData(c)

	c.HTML(http.StatusOK, "index.html", gin.H{
		"Route":          c.Request.URL.Path,
		"Alerts":         GetFlashes(c),
		"Session":        currentSession,
		"Static":         h.getStaticData(),
		"Interface":      nil, // TODO: load interface specified in the session
		"InterfaceNames": map[string]string{"wgX": "wgX descr"},
	})
}

type LoginProviderInfo struct {
	Name template.HTML
	Url  string
}

func (h *Handler) GetLogin(c *gin.Context) {
	currentSession := GetSessionData(c)
	if currentSession.LoggedIn {
		c.Redirect(http.StatusSeeOther, "/") // already logged in
	}

	deepLink := c.DefaultQuery("dl", "")
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

	c.HTML(http.StatusOK, "login.html", gin.H{
		"HasError":       authError != "",
		"Message":        errMsg,
		"DeepLink":       deepLink,
		"Static":         h.getStaticData(),
		"Csrf":           csrf.GetToken(c),
		"LoginProviders": authProviders,
	})
}

func (h *Handler) PostLogin(c *gin.Context) {
	currentSession := GetSessionData(c)
	if currentSession.LoggedIn {
		c.Redirect(http.StatusSeeOther, "/") // already logged in
	}

	deepLink := c.DefaultQuery("dl", "")
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
		"HasError": authError != "",
		"Message":  errMsg,
		"DeepLink": deepLink,
		"Static":   h.getStaticData(),
		"Csrf":     csrf.GetToken(c),
	})
}

func (h *Handler) GetLoginOauth(c *gin.Context) {
	provider := c.Param("provider")
	if _, ok := h.authProviderNames[provider]; !ok {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=invalidprovider")
		return
	}

	currentSession := GetSessionData(c)
	if currentSession.LoggedIn {
		c.Redirect(http.StatusSeeOther, "/") // already logged in
	}

	// Prepare authentication flow, set state cookies
	state, err := randString(16)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=randsrcunavailable")
		return
	}
	currentSession.OauthState = state

	switch h.authProviderNames[provider] {
	case AuthProviderTypeOAuth:
		c.Redirect(http.StatusFound, h.oauthConfigs[provider].AuthCodeURL(state))
		return
	case AuthProviderTypeOpenIDConnect:
		nonce, err := randString(16)
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/auth/login?err=randsrcunavailable")
			return
		}
		currentSession.OidcNonce = nonce

		c.Redirect(http.StatusFound, h.oauthConfigs[provider].AuthCodeURL(state, oidc.Nonce(nonce)))
		return
	}
}

func (h *Handler) GetLoginOauthCallback(c *gin.Context) {
	provider := c.Param("provider")
	if _, ok := h.authProviderNames[provider]; !ok {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=invalidprovider")
		return
	}

	currentSession := GetSessionData(c)
	ctx := c.Request.Context()

	if state := c.Query("state"); state != currentSession.OauthState {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=invalidstate")
		return
	}

	oauth2Token, err := h.oauthConfigs[provider].Exchange(ctx, c.Query("code"))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=tokenexchange")
		return
	}

	switch h.authProviderNames[provider] {
	case AuthProviderTypeOAuth:
		// TODO
	case AuthProviderTypeOpenIDConnect:
		rawIDToken, ok := oauth2Token.Extra("id_token").(string)
		if !ok {
			c.Redirect(http.StatusSeeOther, "/auth/login?err=missingidtoken")
			return
		}
		idToken, err := h.oidcVerifiers[provider].Verify(ctx, rawIDToken)
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/auth/login?err=idtokeninvalid")
			return
		}
		if idToken.Nonce != currentSession.OidcNonce {
			c.Redirect(http.StatusSeeOther, "/auth/login?err=idtokennonce")
			return
		}

		// TODO: check if user exists in db, if not, maybe create? (if registration is allowed)

		currentSession.LoggedIn = true
		currentSession.UserIdentifier = persistence.UserIdentifier(idToken.Subject)

		var extraFields map[string]interface{}
		if err = idToken.Claims(&extraFields); err != nil {
			c.Redirect(http.StatusSeeOther, "/auth/login?err=claimsparsing")
			return
		}

		// TODO: use FieldMap to get extra fields
		//currentSession.Email = extraFields[mappedName]
	}
}

func randString(nByte int) (string, error) {
	b := make([]byte, nByte)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
