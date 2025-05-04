package handlers

import (
	"context"
	"net/http"

	"github.com/h44z/wg-portal/internal/app/api/core/respond"
	"github.com/h44z/wg-portal/internal/app/api/v0/model"
	"github.com/h44z/wg-portal/internal/domain"
)

type Scope string

const (
	ScopeAdmin Scope = "ADMIN" // Admin scope contains all other scopes
)

type UserAuthenticator interface {
	GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
}

type AuthenticationHandler struct {
	authenticator UserAuthenticator
}

func NewAuthenticationHandler(authenticator UserAuthenticator) AuthenticationHandler {
	return AuthenticationHandler{
		authenticator: authenticator,
	}
}

// LoggedIn checks if a user is logged in. If scopes are given, they are validated as well.
func (h AuthenticationHandler) LoggedIn(scopes ...Scope) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username, password, ok := r.BasicAuth()
			if !ok || username == "" || password == "" {
				// Abort the request with the appropriate error code
				respond.JSON(w, http.StatusUnauthorized,
					model.Error{Code: http.StatusUnauthorized, Message: "missing credentials"})
				return
			}

			// check if user exists in DB

			ctx := domain.SetUserInfo(r.Context(), domain.SystemAdminContextUserInfo())
			user, err := h.authenticator.GetUser(ctx, domain.UserIdentifier(username))
			if err != nil {
				// Abort the request with the appropriate error code
				respond.JSON(w, http.StatusUnauthorized,
					model.Error{Code: http.StatusUnauthorized, Message: "invalid credentials"})
				return
			}

			// validate API token
			if err := user.CheckApiToken(password); err != nil {
				// Abort the request with the appropriate error code
				respond.JSON(w, http.StatusUnauthorized,
					model.Error{Code: http.StatusUnauthorized, Message: "invalid credentials"})
				return
			}

			if !UserHasScopes(user, scopes...) {
				// Abort the request with the appropriate error code
				respond.JSON(w, http.StatusForbidden,
					model.Error{Code: http.StatusForbidden, Message: "not enough permissions"})
				return
			}

			ctx = context.WithValue(r.Context(), domain.CtxUserInfo, &domain.ContextUserInfo{
				Id:      user.Identifier,
				IsAdmin: user.IsAdmin,
			})
			r = r.WithContext(ctx)

			// Continue down the chain to Handler etc
			next.ServeHTTP(w, r)
		})
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
