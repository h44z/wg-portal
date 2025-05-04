package logging

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriterWrapper_WriteHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	ww := newWriterWrapper(rr)

	ww.WriteHeader(http.StatusNotFound)

	if ww.StatusCode != http.StatusNotFound {
		t.Errorf("expected status code to be %v, got %v", http.StatusNotFound, ww.StatusCode)
	}
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected recorder status code to be %v, got %v", http.StatusNotFound, rr.Code)
	}
}

func TestWriterWrapper_Write(t *testing.T) {
	rr := httptest.NewRecorder()
	ww := newWriterWrapper(rr)

	data := []byte("Hello, World!")
	n, err := ww.Write(data)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected written bytes to be %v, got %v", len(data), n)
	}
	if ww.WrittenBytes != int64(len(data)) {
		t.Errorf("expected WrittenBytes to be %v, got %v", len(data), ww.WrittenBytes)
	}
	if rr.Body.String() != string(data) {
		t.Errorf("expected response body to be %v, got %v", string(data), rr.Body.String())
	}
}

func TestWriterWrapper_WriteWithHeaders(t *testing.T) {
	rr := httptest.NewRecorder()
	ww := newWriterWrapper(rr)

	data := []byte("Hello, World!")
	n, err := ww.Write(data)

	ww.Header().Set("Content-Type", "text/plain")
	ww.Header().Set("X-Some-Header", "some-value")
	ww.WriteHeader(http.StatusTeapot)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected written bytes to be %v, got %v", len(data), n)
	}
	if ww.WrittenBytes != int64(len(data)) {
		t.Errorf("expected WrittenBytes to be %v, got %v", len(data), ww.WrittenBytes)
	}
	if rr.Body.String() != string(data) {
		t.Errorf("expected response body to be %v, got %v", string(data), rr.Body.String())
	}
	if ww.StatusCode != http.StatusTeapot {
		t.Errorf("expected status code to be %v, got %v", http.StatusTeapot, ww.StatusCode)
	}
}

func TestNewWriterWrapper(t *testing.T) {
	rr := httptest.NewRecorder()
	ww := newWriterWrapper(rr)

	if ww.StatusCode != http.StatusOK {
		t.Errorf("expected initial status code to be %v, got %v", http.StatusOK, ww.StatusCode)
	}
	if ww.WrittenBytes != 0 {
		t.Errorf("expected initial WrittenBytes to be %v, got %v", 0, ww.WrittenBytes)
	}
	if ww.ResponseWriter != rr {
		t.Errorf("expected ResponseWriter to be %v, got %v", rr, ww.ResponseWriter)
	}
}
