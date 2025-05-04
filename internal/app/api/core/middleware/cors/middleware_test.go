package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddleware_New(t *testing.T) {
	m := New(WithAllowedOrigins("*"))

	if len(m.varyHeaders) == 0 {
		t.Errorf("expected vary headers to be populated, got %v", m.varyHeaders)
	}
	if !m.allOrigins {
		t.Errorf("expected allOrigins to be true, got %v", m.allOrigins)
	}
}

func TestMiddleware_Handler_normal(t *testing.T) {
	m := New(WithAllowedOrigins("http://example.com"))

	handler := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	req.Header.Set("Origin", "http://example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status code 200, got %d", w.Result().StatusCode)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Errorf("expected Access-Control-Allow-Origin to be 'http://example.com', got %s",
			w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestMiddleware_Handler_preflight(t *testing.T) {
	m := New(WithAllowedOrigins("http://example.com"))

	handler := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "http://example.com", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Result().StatusCode != http.StatusNoContent {
		t.Errorf("expected status code 204, got %d", w.Result().StatusCode)
	}

	if w.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Errorf("expected Access-Control-Allow-Origin to be 'http://example.com', got %s",
			w.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestMiddleware_originAllowed(t *testing.T) {
	m := New(WithAllowedOrigins("http://example.com"))

	if !m.originAllowed("http://example.com") {
		t.Errorf("expected origin 'http://example.com' to be allowed")
	}

	if m.originAllowed("http://notallowed.com") {
		t.Errorf("expected origin 'http://notallowed.com' to be not allowed")
	}
}

func TestMiddleware_methodAllowed(t *testing.T) {
	m := New(WithAllowedMethods(http.MethodGet, http.MethodPost))

	if !m.methodAllowed(http.MethodGet) {
		t.Errorf("expected method 'GET' to be allowed")
	}

	if m.methodAllowed(http.MethodDelete) {
		t.Errorf("expected method 'DELETE' to be not allowed")
	}
}

func TestMiddleware_headersAllowed(t *testing.T) {
	m := New(WithAllowedHeaders("Content-Type", "Authorization"))

	if !m.headersAllowed("content-type, authorization") {
		t.Errorf("expected headers 'Content-Type, Authorization' to be allowed")
	}

	if m.headersAllowed("x-custom-header") {
		t.Errorf("expected header 'X-Custom-Header' to be not allowed")
	}
}
