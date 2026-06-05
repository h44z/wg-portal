package auth

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/h44z/wg-portal/internal/testutil"
)

// makeOauthFieldMapping returns a minimal OauthFields mapping for testing.
func makeOauthFieldMapping() config.OauthFields {
	return config.OauthFields{
		BaseFields: config.BaseFields{
			UserIdentifier: "sub",
			Email:          "email",
			Firstname:      "given_name",
			Lastname:       "family_name",
			Phone:          "phone",
			Department:     "department",
		},
	}
}

// makeOauthRaw builds a minimal raw OAuth user info map.
func makeOauthRaw(sub, email, givenName, familyName, phone, department string) map[string]any {
	return map[string]any{
		"sub":         sub,
		"email":       email,
		"given_name":  givenName,
		"family_name": familyName,
		"phone":       phone,
		"department":  department,
	}
}

// Test: email containing \r\n → output email is "",
// one WARN log entry with field: "email" and cleared indication.
func TestParseOauthUserInfo_CRLFInEmail(t *testing.T) {
	mapping := makeOauthFieldMapping()
	adminMapping := &config.OauthAdminMapping{}
	raw := makeOauthRaw("user123", "user\r\n@example.com", "Alice", "Smith", "", "")

	restore := testutil.CaptureWarnLogs(t)
	info, err := parseOauthUserInfo(mapping, adminMapping, raw, "oauth", "test-provider")
	records := restore()

	require.NoError(t, err)
	assert.Equal(t, "", info.Email, "email should be cleared when it contains CR/LF")

	warnCount := testutil.CountWarnEntries(records)
	assert.Equal(t, 1, warnCount, "expected exactly one WARN log entry")

	rec, found := testutil.FindWarnWithField(records, "email")
	assert.True(t, found, "expected WARN log entry with field=email")
	if found {
		msg, _ := rec["msg"].(string)
		assert.Contains(t, msg, "cleared", "expected 'cleared' in log message when email is cleared")
	}
}

// Test: two fields modified (email cleared, firstname truncated) →
// two separate WARN log entries.
func TestParseOauthUserInfo_TwoFieldsModified(t *testing.T) {
	mapping := makeOauthFieldMapping()
	adminMapping := &config.OauthAdminMapping{}

	longFirstname := strings.Repeat("A", 200)
	raw := makeOauthRaw("user123", "bad\r\nemail@example.com", longFirstname, "Smith", "", "")

	restore := testutil.CaptureWarnLogs(t)
	info, err := parseOauthUserInfo(mapping, adminMapping, raw, "oauth", "test-provider")
	records := restore()

	require.NoError(t, err)
	assert.Equal(t, "", info.Email, "email should be cleared")
	assert.Equal(t, 128, len([]rune(info.Firstname)), "firstname should be truncated to 128 runes")

	warnCount := testutil.CountWarnEntries(records)
	assert.Equal(t, 2, warnCount, "expected exactly two WARN log entries (one per modified field)")

	_, emailFound := testutil.FindWarnWithField(records, "email")
	assert.True(t, emailFound, "expected WARN log entry with field=email")

	_, firstnameFound := testutil.FindWarnWithField(records, "firstname")
	assert.True(t, firstnameFound, "expected WARN log entry with field=firstname")
}

// Test: identifier "all" → returns ErrInvalidData.
func TestParseOauthUserInfo_IdentifierAll(t *testing.T) {
	mapping := makeOauthFieldMapping()
	adminMapping := &config.OauthAdminMapping{}
	raw := makeOauthRaw("all", "all@example.com", "Alice", "Smith", "", "")

	restore := testutil.CaptureWarnLogs(t)
	_, err := parseOauthUserInfo(mapping, adminMapping, raw, "oauth", "test-provider")
	_ = restore()

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInvalidData), "expected ErrInvalidData when identifier is 'all'")
}

func TestParseOauthUserInfo_DropsModifiedGroupBeforeAdminMatch(t *testing.T) {
	mapping := makeOauthFieldMapping()
	mapping.UserGroups = "groups"
	adminMapping := &config.OauthAdminMapping{
		AdminGroupRegex: "^wgportal-admins$",
	}
	raw := makeOauthRaw("user123", "user@example.com", "Alice", "Smith", "", "")
	raw["groups"] = []any{"wgportal-\u200badmins"}

	restore := testutil.CaptureWarnLogs(t)
	info, err := parseOauthUserInfo(mapping, adminMapping, raw, "oidc", "test-provider")
	records := restore()

	require.NoError(t, err)
	require.NotNil(t, info)
	assert.False(t, info.IsAdmin, "sanitization must not repair a modified group into an admin match")
	assert.Empty(t, info.UserGroups)

	rec, found := testutil.FindWarnWithField(records, "user_group")
	assert.True(t, found, "expected WARN log entry with field=user_group")
	if found {
		assert.Equal(t, "oidc", rec["provider_type"])
	}
}

func TestParseOauthUserInfo_AllowsWhitespaceOnlyGroupTrim(t *testing.T) {
	mapping := makeOauthFieldMapping()
	mapping.UserGroups = "groups"
	adminMapping := &config.OauthAdminMapping{
		AdminGroupRegex: "^wgportal-admins$",
	}
	raw := makeOauthRaw("user123", "user@example.com", "Alice", "Smith", "", "")
	raw["groups"] = []any{" wgportal-admins "}

	info, err := parseOauthUserInfo(mapping, adminMapping, raw, "oidc", "test-provider")

	require.NoError(t, err)
	require.NotNil(t, info)
	assert.True(t, info.IsAdmin)
	assert.Equal(t, []string{"wgportal-admins"}, info.UserGroups)
}
