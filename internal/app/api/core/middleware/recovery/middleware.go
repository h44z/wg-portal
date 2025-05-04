package recovery

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
)

// Logger is an interface that defines the methods that a logger must implement.
// This allows the logging middleware to be used with different logging libraries.
type Logger interface {
	// Errorf logs a message at error level.
	Errorf(format string, args ...any)
}

// Middleware is a type that creates a new recovery middleware. The recovery middleware
// recovers from panics and returns an Internal Server Error response. This middleware should
// be the first middleware in the middleware chain, so that it can recover from panics in other
// middlewares.
type Middleware struct {
	o options
}

// New returns a new recovery middleware with the provided options.
func New(opts ...Option) *Middleware {
	o := newOptions(opts...)

	m := &Middleware{
		o: o,
	}

	return m
}

// Handler returns the recovery middleware handler.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()

				realErr, ok := err.(error)
				if !ok {
					realErr = fmt.Errorf("%v", err)
				}

				// Check for a broken connection, as it is not really a
				// condition that warrants a panic stack trace.
				brokenPipe := isBrokenPipeError(realErr)

				// Log the error and stack trace
				if m.o.logCallback != nil {
					m.o.logCallback(realErr, stack, brokenPipe)
				}

				switch {
				case brokenPipe && m.o.brokenPipeCallback != nil:
					m.o.brokenPipeCallback(realErr, stack, w, r)
				case !brokenPipe && m.o.errCallback != nil:
					m.o.errCallback(realErr, stack, w, r)
				default:
					// no callback set, simply recover and do nothing...
				}
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func addPrefix(o options, message string) string {
	if o.defaultLogPrefix != "" {
		return o.defaultLogPrefix + " " + message
	}
	return message
}

// defaultErrCallback is the default error callback function for the recovery middleware.
// It writes a JSON response with an Internal Server Error status code. If the exposeStackTrace option is
// enabled, the stack trace is included in the response.
func getDefaultErrCallback(o options) func(err error, stack []byte, w http.ResponseWriter, r *http.Request) {
	return func(err error, stack []byte, w http.ResponseWriter, r *http.Request) {
		responseBody := map[string]interface{}{
			"error": "Internal Server Error",
		}
		if o.exposeStackTrace && len(stack) > 0 {
			responseBody["stack"] = string(stack)
		}

		jsonBody, _ := json.Marshal(responseBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(jsonBody)
	}
}

// getDefaultLogCallback is the default log callback function for the recovery middleware.
// It logs the error and stack trace using the structured slog logger or the provided logger in Error level.
func getDefaultLogCallback(o options) func(error, []byte, bool) {
	return func(err error, stack []byte, brokenPipe bool) {
		if brokenPipe {
			return // by default, ignore broken pipe errors
		}

		switch {
		case o.useSlog:
			slog.Error(addPrefix(o, err.Error()), "stack", string(stack))
		case o.logger != nil:
			o.logger.Errorf(fmt.Sprintf("%s; stacktrace=%s", addPrefix(o, err.Error()), string(stack)))
		default:
			// no logger set, do nothing...
		}
	}
}

func isBrokenPipeError(err error) bool {
	var syscallErr *os.SyscallError
	if errors.As(err, &syscallErr) {
		errMsg := strings.ToLower(syscallErr.Err.Error())
		if strings.Contains(errMsg, "broken pipe") ||
			strings.Contains(errMsg, "connection reset by peer") {
			return true
		}
	}

	return false
}
