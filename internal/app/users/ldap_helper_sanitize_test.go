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

// Test: identifier "all" → returns ErrInvalidData,
// one WARN log entry with field: "identifier" and cleared indication.
func TestConvertRawLdapUser_IdentifierAll(t *testing.T) {
	fields := makeTestLdapFields()
	adminGroupDN := makeTestAdminGroupDN(t)
	raw := makeRawLdapUser("all", "all@example.com", "Alice", "Smith", "", "")

	restore := testutil.CaptureWarnLogs(t)
	user, err := convertRawLdapUser("test-ldap", raw, fields, adminGroupDN)
	records := restore()

	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrInvalidData), "expected ErrInvalidData when identifier is 'all'")
	assert.Nil(t, user)

	warnCount := testutil.CountWarnEntries(records)
	assert.Equal(t, 1, warnCount, "expected exactly one WARN log entry")

	rec, found := testutil.FindWarnWithField(records, "identifier")
	assert.True(t, found, "expected WARN log entry with field=identifier")
	if found {
		msg, _ := rec["msg"].(string)
		assert.Contains(t, msg, "cleared", "expected 'cleared' in log message when identifier is cleared")
	}
}

// Test: firstname contains \x00 → output firstname has null byte removed,
// one WARN log entry with field: "firstname".
func TestConvertRawLdapUser_NullByteInFirstname(t *testing.T) {
	fields := makeTestLdapFields()
	adminGroupDN := makeTestAdminGroupDN(t)
	raw := makeRawLdapUser("alice", "alice@example.com", "Ali\x00ce", "Smith", "", "")

	restore := testutil.CaptureWarnLogs(t)
	user, err := convertRawLdapUser("test-ldap", raw, fields, adminGroupDN)
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

// Test: all fields clean → no WARN log entries emitted.
func TestConvertRawLdapUser_AllFieldsClean(t *testing.T) {
	fields := makeTestLdapFields()
	adminGroupDN := makeTestAdminGroupDN(t)
	raw := makeRawLdapUser("alice", "alice@example.com", "Alice", "Smith", "+1 555-1234", "Engineering")

	restore := testutil.CaptureWarnLogs(t)
	user, err := convertRawLdapUser("test-ldap", raw, fields, adminGroupDN)
	records := restore()

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, domain.UserIdentifier("alice"), user.Identifier)

	warnCount := testutil.CountWarnEntries(records)
	assert.Equal(t, 0, warnCount, "expected no WARN log entries when all fields are clean")
}

func TestLdapUserIdentifier_NormalizesSyncComparisons(t *testing.T) {
	raw := map[string]any{"uid": " alice\x00 "}

	got := ldapUserIdentifier(raw, "uid")

	assert.Equal(t, domain.UserIdentifier("alice"), got)
}

func TestLdapUserIdentifier_RejectsReservedIdentifier(t *testing.T) {
	raw := map[string]any{"uid": " all "}

	got := ldapUserIdentifier(raw, "uid")

	assert.Empty(t, got)
}
