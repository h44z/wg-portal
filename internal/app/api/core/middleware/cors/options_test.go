package cors

import (
	"maps"
	"net/http"
	"slices"
	"testing"
)

func TestWithAllowedOrigins(t *testing.T) {
	tests := []struct {
		name         string
		origins      []string
		wantNormal   []string
		wantWildcard []wildcard
	}{
		{
			name:         "No origins",
			origins:      []string{},
			wantNormal:   nil,
			wantWildcard: nil,
		},
		{
			name:         "Single origin",
			origins:      []string{"http://example.com"},
			wantNormal:   []string{"http://example.com"},
			wantWildcard: nil,
		},
		{
			name:         "Wildcard origin",
			origins:      []string{"http://*.example.com"},
			wantNormal:   nil,
			wantWildcard: []wildcard{newWildcard("http://*.example.com")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := newOptions(WithAllowedOrigins(tt.origins...))
			if !slices.Equal(o.allowedOrigins, tt.wantNormal) {
				t.Errorf("got %v, want %v", o, tt.wantNormal)
			}
			if !slices.Equal(o.allowedOriginPatterns, tt.wantWildcard) {
				t.Errorf("got %v, want %v", o, tt.wantWildcard)
			}
		})
	}
}

func TestWithAllowedMethods(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPost}
	o := newOptions(WithAllowedMethods(methods...))
	if !slices.Equal(o.allowedMethods, methods) {
		t.Errorf("got %v, want %v", o.allowedMethods, methods)
	}
}

func TestWithAllowedHeaders(t *testing.T) {
	headers := []string{"Content-Type", "Authorization"}
	o := newOptions(WithAllowedHeaders(headers...))
	expectedHeaders := map[string]void{"content-type": {}, "authorization": {}}
	if !maps.Equal(o.allowedHeaders, expectedHeaders) {
		t.Errorf("got %v, want %v", o.allowedHeaders, expectedHeaders)
	}
}

func TestWithExposedHeaders(t *testing.T) {
	headers := []string{"X-Custom-Header"}
	o := newOptions(WithExposedHeaders(headers...))
	expectedHeaders := []string{http.CanonicalHeaderKey("X-Custom-Header")}
	if !slices.Equal(o.exposedHeaders, expectedHeaders) {
		t.Errorf("got %v, want %v", o.exposedHeaders, expectedHeaders)
	}
}

func TestWithAllowCredentials(t *testing.T) {
	o := newOptions(WithAllowCredentials(true))
	if !o.allowCredentials {
		t.Errorf("got %v, want %v", o.allowCredentials, true)
	}
}

func TestWithAllowPrivateNetworks(t *testing.T) {
	o := newOptions(WithAllowPrivateNetworks(true))
	if !o.allowPrivateNetworks {
		t.Errorf("got %v, want %v", o.allowPrivateNetworks, true)
	}
}

func TestWithMaxAge(t *testing.T) {
	maxAge := 3600
	o := newOptions(WithMaxAge(maxAge))
	if o.maxAge != maxAge {
		t.Errorf("got %v, want %v", o.maxAge, maxAge)
	}
}
