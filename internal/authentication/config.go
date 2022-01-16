package authentication

import (
	"github.com/go-ldap/ldap/v3"
)

type BaseFields struct {
	UserIdentifier string `yaml:"user_identifier"`
	Email          string `yaml:"email"`
	Firstname      string `yaml:"firstname"`
	Lastname       string `yaml:"lastname"`
	Phone          string `yaml:"phone"`
	Department     string `yaml:"department"`
}

type OauthFields struct {
	BaseFields `yaml:",inline"`
	IsAdmin    string `yaml:"is_admin"`
}

type LdapFields struct {
	BaseFields      `yaml:",inline"`
	GroupMembership string `yaml:"memberof"`
}

type LdapProvider struct {
	URL            string `yaml:"url"`
	StartTLS       bool   `yaml:"start_tls"`
	CertValidation bool   `yaml:"cert_validation"`
	BaseDN         string `yaml:"base_dn"`
	BindUser       string `yaml:"bind_user"`
	BindPass       string `yaml:"bind_pass"`

	FieldMap LdapFields `yaml:"field_map"`

	LoginFilter  string   `yaml:"login_filter"` // {{login_identifier}} gets replaced with the login email address
	AdminGroupDN string   `yaml:"admin_group"`  // Members of this group receive admin rights in WG-Portal
	adminGroupDN *ldap.DN `yaml:"-"`

	Synchronize bool `yaml:"synchronize"`

	// If DeleteMissing is false, missing users will be deactivated
	DeleteMissing bool   `yaml:"delete_missing"`
	SyncFilter    string `yaml:"sync_filter"`

	// If RegistrationEnabled is set to true, wg-portal will create new users that do not exist in the database.
	RegistrationEnabled bool `yaml:"registration_enabled"`
}

type OpenIDConnectProvider struct {
	// ProviderName is an internal name that is used to distinguish oauth endpoints. It must not contain spaces or special characters.
	ProviderName string `yaml:"provider_name"`

	// DisplayName is shown to the user on the login page. If it is empty, ProviderName will be displayed.
	DisplayName string `yaml:"display_name"`

	BaseUrl string `yaml:"base_url"`

	// ClientID is the application's ID.
	ClientID string `yaml:"client_id"`

	// ClientSecret is the application's secret.
	ClientSecret string `yaml:"client_secret"`

	// ExtraScopes specifies optional requested permissions.
	ExtraScopes []string `yaml:"extra_scopes"`

	// FieldMap is used to map the names of the user-info endpoint fields to wg-portal fields
	FieldMap OauthFields `yaml:"field_map"`

	// If RegistrationEnabled is set to true, missing users will be created in the database
	RegistrationEnabled bool `yaml:"registration_enabled"`
}

type OAuthProvider struct {
	// ProviderName is an internal name that is used to distinguish oauth endpoints. It must not contain spaces or special characters.
	ProviderName string `yaml:"provider_name"`

	// DisplayName is shown to the user on the login page. If it is empty, ProviderName will be displayed.
	DisplayName string `yaml:"display_name"`

	BaseUrl string `yaml:"base_url"`

	// ClientID is the application's ID.
	ClientID string `yaml:"client_id"`

	// ClientSecret is the application's secret.
	ClientSecret string `yaml:"client_secret"`

	AuthURL     string `yaml:"auth_url"`
	TokenURL    string `yaml:"token_url"`
	UserInfoURL string `yaml:"user_info_url"`

	// RedirectURL is the URL to redirect users going through
	// the OAuth flow, after the resource owner's URLs.
	RedirectURL string `yaml:"redirect_url"`

	// Scope specifies optional requested permissions.
	Scopes []string `yaml:"scopes"`

	// FieldMap is used to map the names of the user-info endpoint fields to wg-portal fields
	FieldMap OauthFields `yaml:"field_map"`

	// If RegistrationEnabled is set to true, wg-portal will create new users that do not exist in the database.
	RegistrationEnabled bool `yaml:"registration_enabled"`
}
