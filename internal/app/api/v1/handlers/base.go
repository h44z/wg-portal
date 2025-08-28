package handlers

import (
	"errors"
	"net/http"

	"github.com/go-pkgz/routegroup"

	"github.com/fedor-git/wg-portal-2/internal/app/api/core"
	"github.com/fedor-git/wg-portal-2/internal/app/api/core/middleware/cors"
	"github.com/fedor-git/wg-portal-2/internal/app/api/v1/models"
	"github.com/fedor-git/wg-portal-2/internal/domain"
)

type Handler interface {
	// GetName returns the name of the handler.
	GetName() string
	// RegisterRoutes registers the routes for the handler. The session manager is passed to the handler.
	RegisterRoutes(g *routegroup.Bundle)
}

// To compile the API documentation use the
// api_build_tool
// command that can be found in the $PROJECT_ROOT/cmd/api_build_tool directory.

// @title WireGuard Portal Public API
// @version 1.0
// @description The WireGuard Portal REST API enables efficient management of WireGuard VPN configurations through a set of JSON-based endpoints.
// @description It supports creating and editing peers, interfaces, and user profiles, while also providing role-based access control and auditing.
// @description This API allows seamless integration with external tools or scripts for automated network configuration and administration.

// @license.name MIT
// @license.url https://github.com/fedor-git/wg-portal-2/blob/master/LICENSE.txt

// @contact.name WireGuard Portal Project
// @contact.url https://github.com/fedor-git/wg-portal-2

// @securityDefinitions.basic BasicAuth

// @BasePath /api/v1
// @query.collection.format multi

func NewRestApi(handlers ...Handler) core.ApiEndpointSetupFunc {
	return func() (core.ApiVersion, core.GroupSetupFn) {
		return "v1", func(group *routegroup.Bundle) {
			group.Use(cors.New().Handler)

			// Handler functions
			for _, h := range handlers {
				h.RegisterRoutes(group)
			}
		}
	}
}

func ParseServiceError(err error) (int, models.Error) {
	if err == nil {
		return 500, models.Error{
			Code:    500,
			Message: "unknown server error",
		}
	}

	code := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, domain.ErrNoPermission):
		code = http.StatusForbidden
	case errors.Is(err, domain.ErrDuplicateEntry):
		code = http.StatusConflict
	case errors.Is(err, domain.ErrInvalidData):
		code = http.StatusBadRequest
	}

	return code, models.Error{
		Code:    code,
		Message: err.Error(),
	}
}

// region handler-interfaces

type Authenticator interface {
	// LoggedIn checks if a user is logged in. If scopes are given, they are validated as well.
	LoggedIn(scopes ...Scope) func(next http.Handler) http.Handler
}

type Validator interface {
	// Struct validates the given struct.
	Struct(s interface{}) error
}

// endregion handler-interfaces
