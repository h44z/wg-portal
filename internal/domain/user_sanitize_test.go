package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal/testutil"
)

func TestUser_SanitizeExternalData_NullByteInFirstname(t *testing.T) {
	u := &User{
		Identifier: "alice",
		Email:      "alice@example.com",
		Firstname:  "Ali\x00ce",
		Lastname:   "Smith",
	}

	restore := testutil.CaptureWarnLogs(t)
	err := u.SanitizeExternalData("ldap", "test-provider")
	records := restore()

	require.NoError(t, err)
	assert.Equal(t, "Alice", u.Firstname)
	assert.Equal(t, 1, testutil.CountWarnEntries(records))

	_, found := testutil.FindWarnWithField(records, "firstname")
	assert.True(t, found)
}

func TestUser_SanitizeExternalData_IdentifierAll(t *testing.T) {
	u := &User{
		Identifier: "all",
		Email:      "all@example.com",
		Firstname:  "Alice",
		Lastname:   "Smith",
	}

	err := u.SanitizeExternalData("ldap", "test-provider")

	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidData))
}

func TestUser_SanitizeExternalData_AllFieldsClean(t *testing.T) {
	u := &User{
		Identifier: "alice",
		Email:      "alice@example.com",
		Firstname:  "Alice",
		Lastname:   "Smith",
		Phone:      "+1 555-1234",
		Department: "Engineering",
	}

	restore := testutil.CaptureWarnLogs(t)
	err := u.SanitizeExternalData("ldap", "test-provider")
	records := restore()

	require.NoError(t, err)
	assert.Equal(t, UserIdentifier("alice"), u.Identifier)
	assert.Equal(t, 0, testutil.CountWarnEntries(records))
}
