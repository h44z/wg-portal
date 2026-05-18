package sanitize

import (
	"log/slog"

	"github.com/h44z/wg-portal/internal/domain"
)

// LogChange applies sanitizeFn to raw, logs when the value changes, and writes
// the sanitized value to dest. Raw and sanitized values are intentionally omitted.
func LogChange(
	providerType string,
	providerName string,
	field string,
	raw string,
	sanitizeFn func() string,
	dest *string,
) {
	sanitized := sanitizeFn()
	if sanitized != raw {
		message := "sanitization modified field value from external provider"
		if sanitized == "" {
			message = "sanitization cleared field value from external provider"
		}
		slog.Warn(message,
			"provider_type", domain.SanitizeString(providerType, 64),
			"provider", domain.SanitizeString(providerName, 128),
			"field", domain.SanitizeString(field, 64),
		)
	}
	*dest = sanitized
}
