package csrf

import (
	"net/http"
	"testing"
)

func TestWithTokenLength(t *testing.T) {
	o := newOptions(WithTokenLength(64))
	if o.tokenLength != 64 {
		t.Errorf("WithTokenLength() = %d, want %d", o.tokenLength, 64)
	}
}

func TestWithErrorCallback(t *testing.T) {
	callback := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}
	o := newOptions(WithErrorCallback(callback))
	if !o.errCallbackOverride {
		t.Errorf("WithErrorCallback() did not set errCallbackOverride to true")
	}
	if o.errCallback == nil {
		t.Errorf("WithErrorCallback() did not set errCallback")
	}
}

func TestWithTokenGetter(t *testing.T) {
	getter := func(r *http.Request) string {
		return "test-token"
	}
	o := newOptions(WithTokenGetter(getter))
	if !o.tokenGetterOverride {
		t.Errorf("WithTokenGetter() did not set tokenGetterOverride to true")
	}
	if o.tokenGetter == nil {
		t.Errorf("WithTokenGetter() did not set tokenGetter")
	}
}

func TestWithSessionReader(t *testing.T) {
	reader := func(r *http.Request) string {
		return "session-token"
	}
	o := newOptions(withSessionReader(reader))
	if o.sessionGetter == nil {
		t.Errorf("withSessionReader() did not set sessionGetter")
	}
}

func TestWithSessionWriter(t *testing.T) {
	writer := func(r *http.Request, token string) {
		// do nothing
	}
	o := newOptions(withSessionWriter(writer))
	if o.sessionWriter == nil {
		t.Errorf("withSessionWriter() did not set sessionWriter")
	}
}

func TestNewOptionsDefaults(t *testing.T) {
	o := newOptions()
	if o.tokenLength != 32 {
		t.Errorf("newOptions() default tokenLength = %d, want %d", o.tokenLength, 32)
	}
	if len(o.ignoreMethods) != 3 {
		t.Errorf("newOptions() default ignoreMethods length = %d, want %d", len(o.ignoreMethods), 3)
	}
	if o.errCallback == nil {
		t.Errorf("newOptions() default errCallback is nil")
	}
	if o.tokenGetter == nil {
		t.Errorf("newOptions() default tokenGetter is nil")
	}
}
