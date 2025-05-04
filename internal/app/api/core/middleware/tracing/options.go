package tracing

import "time"

// options is a struct that contains options for the tracing middleware.
// It uses the functional options pattern for flexible configuration.
type options struct {
	upstreamReqIdHeader string
	headerIdentifier    string
	contextIdentifier   string
	generateLength      int
	generateCharset     string
	generateSeed        int64
}

// Option is a type that is used to set options for the tracing middleware.
// It implements the functional options pattern.
type Option func(*options)

// WithIdSeed sets the seed for the random request id.
// If no seed is provided, the current timestamp is used.
func WithIdSeed(seed int64) Option {
	return func(o *options) {
		o.generateSeed = seed
	}
}

// WithIdCharset sets the charset that is used to generate a random request id.
// By default, upper-case letters and numbers are used.
func WithIdCharset(charset string) Option {
	return func(o *options) {
		o.generateCharset = charset
	}
}

// WithIdLength specifies the length of generated random ids.
// By default, a length of 8 is used. If the length is 0, no request id will be generated.
func WithIdLength(len int) Option {
	return func(o *options) {
		o.generateLength = len
	}
}

// WithHeaderIdentifier specifies the header name for the request id that is added to the response headers.
// If the identifier is empty, the request id will not be added to the response headers.
func WithHeaderIdentifier(identifier string) Option {
	return func(o *options) {
		o.headerIdentifier = identifier
	}
}

// WithUpstreamHeader sets the upstream header name, that should be used to fetch the request id.
// If no upstream header is found, a random id will be generated if the id-length parameter is set to a value > 0.
func WithUpstreamHeader(header string) Option {
	return func(o *options) {
		o.upstreamReqIdHeader = header
	}
}

// WithContextIdentifier specifies the value-key for the request id that is added to the request context.
// If the identifier is empty, the request id will not be added to the context.
// If the request id is added to the context, it can be retrieved with:
// `id := r.Context().Value(THE-IDENTIFIER).(string)`
func WithContextIdentifier(identifier string) Option {
	return func(o *options) {
		o.contextIdentifier = identifier
	}
}

// newOptions is a function that returns a new options struct with sane default values.
func newOptions(opts ...Option) options {
	o := options{
		headerIdentifier:  "X-Request-Id",
		contextIdentifier: "RequestId",
		generateSeed:      time.Now().UnixNano(),
		generateCharset:   "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		generateLength:    8,
	}

	for _, opt := range opts {
		opt(&o)
	}

	return o
}
