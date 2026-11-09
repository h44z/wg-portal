package handlers

import (
	"context"
	"net/http"
	"slices"

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
	GetValidUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, bool)
	IsDefaultPasswordInUse(ctx context.Context) bool
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

			if !h.refreshSession(r.Context(), &session) {
				respond.JSON(w, http.StatusUnauthorized,
					model.Error{Code: http.StatusUnauthorized, Message: "session no longer available"})
				return
			}

			if !UserHasScopes(session, scopes...) {
				// Abort the request with the appropriate error code
				respond.JSON(w, http.StatusForbidden,
					model.Error{Code: http.StatusForbidden, Message: "not enough permissions"})
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

// InfoOnly adds current user info to the context without requiring a login.
// If the user is not logged in or no longer valid, the context user id is set to domain.CtxUnknownUserId.
func (h AuthenticationHandler) InfoOnly() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session := h.session.GetData(r.Context())

			var newContext context.Context

			if !h.refreshSession(r.Context(), &session) {
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

// refreshSession validates the account and refreshes its role before authorization.
func (h AuthenticationHandler) refreshSession(ctx context.Context, session *SessionData) bool {
	if !session.LoggedIn {
		return false
	}

	user, valid := h.authenticator.GetValidUser(ctx, domain.UserIdentifier(session.UserIdentifier))
	if !valid {
		h.session.DestroyData(ctx)
		return false
	}
	if session.IsAdmin != user.IsAdmin {
		session.IsAdmin = user.IsAdmin
		h.session.SetData(ctx, *session)
	}
	return true
}

// UserIdMatch checks if the user id in the session matches the user id in the request. If not, the request is aborted.
// It must run after LoggedIn has refreshed the session role.
func (h AuthenticationHandler) UserIdMatch(idParameter string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session := h.session.GetData(r.Context())

			if session.IsAdmin {
				next.ServeHTTP(w, r) // Admins can do everything
				return
			}

			sessionUserId := domain.UserIdentifier(session.UserIdentifier)
			requestUserId := domain.UserIdentifier(domain.Base64UrlDecode(request.Path(r, idParameter)))

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

// RequireNonDefaultPassword prohibits actions if the default admin password is still in use.
func (h AuthenticationHandler) RequireNonDefaultPassword() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if h.authenticator.IsDefaultPasswordInUse(r.Context()) {
				respond.JSON(w, http.StatusForbidden,
					model.Error{Code: http.StatusForbidden, Message: "default admin password must be changed first"})
				return
			}

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
	if slices.Contains(scopes, ScopeAdmin) {
		return false
	}

	// For all other scopes, a logged-in user is sufficient (for now)
	if session.LoggedIn {
		return true
	}

	return false
}
