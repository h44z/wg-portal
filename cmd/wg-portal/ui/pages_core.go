package ui

import (
	"html/template"
	"net/http"
	"strings"
	"time"

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
	currentSession := GetSessionData(c)
	if currentSession.LoggedIn {
		c.Redirect(http.StatusSeeOther, "/") // already logged in
	}

	provider := c.Param("provider")
	if _, ok := h.authProviderNames[provider]; !ok {
		c.Redirect(http.StatusSeeOther, "/auth/login?err=invalidprovider")
		return
	}

	switch h.authProviderNames[provider] {
	case AuthProviderTypeOAuth:
	case AuthProviderTypeOpenIDConnect:
	}
}

func (h *Handler) GetLoginOauthCallback(c *gin.Context) {
	//code := c.PostForm("code")
}
