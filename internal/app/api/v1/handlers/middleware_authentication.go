package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/h44z/wg-portal/internal/app/api/v0/model"
	"github.com/h44z/wg-portal/internal/domain"
)

type Scope string

const (
	ScopeAdmin Scope = "ADMIN" // Admin scope contains all other scopes
)

type UserSource interface {
	GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
}

type authenticationHandler struct {
	userSource UserSource
}

// LoggedIn checks if a user is logged in. If scopes are given, they are validated as well.
func (h authenticationHandler) LoggedIn(scopes ...Scope) gin.HandlerFunc {
	return func(c *gin.Context) {
		username, password, ok := c.Request.BasicAuth()
		if !ok || username == "" || password == "" {
			// Abort the request with the appropriate error code
			c.Abort()
			c.JSON(http.StatusUnauthorized, model.Error{Code: http.StatusUnauthorized, Message: "missing credentials"})
			return
		}

		// check if user exists in DB

		ctx := domain.SetUserInfo(c.Request.Context(), domain.SystemAdminContextUserInfo())
		user, err := h.userSource.GetUser(ctx, domain.UserIdentifier(username))
		if err != nil {
			// Abort the request with the appropriate error code
			c.Abort()
			c.JSON(http.StatusUnauthorized, model.Error{Code: http.StatusUnauthorized, Message: "invalid credentials"})
			return
		}

		// validate API token
		if err := user.CheckApiToken(password); err != nil {
			// Abort the request with the appropriate error code
			c.Abort()
			c.JSON(http.StatusUnauthorized, model.Error{Code: http.StatusUnauthorized, Message: "invalid credentials"})
			return
		}

		if !UserHasScopes(user, scopes...) {
			// Abort the request with the appropriate error code
			c.Abort()
			c.JSON(http.StatusForbidden, model.Error{Code: http.StatusForbidden, Message: "not enough permissions"})
			return
		}

		c.Set(domain.CtxUserInfo, &domain.ContextUserInfo{
			Id:      user.Identifier,
			IsAdmin: user.IsAdmin,
		})

		// Continue down the chain to Handler etc
		c.Next()
	}
}

func UserHasScopes(user *domain.User, scopes ...Scope) bool {
	// No scopes give, so the check should succeed
	if len(scopes) == 0 {
		return true
	}

	// check if user has admin scope
	if user.IsAdmin {
		return true
	}

	// Check if admin scope is required
	for _, scope := range scopes {
		if scope == ScopeAdmin {
			return false
		}
	}

	return true
}
