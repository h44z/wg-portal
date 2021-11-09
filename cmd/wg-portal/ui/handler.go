package ui

import (
	"context"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/cmd/wg-portal/common"
	"github.com/h44z/wg-portal/internal/portal"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	csrf "github.com/utrack/gin-csrf"
)

type Handler struct {
	config *common.Config

	backend             portal.Backend
	oauthAuthenticators map[string]common.Authenticator
}

func NewHandler(config *common.Config, backend portal.Backend) (*Handler, error) {
	h := &Handler{
		config:              config,
		backend:             backend,
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

func (h *Handler) setupAuthProviders(ctx context.Context) error {
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

func UpdateSessionData(c *gin.Context, data common.SessionData) error {
	session := sessions.Default(c)
	session.Set(SessionIdentifier, data)
	if err := session.Save(); err != nil {
		logrus.Errorf("failed to store session: %v", err)
		return errors.Wrap(err, "failed to store session")
	}
	return nil
}

func DestroySessionData(c *gin.Context) error {
	session := sessions.Default(c)
	session.Delete(SessionIdentifier)
	if err := session.Save(); err != nil {
		logrus.Errorf("failed to destroy session: %v", err)
		return errors.Wrap(err, "failed to destroy session")
	}
	return nil
}
