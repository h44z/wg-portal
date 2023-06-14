package handlers

import (
	"bytes"
	"embed"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/app"
	"html/template"
	"net/http"
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
			backendUrl = "http://localhost:5000/api/v0" // override if reqest comes from frontend started with npm run dev
		}
		buf := &bytes.Buffer{}
		err := e.tpl.ExecuteTemplate(buf, "frontend_config.js.gotpl", gin.H{
			"BackendUrl": backendUrl,
		})
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}

		c.Data(http.StatusOK, "application/javascript", buf.Bytes())
	}
}
