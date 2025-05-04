package csrf

import "net/http"

type SessionReader func(r *http.Request) string
type SessionWriter func(r *http.Request, token string)

// options is a struct that contains options for the CSRF middleware.
// It uses the functional options pattern for flexible configuration.
type options struct {
	tokenLength   int
	ignoreMethods []string

	errCallbackOverride bool
	errCallback         func(w http.ResponseWriter, r *http.Request)

	tokenGetterOverride bool
	tokenGetter         func(r *http.Request) string

	sessionGetter SessionReader
	sessionWriter SessionWriter
}

// Option is a type that is used to set options for the CSRF middleware.
// It implements the functional options pattern.
type Option func(*options)

// WithTokenLength is a method that sets the token length for the CSRF middleware.
// The default value is 32.
func WithTokenLength(length int) Option {
	return func(o *options) {
		o.tokenLength = length
	}
}

// WithErrorCallback is a method that sets the error callback function for the CSRF middleware.
// The error callback function is called when the CSRF token is invalid.
// The default behavior is to write a 403 Forbidden response.
func WithErrorCallback(fn func(w http.ResponseWriter, r *http.Request)) Option {
	return func(o *options) {
		o.errCallback = fn
		o.errCallbackOverride = true
	}
}

// WithTokenGetter is a method that sets the token getter function for the CSRF middleware.
// The token getter function is called to get the CSRF token from the request.
// The default behavior is to get the token from the "X-CSRF-Token" header.
func WithTokenGetter(fn func(r *http.Request) string) Option {
	return func(o *options) {
		o.tokenGetter = fn
		o.tokenGetterOverride = true
	}
}

// withSessionReader is a method that sets the session reader function for the CSRF middleware.
// The session reader function is called to get the CSRF token from the session.
func withSessionReader(fn SessionReader) Option {
	return func(o *options) {
		o.sessionGetter = fn
	}
}

// withSessionWriter is a method that sets the session writer function for the CSRF middleware.
// The session writer function is called to write the CSRF token to the session.
func withSessionWriter(fn SessionWriter) Option {
	return func(o *options) {
		o.sessionWriter = fn
	}
}

// newOptions is a function that returns a new options struct with sane default values.
func newOptions(opts ...Option) options {
	o := options{
		tokenLength:         32,
		ignoreMethods:       []string{"GET", "HEAD", "OPTIONS"},
		errCallbackOverride: false,
		errCallback:         defaultErrorHandler,
		tokenGetterOverride: false,
		tokenGetter:         defaultTokenGetter,
	}

	for _, opt := range opts {
		opt(&o)
	}

	return o
}
