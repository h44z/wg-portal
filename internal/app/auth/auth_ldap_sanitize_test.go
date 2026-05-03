package auth

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/h44z/wg-portal/internal/testutil"
)

// makeLdapAuthenticator creates a minimal LdapAuthenticator for testing ParseUserInfo.
// It does not connect to any LDAP server; only ParseUserInfo is exercised.
func makeLdapAuthenticator(sanitize bool) *LdapAuthenticator {
	return &LdapAuthenticator{
		cfg: &config.LdapProvider{
			ProviderName: "test-ldap",
			FieldMap: config.LdapFields{
				BaseFields: config.BaseFields{
					UserIdentifier: "uid",
					Email:          "mail",
					Firstname:      "givenName",
					Lastname:       "sn",
					Phone:          "telephoneNumber",
					Department:     "department",
				},
				GroupMembership: "", // no group membership check
			},
			SanitizeUserData: sanitize,
		},
	}
}

// makeRawLdapMap builds a minimal raw LDAP attribute map for ParseUserInfo.
func makeRawLdapMap(uid, mail, givenName, sn, phone, department string) map[string]any {
	return map[string]any{
		"uid":             uid,
		"mail":            mail,
		"givenName":       givenName,
		"sn":              sn,
		"telephoneNumber": phone,
		"department":      department,
	}
}

// ---------------------------------------------------------------------------
// LdapAuthenticator.ParseUserInfo sanitization wiring
// ---------------------------------------------------------------------------

// Test: SanitizeUserData=true, firstname contains \x00 → output firstname has no null byte,
// one WARN log entry with field: "firstname".
func TestLdapParseUserInfo_SanitizeTrue_NullByteInFirstname(t *testing.T) {
	auth := makeLdapAuthenticator(true)
	raw := makeRawLdapMap("alice", "alice@example.com", "Ali\x00ce", "Smith", "", "")

	restore := testutil.CaptureWarnLogs(t)
	info, err := auth.ParseUserInfo(raw)
	records := restore()

	require.NoError(t, err)
	assert.NotContains(t, info.Firstname, "\x00", "firstname should have null byte removed")
	assert.Equal(t, "Alice", info.Firstname)

	warnCount := testutil.CountWarnEntries(records)
	assert.Equal(t, 1, warnCount, "expected exactly one WARN log entry")

	rec, found := testutil.FindWarnWithField(records, "firstname")
	assert.True(t, found, "expected WARN log entry with field=firstname")
	if found {
		assert.Equal(t, "WARN", rec["level"])
	}
}

// Test: SanitizeUserData=true, all fields clean → no WARN log entries emitted.
func TestLdapParseUserInfo_SanitizeTrue_AllFieldsClean(t *testing.T) {
	auth := makeLdapAuthenticator(true)
	raw := makeRawLdapMap("alice", "alice@example.com", "Alice", "Smith", "+1 555-1234", "Engineering")

	restore := testutil.CaptureWarnLogs(t)
	info, err := auth.ParseUserInfo(raw)
	records := restore()

	require.NoError(t, err)
	assert.Equal(t, domain.UserIdentifier("alice"), info.Identifier)

	warnCount := testutil.CountWarnEntries(records)
	assert.Equal(t, 0, warnCount, "expected no WARN log entries when all fields are clean")
}

// Test: SanitizeUserData=false, firstname contains \x00 → output firstname unchanged, no WARN log entries.
func TestLdapParseUserInfo_SanitizeFalse_NullByteInFirstname(t *testing.T) {
	auth := makeLdapAuthenticator(false)
	raw := makeRawLdapMap("alice", "alice@example.com", "Ali\x00ce", "Smith", "", "")

	restore := testutil.CaptureWarnLogs(t)
	info, err := auth.ParseUserInfo(raw)
	records := restore()

	require.NoError(t, err)
	assert.Equal(t, "Ali\x00ce", info.Firstname, "firstname should be unchanged when sanitization is disabled")

	warnCount := testutil.CountWarnEntries(records)
	assert.Equal(t, 0, warnCount, "expected no WARN log entries when sanitization is disabled")
}

// Test: SanitizeUserData=true, identifier is "all" → returns ErrInvalidData.
func TestLdapParseUserInfo_SanitizeTrue_IdentifierAll(t *testing.T) {
	auth := makeLdapAuthenticator(true)
	raw := makeRawLdapMap("all", "all@example.com", "Alice", "Smith", "", "")

	restore := testutil.CaptureWarnLogs(t)
	_, err := auth.ParseUserInfo(raw)
	_ = restore()

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInvalidData), "expected ErrInvalidData when identifier is 'all'")
}
