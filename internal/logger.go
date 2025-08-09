package internal

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// GetLoggingHandler initializes a slog.Handler based on the provided logging level and format options.
func GetLoggingHandler(level string, pretty, json bool) slog.Handler {
	var logLevel = new(slog.LevelVar)

	switch strings.ToLower(level) {
	case "trace", "debug":
		logLevel.Set(slog.LevelDebug)
	case "info", "information":
		logLevel.Set(slog.LevelInfo)
	case "warn", "warning":
		logLevel.Set(slog.LevelWarn)
	case "error":
		logLevel.Set(slog.LevelError)
	default:
		logLevel.Set(slog.LevelInfo)
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	// send everything to stderr as suggested in https://www.gnu.org/software/libc/manual/html_node/Standard-Streams.html
	output := os.Stderr

	var handler slog.Handler
	switch {
	case json:
		handler = slog.NewJSONHandler(output, opts)
	case pretty:
		handler = NewPrettyHandler(output, opts)
	default:
		handler = slog.NewTextHandler(output, opts)
	}

	return handler
}

// SetupLogging initializes the global logger with the given level and format
func SetupLogging(level string, pretty, json bool) {
	handler := GetLoggingHandler(level, pretty, json)

	logger := slog.New(handler)

	slog.SetDefault(logger)
}

// PrettyHandler is a slog.Handler that formats log records in a human-readable way.
// It mimics the behavior of the slog.Default() handler.
type PrettyHandler struct {
	opts       slog.HandlerOptions
	prefix     string // preformatted group names followed by a dot
	preformat  string // preformatted Attrs, with an initial space
	timeFormat string

	mu sync.Mutex
	w  io.Writer
}

// NewPrettyHandler creates a new PrettyHandler.
func NewPrettyHandler(w io.Writer, opts *slog.HandlerOptions) *PrettyHandler {
	h := &PrettyHandler{w: w}
	if opts != nil {
		h.opts = *opts
	}
	if h.opts.ReplaceAttr == nil {
		h.opts.ReplaceAttr = func(_ []string, a slog.Attr) slog.Attr { return a }
	}

	h.timeFormat = "2006/01/02 15:04:05"

	return h
}

// Enabled reports whether the handler handles records at the given level.
func (h *PrettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

// WithGroup returns a new Handler with the given group appended to the handler's
func (h *PrettyHandler) WithGroup(name string) slog.Handler {
	return &PrettyHandler{
		w:         h.w,
		opts:      h.opts,
		preformat: h.preformat,
		prefix:    h.prefix + name + ".",
	}
}

// WithAttrs returns a new Handler whose attributes consist of the handler's
func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var buf []byte
	for _, a := range attrs {
		buf = h.appendAttr(buf, h.prefix, a)
	}
	return &PrettyHandler{
		w:         h.w,
		opts:      h.opts,
		prefix:    h.prefix,
		preformat: h.preformat + string(buf),
	}
}

// Handle formats its argument Record as a single line of text ending in a newline.
func (h *PrettyHandler) Handle(_ context.Context, r slog.Record) error {
	var buf []byte
	if !r.Time.IsZero() {
		buf = r.Time.AppendFormat(buf, h.timeFormat)
		buf = append(buf, ' ')
	}

	// Make sure that each level has the same length.
	// The shortest level is "INFO", thus we add a space to the end of the level string
	levText := (r.Level.String() + " ")[0:5]

	buf = append(buf, levText...)
	buf = append(buf, ' ')
	if h.opts.AddSource && r.PC != 0 {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		buf = append(buf, f.File...)
		buf = append(buf, ':')
		buf = strconv.AppendInt(buf, int64(f.Line), 10)
		buf = append(buf, ' ')
	}
	buf = append(buf, r.Message...)
	buf = append(buf, h.preformat...)
	r.Attrs(func(a slog.Attr) bool {
		buf = h.appendAttr(buf, h.prefix, a)
		return true
	})
	buf = append(buf, '\n')
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf)
	return err
}

func (h *PrettyHandler) appendAttr(buf []byte, prefix string, a slog.Attr) []byte {
	if a.Equal(slog.Attr{}) {
		return buf
	}
	if a.Value.Kind() != slog.KindGroup {
		buf = append(buf, ' ')
		buf = append(buf, prefix...)
		buf = append(buf, a.Key...)
		buf = append(buf, '=')
		return fmt.Appendf(buf, "%v", a.Value.Any())
	}
	// Group
	if a.Key != "" {
		prefix += a.Key + "."
	}
	for _, a := range a.Value.Group() {
		buf = h.appendAttr(buf, prefix, a)
	}
	return buf
}
