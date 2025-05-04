package logging

import (
	"testing"
)

func TestWithLevel(t *testing.T) {
	// table test to check all possible log levels
	levels := []LogLevel{
		LogLevelDebug,
		LogLevelInfo,
		LogLevelWarn,
		LogLevelError,
	}

	for _, level := range levels {
		opt := WithLevel(level)
		o := newOptions(opt)

		if o.logLevel != level {
			t.Errorf("expected log level to be %v, got %v", level, o.logLevel)
		}
	}
}

func TestWithPrefix(t *testing.T) {
	prefix := "TEST"
	opt := WithPrefix(prefix)
	o := newOptions(opt)

	if o.prefix != prefix {
		t.Errorf("expected prefix to be %v, got %v", prefix, o.prefix)
	}
}

func TestWithContextRequestIdKey(t *testing.T) {
	key := "contextKey"
	opt := WithContextRequestIdKey(key)
	o := newOptions(opt)

	if o.contextRequestIdKey != key {
		t.Errorf("expected contextRequestIdKey to be %v, got %v", key, o.contextRequestIdKey)
	}
}

func TestWithHeaderRequestIdKey(t *testing.T) {
	key := "headerKey"
	opt := WithHeaderRequestIdKey(key)
	o := newOptions(opt)

	if o.headerRequestIdKey != key {
		t.Errorf("expected headerRequestIdKey to be %v, got %v", key, o.headerRequestIdKey)
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

func TestDefaults(t *testing.T) {
	o := newOptions()

	if o.logLevel != LogLevelInfo {
		t.Errorf("expected log level to be %v, got %v", LogLevelInfo, o.logLevel)
	}

	if o.logger != nil {
		t.Errorf("expected logger to be nil, got %v", o.logger)
	}

	if o.prefix != "" {
		t.Errorf("expected prefix to be empty, got %v", o.prefix)
	}

	if o.contextRequestIdKey != "" {
		t.Errorf("expected contextRequestIdKey to be empty, got %v", o.contextRequestIdKey)
	}

	if o.headerRequestIdKey != "" {
		t.Errorf("expected headerRequestIdKey to be empty, got %v", o.headerRequestIdKey)
	}
}
