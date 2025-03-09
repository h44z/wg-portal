package respond

import (
	"fmt"
	"io"
	"net/http"
)

// TplData is a map of template data. This is a convenience type for passing data to templates.
type TplData map[string]any

// TemplateInstance is an interface that wraps the ExecuteTemplate method.
// It is implemented by the html/template and text/template packages.
type TemplateInstance interface {
	// ExecuteTemplate executes a template with the given name and data.
	ExecuteTemplate(wr io.Writer, name string, data any) error
}

// TemplateRenderer is a renderer that uses a template instance to render HTML or Text templates.
type TemplateRenderer struct {
	t TemplateInstance
}

// NewTemplateRenderer creates a new HTML or Text template renderer with the given template instance.
func NewTemplateRenderer(t TemplateInstance) *TemplateRenderer {
	return &TemplateRenderer{t: t}
}

// Render renders a template with the given name and data.
// If rendering fails, it will panic with an error.
func (r *TemplateRenderer) Render(w http.ResponseWriter, code int, name, contentType string, data any) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(code)

	err := r.t.ExecuteTemplate(w, name, data)
	if err != nil {
		panic(fmt.Errorf("error rendering template %s: %v", name, err))
	}
}

// HTML renders a template with the given name and data. It is a convenience method for Render.
// The content type is set to "text/html" and the encoding to "utf-8".
// If rendering fails, it will panic with an error.
func (r *TemplateRenderer) HTML(w http.ResponseWriter, code int, name string, data any) {
	r.Render(w, code, name, "text/html;charset=utf-8", data)
}
