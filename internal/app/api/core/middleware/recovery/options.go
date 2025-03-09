package recovery

import "net/http"

// options is a struct that contains options for the recovery middleware.
// It uses the functional options pattern for flexible configuration.
type options struct {
	logger  Logger
	useSlog bool

	errCallbackOverride        bool
	errCallback                func(err error, stack []byte, w http.ResponseWriter, r *http.Request)
	brokenPipeCallbackOverride bool
	brokenPipeCallback         func(err error, stack []byte, w http.ResponseWriter, r *http.Request)

	exposeStackTrace    bool
	defaultLogPrefix    string
	logCallbackOverride bool
	logCallback         func(err error, stack []byte, brokenPipe bool)
}

// Option is a type that is used to set options for the recovery middleware.
// It implements the functional options pattern.
type Option func(*options)

// WithErrCallback sets the error callback function for the recovery middleware.
// The error callback function is called when a panic is recovered by the middleware.
// This function completely overrides the default behavior of the middleware. It is the
// responsibility of the user to handle the error and write a response to the client.
//
// Ensure that this function does not panic, as it will be called in a deferred function!
func WithErrCallback(fn func(err error, stack []byte, w http.ResponseWriter, r *http.Request)) Option {
	return func(o *options) {
		o.errCallback = fn
		o.errCallbackOverride = true
	}
}

// WithBrokenPipeCallback sets the broken pipe callback function for the recovery middleware.
// The broken pipe callback function is called when a broken pipe error is recovered by the middleware.
// This function completely overrides the default behavior of the middleware. It is the responsibility
// of the user to handle the error and write a response to the client.
//
// Ensure that this function does not panic, as it will be called in a deferred function!
func WithBrokenPipeCallback(fn func(err error, stack []byte, w http.ResponseWriter, r *http.Request)) Option {
	return func(o *options) {
		o.brokenPipeCallback = fn
		o.brokenPipeCallbackOverride = true
	}
}

// WithLogCallback sets the log callback function for the recovery middleware.
// The log callback function is called when a panic is recovered by the middleware.
// This function allows the user to log the error and stack trace. The default behavior is to log
// the error and stack trace in Error level.
// This function completely overrides the default behavior of the middleware.
//
// Ensure that this function does not panic, as it will be called in a deferred function!
func WithLogCallback(fn func(err error, stack []byte, brokenPipe bool)) Option {
	return func(o *options) {
		o.logCallback = fn
		o.logCallbackOverride = true
	}
}

// WithLogger is a method that sets the logger for the logging middleware.
// If a logger is set, the logging middleware will use this logger to log messages.
// The default logger is the structured slog logger, see WithSlog.
func WithLogger(logger Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

// WithSlog is a method that sets whether the recovery middleware should use the structured slog logger.
// If set to true, the middleware will use the structured slog logger. If set to false, the middleware
// will not use any logger unless one is explicitly set with the WithLogger option.
// The default value is true.
func WithSlog(useSlog bool) Option {
	return func(o *options) {
		o.useSlog = useSlog
	}
}

// WithDefaultLogPrefix is a method that sets the default log prefix for the recovery middleware.
// If a default log prefix is set and the default log callback is used, the prefix will be prepended
// to each log message. A space will be added between the prefix and the log message.
// The default value is an empty string.
func WithDefaultLogPrefix(defaultLogPrefix string) Option {
	return func(o *options) {
		o.defaultLogPrefix = defaultLogPrefix
	}
}

// WithExposeStackTrace is a method that sets whether the stack trace should be exposed in the response.
// If set to true, the stack trace will be included in the response body. If set to false, the stack trace
// will not be included in the response body. This only applies to the default error callback.
// The default value is false.
func WithExposeStackTrace(exposeStackTrace bool) Option {
	return func(o *options) {
		o.exposeStackTrace = exposeStackTrace
	}
}

// newOptions is a function that returns a new options struct with sane default values.
func newOptions(opts ...Option) options {
	o := options{
		logger:             nil,
		useSlog:            true,
		errCallback:        nil,
		brokenPipeCallback: nil, // by default, ignore broken pipe errors
		exposeStackTrace:   false,
		defaultLogPrefix:   "",
		logCallback:        nil,
	}

	for _, opt := range opts {
		opt(&o)
	}

	if o.errCallback == nil && !o.errCallbackOverride {
		o.errCallback = getDefaultErrCallback(o)
	}
	if o.logCallback == nil && !o.logCallbackOverride {
		o.logCallback = getDefaultLogCallback(o)
	}

	return o
}
