package logging

import (
	"net/http"
)

// writerWrapper wraps a http.ResponseWriter and tracks the number of bytes written to it.
// It also tracks the http response code passed to the WriteHeader func of
// the ResponseWriter.
type writerWrapper struct {
	http.ResponseWriter

	// StatusCode is the last http response code passed to the WriteHeader func of
	// the ResponseWriter. If no such call is made, a default code of http.StatusOK
	// is assumed instead.
	StatusCode int

	// WrittenBytes is the number of bytes successfully written by the Write or
	// ReadFrom function of the ResponseWriter. ResponseWriters may also write
	// data to their underlaying connection directly (e.g. headers), but those
	// are not tracked. Therefor the number of Written bytes will usually match
	// the size of the response body.
	WrittenBytes int64
}

// WriteHeader wraps the WriteHeader method of the ResponseWriter and tracks the
// http response code passed to it.
func (w *writerWrapper) WriteHeader(code int) {
	w.StatusCode = code
	w.ResponseWriter.WriteHeader(code)
}

// Write wraps the Write method of the ResponseWriter and tracks the number of bytes
// written to it.
func (w *writerWrapper) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.WrittenBytes += int64(n)
	return n, err
}

// newWriterWrapper returns a new writerWrapper that wraps the given http.ResponseWriter.
// It initializes the StatusCode to http.StatusOK.
func newWriterWrapper(w http.ResponseWriter) *writerWrapper {
	return &writerWrapper{ResponseWriter: w, StatusCode: http.StatusOK}
}
