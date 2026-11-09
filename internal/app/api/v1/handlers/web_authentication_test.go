package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/h44z/wg-portal/internal/domain"
)

type stubUserAuthenticator struct {
	valid                bool
	defaultPasswordInUse bool
}

func (s stubUserAuthenticator) IsUserValid(ctx context.Context, id domain.UserIdentifier) bool {
	return s.valid
}

func (s stubUserAuthenticator) IsDefaultPasswordInUse(ctx context.Context) bool {
	return s.defaultPasswordInUse
}

type stubUserRepository struct {
	user *domain.User
	err  error
}

func (s stubUserRepository) GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.user, nil
}

func TestAuthenticationHandler_LoggedInRejectsDisabledOrLockedUser(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*domain.User)
	}{
		{
			name: "disabled user",
			configure: func(user *domain.User) {
				user.Disabled = &[]time.Time{time.Now()}[0]
			},
		},
		{
			name: "locked user",
			configure: func(user *domain.User) {
				user.Locked = &[]time.Time{time.Now()}[0]
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &domain.User{
				Identifier: "test-user",
				ApiToken:   "token",
			}
			tt.configure(user)

			handler := NewAuthenticationHandler(stubUserAuthenticator{valid: false}, stubUserRepository{user: user})
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.SetBasicAuth(string(user.Identifier), user.ApiToken)
			rr := httptest.NewRecorder()

			handler.LoggedIn()(next).ServeHTTP(rr, req)

			if rr.Code != http.StatusForbidden {
				t.Fatalf("expected status %d, got %d", http.StatusForbidden, rr.Code)
			}

			if nextCalled {
				t.Fatal("expected downstream handler not to be called")
			}

			if !strings.Contains(rr.Body.String(), "account disabled or locked") {
				t.Fatalf("expected error message to mention disabled or locked, got %q", rr.Body.String())
			}
		})
	}
}

func TestAuthenticationHandler_LoggedInAllowsValidUser(t *testing.T) {
	user := &domain.User{
		Identifier: "test-user",
		ApiToken:   "token",
	}

	handler := NewAuthenticationHandler(stubUserAuthenticator{valid: true}, stubUserRepository{user: user})
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth(string(user.Identifier), user.ApiToken)
	rr := httptest.NewRecorder()

	handler.LoggedIn()(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	if !nextCalled {
		t.Fatal("expected downstream handler to be called")
	}
}

func TestAuthenticationHandler_LoggedInRejectsDefaultPassword(t *testing.T) {
	user := &domain.User{
		Identifier: "test-user",
		ApiToken:   "token",
	}

	handler := NewAuthenticationHandler(stubUserAuthenticator{valid: true, defaultPasswordInUse: true}, stubUserRepository{user: user})
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetBasicAuth(string(user.Identifier), user.ApiToken)
	rr := httptest.NewRecorder()

	handler.LoggedIn()(next).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, rr.Code)
	}

	if nextCalled {
		t.Fatal("expected downstream handler not to be called")
	}

	if !strings.Contains(rr.Body.String(), "default admin password must be changed first") {
		t.Fatalf("expected error message to mention default password, got %q", rr.Body.String())
	}
}
