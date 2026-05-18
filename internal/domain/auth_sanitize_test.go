package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal/testutil"
)

func TestAuthenticatorUserInfo_Sanitize_NullByteInFirstname(t *testing.T) {
	info := &AuthenticatorUserInfo{
		Identifier: "alice",
		Email:      "alice@example.com",
		Firstname:  "Ali\x00ce",
		Lastname:   "Smith",
	}

	restore := testutil.CaptureWarnLogs(t)
	err := info.Sanitize("ldap", "test-provider")
	records := restore()

	require.NoError(t, err)
	assert.Equal(t, "Alice", info.Firstname)

	warnCount := testutil.CountWarnEntries(records)
	assert.Equal(t, 1, warnCount)

	_, found := testutil.FindWarnWithField(records, "firstname")
	assert.True(t, found)
}

func TestAuthenticatorUserInfo_Sanitize_AllFieldsClean(t *testing.T) {
	info := &AuthenticatorUserInfo{
		Identifier: "alice",
		Email:      "alice@example.com",
		Firstname:  "Alice",
		Lastname:   "Smith",
		Phone:      "+1 555-1234",
		Department: "Engineering",
	}

	restore := testutil.CaptureWarnLogs(t)
	err := info.Sanitize("ldap", "test-provider")
	records := restore()

	require.NoError(t, err)
	assert.Equal(t, UserIdentifier("alice"), info.Identifier)
	assert.Equal(t, 0, testutil.CountWarnEntries(records))
}

func TestAuthenticatorUserInfo_Sanitize_IdentifierAll(t *testing.T) {
	info := &AuthenticatorUserInfo{
		Identifier: "all",
		Email:      "all@example.com",
		Firstname:  "Alice",
		Lastname:   "Smith",
	}

	err := info.Sanitize("ldap", "test-provider")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidData))
}

func TestAuthenticatorUserInfo_Sanitize_CRLFInEmail(t *testing.T) {
	info := &AuthenticatorUserInfo{
		Identifier: "user123",
		Email:      "user\r\n@example.com",
		Firstname:  "Alice",
		Lastname:   "Smith",
	}

	restore := testutil.CaptureWarnLogs(t)
	err := info.Sanitize("oauth", "test-provider")
	records := restore()

	require.NoError(t, err)
	assert.Equal(t, "", info.Email)

	_, found := testutil.FindWarnWithField(records, "email")
	assert.True(t, found)
}

func TestAuthenticatorUserInfo_Sanitize_GroupsWithZeroWidthChars(t *testing.T) {
	info := &AuthenticatorUserInfo{
		Identifier: "user123",
		Email:      "user@example.com",
		UserGroups: []string{"wgportal-\u200badmins"},
	}

	restore := testutil.CaptureWarnLogs(t)
	err := info.Sanitize("oidc", "test-provider")
	records := restore()

	require.NoError(t, err)
	assert.Empty(t, info.UserGroups)

	_, found := testutil.FindWarnWithField(records, "user_group")
	assert.True(t, found)
}
