package cors

import "testing"

func TestWildcardMatch(t *testing.T) {
	tests := []struct {
		name     string
		wildcard wildcard
		input    string
		expected bool
	}{
		{
			name:     "Match with prefix and suffix",
			wildcard: newWildcard("http://*.example.com"),
			input:    "http://sub.example.com",
			expected: true,
		},
		{
			name:     "No match with different prefix",
			wildcard: newWildcard("http://*.example.com"),
			input:    "https://sub.example.com",
			expected: false,
		},
		{
			name:     "No match with different suffix",
			wildcard: newWildcard("http://*.example.com"),
			input:    "http://sub.example.org",
			expected: false,
		},
		{
			name:     "Match with empty suffix",
			wildcard: newWildcard("http://*"),
			input:    "http://example.com",
			expected: true,
		},
		{
			name:     "Match with empty prefix",
			wildcard: newWildcard("*.example.com"),
			input:    "sub.example.com",
			expected: true,
		},
		{
			name:     "No match with empty prefix and different suffix",
			wildcard: newWildcard("*.example.com"),
			input:    "sub.example.org",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.wildcard.match(tt.input); got != tt.expected {
				t.Errorf("wildcard.match(%s) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNewWildcard(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected wildcard
	}{
		{
			name:     "Wildcard with prefix and suffix",
			input:    "http://*.example.com",
			expected: wildcard{prefix: "http://", suffix: ".example.com"},
		},
		{
			name:     "Wildcard with empty suffix",
			input:    "http://*",
			expected: wildcard{prefix: "http://", suffix: ""},
		},
		{
			name:     "Wildcard with empty prefix",
			input:    "*.example.com",
			expected: wildcard{prefix: "", suffix: ".example.com"},
		},
		{
			name:     "No wildcard character",
			input:    "http://example.com",
			expected: wildcard{prefix: "http://example.com", suffix: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newWildcard(tt.input); got != tt.expected {
				t.Errorf("newWildcard(%s) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
