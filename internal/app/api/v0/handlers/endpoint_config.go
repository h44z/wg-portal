package handlers

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/app/api/v0/model"
)

//go:embed frontend_config.js.gotpl
var frontendJs embed.FS

type configEndpoint struct {
	app           *app.App
	authenticator *authenticationHandler

	tpl *template.Template
}

func newConfigEndpoint(app *app.App, authenticator *authenticationHandler) configEndpoint {
	ep := configEndpoint{
		app:           app,
		authenticator: authenticator,
		tpl:           template.Must(template.ParseFS(frontendJs, "frontend_config.js.gotpl")),
	}

	return ep
}

func (e configEndpoint) GetName() string {
	return "ConfigEndpoint"
}

func (e configEndpoint) RegisterRoutes(g *gin.RouterGroup, authenticator *authenticationHandler) {
	apiGroup := g.Group("/config")

	apiGroup.GET("/frontend.js", e.handleConfigJsGet())
	apiGroup.GET("/settings", e.authenticator.LoggedIn(), e.handleSettingsGet())
}

// handleConfigJsGet returns a gorm handler function.
//
// @ID config_handleConfigJsGet
// @Tags Configuration
// @Summary Get the dynamic frontend configuration javascript.
// @Produce text/javascript
// @Success 200 string javascript "The JavaScript contents"
// @Router /config/frontend.js [get]
func (e configEndpoint) handleConfigJsGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		backendUrl := fmt.Sprintf("%s/api/v0", e.app.Config.Web.ExternalUrl)
		if c.GetHeader("x-wg-dev") != "" {
			referer := c.Request.Header.Get("Referer")
			host := "localhost"
			port := "5000"
			parsedReferer, err := url.Parse(referer)
			if err == nil {
				host, port, _ = net.SplitHostPort(parsedReferer.Host)
			}
			backendUrl = fmt.Sprintf("http://%s:%s/api/v0", host,
				port) // override if request comes from frontend started with npm run dev
		}
		buf := &bytes.Buffer{}
		err := e.tpl.ExecuteTemplate(buf, "frontend_config.js.gotpl", gin.H{
			"BackendUrl":      backendUrl,
			"Version":         "unknown",
			"SiteTitle":       e.app.Config.Web.SiteTitle,
			"SiteCompanyName": e.app.Config.Web.SiteCompanyName,
		})
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		c.Data(http.StatusOK, "application/javascript", buf.Bytes())
	}
}

// handleSettingsGet returns a gorm handler function.
//
// @ID config_handleSettingsGet
// @Tags Configuration
// @Summary Get the frontend settings object.
// @Produce json
// @Success 200 {object} model.Settings
// @Success 200 string javascript "The JavaScript contents"
// @Router /config/settings [get]
func (e configEndpoint) handleSettingsGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, model.Settings{
			MailLinkOnly:              e.app.Config.Mail.LinkOnly,
			PersistentConfigSupported: e.app.Config.Advanced.ConfigStoragePath != "",
			SelfProvisioning:          e.app.Config.Core.SelfProvisioningAllowed,
			ApiAdminOnly:              e.app.Config.Advanced.ApiAdminOnly,
		})
	}
}
