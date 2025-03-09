package tracing

import (
	"testing"
)

func TestWithIdSeed(t *testing.T) {
	o := newOptions(WithIdSeed(12345))
	if o.generateSeed != 12345 {
		t.Errorf("expected generateSeed to be 12345, got %d", o.generateSeed)
	}
}

func TestWithIdCharset(t *testing.T) {
	o := newOptions(WithIdCharset("abc123"))
	if o.generateCharset != "abc123" {
		t.Errorf("expected generateCharset to be 'abc123', got %s", o.generateCharset)
	}
}

func TestWithIdLength(t *testing.T) {
	o := newOptions(WithIdLength(16))
	if o.generateLength != 16 {
		t.Errorf("expected generateLength to be 16, got %d", o.generateLength)
	}
}

func TestWithHeaderIdentifier(t *testing.T) {
	o := newOptions(WithHeaderIdentifier("X-Custom-Id"))
	if o.headerIdentifier != "X-Custom-Id" {
		t.Errorf("expected headerIdentifier to be 'X-Custom-Id', got %s", o.headerIdentifier)
	}
}

func TestWithUpstreamHeader(t *testing.T) {
	o := newOptions(WithUpstreamHeader("X-Upstream-Id"))
	if o.upstreamReqIdHeader != "X-Upstream-Id" {
		t.Errorf("expected upstreamReqIdHeader to be 'X-Upstream-Id', got %s", o.upstreamReqIdHeader)
	}
}

func TestWithContextIdentifier(t *testing.T) {
	o := newOptions(WithContextIdentifier("Request-Id"))
	if o.contextIdentifier != "Request-Id" {
		t.Errorf("expected contextIdentifier to be 'Request-Id', got %s", o.contextIdentifier)
	}
}

func TestDefaults(t *testing.T) {
	o := newOptions()

	if o.generateLength != 8 {
		t.Errorf("expected generateLength to be 8, got %d", o.generateLength)
	}

	if o.generateCharset != "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" {
		t.Errorf("expected generateCharset to be 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789', got %s", o.generateCharset)
	}

	if o.generateSeed == 0 {
		t.Errorf("expected generateSeed to be non-zero")
	}

	if o.headerIdentifier != "X-Request-Id" {
		t.Errorf("expected headerIdentifier to be 'X-Request-Id', got %s", o.headerIdentifier)
	}

	if o.upstreamReqIdHeader != "" {
		t.Errorf("expected upstreamReqIdHeader to be empty, got %s", o.upstreamReqIdHeader)
	}

	if o.contextIdentifier != "RequestId" {
		t.Errorf("expected contextIdentifier to be 'RequestId', got %s", o.contextIdentifier)
	}
}
