package cors

import (
	"net/http"
	"strings"
)

type void struct{}

// options is a struct that contains options for the CORS middleware.
// It uses the functional options pattern for flexible configuration.
type options struct {
	allowedOrigins        []string   // origins without wildcards
	allowedOriginPatterns []wildcard // origins with wildcards
	allowedMethods        []string
	allowedHeaders        map[string]void
	exposedHeaders        []string // these are in addition to the CORS-safelisted response headers
	allowCredentials      bool
	allowPrivateNetworks  bool
	maxAge                int
}

// Option is a type that is used to set options for the CORS middleware.
// It implements the functional options pattern.
type Option func(*options)

// WithAllowedOrigins sets the allowed origins for the CORS middleware.
// If the special "*" value is present in the list, all origins will be allowed.
// An origin may contain a wildcard (*) to replace 0 or more characters
// (i.e.: http://*.domain.com). Usage of wildcards implies a small performance penalty.
// Only one wildcard can be used per origin.
// By default, all origins are allowed (*).
func WithAllowedOrigins(origins ...string) Option {
	return func(o *options) {
		o.allowedOrigins = nil
		o.allowedOriginPatterns = nil

		for _, origin := range origins {
			if len(origin) > 1 && strings.Contains(origin, "*") {
				o.allowedOriginPatterns = append(
					o.allowedOriginPatterns,
					newWildcard(origin),
				)
			} else {
				o.allowedOrigins = append(o.allowedOrigins, origin)
			}
		}
	}
}

// WithAllowedMethods sets the allowed methods for the CORS middleware.
// By default, all methods are allowed (*).
func WithAllowedMethods(methods ...string) Option {
	return func(o *options) {
		o.allowedMethods = methods
	}
}

// WithAllowedHeaders sets the allowed headers for the CORS middleware.
// By default, all headers are allowed (*).
func WithAllowedHeaders(headers ...string) Option {
	return func(o *options) {
		o.allowedHeaders = make(map[string]void)

		for _, header := range headers {
			// allowed headers are always checked in lowercase
			o.allowedHeaders[strings.ToLower(header)] = void{}
		}
	}
}

// WithExposedHeaders sets the exposed headers for the CORS middleware.
// By default, no headers are exposed.
func WithExposedHeaders(headers ...string) Option {
	return func(o *options) {
		o.exposedHeaders = nil

		for _, header := range headers {
			o.exposedHeaders = append(o.exposedHeaders, http.CanonicalHeaderKey(header))
		}
	}
}

// WithAllowCredentials sets the allow credentials option for the CORS middleware.
// This setting indicates whether the request can include user credentials like
// cookies, HTTP authentication or client side SSL certificates.
// By default, credentials are not allowed.
func WithAllowCredentials(allow bool) Option {
	return func(o *options) {
		o.allowCredentials = allow
	}
}

// WithAllowPrivateNetworks sets the allow private networks option for the CORS middleware.
// This setting indicates whether to accept cross-origin requests over a private network.
func WithAllowPrivateNetworks(allow bool) Option {
	return func(o *options) {
		o.allowPrivateNetworks = allow
	}
}

// WithMaxAge sets the max age (in seconds) for the CORS middleware.
// The maximum age indicates how long (in seconds) the results of a preflight request
// can be cached. A value of 0 means that no Access-Control-Max-Age header is sent back,
// resulting in browsers using their default value (5s by spec).
// If you need to force a 0 max-age, set it to a negative value (ie: -1).
// By default, the max age is 7200 seconds.
func WithMaxAge(age int) Option {
	return func(o *options) {
		o.maxAge = age
	}
}

// newOptions is a function that returns a new options struct with sane default values.
func newOptions(opts ...Option) options {
	o := options{
		allowedOrigins: []string{"*"},
		allowedMethods: []string{
			http.MethodHead, http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
		},
		allowedHeaders:       map[string]void{"*": {}},
		exposedHeaders:       nil,
		allowCredentials:     false,
		allowPrivateNetworks: false,
		maxAge:               0,
	}

	for _, opt := range opts {
		opt(&o)
	}

	return o
}
