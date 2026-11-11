package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/h44z/wg-portal/internal/domain"
)

type stubV0UserAuthenticator struct {
	valid                bool
	defaultPasswordInUse bool
}

func (s stubV0UserAuthenticator) GetValidUser(_ context.Context, id domain.UserIdentifier) (*domain.User, bool) {
	return &domain.User{Identifier: id}, s.valid
}

func (s stubV0UserAuthenticator) IsDefaultPasswordInUse(_ context.Context) bool {
	return s.defaultPasswordInUse
}

func TestAuthenticationHandler_RequireNonDefaultPassword(t *testing.T) {
	session := &testSession{data: SessionData{
		LoggedIn:       true,
		IsAdmin:        true,
		UserIdentifier: "admin@wgportal.local",
	}}

	t.Run("rejects when default password is in use", func(t *testing.T) {
		handler := NewAuthenticationHandler(stubV0UserAuthenticator{valid: true, defaultPasswordInUse: true}, session)

		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v0/interface/all", nil)
		res := httptest.NewRecorder()

		handler.RequireNonDefaultPassword()(next).ServeHTTP(res, req)

		if res.Code != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, res.Code)
		}

		if nextCalled {
			t.Fatal("expected downstream handler not to be called")
		}

		if !strings.Contains(res.Body.String(), "default admin password must be changed first") {
			t.Fatalf("expected error message to mention default admin password, got %q", res.Body.String())
		}
	})

	t.Run("allows when default password is not in use", func(t *testing.T) {
		handler := NewAuthenticationHandler(stubV0UserAuthenticator{valid: true, defaultPasswordInUse: false}, session)

		nextCalled := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v0/interface/all", nil)
		res := httptest.NewRecorder()

		handler.RequireNonDefaultPassword()(next).ServeHTTP(res, req)

		if res.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
		}

		if !nextCalled {
			t.Fatal("expected downstream handler to be called")
		}
	})
}
