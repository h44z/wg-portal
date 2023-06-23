package handlers

import (
	"encoding/base64"
	"strings"
)

func Base64UrlDecode(in string) string {
	in = strings.ReplaceAll(in, "-", "=")
	in = strings.ReplaceAll(in, "_", "/")
	in = strings.ReplaceAll(in, ".", "+")

	output, _ := base64.StdEncoding.DecodeString(in)
	return string(output)
}
