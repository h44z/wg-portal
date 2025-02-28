package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/app/api/v0/model"
	"github.com/h44z/wg-portal/internal/domain"
)

type Scope string

const (
	ScopeAdmin   Scope = "ADMIN" // Admin scope contains all other scopes
	ScopeSwagger Scope = "SWAGGER"
	ScopeUser    Scope = "USER"
)

type authenticationHandler struct {
	app     *app.App
	Session SessionStore
}

// LoggedIn checks if a user is logged in. If scopes are given, they are validated as well.
func (h authenticationHandler) LoggedIn(scopes ...Scope) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := h.Session.GetData(c)

		if !session.LoggedIn {
			// Abort the request with the appropriate error code
			c.Abort()
			c.JSON(http.StatusUnauthorized, model.Error{Code: http.StatusUnauthorized, Message: "not logged in"})
			return
		}

		if !UserHasScopes(session, scopes...) {
			// Abort the request with the appropriate error code
			c.Abort()
			c.JSON(http.StatusForbidden, model.Error{Code: http.StatusForbidden, Message: "not enough permissions"})
			return
		}

		// Check if logged-in user is still valid
		if !h.app.Authenticator.IsUserValid(c.Request.Context(), domain.UserIdentifier(session.UserIdentifier)) {
			h.Session.DestroyData(c)
			c.Abort()
			c.JSON(http.StatusUnauthorized,
				model.Error{Code: http.StatusUnauthorized, Message: "session no longer available"})
			return
		}

		c.Set(domain.CtxUserInfo, &domain.ContextUserInfo{
			Id:      domain.UserIdentifier(session.UserIdentifier),
			IsAdmin: session.IsAdmin,
		})

		// Continue down the chain to handler etc
		c.Next()
	}
}

// UserIdMatch checks if the user id in the session matches the user id in the request. If not, the request is aborted.
func (h authenticationHandler) UserIdMatch(idParameter string) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := h.Session.GetData(c)

		if session.IsAdmin {
			c.Next() // Admins can do everything
			return
		}

		sessionUserId := domain.UserIdentifier(session.UserIdentifier)
		requestUserId := domain.UserIdentifier(Base64UrlDecode(c.Param(idParameter)))

		if sessionUserId != requestUserId {
			// Abort the request with the appropriate error code
			c.Abort()
			c.JSON(http.StatusForbidden, model.Error{Code: http.StatusForbidden, Message: "not enough permissions"})
			return
		}

		// Continue down the chain to handler etc
		c.Next()
	}
}

func UserHasScopes(session SessionData, scopes ...Scope) bool {
	// No scopes give, so the check should succeed
	if len(scopes) == 0 {
		return true
	}

	// check if user has admin scope
	if session.IsAdmin {
		return true
	}

	// Check if admin scope is required
	for _, scope := range scopes {
		if scope == ScopeAdmin {
			return false
		}
	}

	// For all other scopes, a logged-in user is sufficient (for now)
	if session.LoggedIn {
		return true
	}

	return false
}
