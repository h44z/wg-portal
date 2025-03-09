package recovery

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type mockLogger struct{}

func (m *mockLogger) Errorf(_ string, _ ...any) {}

func TestMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		options        []Option
		panicSimulator func()
		expectedStatus int
		expectedBody   string
		expectStack    bool
	}{
		{
			name:    "default behavior",
			options: []Option{},
			panicSimulator: func() {
				panic(errors.New("test panic"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   `{"error":"Internal Server Error"}`,
		},
		{
			name: "custom error callback",
			options: []Option{
				WithErrCallback(func(err error, stack []byte, w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusTeapot)
					w.Write([]byte("custom error"))
				}),
			},
			panicSimulator: func() {
				panic(errors.New("test panic"))
			},
			expectedStatus: http.StatusTeapot,
			expectedBody:   "custom error",
		},
		{
			name: "broken pipe error",
			options: []Option{
				WithBrokenPipeCallback(func(err error, stack []byte, w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusServiceUnavailable)
					w.Write([]byte("broken pipe"))
				}),
			},
			panicSimulator: func() {
				panic(&os.SyscallError{Err: errors.New("broken pipe")})
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   "broken pipe",
		},
		{
			name:    "default callback broken pipe error",
			options: nil,
			panicSimulator: func() {
				panic(&os.SyscallError{Err: errors.New("broken pipe")})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "",
		},
		{
			name:    "default callback normal error",
			options: nil,
			panicSimulator: func() {
				panic("something went wrong")
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "{\"error\":\"Internal Server Error\"}",
		},
		{
			name: "default callback with stack trace",
			options: []Option{
				WithExposeStackTrace(true),
			},
			panicSimulator: func() {
				panic("something went wrong")
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "\"stack\":",
			expectStack:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := New(tt.options...).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.panicSimulator()
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %v, got %v", tt.expectedStatus, rr.Code)
			}
			if !tt.expectStack && rr.Body.String() != tt.expectedBody {
				t.Errorf("expected body %v, got %v", tt.expectedBody, rr.Body.String())
			}
			if tt.expectStack && !strings.Contains(rr.Body.String(), tt.expectedBody) {
				t.Errorf("expected body to contain %v, got %v", tt.expectedBody, rr.Body.String())
			}
		})
	}
}

func TestIsBrokenPipeError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "broken pipe error",
			err:      &os.SyscallError{Err: errors.New("broken pipe")},
			expected: true,
		},
		{
			name:     "connection reset by peer error",
			err:      &os.SyscallError{Err: errors.New("connection reset by peer")},
			expected: true,
		},
		{
			name:     "other error",
			err:      errors.New("other error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBrokenPipeError(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
