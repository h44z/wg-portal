package users

import (
	"errors"
	"testing"

	"github.com/go-ldap/ldap/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/h44z/wg-portal/internal/testutil"
)

// makeTestLdapFields returns a minimal LdapFields config for testing.
func makeTestLdapFields() *config.LdapFields {
	return &config.LdapFields{
		BaseFields: config.BaseFields{
			UserIdentifier: "uid",
			Email:          "mail",
			Firstname:      "givenName",
			Lastname:       "sn",
			Phone:          "telephoneNumber",
			Department:     "department",
		},
		GroupMembership: "memberOf",
	}
}

// makeTestAdminGroupDN returns a parsed DN for testing (a non-matching group).
func makeTestAdminGroupDN(t *testing.T) *ldap.DN {
	t.Helper()
	dn, err := ldap.ParseDN("cn=admins,dc=example,dc=com")
	require.NoError(t, err)
	return dn
}

// makeRawLdapUser builds a raw LDAP user map for convertRawLdapUser.
// memberOf is set to an empty [][]byte (no group memberships).
func makeRawLdapUser(uid, mail, givenName, sn, phone, department string) map[string]any {
	return map[string]any{
		"uid":             uid,
		"mail":            mail,
		"givenName":       givenName,
		"sn":              sn,
		"telephoneNumber": phone,
		"department":      department,
		"memberOf":        [][]byte{}, // no group memberships
	}
}

// ---------------------------------------------------------------------------
// convertRawLdapUser sanitization wiring
// ---------------------------------------------------------------------------

// Test: sanitize=true, identifier "all" → returns ErrInvalidData,
// one WARN log entry with field: "identifier" and cleared indication.
func TestConvertRawLdapUser_SanitizeTrue_IdentifierAll(t *testing.T) {
	fields := makeTestLdapFields()
	adminGroupDN := makeTestAdminGroupDN(t)
	raw := makeRawLdapUser("all", "all@example.com", "Alice", "Smith", "", "")

	restore := testutil.CaptureWarnLogs(t)
	user, err := convertRawLdapUser("test-ldap", raw, fields, adminGroupDN, true)
	records := restore()

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInvalidData), "expected ErrInvalidData when identifier is 'all'")
	assert.Nil(t, user)

	// Expect exactly one WARN entry for the identifier field
	warnCount := testutil.CountWarnEntries(records)
	assert.Equal(t, 1, warnCount, "expected exactly one WARN log entry")

	rec, found := testutil.FindWarnWithField(records, "identifier")
	assert.True(t, found, "expected WARN log entry with field=identifier")
	if found {
		// The message should indicate the field was cleared (not just modified)
		msg, _ := rec["msg"].(string)
		assert.Contains(t, msg, "cleared", "expected 'cleared' in log message when identifier is cleared")
	}
}

// Test: sanitize=false, identifier "all" → returns user with identifier "all", no WARN log entries.
func TestConvertRawLdapUser_SanitizeFalse_IdentifierAll(t *testing.T) {
	fields := makeTestLdapFields()
	adminGroupDN := makeTestAdminGroupDN(t)
	raw := makeRawLdapUser("all", "all@example.com", "Alice", "Smith", "", "")

	restore := testutil.CaptureWarnLogs(t)
	user, err := convertRawLdapUser("test-ldap", raw, fields, adminGroupDN, false)
	records := restore()

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, domain.UserIdentifier("all"), user.Identifier, "identifier should be 'all' when sanitization is disabled")

	warnCount := testutil.CountWarnEntries(records)
	assert.Equal(t, 0, warnCount, "expected no WARN log entries when sanitization is disabled")
}

// ---------------------------------------------------------------------------
// non-identifier field sanitization in convertRawLdapUser
// ---------------------------------------------------------------------------

// Test: sanitize=true, firstname contains \x00 → output firstname has null byte removed,
// one WARN log entry with field: "firstname".
func TestConvertRawLdapUser_SanitizeTrue_NullByteInFirstname(t *testing.T) {
	fields := makeTestLdapFields()
	adminGroupDN := makeTestAdminGroupDN(t)
	raw := makeRawLdapUser("alice", "alice@example.com", "Ali\x00ce", "Smith", "", "")

	restore := testutil.CaptureWarnLogs(t)
	user, err := convertRawLdapUser("test-ldap", raw, fields, adminGroupDN, true)
	records := restore()

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.NotContains(t, user.Firstname, "\x00", "firstname should have null byte removed")
	assert.Equal(t, "Alice", user.Firstname)

	warnCount := testutil.CountWarnEntries(records)
	assert.Equal(t, 1, warnCount, "expected exactly one WARN log entry")

	rec, found := testutil.FindWarnWithField(records, "firstname")
	assert.True(t, found, "expected WARN log entry with field=firstname")
	if found {
		assert.Equal(t, "WARN", rec["level"])
	}
}

// Test: sanitize=true, all fields clean → no WARN log entries emitted.
func TestConvertRawLdapUser_SanitizeTrue_AllFieldsClean(t *testing.T) {
	fields := makeTestLdapFields()
	adminGroupDN := makeTestAdminGroupDN(t)
	raw := makeRawLdapUser("alice", "alice@example.com", "Alice", "Smith", "+1 555-1234", "Engineering")

	restore := testutil.CaptureWarnLogs(t)
	user, err := convertRawLdapUser("test-ldap", raw, fields, adminGroupDN, true)
	records := restore()

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, domain.UserIdentifier("alice"), user.Identifier)

	warnCount := testutil.CountWarnEntries(records)
	assert.Equal(t, 0, warnCount, "expected no WARN log entries when all fields are clean")
}

// Test: sanitize=false, firstname contains \x00 → output firstname unchanged, no WARN log entries.
func TestConvertRawLdapUser_SanitizeFalse_NullByteInFirstname(t *testing.T) {
	fields := makeTestLdapFields()
	adminGroupDN := makeTestAdminGroupDN(t)
	raw := makeRawLdapUser("alice", "alice@example.com", "Ali\x00ce", "Smith", "", "")

	restore := testutil.CaptureWarnLogs(t)
	user, err := convertRawLdapUser("test-ldap", raw, fields, adminGroupDN, false)
	records := restore()

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, "Ali\x00ce", user.Firstname, "firstname should be unchanged when sanitization is disabled")

	warnCount := testutil.CountWarnEntries(records)
	assert.Equal(t, 0, warnCount, "expected no WARN log entries when sanitization is disabled")
}

func TestLdapUserIdentifier_SanitizeTrue_NormalizesSyncComparisons(t *testing.T) {
	raw := map[string]any{"uid": " alice\x00 "}

	got := ldapUserIdentifier(raw, "uid", true)

	assert.Equal(t, domain.UserIdentifier("alice"), got)
}

func TestLdapUserIdentifier_SanitizeFalseKeepsRawValue(t *testing.T) {
	raw := map[string]any{"uid": " alice\x00 "}

	got := ldapUserIdentifier(raw, "uid", false)

	assert.Equal(t, domain.UserIdentifier(" alice\x00 "), got)
}

func TestLdapUserIdentifier_SanitizeTrueRejectsReservedIdentifier(t *testing.T) {
	raw := map[string]any{"uid": " all "}

	got := ldapUserIdentifier(raw, "uid", true)

	assert.Empty(t, got)
}
