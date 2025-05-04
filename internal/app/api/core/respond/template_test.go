package respond

import (
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockTemplate struct {
	tmpl *template.Template
}

func (m *mockTemplate) ExecuteTemplate(wr io.Writer, name string, data any) error {
	return m.tmpl.ExecuteTemplate(wr, name, data)
}

func TestTemplateRenderer_Render(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse(`{{define "test"}}Hello, {{.}}!{{end}}`))
	renderer := NewTemplateRenderer(&mockTemplate{tmpl: tmpl})

	rec := httptest.NewRecorder()
	renderer.Render(rec, http.StatusOK, "test", "text/plain", "World")

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	if contentType := res.Header.Get("Content-Type"); contentType != "text/plain" {
		t.Errorf("expected content type %s, got %s", "text/plain", contentType)
	}

	body, _ := io.ReadAll(res.Body)
	expectedBody := "Hello, World!"
	if string(body) != expectedBody {
		t.Errorf("expected body %s, got %s", expectedBody, string(body))
	}
}

func TestTemplateRenderer_HTML(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse(`{{define "test"}}<p>Hello, {{.}}!</p>{{end}}`))
	renderer := NewTemplateRenderer(&mockTemplate{tmpl: tmpl})

	rec := httptest.NewRecorder()
	renderer.HTML(rec, http.StatusOK, "test", "World")

	res := rec.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	if contentType := res.Header.Get("Content-Type"); contentType != "text/html;charset=utf-8" {
		t.Errorf("expected content type %s, got %s", "text/html;charset=utf-8", contentType)
	}

	body, _ := io.ReadAll(res.Body)
	expectedBody := "<p>Hello, World!</p>"
	if string(body) != expectedBody {
		t.Errorf("expected body %s, got %s", expectedBody, string(body))
	}
}
