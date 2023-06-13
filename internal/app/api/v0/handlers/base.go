package handlers

import (
	"github.com/h44z/wg-portal/internal/app/api/core"
	"github.com/h44z/wg-portal/internal/app/api/v0/model"
	"net/http"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/memstore"
	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	csrf "github.com/utrack/gin-csrf"
)

type handler interface {
	GetName() string
	RegisterRoutes(g *gin.RouterGroup, authenticator *authenticationHandler)
}

// To compile the API documentation use the
// build_tool
// command that can be found in the $PROJECT_ROOT/internal/ports/api/build_tool directory.

// @title WireGuard Portal API
// @version 0.0
// @description WireGuard Portal API - a testing API endpoint

// @contact.name WireGuard Portal Developers
// @contact.url https://github.com/h44z/wg-portal

// @BasePath /api/v0
// @query.collection.format multi

func NewRestApi(cfg *config.Config, app *app.App) core.ApiEndpointSetupFunc {
	authenticator := &authenticationHandler{
		app:     app,
		Session: GinSessionStore{sessionIdentifier: cfg.Web.SessionIdentifier},
	}

	handlers := make([]handler, 0, 1)
	handlers = append(handlers, testEndpoint{})
	handlers = append(handlers, userEndpoint{app: app, authenticator: authenticator})
	handlers = append(handlers, newConfigEndpoint(app, authenticator))
	handlers = append(handlers, authEndpoint{app: app, authenticator: authenticator})
	handlers = append(handlers, interfaceEndpoint{app: app, authenticator: authenticator})
	handlers = append(handlers, peerEndpoint{app: app, authenticator: authenticator})

	return func() (core.ApiVersion, core.GroupSetupFn) {
		return "v0", func(group *gin.RouterGroup) {
			cookieStore := memstore.NewStore([]byte(cfg.Web.SessionSecret))
			cookieStore.Options(sessions.Options{
				Path:     "/",
				MaxAge:   86400, // auth session is valid for 1 day
				Secure:   strings.HasPrefix(cfg.Web.ExternalUrl, "https"),
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
			group.Use(sessions.Sessions(cfg.Web.SessionIdentifier, cookieStore))
			group.Use(cors.Default())
			group.Use(csrf.Middleware(csrf.Options{
				Secret: cfg.Web.CsrfSecret,
				ErrorFunc: func(c *gin.Context) {
					c.JSON(http.StatusBadRequest, model.Error{
						Code:    http.StatusBadRequest,
						Message: "CSRF token mismatch",
					})
					c.Abort()
				},
			}))

			group.GET("/csrf", handleCsrfGet())

			// Handler functions
			for _, h := range handlers {
				h.RegisterRoutes(group, authenticator)
			}
		}
	}
}

// handleCsrfGet returns a gorm handler function.
//
// @ID base_handleCsrfGet
// @Tags Security
// @Summary Get a CSRF token for the current session.
// @Produce json
// @Success 200 {object} string
// @Router /csrf [get]
func handleCsrfGet() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, csrf.GetToken(c))
	}
}
