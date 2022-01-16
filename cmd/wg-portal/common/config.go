package common

import (
	"os"

	"github.com/h44z/wg-portal/internal/authentication"
	"github.com/h44z/wg-portal/internal/core"
	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Core struct {
		GinDebug bool   `yaml:"gin_debug"`
		LogLevel string `yaml:"log_level"`

		ListeningAddress string `yaml:"listening_address"`
		SessionSecret    string `yaml:"session_secret"`

		ExternalUrl string `yaml:"external_url"`
		Title       string `yaml:"title"`
		CompanyName string `yaml:"company"`

		// AdminUser defines the default administrator account that will be created
		AdminUser     string `yaml:"admin_user"` // must be an email address
		AdminPassword string `yaml:"admin_password"`

		EditableKeys            bool   `yaml:"editable_keys"`
		CreateDefaultPeer       bool   `yaml:"create_default_peer"`
		SelfProvisioningAllowed bool   `yaml:"self_provisioning_allowed"`
		LdapEnabled             bool   `yaml:"ldap_enabled"`
		LogoUrl                 string `yaml:"logo_url"`
	} `yaml:"core"`

	Auth struct {
		OpenIDConnect []authentication.OpenIDConnectProvider `yaml:"oidc"`
		OAuth         []authentication.OAuthProvider         `yaml:"oauth"`
		Ldap          []authentication.LdapProvider          `yaml:"ldap"`
	} `yaml:"auth"`

	Mail     core.MailConfig            `yaml:"email"`
	Database persistence.DatabaseConfig `yaml:"database"`
}

func LoadConfigFile(cfg interface{}, filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return errors.WithMessage(err, "failed to open file")
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(cfg)
	if err != nil {
		return errors.WithMessage(err, "failed to decode config file")
	}

	return nil
}
