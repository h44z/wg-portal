package recovery

import (
	"net/http"
	"testing"
)

func TestWithErrCallback(t *testing.T) {
	callback := func(err error, stack []byte, w http.ResponseWriter, r *http.Request) {}
	opt := WithErrCallback(callback)
	o := newOptions(opt)

	if o.errCallback == nil {
		t.Errorf("expected errCallback to be set, got nil")
	}
}

func TestWithBrokenPipeCallback(t *testing.T) {
	callback := func(err error, stack []byte, w http.ResponseWriter, r *http.Request) {}
	opt := WithBrokenPipeCallback(callback)
	o := newOptions(opt)

	if o.brokenPipeCallback == nil {
		t.Errorf("expected brokenPipeCallback to be set, got nil")
	}
}

func TestWithLogCallback(t *testing.T) {
	callback := func(err error, stack []byte, brokenPipe bool) {}
	opt := WithLogCallback(callback)
	o := newOptions(opt)

	if o.logCallback == nil {
		t.Errorf("expected logCallback to be set, got nil")
	}
}

func TestWithLogger(t *testing.T) {
	logger := &mockLogger{}
	opt := WithLogger(logger)
	o := newOptions(opt)

	if o.logger != logger {
		t.Errorf("expected logger to be %v, got %v", logger, o.logger)
	}
}

func TestWithSlog(t *testing.T) {
	opt := WithSlog(false)
	o := newOptions(opt)

	if o.useSlog != false {
		t.Errorf("expected useSlog to be false, got %v", o.useSlog)
	}
}

func TestWithDefaultLogPrefix(t *testing.T) {
	prefix := "PREFIX"
	opt := WithDefaultLogPrefix(prefix)
	o := newOptions(opt)

	if o.defaultLogPrefix != prefix {
		t.Errorf("expected defaultLogPrefix to be %v, got %v", prefix, o.defaultLogPrefix)
	}
}

func TestWithExposeStackTrace(t *testing.T) {
	opt := WithExposeStackTrace(true)
	o := newOptions(opt)

	if o.exposeStackTrace != true {
		t.Errorf("expected exposeStackTrace to be true, got %v", o.exposeStackTrace)
	}
}

func TestNewOptionsDefaults(t *testing.T) {
	o := newOptions()

	if o.logger != nil {
		t.Errorf("expected logger to be nil, got %v", o.logger)
	}
	if o.useSlog != true {
		t.Errorf("expected useSlog to be true, got %v", o.useSlog)
	}
	if o.errCallback == nil {
		t.Errorf("expected errCallback to be set, got nil")
	}
	if o.brokenPipeCallback != nil {
		t.Errorf("expected brokenPipeCallback to be nil, got %T", o.brokenPipeCallback)
	}
	if o.exposeStackTrace != false {
		t.Errorf("expected exposeStackTrace to be false, got %T", o.exposeStackTrace)
	}
	if o.defaultLogPrefix != "" {
		t.Errorf("expected defaultLogPrefix to be empty, got %T", o.defaultLogPrefix)
	}
	if o.logCallback == nil {
		t.Errorf("expected logCallback to be set, got nil")
	}
}
