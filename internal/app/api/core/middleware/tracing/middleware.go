package tracing

import (
	"context"
	"math/rand"
	"net/http"
)

// Middleware is a type that creates a new tracing middleware. The tracing middleware
// can be used to trace requests based on a request ID header or parameter.
type Middleware struct {
	o options

	seededRand *rand.Rand
}

// New returns a new CORS middleware with the provided options.
func New(opts ...Option) *Middleware {
	o := newOptions(opts...)

	m := &Middleware{
		o:          o,
		seededRand: rand.New(rand.NewSource(o.generateSeed)),
	}

	return m
}

// Handler returns the tracing middleware handler.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqId string

		// read upstream header und re-use it
		if m.o.upstreamReqIdHeader != "" {
			reqId = r.Header.Get(m.o.upstreamReqIdHeader)
		}

		// generate new id
		if reqId == "" && m.o.generateLength > 0 {
			reqId = m.generateRandomId()
		}

		// set response header
		if m.o.headerIdentifier != "" {
			w.Header().Set(m.o.headerIdentifier, reqId)
		}

		// set context value
		if m.o.contextIdentifier != "" {
			ctx := context.WithValue(r.Context(), m.o.contextIdentifier, reqId)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r) // execute the next handler
	})
}

// region internal-helpers

func (m *Middleware) generateRandomId() string {
	b := make([]byte, m.o.generateLength)
	for i := range b {
		b[i] = m.o.generateCharset[m.seededRand.Intn(len(m.o.generateCharset))]
	}
	return string(b)
}

// endregion internal-helpers
