package handlers

import (
	"context"
	"net/http"

	"github.com/go-pkgz/routegroup"

	"github.com/h44z/wg-portal/internal/app/api/core"
	"github.com/h44z/wg-portal/internal/app/api/core/middleware/cors"
	"github.com/h44z/wg-portal/internal/app/api/core/middleware/csrf"
	"github.com/h44z/wg-portal/internal/app/api/core/respond"
)

type SessionMiddleware interface {
	// SetData sets the session data for the given context.
	SetData(ctx context.Context, val SessionData)
	// GetData returns the session data for the given context. If no data is found, the default session data is returned.
	GetData(ctx context.Context) SessionData
	// DestroyData destroys the session data for the given context.
	DestroyData(ctx context.Context)

	// GetString returns the string value for the given key. If no value is found, an empty string is returned.
	GetString(ctx context.Context, key string) string
	// Put sets the value for the given key.
	Put(ctx context.Context, key string, value any)
	// LoadAndSave is a middleware that loads the session data for the given request and saves it after the request is
	// finished.
	LoadAndSave(next http.Handler) http.Handler
}

type Handler interface {
	// GetName returns the name of the handler.
	GetName() string
	// RegisterRoutes registers the routes for the handler. The session manager is passed to the handler.
	RegisterRoutes(g *routegroup.Bundle)
}

// To compile the API documentation use the
// api_build_tool
// command that can be found in the $PROJECT_ROOT/cmd/api_build_tool directory.

// @title WireGuard Portal SPA-UI API
// @version 0.0
// @description WireGuard Portal API - UI Endpoints

// @contact.name WireGuard Portal Developers
// @contact.url https://github.com/h44z/wg-portal

// @BasePath /api/v0
// @query.collection.format multi

func NewRestApi(
	session SessionMiddleware,
	handlers ...Handler,
) core.ApiEndpointSetupFunc {
	return func() (core.ApiVersion, core.GroupSetupFn) {
		return "v0", func(group *routegroup.Bundle) {
			csrfMiddleware := csrf.New(func(r *http.Request) string {
				return session.GetData(r.Context()).CsrfToken
			}, func(r *http.Request, token string) {
				currentSession := session.GetData(r.Context())
				currentSession.CsrfToken = token
				session.SetData(r.Context(), currentSession)
			})

			group.Use(session.LoadAndSave)
			group.Use(csrfMiddleware.Handler)
			group.Use(cors.New().Handler)

			group.With(csrfMiddleware.RefreshToken).HandleFunc("GET /csrf", handleCsrfGet())

			// Handler functions
			for _, h := range handlers {
				h.RegisterRoutes(group)
			}
		}
	}
}

// handleCsrfGet returns a gorm Handler function.
//
// @ID base_handleCsrfGet
// @Tags Security
// @Summary Get a CSRF token for the current session.
// @Produce json
// @Success 200 {object} string
// @Router /csrf [get]
func handleCsrfGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		respond.JSON(w, http.StatusOK, csrf.GetToken(r.Context()))
	}
}

// region handler-interfaces

type Authenticator interface {
	// LoggedIn checks if a user is logged in. If scopes are given, they are validated as well.
	LoggedIn(scopes ...Scope) func(next http.Handler) http.Handler
	// UserIdMatch checks if the user id in the session matches the user id in the request. If not, the request is aborted.
	UserIdMatch(idParameter string) func(next http.Handler) http.Handler
	// InfoOnly only add user info to the request context. No login check is performed.
	InfoOnly() func(next http.Handler) http.Handler
}

type Session interface {
	// SetData sets the session data for the given context.
	SetData(ctx context.Context, val SessionData)
	// GetData returns the session data for the given context. If no data is found, the default session data is returned.
	GetData(ctx context.Context) SessionData
	// DestroyData destroys the session data for the given context.
	DestroyData(ctx context.Context)
}

type Validator interface {
	// Struct validates the given struct.
	Struct(s interface{}) error
}

// endregion handler-interfaces
