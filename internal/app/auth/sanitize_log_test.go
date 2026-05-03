package auth

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/h44z/wg-portal/internal/testutil"
)

// captureWarnLogsInline redirects the default slog logger to a buffer, calls fn,
// restores the original logger, and returns the captured log records.
// Unlike captureWarnLogs, this does not require *testing.T so it can be used inside rapid callbacks.
func captureWarnLogsInline(fn func()) []map[string]any {
	original := slog.Default()
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	slog.SetDefault(slog.New(handler))

	fn()

	slog.SetDefault(original)

	var records []map[string]any
	decoder := json.NewDecoder(&buf)
	for decoder.More() {
		var rec map[string]any
		if err := decoder.Decode(&rec); err == nil {
			records = append(records, rec)
		}
	}
	return records
}

// ---------------------------------------------------------------------------
// Property 7: Sanitization change logging completeness
// ---------------------------------------------------------------------------

// Feature: external-identity-sanitization, Property 7: Sanitization change logging completeness
func TestPropertySanitizationChangeLoggingCompleteness(t *testing.T) {
	mapping := makeOauthFieldMapping()
	adminMapping := &config.OauthAdminMapping{}

	rapid.Check(t, func(t *rapid.T) {
		// Generate arbitrary field values
		sub := rapid.StringMatching(`[a-zA-Z0-9_@.-]{1,50}`).Draw(t, "sub")
		email := rapid.String().Draw(t, "email")
		firstname := rapid.String().Draw(t, "firstname")
		lastname := rapid.String().Draw(t, "lastname")
		phone := rapid.String().Draw(t, "phone")
		department := rapid.String().Draw(t, "department")

		if sub == "" {
			sub = "testuser"
		}

		raw := makeOauthRaw(sub, email, firstname, lastname, phone, department)

		// Count how many fields will actually change after sanitization
		expectedChanges := 0
		if domain.SanitizeIdentifier(sub, 256) != sub {
			expectedChanges++
		}
		if domain.SanitizeEmail(email, 254) != email {
			expectedChanges++
		}
		if domain.SanitizeString(firstname, 128) != firstname {
			expectedChanges++
		}
		if domain.SanitizeString(lastname, 128) != lastname {
			expectedChanges++
		}
		if domain.SanitizePhone(phone, 50) != phone {
			expectedChanges++
		}
		if domain.SanitizeString(department, 128) != department {
			expectedChanges++
		}

		var records []map[string]any
		records = captureWarnLogsInline(func() {
			_, _ = parseOauthUserInfo(mapping, adminMapping, raw, true, "oauth", "test-provider")
		})

		actualWarnCount := testutil.CountWarnEntries(records)
		require.Equal(t, expectedChanges, actualWarnCount,
			"number of WARN log entries (%d) must equal number of fields changed by sanitization (%d)",
			actualWarnCount, expectedChanges)
	})
}
