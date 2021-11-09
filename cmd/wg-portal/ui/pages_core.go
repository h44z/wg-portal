package ui

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/cmd/wg-portal/common"
	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/pkg/errors"
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

	username := strings.ToLower(c.PostForm("username"))
	password := c.PostForm("password")
	deepLink := c.PostForm("_dl")

	// Validate form input
	if strings.Trim(username, " ") == "" || strings.Trim(password, " ") == "" {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=missingdata")
		return
	}

	// TODO: implement db authentication
	/*c.HTML(http.StatusOK, "login.html", gin.H{
		"HasError": authError != "",
		"Message":  errMsg,
		"DeepLink": deepLink,
		"Static":   h.getStaticData(),
		"Csrf":     csrf.GetToken(c),
	})*/

	c.Redirect(http.StatusSeeOther, deepLink)
}

func (h *Handler) GetLoginOauth(c *gin.Context) {
	providerId := c.Param("provider")
	if _, ok := h.oauthAuthenticators[providerId]; !ok {
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

	authenticator := h.oauthAuthenticators[providerId]

	var authCodeUrl string
	switch authenticator.GetType() {
	case common.AuthenticatorTypeOAuth:
		authCodeUrl = authenticator.AuthCodeURL(state)
	case common.AuthenticatorTypeOidc:
		nonce, err := randString(16)
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/auth/login?err=randsrcunavailable")
			return
		}
		currentSession.OidcNonce = nonce

		authCodeUrl = authenticator.AuthCodeURL(state, oidc.Nonce(nonce))
	}

	err = UpdateSessionData(c, currentSession)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=sessionerror")
		return
	}

	c.Redirect(http.StatusFound, authCodeUrl)

}

func (h *Handler) GetLoginOauthCallback(c *gin.Context) {
	providerId := c.Param("provider")
	if _, ok := h.oauthAuthenticators[providerId]; !ok {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=invalidprovider")
		return
	}

	currentSession := GetSessionData(c)
	ctx := c.Request.Context()

	if state := c.Query("state"); state != currentSession.OauthState {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=invalidstate")
		return
	}

	authenticator := h.oauthAuthenticators[providerId]
	oauthCode := c.Query("code")
	oauth2Token, err := authenticator.Exchange(ctx, oauthCode)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=tokenexchange")
		return
	}

	rawUserInfo, err := authenticator.GetUserInfo(c.Request.Context(), oauth2Token, currentSession.OidcNonce)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=userinfofetch")
		return
	}

	userInfo, err := authenticator.ParseUserInfo(rawUserInfo)

	fmt.Println(userInfo) // TODO: implement login/registration process
}

func (h *Handler) passwordAuthentication(username, password string) (*persistence.User, error) {
	err := h.backend.PlaintextAuthentication(persistence.UserIdentifier(username), password)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to authenticate")
	}

	// TODO
	return nil, nil
}

func randString(nByte int) (string, error) {
	b := make([]byte, nByte)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
