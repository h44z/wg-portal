package handlers

import (
	"embed"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"

	"github.com/go-pkgz/routegroup"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/app/api/core/request"
	"github.com/h44z/wg-portal/internal/app/api/core/respond"
	"github.com/h44z/wg-portal/internal/app/api/v0/model"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

//go:embed frontend_config.js.gotpl
var frontendJs embed.FS

type ConfigEndpoint struct {
	cfg           *config.Config
	authenticator Authenticator

	tpl *respond.TemplateRenderer
}

func NewConfigEndpoint(cfg *config.Config, authenticator Authenticator) ConfigEndpoint {
	ep := ConfigEndpoint{
		cfg:           cfg,
		authenticator: authenticator,
		tpl: respond.NewTemplateRenderer(template.Must(template.ParseFS(frontendJs,
			"frontend_config.js.gotpl"))),
	}

	return ep
}

func (e ConfigEndpoint) GetName() string {
	return "ConfigEndpoint"
}

func (e ConfigEndpoint) RegisterRoutes(g *routegroup.Bundle) {
	apiGroup := g.Mount("/config")

	apiGroup.HandleFunc("GET /frontend.js", e.handleConfigJsGet())
	apiGroup.With(e.authenticator.InfoOnly()).HandleFunc("GET /settings", e.handleSettingsGet())
}

// handleConfigJsGet returns a gorm Handler function.
//
// @ID config_handleConfigJsGet
// @Tags Configuration
// @Summary Get the dynamic frontend configuration javascript.
// @Produce text/javascript
// @Success 200 string javascript "The JavaScript contents"
// @Failure 500
// @Router /config/frontend.js [get]
func (e ConfigEndpoint) handleConfigJsGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		backendUrl := fmt.Sprintf("%s/api/v0", e.cfg.Web.ExternalUrl)
		if request.Header(r, "x-wg-dev") != "" {
			referer := request.Header(r, "Referer")
			host := "localhost"
			port := "5000"
			parsedReferer, err := url.Parse(referer)
			if err == nil {
				host, port, _ = net.SplitHostPort(parsedReferer.Host)
			}
			backendUrl = fmt.Sprintf("http://%s:%s/api/v0", host,
				port) // override if request comes from frontend started with npm run dev
		}

		e.tpl.Render(w, http.StatusOK, "frontend_config.js.gotpl", "text/javascript", map[string]any{
			"BackendUrl":      backendUrl,
			"Version":         internal.Version,
			"SiteTitle":       e.cfg.Web.SiteTitle,
			"SiteCompanyName": e.cfg.Web.SiteCompanyName,
		})
	}
}

// handleSettingsGet returns a gorm Handler function.
//
// @ID config_handleSettingsGet
// @Tags Configuration
// @Summary Get the frontend settings object.
// @Produce json
// @Success 200 {object} model.Settings
// @Success 200 string javascript "The JavaScript contents"
// @Router /config/settings [get]
func (e ConfigEndpoint) handleSettingsGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionUser := domain.GetUserInfo(r.Context())

		// For anonymous users, we return the settings object with minimal information
		if sessionUser.Id == domain.CtxUnknownUserId || sessionUser.Id == "" {
			respond.JSON(w, http.StatusOK, model.Settings{
				WebAuthnEnabled: e.cfg.Auth.WebAuthn.Enabled,
			})
		} else {
			respond.JSON(w, http.StatusOK, model.Settings{
				MailLinkOnly:              e.cfg.Mail.LinkOnly,
				PersistentConfigSupported: e.cfg.Advanced.ConfigStoragePath != "",
				SelfProvisioning:          e.cfg.Core.SelfProvisioningAllowed,
				ApiAdminOnly:              e.cfg.Advanced.ApiAdminOnly,
				WebAuthnEnabled:           e.cfg.Auth.WebAuthn.Enabled,
				MinPasswordLength:         e.cfg.Auth.MinPasswordLength,
			})
		}
	}
}
