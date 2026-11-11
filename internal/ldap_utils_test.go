package internal

import (
	"testing"

	"github.com/go-ldap/ldap/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal/config"
)

func testLdapFields() *config.LdapFields {
	return &config.LdapFields{
		BaseFields:      config.BaseFields{UserIdentifier: "uid", Email: "mail"},
		GroupMembership: "memberOf",
	}
}

func testGroupDN(t *testing.T) *ldap.DN {
	t.Helper()

	dn, err := ldap.ParseDN("cn=admins,dc=example,dc=com")
	require.NoError(t, err)

	return dn
}

// testEntry builds a result the way the library's own decoder would, so the
// fixtures carry both string and raw values.
func testEntry(attrs map[string][]string) *ldap.SearchResult {
	entry := &ldap.Entry{DN: "uid=alice,dc=example,dc=com"}
	for name, values := range attrs {
		entry.Attributes = append(entry.Attributes, ldap.NewEntryAttribute(name, values))
	}

	return &ldap.SearchResult{Entries: []*ldap.Entry{entry}}
}

// A field_map naming an attribute the server does not return leaves the key in
// place with an empty value rather than omitting it. Callers rely on this when
// they decide whether a sync learned anything about who is entitled.
func TestLdapConvertEntriesKeepsKeyForMissingAttribute(t *testing.T) {
	sr := testEntry(map[string][]string{"cn": {"alice"}})

	users := LdapConvertEntries(sr, testLdapFields())

	require.Len(t, users, 1)
	identifier, present := users[0]["uid"]
	assert.True(t, present, "the identifier key must exist even when the attribute is absent")
	assert.Equal(t, "", identifier)
}

// Directories may hold several values for an attribute. Only the first is used.
func TestLdapConvertEntriesUsesFirstValue(t *testing.T) {
	sr := testEntry(map[string][]string{"mail": {"first@example.com", "second@example.com"}})

	users := LdapConvertEntries(sr, testLdapFields())

	require.Len(t, users, 1)
	assert.Equal(t, "first@example.com", users[0]["mail"])
}

// Group membership stays raw because LdapIsMemberOf parses the values as DNs.
// Converting it to strings here would break admin group detection.
func TestLdapConvertEntriesKeepsGroupMembershipRaw(t *testing.T) {
	sr := testEntry(map[string][]string{"memberOf": {"cn=admins,dc=example,dc=com"}})

	users := LdapConvertEntries(sr, testLdapFields())

	require.Len(t, users, 1)
	assert.Equal(t, [][]byte{[]byte("cn=admins,dc=example,dc=com")}, users[0]["memberOf"])
}

// Unset optional fields must not be requested, otherwise the search asks the
// server for an attribute named "".
func TestLdapSearchAttributesOmitsUnsetFields(t *testing.T) {
	attrs := LdapSearchAttributes(&config.LdapFields{
		BaseFields: config.BaseFields{UserIdentifier: "uid"},
	})

	assert.Equal(t, []string{"dn", "uid"}, attrs)
}

// The default field map maps both the identifier and the email to "mail", so the
// attribute list has to be deduplicated.
func TestLdapSearchAttributesDeduplicates(t *testing.T) {
	attrs := LdapSearchAttributes(&config.LdapFields{
		BaseFields: config.BaseFields{UserIdentifier: "mail", Email: "mail"},
	})

	assert.Equal(t, []string{"dn", "mail"}, attrs)
}

// Servers are free to return a DN with different spacing or attribute case than
// the configured admin group, so the comparison parses both sides.
func TestLdapIsMemberOfIgnoresDnFormatting(t *testing.T) {
	isMember, err := LdapIsMemberOf([][]byte{[]byte("CN=admins, DC=example, DC=com")}, testGroupDN(t))

	require.NoError(t, err)
	assert.True(t, isMember)
}

func TestLdapIsMemberOfReportsNonMember(t *testing.T) {
	isMember, err := LdapIsMemberOf([][]byte{[]byte("cn=users,dc=example,dc=com")}, testGroupDN(t))

	require.NoError(t, err)
	assert.False(t, isMember)
}

// An unparseable group value is an error rather than a silent non-match, so a
// malformed directory entry cannot quietly drop someone's admin rights.
func TestLdapIsMemberOfRejectsMalformedDn(t *testing.T) {
	_, err := LdapIsMemberOf([][]byte{[]byte("not-a-dn")}, testGroupDN(t))

	assert.Error(t, err)
}
