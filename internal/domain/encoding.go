package domain

import (
	"encoding/base64"
	"strings"
)

// Base64UrlDecode decodes a base64 url encoded string.
// In comparison to the standard base64 encoding, the url encoding uses - instead of + and _ instead of /
// as well as . instead of =.
func Base64UrlDecode(in string) string {
	in = strings.ReplaceAll(in, "-", "=")
	in = strings.ReplaceAll(in, "_", "/")
	in = strings.ReplaceAll(in, ".", "+")

	output, _ := base64.StdEncoding.DecodeString(in)
	return string(output)
}

// Base64UrlEncode encodes the given input using the URL-safe base64 variant that the WireGuard Portal
// API expects. In comparison to the standard base64 encoding, it uses . instead of +, _ instead of /
// and - instead of =.
func Base64UrlEncode(in string) string {
	out := base64.StdEncoding.EncodeToString([]byte(in))
	out = strings.ReplaceAll(out, "+", ".")
	out = strings.ReplaceAll(out, "/", "_")
	out = strings.ReplaceAll(out, "=", "-")
	return out
}
