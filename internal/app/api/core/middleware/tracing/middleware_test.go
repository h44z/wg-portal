package tracing

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const defaultLength = 8
const upstreamHeaderValue = "upstream-id"

func TestMiddleware_Handler_WithUpstreamHeader(t *testing.T) {
	m := New(WithUpstreamHeader("X-Upstream-Id"))
	handler := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqId := r.Header.Get("X-Upstream-Id")
		if reqId != upstreamHeaderValue {
			t.Errorf("expected upstream request id to be 'upstream-id', got %s", reqId)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Upstream-Id", upstreamHeaderValue)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Request-Id") != upstreamHeaderValue {
		t.Errorf("expected X-Request-Id header to be set in the response")
	}
}

func TestMiddleware_Handler_GenerateNewId(t *testing.T) {
	idLen := 18
	m := New(WithIdLength(idLen))
	handler := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqId := w.Header().Get("X-Request-Id")
		if len(reqId) != 18 {
			t.Errorf("expected generated request id length to be %d, got %d", idLen, len(reqId))
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Request-Id") == "" || len(rr.Header().Get("X-Request-Id")) != idLen {
		t.Errorf("expected X-Request-Id header to be set in the response")
	}
}

func TestMiddleware_Handler_SetContextValue(t *testing.T) {
	m := New()
	handler := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqId := r.Context().Value("RequestId").(string)
		if reqId == "" || len(reqId) != defaultLength {
			t.Errorf("expected context request id to be set, got empty string")
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
}

func TestMiddleware_Handler_SetCustomContextValue(t *testing.T) {
	m := New(WithContextIdentifier("Custom-Id"))
	handler := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqId := r.Context().Value("Custom-Id").(string)
		if reqId == "" || len(reqId) != defaultLength {
			t.Errorf("expected context request id to be set, got empty string")
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
}

func TestMiddleware_Handler_NoIdGenerated(t *testing.T) {
	m := New(WithIdLength(0))
	handler := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqId := w.Header().Get("X-Request-Id")
		if reqId != "" {
			t.Errorf("expected no request id to be generated, got %s", reqId)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
}

func TestMiddleware_Handler_NoIdHeaderSet(t *testing.T) {
	m := New(WithHeaderIdentifier(""))
	handler := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqId := w.Header().Get("X-Request-Id")
		if reqId != "" {
			t.Errorf("expected no request id to be generated, got %s", reqId)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
}

func TestMiddleware_Handler_NoIdContextSet(t *testing.T) {
	m := New(WithHeaderIdentifier(""))
	handler := m.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqId := r.Context().Value("Request-Id")
		if reqId != nil {
			t.Errorf("expected no context request id to be set, got %v", reqId)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
}
