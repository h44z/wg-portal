package cors

import (
	"net/http"
	"slices"
	"strconv"
	"strings"
)

// Middleware is a type that creates a new CORS middleware. The CORS middleware
// adds Cross-Origin Resource Sharing headers to the response. This middleware should
// be used to allow cross-origin requests to your server.
type Middleware struct {
	o options

	varyHeaders string // precomputed Vary header
	allOrigins  bool   // all origins are allowed
}

// New returns a new CORS middleware with the provided options.
func New(opts ...Option) *Middleware {
	o := newOptions(opts...)

	m := &Middleware{
		o: o,
	}

	// set vary headers
	if m.o.allowPrivateNetworks {
		m.varyHeaders = "Origin, Access-Control-Request-Method, Access-Control-Request-Headers, Access-Control-Request-Private-Network"
	} else {
		m.varyHeaders = "Origin, Access-Control-Request-Method, Access-Control-Request-Headers"
	}

	if len(m.o.allowedOrigins) == 1 && m.o.allowedOrigins[0] == "*" {
		m.allOrigins = true
	}

	return m
}

// Handler returns the CORS middleware handler.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle preflight requests and stop the chain as some other
		// middleware may not handle OPTIONS requests correctly.
		// https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS#preflighted_requests
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			m.handlePreflight(w, r)
			w.WriteHeader(http.StatusNoContent) // always return 204 No Content
			return
		}

		// handle normal CORS requests
		m.handleNormal(w, r)
		next.ServeHTTP(w, r) // execute the next handler
	})
}

// region internal-helpers

// handlePreflight handles preflight requests. If the request was successful, this function will
// write the CORS headers and return. If the request was not successful, this function will
// not add any CORS headers and return - thus the CORS request is considered invalid.
func (m *Middleware) handlePreflight(w http.ResponseWriter, r *http.Request) {
	// Always set Vary headers
	// see https://github.com/rs/cors/issues/10,
	// https://github.com/rs/cors/commit/dbdca4d95feaa7511a46e6f1efb3b3aa505bc43f#commitcomment-12352001
	w.Header().Add("Vary", m.varyHeaders)

	// check origin
	origin := r.Header.Get("Origin")
	if origin == "" {
		return // not a valid CORS request
	}

	if !m.originAllowed(origin) {
		return
	}

	// check method
	reqMethod := r.Header.Get("Access-Control-Request-Method")
	if !m.methodAllowed(reqMethod) {
		return
	}

	// check headers
	reqHeaders := r.Header.Get("Access-Control-Request-Headers")
	if !m.headersAllowed(reqHeaders) {
		return
	}

	// set CORS headers for the successful preflight request
	if m.allOrigins {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", origin) // return original origin
	}
	w.Header().Set("Access-Control-Allow-Methods", reqMethod)
	if reqHeaders != "" {
		// Spec says: Since the list of headers can be unbounded, simply returning supported headers
		// from Access-Control-Request-Headers can be enough
		w.Header().Set("Access-Control-Allow-Headers", reqHeaders)
	}
	if m.o.allowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	if m.o.allowPrivateNetworks && r.Header.Get("Access-Control-Request-Private-Network") == "true" {
		w.Header().Set("Access-Control-Allow-Private-Network", "true")
	}
	if m.o.maxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", strconv.Itoa(m.o.maxAge))
	}
}

// handleNormal handles normal CORS requests. If the request was successful, this function will
// write the CORS headers to the response. If the request was not successful, this function will
// not add any CORS headers to the response. In this case, the CORS request is considered invalid.
func (m *Middleware) handleNormal(w http.ResponseWriter, r *http.Request) {
	// Always set Vary headers
	// see https://github.com/rs/cors/issues/10,
	// https://github.com/rs/cors/commit/dbdca4d95feaa7511a46e6f1efb3b3aa505bc43f#commitcomment-12352001
	w.Header().Add("Vary", "Origin")

	// check origin
	origin := r.Header.Get("Origin")
	if origin == "" {
		return // not a valid CORS request
	}

	if !m.originAllowed(origin) {
		return
	}

	// check method
	if !m.methodAllowed(r.Method) {
		return
	}

	// set CORS headers for the successful CORS request
	if m.allOrigins {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", origin) // return original origin
	}
	if len(m.o.exposedHeaders) > 0 {
		w.Header().Set("Access-Control-Expose-Headers", strings.Join(m.o.exposedHeaders, ", "))
	}
	if m.o.allowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
}

func (m *Middleware) originAllowed(origin string) bool {
	if len(m.o.allowedOrigins) == 1 && m.o.allowedOrigins[0] == "*" {
		return true // everything is allowed
	}

	// check simple origins
	if slices.Contains(m.o.allowedOrigins, origin) {
		return true
	}

	// check wildcard origins
	for _, allowedOrigin := range m.o.allowedOriginPatterns {
		if allowedOrigin.match(origin) {
			return true
		}
	}

	return false
}

func (m *Middleware) methodAllowed(method string) bool {
	if method == http.MethodOptions {
		return true // preflight request is always allowed
	}

	if len(m.o.allowedMethods) == 1 && m.o.allowedMethods[0] == "*" {
		return true // everything is allowed
	}

	if slices.Contains(m.o.allowedMethods, method) {
		return true
	}

	return false
}

func (m *Middleware) headersAllowed(headers string) bool {
	if headers == "" {
		return true // no headers are requested
	}

	if len(m.o.allowedHeaders) == 0 {
		return false // no headers are allowed
	}

	if _, ok := m.o.allowedHeaders["*"]; ok {
		return true // everything is allowed
	}

	// split headers by comma (according to definition, the headers are sorted and in lowercase)
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Request-Headers
	for header := range strings.SplitSeq(headers, ",") {
		if _, ok := m.o.allowedHeaders[strings.TrimSpace(header)]; !ok {
			return false
		}
	}

	return true
}

// endregion internal-helpers
