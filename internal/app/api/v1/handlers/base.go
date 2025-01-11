package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/app/api/core"
	"github.com/h44z/wg-portal/internal/app/api/v1/models"
	"github.com/h44z/wg-portal/internal/domain"
)

type Handler interface {
	GetName() string
	RegisterRoutes(g *gin.RouterGroup, authenticator *authenticationHandler)
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
// @license.url https://github.com/h44z/wg-portal/blob/master/LICENSE.txt

// @contact.name WireGuard Portal Project
// @contact.url https://github.com/h44z/wg-portal

// @securityDefinitions.basic BasicAuth

// @BasePath /api/v1
// @query.collection.format multi

func NewRestApi(userSource UserSource, handlers ...Handler) core.ApiEndpointSetupFunc {
	authenticator := &authenticationHandler{
		userSource: userSource,
	}

	return func() (core.ApiVersion, core.GroupSetupFn) {
		return "v1", func(group *gin.RouterGroup) {
			group.Use(cors.Default())

			// Handler functions
			for _, h := range handlers {
				h.RegisterRoutes(group, authenticator)
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
