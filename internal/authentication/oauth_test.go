package authentication

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_getOauthFieldMapping(t *testing.T) {
	defaultFields := OauthFields{
		BaseFields: BaseFields{
			UserIdentifier: "sub",
			Email:          "email",
			Firstname:      "given_name",
			Lastname:       "family_name",
			Phone:          "phone",
			Department:     "department",
		},
		IsAdmin: "admin_flag",
	}

	got := getOauthFieldMapping(OauthFields{})
	assert.Equal(t, defaultFields, got)

	customFields := OauthFields{
		BaseFields: BaseFields{
			UserIdentifier: "field_uid",
			Email:          "field_email",
			Firstname:      "field_fn",
			Lastname:       "field_ln",
			Phone:          "field_phone",
			Department:     "field_dep",
		},
		IsAdmin: "field_admin",
	}

	got = getOauthFieldMapping(customFields)
	assert.Equal(t, customFields, got)
}
