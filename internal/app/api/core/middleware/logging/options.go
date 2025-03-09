package logging

// options is a struct that contains options for the logging middleware.
// It uses the functional options pattern for flexible configuration.
type options struct {
	logLevel LogLevel
	logger   Logger
	prefix   string

	contextRequestIdKey string
	headerRequestIdKey  string
}

// Option is a type that is used to set options for the logging middleware.
// It implements the functional options pattern.
type Option func(*options)

// WithLevel is a method that sets the log level for the logging middleware.
// Possible values are LogLevelDebug, LogLevelInfo, LogLevelWarn, and LogLevelError.
// The default value is LogLevelInfo.
func WithLevel(level LogLevel) Option {
	return func(o *options) {
		o.logLevel = level
	}
}

// WithPrefix is a method that sets the prefix for the logging middleware.
// If a prefix is set, it will be prepended to each log message. A space will
// be added between the prefix and the log message.
// The default value is an empty string.
func WithPrefix(prefix string) Option {
	return func(o *options) {
		o.prefix = prefix
	}
}

// WithContextRequestIdKey is a method that sets the key for the request ID in the
// request context. If a key is set, the logging middleware will use this key to
// retrieve the request ID from the request context.
// The default value is an empty string, meaning the request ID will not be logged.
func WithContextRequestIdKey(key string) Option {
	return func(o *options) {
		o.contextRequestIdKey = key
	}
}

// WithHeaderRequestIdKey is a method that sets the key for the request ID in the
// request headers. If a key is set, the logging middleware will use this key to
// retrieve the request ID from the request headers.
// The default value is an empty string, meaning the request ID will not be logged.
func WithHeaderRequestIdKey(key string) Option {
	return func(o *options) {
		o.headerRequestIdKey = key
	}
}

// WithLogger is a method that sets the logger for the logging middleware.
// If a logger is set, the logging middleware will use this logger to log messages.
// The default logger is the structured slog logger.
func WithLogger(logger Logger) Option {
	return func(o *options) {
		o.logger = logger
	}
}

// newOptions is a function that returns a new options struct with sane default values.
func newOptions(opts ...Option) options {
	o := options{
		logLevel:            LogLevelInfo,
		logger:              nil,
		prefix:              "",
		contextRequestIdKey: "",
	}

	for _, opt := range opts {
		opt(&o)
	}

	return o
}
