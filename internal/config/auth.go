package config

import (
	"log/slog"
	"regexp"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// Auth contains all authentication providers.
type Auth struct {
	// OpenIDConnect contains a list of OpenID Connect providers.
	OpenIDConnect []OpenIDConnectProvider `yaml:"oidc"`
	// OAuth contains a list of plain OAuth providers.
	OAuth []OAuthProvider `yaml:"oauth"`
	// Ldap contains a list of LDAP providers.
	Ldap []LdapProvider `yaml:"ldap"`
	// Webauthn contains the configuration for the WebAuthn authenticator.
	WebAuthn WebauthnConfig `yaml:"webauthn"`
	// MinPasswordLength is the minimum password length for user accounts. This also applies to the admin user.
	// It is encouraged to set this value to at least 16 characters.
	MinPasswordLength int `yaml:"min_password_length"`
}

// BaseFields contains the basic fields that are used to map user information from the authentication providers.
type BaseFields struct {
	// UserIdentifier is the name of the field that contains the user identifier.
	UserIdentifier string `yaml:"user_identifier"`
	// Email is the name of the field that contains the user's email address.
	Email string `yaml:"email"`
	// Firstname is the name of the field that contains the user's first name.
	Firstname string `yaml:"firstname"`
	// Lastname is the name of the field that contains the user's last name.
	Lastname string `yaml:"lastname"`
	// Phone is the name of the field that contains the user's phone number.
	Phone string `yaml:"phone"`
	// Department is the name of the field that contains the user's department.
	Department string `yaml:"department"`
}

// OauthFields contains extra fields that are used to map user information from OAuth providers.
type OauthFields struct {
	BaseFields `yaml:",inline"`
	// IsAdmin is the name of the field that contains the admin flag.
	// If the value matches the admin_value_regex, the user is an admin. See OauthAdminMapping for more details.
	IsAdmin string `yaml:"is_admin"`
	// UserGroups is the name of the field that contains the user's groups.
	// If the value matches the admin_group_regex, the user is an admin. See OauthAdminMapping for more details.
	UserGroups string `yaml:"user_groups"`
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

	// If any of the groups listed in the groups field matches the group specified in the admin_group_regex field,
	// the user is an admin.
	AdminGroupRegex string `yaml:"admin_group_regex"`

	// internal cache fields

	adminValueRegex *regexp.Regexp
	adminGroupRegex *regexp.Regexp
}

// GetAdminValueRegex returns the compiled regular expression for the admin_value_regex field.
// If the field is empty, the default value "^true$" is used.
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
		slog.Error("failed to compile admin_value_regex", "error", err)
		panic("failed to compile admin_value_regex")
	}
	o.adminValueRegex = adminRegex

	return o.adminValueRegex
}

// GetAdminGroupRegex returns the compiled regular expression for the admin_group_regex field.
// If the field is empty, the default value "^wg_portal_default_admin_group$" is used.
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
		slog.Error("failed to compile admin_group_regex", "error", err)
		panic("failed to compile admin_group_regex")
	}
	o.adminGroupRegex = groupRegex

	return o.adminGroupRegex
}

// LdapFields contains extra fields that are used to map user information from LDAP providers.
type LdapFields struct {
	BaseFields `yaml:",inline"`
	// GroupMembership is the name of the LDAP field that contains the groups to which the user belongs.
	GroupMembership string `yaml:"memberof"`
}

// LdapProvider contains the configuration for the LDAP connection.
type LdapProvider struct {
	// ProviderName is an internal name that is used to distinguish LDAP servers. It must not contain spaces or special characters.
	ProviderName string `yaml:"provider_name"`

	// URL is the LDAP server URL, e.g. ldap://srv-ad01.company.local:389
	URL string `yaml:"url"`
	// StartTLS specifies whether STARTTLS should be used to secure the LDAP connection
	StartTLS bool `yaml:"start_tls"`
	// CertValidation specifies whether the LDAP server's TLS certificate should be validated
	CertValidation bool `yaml:"cert_validation"`
	// TlsCertificatePath is the path to a TLS certificate if needed for LDAP connections
	TlsCertificatePath string `yaml:"tls_certificate_path"`
	// TlsKeyPath is the path to the corresponding TLS certificate key
	TlsKeyPath string `yaml:"tls_key_path"`

	// BaseDN is the base DN for user searches
	BaseDN string `yaml:"base_dn"`
	// BindUser is the bind user for LDAP. It is used to search for users.
	BindUser string `yaml:"bind_user"`
	// BindPass is the bind password for LDAP
	BindPass string `yaml:"bind_pass"`

	// FieldMap is used to map the names of the LDAP fields to wg-portal fields
	FieldMap LdapFields `yaml:"field_map"`

	// LoginFilter is used to select which users can log in.
	// Use the placeholder {{login_identifier}} to insert the username.
	LoginFilter string `yaml:"login_filter"`
	// AdminGroupDN is the DN of the group that contains the administrators.
	// Members of this group receive admin rights in wg-portal
	AdminGroupDN string `yaml:"admin_group"`
	// ParsedAdminGroupDN is the parsed version of AdminGroupDN
	ParsedAdminGroupDN *ldap.DN `yaml:"-"`

	// If DisableMissing is true, missing users will be deactivated
	DisableMissing bool `yaml:"disable_missing"`
	// If AutoReEnable is true, users that where disabled because they were missing will be re-enabled once they are found again
	AutoReEnable bool `yaml:"auto_re_enable"`
	// SyncFilter is used to select which users get synchronized into wg-portal
	SyncFilter string `yaml:"sync_filter"`
	// SyncInterval is the interval between consecutive LDAP user syncs. If it is 0, sync is disabled.
	SyncInterval time.Duration `yaml:"sync_interval"`

	// If RegistrationEnabled is set to true, wg-portal will create new users that do not exist in the database.
	RegistrationEnabled bool `yaml:"registration_enabled"`

	// If LogUserInfo is set to true, the user info retrieved from the LDAP provider will be logged in trace level.
	LogUserInfo bool `yaml:"log_user_info"`
}

// OpenIDConnectProvider contains the configuration for the OpenID Connect provider.
type OpenIDConnectProvider struct {
	// ProviderName is an internal name that is used to distinguish oauth endpoints. It must not contain spaces or special characters.
	ProviderName string `yaml:"provider_name"`

	// DisplayName is shown to the user on the login page. If it is empty, ProviderName will be displayed.
	DisplayName string `yaml:"display_name"`

	// BaseUrl is the base URL of the OIDC provider.
	BaseUrl string `yaml:"base_url"`

	// ClientID is the application's ID.
	ClientID string `yaml:"client_id"`

	// ClientSecret is the application's secret.
	ClientSecret string `yaml:"client_secret"`

	// ExtraScopes specifies optional requested permissions.
	ExtraScopes []string `yaml:"extra_scopes"`

	// AllowedDomains defines the list of allowed domains
	AllowedDomains []string `yaml:"allowed_domains"`

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

// OAuthProvider contains the configuration for the OAuth provider.
type OAuthProvider struct {
	// ProviderName is an internal name that is used to distinguish oauth endpoints. It must not contain spaces or special characters.
	ProviderName string `yaml:"provider_name"`

	// DisplayName is shown to the user on the login page. If it is empty, ProviderName will be displayed.
	DisplayName string `yaml:"display_name"`

	// ClientID is the application's ID.
	ClientID string `yaml:"client_id"`

	// ClientSecret is the application's secret.
	ClientSecret string `yaml:"client_secret"`

	// AuthURL is the URL to request OAuth user authorization.
	AuthURL string `yaml:"auth_url"`
	// TokenURL is the URL to request a token.
	TokenURL string `yaml:"token_url"`
	// UserInfoURL is the URL to request user information.
	UserInfoURL string `yaml:"user_info_url"`

	// Scope specifies optional requested permissions.
	Scopes []string `yaml:"scopes"`

	// AllowedDomains defines the list of allowed domains
	AllowedDomains []string `yaml:"allowed_domains"`

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

// WebauthnConfig contains the configuration for the WebAuthn authenticator.
type WebauthnConfig struct {
	// Enabled specifies whether WebAuthn is enabled.
	Enabled bool `yaml:"enabled"`
}
