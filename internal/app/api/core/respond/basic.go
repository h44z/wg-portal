// Package respond provides a set of utility functions to help with the HTTP response handling.
package respond

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
)

// Status writes a response with the given status code.
// The response will not contain any data.
func Status(w http.ResponseWriter, code int) {
	w.WriteHeader(code)
}

// String writes a plain text response with the given status code and data.
// The Content-Type header is set to text/plain with a charset of utf-8.
func String(w http.ResponseWriter, code int, data string) {
	w.Header().Set("Content-Type", "text/plain;charset=utf-8")
	w.WriteHeader(code)

	_, _ = w.Write([]byte(data))
}

// JSON writes a JSON response with the given status code and data.
// If data is nil, the response will null. The status code is set to the given code.
// The Content-Type header is set to application/json.
// If the given data is not JSON serializable, the response will not contain any data.
// All encoding errors are silently ignored.
func JSON(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")

	// if no data was given, simply return null
	if data == nil {
		w.WriteHeader(code)
		_, _ = w.Write([]byte("null"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	_ = json.NewEncoder(w).Encode(data)
}

// Data writes a response with the given status code, content type, and data.
// If no content type is provided, it is detected from the data.
func Data(w http.ResponseWriter, code int, contentType string, data []byte) {
	if contentType == "" {
		contentType = http.DetectContentType(data) // ensure content type is set
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(code)

	_, _ = w.Write(data)
}

// Reader writes a response with the given status code, content type, and data.
// The content length is optional, it is only set if the given length is greater than 0.
func Reader(w http.ResponseWriter, code int, contentType string, contentLength int, data io.Reader) {
	w.Header().Set("Content-Type", contentType)
	if contentLength > 0 {
		w.Header().Set("Content-Length", strconv.Itoa(contentLength))
	}
	w.WriteHeader(code)

	_, _ = io.Copy(w, data)
}

// Attachment writes a response with the given status code, content type, filename, and data.
// If no content type is provided, it is detected from the data.
func Attachment(w http.ResponseWriter, code int, filename, contentType string, data []byte) {
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)

	Data(w, code, contentType, data)
}

// AttachmentReader writes a response with the given status code, content type, filename, content length, and data.
// The content length is optional, it is only set if the given length is greater than 0.
func AttachmentReader(
	w http.ResponseWriter,
	code int,
	filename, contentType string,
	contentLength int,
	data io.Reader,
) {
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)

	Reader(w, code, contentType, contentLength, data)
}

// Redirect writes a response with the given status code and redirects to the given URL.
// The redirect url will always be an absolute URL. If the given URL is relative,
// the original request URL is used as the base.
func Redirect(w http.ResponseWriter, r *http.Request, code int, url string) {
	http.Redirect(w, r, url, code)
}
