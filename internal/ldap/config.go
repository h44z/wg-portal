package ldap

import (
	gldap "github.com/go-ldap/ldap/v3"
)

type Type string

const (
	TypeActiveDirectory Type = "AD"
	TypeOpenLDAP        Type = "OpenLDAP"
)

type Config struct {
	URL            string `yaml:"url" envconfig:"LDAP_URL"`
	StartTLS       bool   `yaml:"startTLS" envconfig:"LDAP_STARTTLS"`
	CertValidation bool   `yaml:"certcheck" envconfig:"LDAP_CERT_VALIDATION"`
	BaseDN         string `yaml:"dn" envconfig:"LDAP_BASEDN"`
	BindUser       string `yaml:"user" envconfig:"LDAP_USER"`
	BindPass       string `yaml:"pass" envconfig:"LDAP_PASSWORD"`

	EmailAttribute       string `yaml:"attrEmail" envconfig:"LDAP_ATTR_EMAIL"`
	FirstNameAttribute   string `yaml:"attrFirstname" envconfig:"LDAP_ATTR_FIRSTNAME"`
	LastNameAttribute    string `yaml:"attrLastname" envconfig:"LDAP_ATTR_LASTNAME"`
	PhoneAttribute       string `yaml:"attrPhone" envconfig:"LDAP_ATTR_PHONE"`
	GroupMemberAttribute string `yaml:"attrGroups" envconfig:"LDAP_ATTR_GROUPS"`

	LoginFilter     string    `yaml:"loginFilter" envconfig:"LDAP_LOGIN_FILTER"` // {{login_identifier}} gets replaced with the login email address
	SyncFilter      string    `yaml:"syncFilter" envconfig:"LDAP_SYNC_FILTER"`
	AdminLdapGroup  string    `yaml:"adminGroup" envconfig:"LDAP_ADMIN_GROUP"` // Members of this group receive admin rights in WG-Portal
	AdminLdapGroup_ *gldap.DN `yaml:"-"`
	LdapCertConn    bool      `yaml:"ldapCertConn" envconfig:"LDAP_CERT_CONN"`
	LdapTlsCert     string    `yaml:"ldapTlsCert" envconfig:"LDAPTLS_CERT"`
	LdapTlsKey      string    `yaml:"ldapTlsKey" envconfig:"LDAPTLS_KEY"`
}
