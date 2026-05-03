package domain

import (
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

var reservedUserIdentifiers = map[string]struct{}{
	"all":                       {},
	"new":                       {},
	"id":                        {},
	string(CtxSystemAdminId):    {},
	string(CtxUnknownUserId):    {},
	string(CtxSystemLdapSyncer): {},
	string(CtxSystemWgImporter): {},
	string(CtxSystemV1Migrator): {},
	string(CtxSystemDBMigrator): {},
}

// SanitizeString normalizes to NFC, trims leading and trailing whitespace, strips Unicode
// control and format characters, drops invalid UTF-8 bytes, and truncates the result to
// maxLen runes. If maxLen <= 0, returns "".
func SanitizeString(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}

	s = norm.NFC.String(strings.TrimSpace(s))

	var b strings.Builder
	b.Grow(len(s))
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		if r == utf8.RuneError && size == 1 {
			continue
		}
		if !unicode.IsControl(r) && !unicode.Is(unicode.Cf, r) {
			b.WriteRune(r)
		}
	}
	s = b.String()

	if utf8.RuneCountInString(s) > maxLen {
		runes := []rune(s)
		s = string(runes[:maxLen])
	}

	return strings.TrimSpace(s)
}

// SanitizeEmail applies SanitizeString first, then returns "" if the original s
// contains CR/LF or if the sanitized result is not a plain email address.
func SanitizeEmail(s string, maxLen int) string {
	if strings.ContainsRune(s, '\r') || strings.ContainsRune(s, '\n') {
		return ""
	}

	sanitized := SanitizeString(s, maxLen)

	if sanitized == "" || strings.Count(sanitized, "@") != 1 {
		return ""
	}
	addr, err := mail.ParseAddress(sanitized)
	if err != nil || addr.Name != "" || addr.Address != sanitized {
		return ""
	}

	return sanitized
}

// SanitizePhone applies SanitizeString first, then removes all characters not in the
// set [0-9+\-() .]. Returns "" if the result after filtering is empty.
func SanitizePhone(s string, maxLen int) string {
	sanitized := SanitizeString(s, maxLen)

	// Remove all characters not in [0-9+\-() .]
	var b strings.Builder
	b.Grow(len(sanitized))
	for _, r := range sanitized {
		if isAllowedPhoneRune(r) {
			b.WriteRune(r)
		}
	}
	result := strings.TrimSpace(b.String())

	if result == "" {
		return ""
	}

	return result
}

// isAllowedPhoneRune reports whether r is in the allowed phone character set [0-9+\-() .].
func isAllowedPhoneRune(r rune) bool {
	switch {
	case r >= '0' && r <= '9':
		return true
	case r == '+', r == '-', r == '(', r == ')', r == ' ', r == '.':
		return true
	default:
		return false
	}
}

// SanitizeIdentifier applies SanitizeString first, then returns "" if the result equals
// a reserved user identifier (case-sensitive, exact match).
func SanitizeIdentifier(s string, maxLen int) string {
	sanitized := SanitizeString(s, maxLen)

	if IsReservedUserIdentifier(UserIdentifier(sanitized)) {
		return ""
	}

	return sanitized
}

func IsReservedUserIdentifier(identifier UserIdentifier) bool {
	_, reserved := reservedUserIdentifiers[string(identifier)]
	return reserved
}
