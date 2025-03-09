package logging

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// LogLevel is an enumeration of the different log levels.
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// Logger is an interface that defines the methods that a logger must implement.
// This allows the logging middleware to be used with different logging libraries.
type Logger interface {
	// Debugf logs a message at debug level.
	Debugf(format string, args ...any)
	// Infof logs a message at info level.
	Infof(format string, args ...any)
	// Warnf logs a message at warn level.
	Warnf(format string, args ...any)
	// Errorf logs a message at error level.
	Errorf(format string, args ...any)
}

// Middleware is a type that creates a new logging middleware. The logging middleware
// logs information about each request.
type Middleware struct {
	o options
}

// New returns a new logging middleware with the provided options.
func New(opts ...Option) *Middleware {
	o := newOptions(opts...)

	m := &Middleware{
		o: o,
	}

	return m
}

// Handler returns the logging middleware handler.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := newWriterWrapper(w)
		start := time.Now()
		defer func() {
			info := m.extractInfoMap(r, start, ww)

			if m.o.logger == nil {
				msg, args := m.buildSlogMessageAndArguments(info)
				m.logMsg(msg, args...)
			} else {
				msg := m.buildNormalLogMessage(info)
				m.logMsg(msg)
			}
		}()

		next.ServeHTTP(ww, r)
	})
}

func (m *Middleware) extractInfoMap(r *http.Request, start time.Time, ww *writerWrapper) map[string]any {
	info := make(map[string]any)

	info["method"] = r.Method
	info["path"] = r.URL.Path
	info["protocol"] = r.Proto
	info["clientIP"] = r.Header.Get("X-Forwarded-For")
	if info["clientIP"] == "" {
		// If the X-Forwarded-For header is not set, use the remote address without the port number.
		lastColonIndex := strings.LastIndex(r.RemoteAddr, ":")
		switch lastColonIndex {
		case -1:
			info["clientIP"] = r.RemoteAddr
		default:
			info["clientIP"] = r.RemoteAddr[:lastColonIndex]
		}
	}
	info["userAgent"] = r.UserAgent()
	info["referer"] = r.Header.Get("Referer")
	info["duration"] = time.Since(start).String()
	info["status"] = ww.StatusCode
	info["dataLength"] = ww.WrittenBytes

	if m.o.headerRequestIdKey != "" {
		info["headerRequestId"] = r.Header.Get(m.o.headerRequestIdKey)
	}
	if m.o.contextRequestIdKey != "" {
		info["contextRequestId"], _ = r.Context().Value(m.o.contextRequestIdKey).(string)
	}

	return info
}

func (m *Middleware) buildNormalLogMessage(info map[string]any) string {
	switch {
	case info["headerRequestId"] != nil && info["contextRequestId"] != nil:
		return fmt.Sprintf("%s %s %s - %d %d - %s - %s %s %s - rid=%s ctx=%s",
			info["method"], info["path"], info["protocol"],
			info["status"], info["dataLength"],
			info["duration"],
			info["clientIP"], info["userAgent"], info["referer"],
			info["headerRequestId"], info["contextRequestId"])
	case info["headerRequestId"] != nil:
		return fmt.Sprintf("%s %s %s - %d %d - %s - %s %s %s - rid=%s",
			info["method"], info["path"], info["protocol"],
			info["status"], info["dataLength"],
			info["duration"],
			info["clientIP"], info["userAgent"], info["referer"],
			info["headerRequestId"])
	case info["contextRequestId"] != nil:
		return fmt.Sprintf("%s %s %s - %d %d - %s - %s %s %s - ctx=%s",
			info["method"], info["path"], info["protocol"],
			info["status"], info["dataLength"],
			info["duration"],
			info["clientIP"], info["userAgent"], info["referer"],
			info["contextRequestId"])
	default:
		return fmt.Sprintf("%s %s %s - %d %d - %s - %s %s %s",
			info["method"], info["path"], info["protocol"],
			info["status"], info["dataLength"],
			info["duration"],
			info["clientIP"], info["userAgent"], info["referer"])
	}
}

func (m *Middleware) buildSlogMessageAndArguments(info map[string]any) (message string, args []any) {
	message = fmt.Sprintf("%s %s", info["method"], info["path"])

	// Use a fixed order for the keys, so that the message is always the same.
	// Skip method and path as they are already in the message.
	keys := []string{
		"protocol",
		"status",
		"dataLength",
		"duration",
		"clientIP",
		"userAgent",
		"referer",
		"headerRequestId",
		"contextRequestId",
	}
	for _, k := range keys {
		if v, ok := info[k]; ok {
			args = append(args, k, v) // only add key, value if it exists
		}
	}

	return
}

func (m *Middleware) addPrefix(message string) string {
	if m.o.prefix != "" {
		return m.o.prefix + " " + message
	}
	return message
}

func (m *Middleware) logMsg(message string, args ...any) {
	message = m.addPrefix(message)

	if m.o.logger != nil {
		switch m.o.logLevel {
		case LogLevelDebug:
			m.o.logger.Debugf(message, args...)
		case LogLevelInfo:
			m.o.logger.Infof(message, args...)
		case LogLevelWarn:
			m.o.logger.Warnf(message, args...)
		case LogLevelError:
			m.o.logger.Errorf(message, args...)
		default:
			m.o.logger.Infof(message, args...)
		}
	} else {
		switch m.o.logLevel {
		case LogLevelDebug:
			slog.Debug(message, args...)
		case LogLevelInfo:
			slog.Info(message, args...)
		case LogLevelWarn:
			slog.Warn(message, args...)
		case LogLevelError:
			slog.Error(message, args...)
		default:
			slog.Info(message, args...)
		}
	}
}
