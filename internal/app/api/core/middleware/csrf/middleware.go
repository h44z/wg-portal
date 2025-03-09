package csrf

import (
	"context"
	"net/http"
	"slices"
)

// ContextValueIdentifier is the context value identifier for the CSRF token.
// The token is only stored in the context if the RefreshToken function was called before.
const ContextValueIdentifier = "_csrf_token"

// Middleware is a type that creates a new CSRF middleware. The CSRF middleware
// can be used to mitigate Cross-Site Request Forgery attacks.
type Middleware struct {
	o options
}

// New returns a new CSRF middleware with the provided options.
func New(sessionReader SessionReader, sessionWriter SessionWriter, opts ...Option) *Middleware {
	opts = append(opts, withSessionReader(sessionReader), withSessionWriter(sessionWriter))
	o := newOptions(opts...)

	m := &Middleware{
		o: o,
	}

	checkForPRNG()

	return m
}

// Handler returns the CSRF middleware handler. This middleware validates the CSRF token and calls the specified
// error handler if an invalid CSRF token was found.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if slices.Contains(m.o.ignoreMethods, r.Method) {
			next.ServeHTTP(w, r) // skip CSRF check for ignored methods
			return
		}

		// get the token from the request
		token := m.o.tokenGetter(r)
		storedToken := m.o.sessionGetter(r)

		if !tokenEqual(token, storedToken) {
			m.o.errCallback(w, r)
			return
		}

		next.ServeHTTP(w, r) // execute the next handler
	})
}

// RefreshToken generates a new CSRF Token and stores it in the session. The token is also passed to subsequent handlers
// via the context value ContextValueIdentifier.
func (m *Middleware) RefreshToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetToken(r.Context()) != "" {
			// token already generated higher up in the chain
			next.ServeHTTP(w, r)
			return
		}

		// generate a new token
		token := generateToken(m.o.tokenLength)
		key := generateToken(m.o.tokenLength)

		// mask the token
		maskedToken := maskToken(token, key)

		// store the encoded token in the session
		encodedToken := encodeToken(maskedToken)
		m.o.sessionWriter(r, encodedToken)

		// pass the token down the chain via the context
		r = r.WithContext(setToken(r.Context(), encodedToken))

		next.ServeHTTP(w, r)
	})
}

// region token-access

// GetToken retrieves the CSRF token from the given context. Ensure that the RefreshToken function was called before,
// otherwise, no token is populated in the context.
func GetToken(ctx context.Context) string {
	token, ok := ctx.Value(ContextValueIdentifier).(string)
	if !ok {
		return ""
	}

	return token
}

// endregion token-access

// region internal-helpers

func setToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, ContextValueIdentifier, token)
}

// defaultTokenGetter is the default token getter function for the CSRF middleware.
// It checks the request form values, URL query parameters, and headers for the CSRF token.
// The order of precedence is:
//  1. Header "X-CSRF-TOKEN"
//  2. Header "X-XSRF-TOKEN"
//  3. URL query parameter "_csrf"
//  4. Form value "_csrf"
func defaultTokenGetter(r *http.Request) string {
	if t := r.Header.Get("X-CSRF-TOKEN"); len(t) > 0 {
		return t
	}

	if t := r.Header.Get("X-XSRF-TOKEN"); len(t) > 0 {
		return t
	}

	if t := r.URL.Query().Get("_csrf"); len(t) > 0 {
		return t
	}

	if t := r.FormValue("_csrf"); len(t) > 0 {
		return t
	}

	return ""
}

// defaultErrorHandler is the default error handler function for the CSRF middleware.
// It writes a 403 Forbidden response.
func defaultErrorHandler(w http.ResponseWriter, _ *http.Request) {
	http.Error(w, "CSRF token mismatch", http.StatusForbidden)
}

// endregion internal-helpers
