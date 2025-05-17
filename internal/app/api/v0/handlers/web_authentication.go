package handlers

import (
	"context"
	"net/http"

	"github.com/h44z/wg-portal/internal/app/api/core/request"
	"github.com/h44z/wg-portal/internal/app/api/core/respond"
	"github.com/h44z/wg-portal/internal/app/api/v0/model"
	"github.com/h44z/wg-portal/internal/domain"
)

type Scope string

const (
	ScopeAdmin Scope = "ADMIN" // Admin scope contains all other scopes
)

type UserAuthenticator interface {
	IsUserValid(ctx context.Context, id domain.UserIdentifier) bool
}

type AuthenticationHandler struct {
	authenticator UserAuthenticator
	session       Session
}

func NewAuthenticationHandler(authenticator UserAuthenticator, session Session) AuthenticationHandler {
	return AuthenticationHandler{
		authenticator: authenticator,
		session:       session,
	}
}

// LoggedIn checks if a user is logged in. If scopes are given, they are validated as well.
func (h AuthenticationHandler) LoggedIn(scopes ...Scope) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session := h.session.GetData(r.Context())

			if !session.LoggedIn {
				// Abort the request with the appropriate error code
				respond.JSON(w, http.StatusUnauthorized,
					model.Error{Code: http.StatusUnauthorized, Message: "not logged in"})
				return
			}

			if !UserHasScopes(session, scopes...) {
				// Abort the request with the appropriate error code
				respond.JSON(w, http.StatusForbidden,
					model.Error{Code: http.StatusForbidden, Message: "not enough permissions"})
				return
			}

			// Check if logged-in user is still valid
			if !h.authenticator.IsUserValid(r.Context(), domain.UserIdentifier(session.UserIdentifier)) {
				h.session.DestroyData(r.Context())
				respond.JSON(w, http.StatusUnauthorized,
					model.Error{Code: http.StatusUnauthorized, Message: "session no longer available"})
				return
			}

			ctx := context.WithValue(r.Context(), domain.CtxUserInfo, &domain.ContextUserInfo{
				Id:      domain.UserIdentifier(session.UserIdentifier),
				IsAdmin: session.IsAdmin,
			})
			r = r.WithContext(ctx)

			// Continue down the chain to Handler etc
			next.ServeHTTP(w, r)
		})
	}
}

// InfoOnly only checks if the user is logged in and adds the user id to the context.
// If the user is not logged in, the context user id is set to domain.CtxUnknownUserId.
func (h AuthenticationHandler) InfoOnly() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session := h.session.GetData(r.Context())

			var newContext context.Context

			if !session.LoggedIn {
				newContext = domain.SetUserInfo(r.Context(), domain.DefaultContextUserInfo())
			} else {
				newContext = domain.SetUserInfo(r.Context(), &domain.ContextUserInfo{
					Id:      domain.UserIdentifier(session.UserIdentifier),
					IsAdmin: session.IsAdmin,
				})
			}

			r = r.WithContext(newContext)

			// Continue down the chain to Handler etc
			next.ServeHTTP(w, r)
		})
	}
}

// UserIdMatch checks if the user id in the session matches the user id in the request. If not, the request is aborted.
func (h AuthenticationHandler) UserIdMatch(idParameter string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session := h.session.GetData(r.Context())

			if session.IsAdmin {
				next.ServeHTTP(w, r) // Admins can do everything
				return
			}

			sessionUserId := domain.UserIdentifier(session.UserIdentifier)
			requestUserId := domain.UserIdentifier(Base64UrlDecode(request.Path(r, idParameter)))

			if sessionUserId != requestUserId {
				// Abort the request with the appropriate error code
				respond.JSON(w, http.StatusForbidden,
					model.Error{Code: http.StatusForbidden, Message: "not enough permissions"})
				return
			}

			// Continue down the chain to Handler etc
			next.ServeHTTP(w, r)
		})
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
