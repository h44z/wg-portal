package ui

import (
	"context"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/cmd/wg-portal/common"
	"github.com/h44z/wg-portal/internal/portal"
	"github.com/pkg/errors"
	csrf "github.com/utrack/gin-csrf"
)

type handler struct {
	config *common.Config

	session             SessionStore
	backend             portal.Backend
	oauthAuthenticators map[string]common.Authenticator
}

func NewHandler(config *common.Config, backend portal.Backend) (*handler, error) {
	h := &handler{
		config:              config,
		backend:             backend,
		session:             GinSessionStore{sessionIdentifier: "wgPortalSession"},
		oauthAuthenticators: make(map[string]common.Authenticator),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := h.setupAuthProviders(ctx)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to setup authentication providers")
	}

	return h, nil
}

func (h *handler) setupAuthProviders(ctx context.Context) error {
	extUrl, err := url.Parse(h.config.Core.ExternalUrl)
	if err != nil {
		return errors.WithMessage(err, "failed to parse external url")
	}

	for i := range h.config.Auth.OpenIDConnect {
		providerCfg := &h.config.Auth.OpenIDConnect[i]
		providerId := strings.ToLower(providerCfg.ProviderName)

		if _, exists := h.oauthAuthenticators[providerId]; exists {
			return errors.Errorf("auth provider with name %s is already registerd", providerId)
		}

		redirectUrl := *extUrl
		redirectUrl.Path = path.Join(redirectUrl.Path, "/auth/login/", providerId, "/callback")

		authenticator, err := common.NewOidcAuthenticator(ctx, redirectUrl.String(), providerCfg)
		if err != nil {
			return errors.WithMessagef(err, "failed to setup oidc authentication provider %s", providerCfg.ProviderName)
		}
		h.oauthAuthenticators[providerId] = authenticator
	}
	for i := range h.config.Auth.OAuth {
		providerCfg := &h.config.Auth.OAuth[i]
		providerId := strings.ToLower(providerCfg.ProviderName)

		if _, exists := h.oauthAuthenticators[providerId]; exists {
			return errors.Errorf("auth provider with name %s is already registerd", providerId)
		}

		redirectUrl := *extUrl
		redirectUrl.Path = path.Join(redirectUrl.Path, "/auth/login/", providerId, "/callback")

		authenticator, err := common.NewPlainOauthAuthenticator(ctx, redirectUrl.String(), providerCfg)
		if err != nil {
			return errors.WithMessagef(err, "failed to setup oauth authentication provider %s", providerId)
		}
		h.oauthAuthenticators[providerId] = authenticator
	}

	return nil
}

func (h *handler) RegisterRoutes(g *gin.Engine) {
	csrfMiddleware := csrf.Middleware(csrf.Options{
		Secret: h.config.Core.SessionSecret,
		ErrorFunc: func(c *gin.Context) {
			c.String(400, "CSRF token mismatch")
			c.Abort()
		},
	})

	// Entrypoint
	g.GET("/", h.GetIndex())

	// Auth routes
	auth := g.Group("/auth")
	auth.Use(csrfMiddleware)
	auth.GET("/login", h.handleLoginGet())
	auth.POST("/login", h.handleLoginPost())
	auth.GET("/login/:provider", h.handleLoginGetOauth())
	auth.GET("/login/:provider/callback", h.handleLoginGetOauthCallback())
	auth.GET("/logout", h.handleLogoutGet())

	// Admin routes

	// User routes
}

//
// --
//

type StaticData struct {
	WebsiteTitle string
	WebsiteLogo  string
	CompanyName  string
	Year         int
	Version      string
}
