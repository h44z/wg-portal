package respond

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	Status(rec, http.StatusNoContent)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		t.Errorf("expected status %d, got %d", http.StatusNoContent, res.StatusCode)
	}

	body, _ := io.ReadAll(res.Body)
	if len(body) != 0 {
		t.Errorf("expected no body, got %s", body)
	}
}

func TestString(t *testing.T) {
	rec := httptest.NewRecorder()
	String(rec, http.StatusOK, "Hello, World!")

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	if contentType := res.Header.Get("Content-Type"); contentType != "text/plain;charset=utf-8" {
		t.Errorf("expected content type %s, got %s", "text/plain;charset=utf-8", contentType)
	}

	body, _ := io.ReadAll(res.Body)
	if string(body) != "Hello, World!" {
		t.Errorf("expected body %s, got %s", "Hello, World!", string(body))
	}
}

func TestJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	data := map[string]string{"hello": "world"}
	JSON(rec, http.StatusOK, data)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	if contentType := res.Header.Get("Content-Type"); contentType != "application/json" {
		t.Errorf("expected content type %s, got %s", "application/json", contentType)
	}

	var body map[string]string
	_ = json.NewDecoder(res.Body).Decode(&body)
	if body["hello"] != "world" {
		t.Errorf("expected body %v, got %v", data, body)
	}
}

func TestJSON_empty(t *testing.T) {
	rec := httptest.NewRecorder()
	JSON(rec, http.StatusOK, nil)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	if contentType := res.Header.Get("Content-Type"); contentType != "application/json" {
		t.Errorf("expected content type %s, got %s", "application/json", contentType)
	}

	body, _ := io.ReadAll(res.Body)
	if string(body) != "null" {
		t.Errorf("expected body %s, got %s", "null", body)
	}
}

func TestData(t *testing.T) {
	rec := httptest.NewRecorder()
	data := []byte("Hello, World!")
	Data(rec, http.StatusOK, "text/plain", data)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	if contentType := res.Header.Get("Content-Type"); contentType != "text/plain" {
		t.Errorf("expected content type %s, got %s", "text/plain", contentType)
	}

	body, _ := io.ReadAll(res.Body)
	if !bytes.Equal(body, data) {
		t.Errorf("expected body %s, got %s", data, body)
	}
}

func TestData_noContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	data := []byte{0x1, 0x2, 0x3, 0x4, 0x5}
	Data(rec, http.StatusOK, "", data)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	if contentType := res.Header.Get("Content-Type"); contentType != "application/octet-stream" {
		t.Errorf("expected content type %s, got %s", "application/octet-stream", contentType)
	}

	body, _ := io.ReadAll(res.Body)
	if !bytes.Equal(body, data) {
		t.Errorf("expected body %s, got %s", data, body)
	}
}

func TestReader(t *testing.T) {
	rec := httptest.NewRecorder()
	data := []byte("Hello, World!")
	reader := bytes.NewBufferString(string(data))
	Reader(rec, http.StatusOK, "text/plain", len(data), reader)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	if contentType := res.Header.Get("Content-Type"); contentType != "text/plain" {
		t.Errorf("expected content type %s, got %s", "text/plain", contentType)
	}

	if contentLength := res.Header.Get("Content-Length"); contentLength != strconv.Itoa(len(data)) {
		t.Errorf("expected content length %d, got %s", len(data), contentLength)
	}

	body, _ := io.ReadAll(res.Body)
	if string(body) != "Hello, World!" {
		t.Errorf("expected body %s, got %s", "Hello, World!", string(body))
	}
}

func TestReader_unknownLength(t *testing.T) {
	rec := httptest.NewRecorder()
	data := bytes.NewBufferString("Hello, World!")
	Reader(rec, http.StatusOK, "text/plain", 0, data)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	if contentType := res.Header.Get("Content-Type"); contentType != "text/plain" {
		t.Errorf("expected content type %s, got %s", "text/plain", contentType)
	}

	if contentLength := res.Header.Get("Content-Length"); contentLength != "" {
		t.Errorf("expected no content length, got %s", contentLength)
	}

	body, _ := io.ReadAll(res.Body)
	if string(body) != "Hello, World!" {
		t.Errorf("expected body %s, got %s", "Hello, World!", string(body))
	}
}

func TestAttachment(t *testing.T) {
	rec := httptest.NewRecorder()
	data := []byte("Hello, World!")
	Attachment(rec, http.StatusOK, "example.txt", "text/plain", data)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	if contentDisposition := res.Header.Get("Content-Disposition"); contentDisposition != "attachment; filename=example.txt" {
		t.Errorf("expected content disposition %s, got %s", "attachment; filename=example.txt", contentDisposition)
	}

	body, _ := io.ReadAll(res.Body)
	if !bytes.Equal(body, data) {
		t.Errorf("expected body %s, got %s", data, body)
	}
}

func TestAttachmentReader(t *testing.T) {
	rec := httptest.NewRecorder()
	data := bytes.NewBufferString("Hello, World!")
	AttachmentReader(rec, http.StatusOK, "example.txt", "text/plain", data.Len(), data)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	if contentDisposition := res.Header.Get("Content-Disposition"); contentDisposition != "attachment; filename=example.txt" {
		t.Errorf("expected content disposition %s, got %s", "attachment; filename=example.txt", contentDisposition)
	}

	body, _ := io.ReadAll(res.Body)
	if string(body) != "Hello, World!" {
		t.Errorf("expected body %s, got %s", "Hello, World!", string(body))
	}
}

func TestRedirect(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/old", nil)
	url := "http://example.com/new"

	Redirect(rec, req, http.StatusMovedPermanently, url)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusMovedPermanently {
		t.Errorf("expected status %d, got %d", http.StatusMovedPermanently, res.StatusCode)
	}

	if location := res.Header.Get("Location"); location != url {
		t.Errorf("expected location %s, got %s", url, location)
	}
}

func TestRedirect_relative(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/old/dir", nil)
	url := "newlocation/sub"
	want := "/old/newlocation/sub"

	Redirect(rec, req, http.StatusMovedPermanently, url)

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusMovedPermanently {
		t.Errorf("expected status %d, got %d", http.StatusMovedPermanently, res.StatusCode)
	}

	if location := res.Header.Get("Location"); location != want {
		t.Errorf("expected location %s, got %s", want, location)
	}
}
