package handlers

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
