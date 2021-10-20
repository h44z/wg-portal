package ui

import (
	"context"
	"net/url"
	"path"

	"golang.org/x/oauth2"

	"github.com/coreos/go-oidc"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/cmd/wg-portal/common"
	"github.com/h44z/wg-portal/internal/portal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	csrf "github.com/utrack/gin-csrf"
)

type AuthProviderType string

const (
	AuthProviderTypeOAuth         = "oauth"
	AuthProviderTypeOpenIDConnect = "oidc"
)

type Handler struct {
	config *common.Config

	backend           portal.Backend
	authProviderNames map[string]AuthProviderType
	oidcProviders     map[string]*oidc.Provider
	oauthConfigs      map[string]oauth2.Config
}

func NewHandler(config *common.Config, backend portal.Backend) (*Handler, error) {
	h := &Handler{
		config:            config,
		backend:           backend,
		authProviderNames: make(map[string]AuthProviderType),
		oidcProviders:     make(map[string]*oidc.Provider),
		oauthConfigs:      make(map[string]oauth2.Config),
	}

	extUrl, err := url.Parse(config.Core.ExternalUrl)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to parse external url")
	}

	for _, provider := range h.config.Auth.OpenIDConnect {
		if _, exists := h.authProviderNames[provider.ProviderName]; exists {
			return nil, errors.Errorf("auth provider with name %s is already registerd", provider.ProviderName)
		}
		h.authProviderNames[provider.ProviderName] = AuthProviderTypeOpenIDConnect

		var err error
		h.oidcProviders[provider.ProviderName], err = oidc.NewProvider(context.Background(), provider.BaseUrl)
		if err != nil {
			return nil, errors.WithMessagef(err, "failed to setup oidc provider %s", provider.ProviderName)
		}

		redirecUrl := *extUrl
		redirecUrl.Path = path.Join(redirecUrl.Path, "/auth/login/", provider.ProviderName, "/callback")
		scopes := []string{oidc.ScopeOpenID}
		scopes = append(scopes, provider.Scopes...)
		h.oauthConfigs[provider.ProviderName] = oauth2.Config{
			ClientID:     provider.ClientID,
			ClientSecret: provider.ClientSecret,
			Endpoint:     h.oidcProviders[provider.ProviderName].Endpoint(),
			RedirectURL:  redirecUrl.String(),
			Scopes:       scopes,
		}
	}
	for _, provider := range h.config.Auth.OAuth {
		if _, exists := h.authProviderNames[provider.ProviderName]; exists {
			return nil, errors.Errorf("auth provider with name %s is already registerd", provider.ProviderName)
		}
		h.authProviderNames[provider.ProviderName] = AuthProviderTypeOAuth

		// TODO
	}

	return h, nil
}

func (h *Handler) RegisterRoutes(g *gin.Engine) {
	csrfMiddleware := csrf.Middleware(csrf.Options{
		Secret: h.config.Core.SessionSecret,
		ErrorFunc: func(c *gin.Context) {
			c.String(400, "CSRF token mismatch")
			c.Abort()
		},
	})

	// Entrypoint
	g.GET("/", h.GetIndex)

	// Auth routes
	auth := g.Group("/auth")
	auth.Use(csrfMiddleware)
	auth.GET("/login", h.GetLogin)
	auth.POST("/login", h.PostLogin)
	auth.GET("/login/:provider", h.GetLoginOauth)
	auth.GET("/login/:provider/callback", h.GetLoginOauthCallback)
	//auth.GET("/logout", s.GetLogout)

	// Admin routes

	// User routes
}

//
// --
//

const SessionIdentifier = "wgPortalSession"

type StaticData struct {
	WebsiteTitle string
	WebsiteLogo  string
	CompanyName  string
	Year         int
	Version      string
}

func GetSessionData(c *gin.Context) common.SessionData {
	session := sessions.Default(c)
	rawSessionData := session.Get(SessionIdentifier)

	var sessionData common.SessionData
	if rawSessionData != nil {
		sessionData = rawSessionData.(common.SessionData)
	} else {
		// init a new default session
		sessionData = common.SessionData{
			Search:              map[string]string{"peers": "", "userpeers": "", "users": ""},
			SortedBy:            map[string]string{"peers": "handshake", "userpeers": "id", "users": "email"},
			SortDirection:       map[string]string{"peers": "desc", "userpeers": "asc", "users": "asc"},
			Email:               "",
			Firstname:           "",
			Lastname:            "",
			InterfaceIdentifier: "",
			IsAdmin:             false,
			LoggedIn:            false,
		}
		session.Set(SessionIdentifier, sessionData)
		if err := session.Save(); err != nil {
			logrus.Errorf("failed to store session: %v", err)
		}
	}

	return sessionData
}

func GetFlashes(c *gin.Context) []common.FlashData {
	session := sessions.Default(c)
	flashes := session.Flashes()
	if err := session.Save(); err != nil {
		logrus.Errorf("failed to store session after setting flash: %v", err)
	}

	flashData := make([]common.FlashData, len(flashes))
	for i := range flashes {
		flashData[i] = flashes[i].(common.FlashData)
	}

	return flashData
}
