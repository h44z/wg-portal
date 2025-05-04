package logging

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type mockLogger struct {
	messages []string
}

func (m *mockLogger) Debugf(format string, _ ...any) {
	m.messages = append(m.messages, "DEBUG: "+format)
}
func (m *mockLogger) Infof(format string, _ ...any) {
	m.messages = append(m.messages, "INFO: "+format)
}
func (m *mockLogger) Warnf(format string, _ ...any) {
	m.messages = append(m.messages, "WARN: "+format)
}
func (m *mockLogger) Errorf(format string, _ ...any) {
	m.messages = append(m.messages, "ERROR: "+format)
}

func TestMiddleware_Normal(t *testing.T) {
	logger := &mockLogger{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("Hello, World!"))
	})

	middleware := New(WithLogger(logger), WithLevel(LogLevelError)).Handler(handler)
	req := httptest.NewRequest("GET", "http://example.com/foo", nil)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusTeapot {
		t.Errorf("expected status code to be %v, got %v", http.StatusTeapot, status)
	}

	expected := "Hello, World!"
	if rr.Body.String() != expected {
		t.Errorf("expected response body to be %v, got %v", expected, rr.Body.String())
	}

	if len(logger.messages) == 0 {
		t.Errorf("expected log messages, got none")
	}

	if len(logger.messages) != 0 && !strings.Contains(logger.messages[0], "ERROR: GET /foo") {
		t.Errorf("expected log message to contain request info, got %v", logger.messages[0])
	}
}

func TestMiddleware_Extended(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("Hello, World!"))
	})

	middleware := New(WithContextRequestIdKey("requestId"), WithHeaderRequestIdKey("X-Request-Id")).
		Handler(handler)
	req := httptest.NewRequest("GET", "http://example.com/foo", nil)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusTeapot {
		t.Errorf("expected status code to be %v, got %v", http.StatusTeapot, status)
	}

	expected := "Hello, World!"
	if rr.Body.String() != expected {
		t.Errorf("expected response body to be %v, got %v", expected, rr.Body.String())
	}
}

func TestMiddleware_Logger_remoteAddr(t *testing.T) {
	logger := &mockLogger{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("Hello, World!"))
	})

	middleware := New(WithLogger(logger), WithLevel(LogLevelError)).Handler(handler)
	req := httptest.NewRequest("GET", "http://example.com/foo", nil)
	req.RemoteAddr = "xhamster.com:1234"
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

}

func TestMiddleware_Logger_remoteAddrNoPort(t *testing.T) {
	logger := &mockLogger{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("Hello, World!"))
	})

	middleware := New(WithLogger(logger), WithLevel(LogLevelError)).Handler(handler)
	req := httptest.NewRequest("GET", "http://example.com/foo", nil)
	req.RemoteAddr = "xhamster.com"
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

}

func TestMiddleware_Logger_remoteAddrV6(t *testing.T) {
	logger := &mockLogger{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("Hello, World!"))
	})

	middleware := New(WithLogger(logger), WithLevel(LogLevelError)).Handler(handler)
	req := httptest.NewRequest("GET", "http://example.com/foo", nil)
	req.RemoteAddr = "[::1]:4711"
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

}

func TestMiddleware_Logger_remoteAddrV6NoPort(t *testing.T) {
	logger := &mockLogger{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("Hello, World!"))
	})

	middleware := New(WithLogger(logger), WithLevel(LogLevelError)).Handler(handler)
	req := httptest.NewRequest("GET", "http://example.com/foo", nil)
	req.RemoteAddr = "[::1]"
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

}
