package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestCompileInterfaceFilter(t *testing.T) {
	compiled, err := CompileInterfaceFilter(map[string]string{"wg0": "^vpn-(data|ops)$"})
	require.NoError(t, err)
	require.Contains(t, compiled, "wg0")
	assert.True(t, compiled["wg0"].MatchString("vpn-ops"))
	assert.False(t, compiled["wg0"].MatchString("vpn-data-old"))

	compiled, err = CompileInterfaceFilter(nil)
	require.NoError(t, err)
	assert.Empty(t, compiled)

	_, err = CompileInterfaceFilter(map[string]string{"wg0": "("})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wg0")

	for _, expr := range []string{"", "|", ".*", "^vpn$|", "(vpn)?"} {
		_, err = CompileInterfaceFilter(map[string]string{"wg0": expr})
		require.Error(t, err, "%q matches the empty string and therefore every group", expr)
	}

	_, err = CompileInterfaceFilter(map[string]string{"": "^vpn$"})
	require.Error(t, err, "an empty interface identifier is invalid")
}

func TestOauthProviderValidate(t *testing.T) {
	negative := -time.Minute
	tests := []struct {
		name    string
		oidc    OpenIDConnectProvider
		wantErr string
	}{
		{name: "no filter", oidc: OpenIDConnectProvider{ProviderName: "p"}},
		{
			name: "valid filter",
			oidc: OpenIDConnectProvider{
				InterfaceFilter: map[string]string{"wg0": "^vpn$"},
				FieldMap:        OauthFields{UserGroups: "groups"},
			},
		},
		{
			name:    "filter without groups claim",
			oidc:    OpenIDConnectProvider{InterfaceFilter: map[string]string{"wg0": "^vpn$"}},
			wantErr: "field_map.user_groups",
		},
		{
			name: "empty expression",
			oidc: OpenIDConnectProvider{
				InterfaceFilter: map[string]string{"wg0": ""},
				FieldMap:        OauthFields{UserGroups: "groups"},
			},
			wantErr: "matches every group",
		},
		{
			name:    "negative max age",
			oidc:    OpenIDConnectProvider{InterfaceFilterMaxAge: &negative},
			wantErr: "must not be negative",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oauth := OAuthProvider{
				InterfaceFilter:       tt.oidc.InterfaceFilter,
				InterfaceFilterMaxAge: tt.oidc.InterfaceFilterMaxAge,
				FieldMap:              tt.oidc.FieldMap,
			}
			for _, err := range []error{tt.oidc.Validate(), oauth.Validate()} {
				if tt.wantErr == "" {
					assert.NoError(t, err)
				} else {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestInterfaceFilterMaxAge(t *testing.T) {
	var unset OpenIDConnectProvider
	require.NoError(t, yaml.Unmarshal([]byte("provider_name: a"), &unset))
	assert.Equal(t, DefaultInterfaceFilterMaxAge, unset.GetInterfaceFilterMaxAge())

	var set OAuthProvider
	require.NoError(t, yaml.Unmarshal([]byte("interface_filter_max_age: 90m"), &set))
	assert.Equal(t, 90*time.Minute, set.GetInterfaceFilterMaxAge())

	var disabled OpenIDConnectProvider
	require.NoError(t, yaml.Unmarshal([]byte("interface_filter_max_age: 0s"), &disabled))
	assert.Equal(t, time.Duration(0), disabled.GetInterfaceFilterMaxAge())
}

func TestValidateOauthProviderNames(t *testing.T) {
	unique := Auth{
		OpenIDConnect: []OpenIDConnectProvider{{ProviderName: "google"}},
		OAuth:         []OAuthProvider{{ProviderName: "github"}},
		Ldap:          []LdapProvider{{ProviderName: "google"}}, // LDAP providers have their own namespace
	}
	assert.NoError(t, unique.validateOauthProviderNames())

	duplicate := Auth{
		OpenIDConnect: []OpenIDConnectProvider{{ProviderName: "Google"}},
		OAuth:         []OAuthProvider{{ProviderName: "google"}},
	}
	err := duplicate.validateOauthProviderNames()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "google")

	duplicateOidc := Auth{OpenIDConnect: []OpenIDConnectProvider{{ProviderName: "a"}, {ProviderName: "A"}}}
	assert.Error(t, duplicateOidc.validateOauthProviderNames())
}

func loadTestConfig(t *testing.T, auth string) error {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("auth:\n"+auth), 0600))
	t.Setenv("WG_PORTAL_CONFIG", path)
	_, err := GetConfig()
	return err
}

func TestGetConfig_ValidatesOauthProviders(t *testing.T) {
	require.NoError(t, loadTestConfig(t, `
  oidc:
    - provider_name: oidc1
      field_map:
        user_groups: groups
      interface_filter:
        wg0: "^vpn$"
      interface_filter_max_age: 8h
`))

	err := loadTestConfig(t, `
  oidc:
    - provider_name: Shared
  oauth:
    - provider_name: shared
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not unique")

	err = loadTestConfig(t, `
  oidc:
    - provider_name: oidc1
      interface_filter:
        wg0: "^vpn$"
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oidc1")

	err = loadTestConfig(t, `
  oauth:
    - provider_name: oauth1
      field_map:
        user_groups: groups
      interface_filter:
        wg0: ".*"
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oauth1")
}
