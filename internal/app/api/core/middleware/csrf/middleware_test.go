package csrf

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fedor-git/wg-portal-2/internal/app/api/core/request"
)

func TestMiddleware_Handler(t *testing.T) {
	sessionToken := "stored-token"
	sessionReader := func(r *http.Request) string {
		return sessionToken
	}
	sessionWriter := func(r *http.Request, token string) {
		sessionToken = token
	}
	m := New(sessionReader, sessionWriter)

	handler := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		method     string
		token      string
		wantStatus int
	}{
		{"ValidToken", "POST", "stored-token", http.StatusOK},
		{"ValidToken2", "PUT", "stored-token", http.StatusOK},
		{"ValidToken3", "GET", "stored-token", http.StatusOK},
		{"InvalidToken", "POST", "invalid-token", http.StatusForbidden},
		{"IgnoredMethod", "GET", "", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/", nil)
			req.Header.Set("X-CSRF-TOKEN", tt.token)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.wantStatus {
				t.Errorf("Handler() status = %d, want %d", status, tt.wantStatus)
			}
		})
	}
}

func TestMiddleware_RefreshToken(t *testing.T) {
	sessionToken := ""
	sessionReader := func(r *http.Request) string {
		return sessionToken
	}
	sessionWriter := func(r *http.Request, token string) {
		sessionToken = token
	}
	m := New(sessionReader, sessionWriter)

	handler := m.RefreshToken(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := GetToken(r.Context())
		if token == "" {
			t.Errorf("RefreshToken() did not set token in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("RefreshToken() status = %d, want %d", status, http.StatusOK)
	}

	if sessionToken == "" {
		t.Errorf("RefreshToken() did not set token in session")
	}
}

func TestMiddleware_RefreshToken_chained(t *testing.T) {
	sessionToken := ""
	tokenWrites := 0
	sessionReader := func(r *http.Request) string {
		return sessionToken
	}
	sessionWriter := func(r *http.Request, token string) {
		sessionToken = token
		tokenWrites++
	}
	m := New(sessionReader, sessionWriter)

	handler := m.RefreshToken(m.RefreshToken(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := GetToken(r.Context())
		if token == "" {
			t.Errorf("RefreshToken() did not set token in context")
		}
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("POST", "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("RefreshToken() status = %d, want %d", status, http.StatusOK)
	}

	if sessionToken == "" {
		t.Errorf("RefreshToken() did not set token in session")
	}

	if tokenWrites != 1 {
		t.Errorf("RefreshToken() wrote token to session more than once: %d", tokenWrites)
	}
}

func TestMiddleware_RefreshToken_Handler(t *testing.T) {
	sessionToken := ""
	sessionReader := func(r *http.Request) string {
		return sessionToken
	}
	sessionWriter := func(r *http.Request, token string) {
		sessionToken = token
	}
	m := New(sessionReader, sessionWriter)

	// simulate two requests: first one GET request with the RefreshToken handler, the next one is a PUT request with
	// the token from the first request added as X-CSRF-TOKEN header

	// first request
	retrievedToken := ""
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	handler := m.RefreshToken(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		retrievedToken = GetToken(r.Context())
		if retrievedToken == "" {
			t.Errorf("RefreshToken() did not set token in context")
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusAccepted {
		t.Errorf("Handler() status = %d, want %d", status, http.StatusAccepted)
	}
	if retrievedToken == "" {
		t.Errorf("no token retrieved")
	}
	if retrievedToken != sessionToken {
		t.Errorf("token in context does not match token in session")
	}

	// second request
	req = httptest.NewRequest("PUT", "/", nil)
	req.Header.Set("X-CSRF-TOKEN", retrievedToken)
	rr = httptest.NewRecorder()
	handler = m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler() status = %d, want %d", status, http.StatusOK)
	}
}

func TestMiddleware_Handler_FormBody(t *testing.T) {
	sessionToken := "stored-token"
	sessionReader := func(r *http.Request) string {
		return sessionToken
	}
	sessionWriter := func(r *http.Request, token string) {
		sessionToken = token
	}
	m := New(sessionReader, sessionWriter)

	handler := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyData, err := request.BodyString(r)
		if err != nil {
			t.Errorf("Handler() error = %v, want nil", err)
		}
		// ensure that the body is empty - ParseForm() should have been called before by the CSRF middleware
		if bodyData != "" {
			t.Errorf("Handler() bodyData = %s, want empty", bodyData)
		}

		if r.FormValue("_csrf") != "stored-token" {
			t.Errorf("Handler() _csrf = %s, want %s", r.FormValue("_csrf"), "stored-token")
		}

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Form = make(map[string][]string)
	req.Form.Add("_csrf", "stored-token")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler() status = %d, want %d", status, http.StatusOK)
	}
}

func TestMiddleware_Handler_FormBodyAvailable(t *testing.T) {
	sessionToken := "stored-token"
	sessionReader := func(r *http.Request) string {
		return sessionToken
	}
	sessionWriter := func(r *http.Request, token string) {
		sessionToken = token
	}
	m := New(sessionReader, sessionWriter)

	handler := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyData, err := request.BodyString(r)
		if err != nil {
			t.Errorf("Handler() error = %v, want nil", err)
		}
		// ensure that the body is not empty, as the CSRF middleware should not have read the body
		if bodyData != "the original body" {
			t.Errorf("Handler() bodyData = %s, want %s", bodyData, "the original body")
		}

		// check if the token is available in the form values (from query parameters)
		if r.FormValue("_csrf") != "stored-token" {
			t.Errorf("Handler() _csrf = %s, want %s", r.FormValue("_csrf"), "stored-token")
		}

		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/?_csrf=stored-token", nil)
	req.Header.Set("Content-Type", "text/plain")
	req.Body = io.NopCloser(strings.NewReader("the original body"))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler() status = %d, want %d", status, http.StatusOK)
	}
}
