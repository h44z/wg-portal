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

type ControllerManager interface {
	GetControllerNames() []config.BackendBase
}

type ConfigEndpoint struct {
	cfg           *config.Config
	authenticator Authenticator
	controllerMgr ControllerManager

	tpl *respond.TemplateRenderer
}

func NewConfigEndpoint(cfg *config.Config, authenticator Authenticator, ctrlMgr ControllerManager) ConfigEndpoint {
	ep := ConfigEndpoint{
		cfg:           cfg,
		authenticator: authenticator,
		controllerMgr: ctrlMgr,
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
		basePath := e.cfg.Web.BasePath
		backendUrl := fmt.Sprintf("%s%s/api/v0", e.cfg.Web.ExternalUrl, basePath)
		if request.Header(r, "x-wg-dev") != "" {
			referer := request.Header(r, "Referer")
			host := "localhost"
			port := "5000"
			parsedReferer, err := url.Parse(referer)
			if err == nil {
				host, port, _ = net.SplitHostPort(parsedReferer.Host)
			}
			backendUrl = fmt.Sprintf("http://%s:%s%s/api/v0", host,
				port, basePath) // override if request comes from frontend started with npm run dev
		}

		e.tpl.Render(w, http.StatusOK, "frontend_config.js.gotpl", "text/javascript", map[string]any{
			"BackendUrl":      backendUrl,
			"BasePath":        basePath,
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

		controllerFn := func() []model.SettingsBackendNames {
			controllers := e.controllerMgr.GetControllerNames()
			names := make([]model.SettingsBackendNames, 0, len(controllers))

			for _, controller := range controllers {
				displayName := controller.GetDisplayName()
				if displayName == "" {
					displayName = controller.Id // fallback to ID if no display name is set
				}
				if controller.Id == config.LocalBackendName {
					displayName = "modals.interface-edit.backend.local" // use a localized string for the local backend
				}
				names = append(names, model.SettingsBackendNames{
					Id:   controller.Id,
					Name: displayName,
				})
			}

			return names

		}

		hasSocialLogin := len(e.cfg.Auth.OAuth) > 0 || len(e.cfg.Auth.OpenIDConnect) > 0 || e.cfg.Auth.WebAuthn.Enabled

		// For anonymous users, we return the settings object with minimal information
		if sessionUser.Id == domain.CtxUnknownUserId || sessionUser.Id == "" {
			respond.JSON(w, http.StatusOK, model.Settings{
				WebAuthnEnabled:   e.cfg.Auth.WebAuthn.Enabled,
				AvailableBackends: []model.SettingsBackendNames{}, // return an empty list instead of null
				LoginFormVisible:  !e.cfg.Auth.HideLoginForm || !hasSocialLogin,
			})
		} else {
			respond.JSON(w, http.StatusOK, model.Settings{
				MailLinkOnly:              e.cfg.Mail.LinkOnly,
				PersistentConfigSupported: e.cfg.Advanced.ConfigStoragePath != "",
				SelfProvisioning:          e.cfg.Core.SelfProvisioningAllowed,
				ApiAdminOnly:              e.cfg.Advanced.ApiAdminOnly,
				WebAuthnEnabled:           e.cfg.Auth.WebAuthn.Enabled,
				MinPasswordLength:         e.cfg.Auth.MinPasswordLength,
				AvailableBackends:         controllerFn(),
				LoginFormVisible:          !e.cfg.Auth.HideLoginForm || !hasSocialLogin,
			})
		}
	}
}
