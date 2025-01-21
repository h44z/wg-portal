package config

import (
	"regexp"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/sirupsen/logrus"
)

type Auth struct {
	OpenIDConnect []OpenIDConnectProvider `yaml:"oidc"`
	OAuth         []OAuthProvider         `yaml:"oauth"`
	Ldap          []LdapProvider          `yaml:"ldap"`
}

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
	IsAdmin    string `yaml:"is_admin"`    // If the value is "true", the user is an admin.
	UserGroups string `yaml:"user_groups"` // This value specifies the claim name that contains the users groups.
}

// OauthAdminMapping contains all necessary information to extract information about administrative privileges
// from the user info fields.
//
// WgPortal can grant a user admin rights by matching the value of the `is_admin` claim against a regular expression.
// Alternatively, a regular expression can be used to check if a user is member of a specific group listed in the
// `user_group` claim.
// If one of the cases evaluates to true, the user is granted admin rights.
type OauthAdminMapping struct {
	// If the regex specified in that field matches the contents of the is_admin field, the user is an admin.
	AdminValueRegex string `yaml:"admin_value_regex"`

	// If any of the groups listed in the groups field matches the group specified in the admin_group_regex field, ]
	// the user is an admin.
	AdminGroupRegex string `yaml:"admin_group_regex"`

	// internal cache fields

	adminValueRegex *regexp.Regexp
	adminGroupRegex *regexp.Regexp
}

func (o *OauthAdminMapping) GetAdminValueRegex() *regexp.Regexp {
	if o.adminValueRegex != nil {
		return o.adminValueRegex // return cached value
	}

	if o.AdminValueRegex == "" {
		o.adminValueRegex = regexp.MustCompile("^true$") // default value is "true"
		return o.adminValueRegex
	}

	adminRegex, err := regexp.Compile(o.AdminValueRegex)
	if err != nil {
		logrus.Fatalf("failed to compile admin_value_regex: %v", err)
	}
	o.adminValueRegex = adminRegex

	return o.adminValueRegex
}

func (o *OauthAdminMapping) GetAdminGroupRegex() *regexp.Regexp {
	if o.adminGroupRegex != nil {
		return o.adminGroupRegex // return cached value
	}

	if o.AdminGroupRegex == "" {
		o.adminGroupRegex = regexp.MustCompile("^wg_portal_default_admin_group$") // default value is "wg_portal_default_admin_group"
		return o.adminGroupRegex
	}

	groupRegex, err := regexp.Compile(o.AdminGroupRegex)
	if err != nil {
		logrus.Fatalf("failed to compile admin_group_regex: %v", err)
	}
	o.adminGroupRegex = groupRegex

	return o.adminGroupRegex
}

type LdapFields struct {
	BaseFields      `yaml:",inline"`
	GroupMembership string `yaml:"memberof"`
}

type LdapProvider struct {
	// ProviderName is an internal name that is used to distinguish LDAP servers. It must not contain spaces or special characters.
	ProviderName string `yaml:"provider_name"`

	URL                string `yaml:"url"`
	StartTLS           bool   `yaml:"start_tls"`
	CertValidation     bool   `yaml:"cert_validation"`
	TlsCertificatePath string `yaml:"tls_certificate_path"`
	TlsKeyPath         string `yaml:"tls_key_path"`

	BaseDN   string `yaml:"base_dn"`
	BindUser string `yaml:"bind_user"`
	BindPass string `yaml:"bind_pass"`

	FieldMap LdapFields `yaml:"field_map"`

	LoginFilter        string   `yaml:"login_filter"` // {{login_identifier}} gets replaced with the login email address / username
	AdminGroupDN       string   `yaml:"admin_group"`  // Members of this group receive admin rights in WG-Portal
	ParsedAdminGroupDN *ldap.DN `yaml:"-"`

	// If DisableMissing is true, missing users will be deactivated
	DisableMissing bool `yaml:"disable_missing"`
	// If AutoReEnable is true, users that where disabled because they were missing will be re-enabled once they are found again
	AutoReEnable bool          `yaml:"auto_re_enable"`
	SyncFilter   string        `yaml:"sync_filter"`
	SyncInterval time.Duration `yaml:"sync_interval"`

	// If RegistrationEnabled is set to true, wg-portal will create new users that do not exist in the database.
	RegistrationEnabled bool `yaml:"registration_enabled"`

	// If LogUserInfo is set to true, the user info retrieved from the LDAP provider will be logged in trace level.
	LogUserInfo bool `yaml:"log_user_info"`
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

	// AdminMapping contains all necessary information to extract information about administrative privileges
	// from the user info fields.
	AdminMapping OauthAdminMapping `yaml:"admin_mapping"`

	// If RegistrationEnabled is set to true, missing users will be created in the database
	RegistrationEnabled bool `yaml:"registration_enabled"`

	// If LogUserInfo is set to true, the user info retrieved from the OIDC provider will be logged in trace level.
	LogUserInfo bool `yaml:"log_user_info"`
}

type OAuthProvider struct {
	// ProviderName is an internal name that is used to distinguish oauth endpoints. It must not contain spaces or special characters.
	ProviderName string `yaml:"provider_name"`

	// DisplayName is shown to the user on the login page. If it is empty, ProviderName will be displayed.
	DisplayName string `yaml:"display_name"`

	// ClientID is the application's ID.
	ClientID string `yaml:"client_id"`

	// ClientSecret is the application's secret.
	ClientSecret string `yaml:"client_secret"`

	AuthURL     string `yaml:"auth_url"`
	TokenURL    string `yaml:"token_url"`
	UserInfoURL string `yaml:"user_info_url"`

	// Scope specifies optional requested permissions.
	Scopes []string `yaml:"scopes"`

	// FieldMap is used to map the names of the user-info endpoint fields to wg-portal fields
	FieldMap OauthFields `yaml:"field_map"`

	// AdminMapping contains all necessary information to extract information about administrative privileges
	// from the user info fields.
	AdminMapping OauthAdminMapping `yaml:"admin_mapping"`

	// If RegistrationEnabled is set to true, wg-portal will create new users that do not exist in the database.
	RegistrationEnabled bool `yaml:"registration_enabled"`

	// If LogUserInfo is set to true, the user info retrieved from the OAuth provider will be logged in trace level.
	LogUserInfo bool `yaml:"log_user_info"`
}
