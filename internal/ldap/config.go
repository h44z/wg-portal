package ldap

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

	Type                 Type   `yaml:"typ" envconfig:"LDAP_TYPE"` // AD for active directory, OpenLDAP for OpenLDAP
	UserClass            string `yaml:"userClass" envconfig:"LDAP_USER_CLASS"`
	EmailAttribute       string `yaml:"attrEmail" envconfig:"LDAP_ATTR_EMAIL"`
	FirstNameAttribute   string `yaml:"attrFirstname" envconfig:"LDAP_ATTR_FIRSTNAME"`
	LastNameAttribute    string `yaml:"attrLastname" envconfig:"LDAP_ATTR_LASTNAME"`
	PhoneAttribute       string `yaml:"attrPhone" envconfig:"LDAP_ATTR_PHONE"`
	GroupMemberAttribute string `yaml:"attrGroups" envconfig:"LDAP_ATTR_GROUPS"`
	DisabledAttribute    string `yaml:"attrDisabled" envconfig:"LDAP_ATTR_DISABLED"`

	AdminLdapGroup string `yaml:"adminGroup" envconfig:"LDAP_ADMIN_GROUP"` // Members of this group receive admin rights in WG-Portal
}
