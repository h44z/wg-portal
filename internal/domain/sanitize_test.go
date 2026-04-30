package domain

import (
	"net/mail"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"pgregory.net/rapid"
)

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "null byte removed",
			input:  "\x00",
			maxLen: 64,
			want:   "",
		},
		{
			name:   "CR removed",
			input:  "\r",
			maxLen: 64,
			want:   "",
		},
		{
			name:   "LF removed",
			input:  "\n",
			maxLen: 64,
			want:   "",
		},
		{
			name:   "tab removed",
			input:  "\t",
			maxLen: 64,
			want:   "",
		},
		{
			name:   "leading and trailing whitespace trimmed",
			input:  "  hello  ",
			maxLen: 64,
			want:   "hello",
		},
		{
			name:   "multi-byte UTF-8 truncation at rune boundary",
			input:  "héllo",
			maxLen: 3,
			want:   "hél", // 3 runes, not 3 bytes
		},
		{
			name:   "empty input",
			input:  "",
			maxLen: 64,
			want:   "",
		},
		{
			name:   "maxLen zero returns empty",
			input:  "hello",
			maxLen: 0,
			want:   "",
		},
		{
			name:   "string longer than maxLen truncated",
			input:  "abcdefgh",
			maxLen: 4,
			want:   "abcd",
		},
		{
			name:   "mixed control chars and normal chars",
			input:  "hel\x00lo\r\nworld",
			maxLen: 64,
			want:   "helloworld",
		},
		{
			name:   "only whitespace returns empty",
			input:  "   ",
			maxLen: 64,
			want:   "",
		},
		{
			name:   "string exactly at maxLen not truncated",
			input:  "abc",
			maxLen: 3,
			want:   "abc",
		},
		{
			name:   "negative maxLen returns empty",
			input:  "hello",
			maxLen: -1,
			want:   "",
		},
		{
			name:   "DEL control removed",
			input:  "hel\x7flo",
			maxLen: 64,
			want:   "hello",
		},
		{
			name:   "zero-width format character removed",
			input:  "ali\u200bce",
			maxLen: 64,
			want:   "alice",
		},
		{
			name:   "invalid UTF-8 byte removed",
			input:  "a\xffb",
			maxLen: 64,
			want:   "ab",
		},
		{
			name:   "unicode normalized to NFC",
			input:  "e\u0301",
			maxLen: 64,
			want:   "é",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeString(tc.input, tc.maxLen)
			if got != tc.want {
				t.Errorf("SanitizeString(%q, %d) = %q; want %q", tc.input, tc.maxLen, got, tc.want)
			}
		})
	}
}

func TestSanitizeEmail(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "valid email passes through unchanged",
			input:  "user@example.com",
			maxLen: 254,
			want:   "user@example.com",
		},
		{
			name:   "CR in email returns empty",
			input:  "user\r@example.com",
			maxLen: 254,
			want:   "",
		},
		{
			name:   "LF in email returns empty",
			input:  "user\n@example.com",
			maxLen: 254,
			want:   "",
		},
		{
			name:   "missing @ returns empty",
			input:  "userexample.com",
			maxLen: 254,
			want:   "",
		},
		{
			name:   "whitespace-only returns empty",
			input:  "   ",
			maxLen: 254,
			want:   "",
		},
		{
			name:   "email with leading/trailing whitespace trimmed and returned",
			input:  "  user@example.com  ",
			maxLen: 254,
			want:   "user@example.com",
		},
		{
			name:   "empty input returns empty",
			input:  "",
			maxLen: 254,
			want:   "",
		},
		{
			name:   "display-name address rejected",
			input:  "User <user@example.com>",
			maxLen: 254,
			want:   "",
		},
		{
			name:   "multiple at signs rejected",
			input:  "user@@example.com",
			maxLen: 254,
			want:   "",
		},
		{
			name:   "invalid address rejected",
			input:  "user@",
			maxLen: 254,
			want:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeEmail(tc.input, tc.maxLen)
			if got != tc.want {
				t.Errorf("SanitizeEmail(%q, %d) = %q; want %q", tc.input, tc.maxLen, got, tc.want)
			}
		})
	}
}

func TestSanitizePhone(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "valid phone passes through unchanged",
			input:  "+1 (555) 123-4567",
			maxLen: 50,
			want:   "+1 (555) 123-4567",
		},
		{
			name:   "non-allowed chars stripped",
			input:  "abc+1def",
			maxLen: 50,
			want:   "+1",
		},
		{
			name:   "all-stripped input returns empty",
			input:  "abc",
			maxLen: 50,
			want:   "",
		},
		{
			name:   "mixed allowed and non-allowed chars",
			input:  "+49 (0) 123-456.789",
			maxLen: 50,
			want:   "+49 (0) 123-456.789",
		},
		{
			name:   "empty input returns empty",
			input:  "",
			maxLen: 50,
			want:   "",
		},
		{
			name:   "only digits passes through",
			input:  "1234567890",
			maxLen: 50,
			want:   "1234567890",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizePhone(tc.input, tc.maxLen)
			if got != tc.want {
				t.Errorf("SanitizePhone(%q, %d) = %q; want %q", tc.input, tc.maxLen, got, tc.want)
			}
		})
	}
}

func TestSanitizeIdentifier(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "reserved value all returns empty",
			input:  "all",
			maxLen: 256,
			want:   "",
		},
		{
			name:   "all with surrounding whitespace returns empty",
			input:  " all ",
			maxLen: 256,
			want:   "",
		},
		{
			name:   "reserved value new returns empty",
			input:  "new",
			maxLen: 256,
			want:   "",
		},
		{
			name:   "reserved value id returns empty",
			input:  "id",
			maxLen: 256,
			want:   "",
		},
		{
			name:   "system admin identifier returns empty",
			input:  string(CtxSystemAdminId),
			maxLen: 256,
			want:   "",
		},
		{
			name:   "unknown user identifier returns empty",
			input:  string(CtxUnknownUserId),
			maxLen: 256,
			want:   "",
		},
		{
			name:   "LDAP syncer identifier returns empty",
			input:  string(CtxSystemLdapSyncer),
			maxLen: 256,
			want:   "",
		},
		{
			name:   "ALL uppercase passes through (case-sensitive)",
			input:  "ALL",
			maxLen: 256,
			want:   "ALL",
		},
		{
			name:   "valid email identifier passes through",
			input:  "alice@example.com",
			maxLen: 256,
			want:   "alice@example.com",
		},
		{
			name:   "normal identifier passes through",
			input:  "alice",
			maxLen: 256,
			want:   "alice",
		},
		{
			name:   "empty input returns empty",
			input:  "",
			maxLen: 256,
			want:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeIdentifier(tc.input, tc.maxLen)
			if got != tc.want {
				t.Errorf("SanitizeIdentifier(%q, %d) = %q; want %q", tc.input, tc.maxLen, got, tc.want)
			}
		})
	}
}

func TestSanitizeXSSPayload(t *testing.T) {
	// XSS payload: null byte removed, angle brackets preserved
	input := "<script>\x00alert(1)</script>"
	want := "<script>alert(1)</script>"
	got := SanitizeString(input, 256)
	if got != want {
		t.Errorf("SanitizeString(%q, 256) = %q; want %q", input, got, want)
	}
}

// ---------------------------------------------------------------------------
// Property 1: SanitizeString output invariants
// ---------------------------------------------------------------------------

// Feature: external-identity-sanitization, Property 1: SanitizeString output is free of control characters and bounded in length
func TestPropertySanitizeStringOutputInvariants(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		maxLen := rapid.IntRange(0, 512).Draw(t, "maxLen")
		result := SanitizeString(s, maxLen)

		// No control or format runes in result
		for _, r := range result {
			if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
				t.Fatalf("result %q contains unsafe character %U", result, r)
			}
		}

		if !utf8.ValidString(result) {
			t.Fatalf("result %q is not valid UTF-8", result)
		}

		// No leading or trailing whitespace
		if result != strings.TrimSpace(result) {
			t.Fatalf("result %q has leading or trailing whitespace", result)
		}

		// Rune count <= maxLen
		runeCount := utf8.RuneCountInString(result)
		if runeCount > maxLen {
			t.Fatalf("result %q has %d runes, exceeds maxLen %d", result, runeCount, maxLen)
		}
	})
}

// ---------------------------------------------------------------------------
// Property 2: SanitizeString is idempotent
// ---------------------------------------------------------------------------

// Feature: external-identity-sanitization, Property 2: SanitizeString is idempotent
func TestPropertySanitizeStringIdempotent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		maxLen := rapid.IntRange(1, 512).Draw(t, "maxLen")

		once := SanitizeString(s, maxLen)
		twice := SanitizeString(once, maxLen)

		if once != twice {
			t.Fatalf("SanitizeString is not idempotent: once=%q, twice=%q (input=%q, maxLen=%d)",
				once, twice, s, maxLen)
		}
	})
}

// ---------------------------------------------------------------------------
// Property 3: SanitizeEmail rejection rules
// ---------------------------------------------------------------------------

// Feature: external-identity-sanitization, Property 3: SanitizeEmail rejects strings without "@" or containing CR/LF
func TestPropertySanitizeEmailRejectionRules(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		maxLen := rapid.IntRange(1, 512).Draw(t, "maxLen")
		result := SanitizeEmail(s, maxLen)

		sanitized := SanitizeString(s, maxLen)
		addr, parseErr := mail.ParseAddress(sanitized)
		reject := strings.ContainsAny(s, "\r\n") ||
			sanitized == "" ||
			strings.Count(sanitized, "@") != 1 ||
			parseErr != nil ||
			addr.Name != "" ||
			addr.Address != sanitized
		if reject {
			if result != "" {
				t.Fatalf("SanitizeEmail(%q, %d) = %q; expected empty string (contains CR/LF or no @)",
					s, maxLen, result)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Property 4: SanitizePhone allowed character set
// ---------------------------------------------------------------------------

// isAllowedPhoneCharTest mirrors the internal isAllowedPhoneRune logic for test assertions.
func isAllowedPhoneCharTest(r rune) bool {
	switch {
	case r >= '0' && r <= '9':
		return true
	case r == '+', r == '-', r == '(', r == ')', r == ' ', r == '.':
		return true
	default:
		return false
	}
}

// Feature: external-identity-sanitization, Property 4: SanitizePhone output contains only allowed characters
func TestPropertySanitizePhoneAllowedChars(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		maxLen := rapid.IntRange(1, 512).Draw(t, "maxLen")
		result := SanitizePhone(s, maxLen)

		for _, r := range result {
			if !isAllowedPhoneCharTest(r) {
				t.Fatalf("SanitizePhone(%q, %d) = %q; contains disallowed rune %U (%c)",
					s, maxLen, result, r, r)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Property 5: SanitizeIdentifier rejects reserved identifiers
// ---------------------------------------------------------------------------

// Feature: external-identity-sanitization, Property 5: SanitizeIdentifier rejects reserved values
func TestPropertySanitizeIdentifierRejectsReservedValues(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := rapid.String().Draw(t, "s")
		maxLen := rapid.IntRange(1, 512).Draw(t, "maxLen")
		result := SanitizeIdentifier(s, maxLen)
		sanitized := SanitizeString(s, maxLen)

		_, reserved := reservedUserIdentifiers[sanitized]
		if reserved {
			if result != "" {
				t.Fatalf("SanitizeIdentifier(%q, %d) = %q; expected empty string when sanitized is reserved",
					s, maxLen, result)
			}
		} else {
			if result != sanitized {
				t.Fatalf("SanitizeIdentifier(%q, %d) = %q; expected %q (== SanitizeString result)",
					s, maxLen, result, sanitized)
			}
		}
	})
}
