package ldap

type Config struct {
	URL      string `yaml:"url" envconfig:"LDAP_URL"`
	StartTLS bool   `yaml:"startTLS" envconfig:"LDAP_STARTTLS"`
	BaseDN   string `yaml:"dn" envconfig:"LDAP_BASEDN"`
	BindUser string `yaml:"user" envconfig:"LDAP_USER"`
	BindPass string `yaml:"pass" envconfig:"LDAP_PASSWORD"`
}
