package authentication

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/go-ldap/ldap/v3"
)

func Test_getLdapFieldMapping(t *testing.T) {
	defaultFields := LdapFields{
		BaseFields: BaseFields{
			UserIdentifier: "mail",
			Email:          "mail",
			Firstname:      "givenName",
			Lastname:       "sn",
			Phone:          "telephoneNumber",
			Department:     "department",
		},
		GroupMembership: "memberOf",
	}

	got := getLdapFieldMapping(LdapFields{})
	assert.Equal(t, defaultFields, got)

	customFields := LdapFields{
		BaseFields: BaseFields{
			UserIdentifier: "field_uid",
			Email:          "field_email",
			Firstname:      "field_fn",
			Lastname:       "field_ln",
			Phone:          "field_phone",
			Department:     "field_dep",
		},
		GroupMembership: "field_member",
	}

	got = getLdapFieldMapping(customFields)
	assert.Equal(t, customFields, got)
}

func Test_userIsInAdminGroup(t *testing.T) {
	adminDN, _ := ldap.ParseDN("CN=admin,OU=groups,DC=TEST,DC=COM")

	tests := []struct {
		name      string
		groupData [][]byte
		want      bool
		wantErr   bool
	}{
		{
			name:      "NoGroups",
			groupData: nil,
			want:      false,
			wantErr:   false,
		},
		{
			name:      "WrongGroups",
			groupData: [][]byte{[]byte("cn=wrong,dc=group"), []byte("CN=wrong2,OU=groups,DC=TEST,DC=COM")},
			want:      false,
			wantErr:   false,
		},
		{
			name:      "CorrectGroups",
			groupData: [][]byte{[]byte("CN=admin,OU=groups,DC=TEST,DC=COM")},
			want:      true,
			wantErr:   false,
		},
		{
			name:      "CorrectGroupsCase",
			groupData: [][]byte{[]byte("cn=admin,OU=groups,dc=TEST,DC=COM")},
			want:      true,
			wantErr:   false,
		},
		{
			name:      "WrongDN",
			groupData: [][]byte{[]byte("i_am_invalid")},
			want:      false,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := userIsInAdminGroup(tt.groupData, adminDN)
			if (err != nil) != tt.wantErr {
				t.Errorf("userIsInAdminGroup() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("userIsInAdminGroup() got = %v, want %v", got, tt.want)
			}
		})
	}
}
