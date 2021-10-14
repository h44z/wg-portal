package common

import (
	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/h44z/wg-portal/internal/portal"
)

type Config struct {
	Core struct {
		GinDebug bool   `yaml:"ginDebug" envconfig:"GIN_DEBUG"`
		LogLevel string `yaml:"logLevel" envconfig:"LOG_LEVEL"`

		ListeningAddress string `yaml:"listeningAddress" envconfig:"LISTENING_ADDRESS"`
		SessionSecret    string `yaml:"sessionSecret" envconfig:"SESSION_SECRET"`

		ExternalUrl string `yaml:"externalUrl" envconfig:"EXTERNAL_URL"`
		Title       string `yaml:"title" envconfig:"WEBSITE_TITLE"`
		CompanyName string `yaml:"company" envconfig:"COMPANY_NAME"`

		// TODO: check...
		AdminUser     string `yaml:"adminUser" envconfig:"ADMIN_USER"` // must be an email address
		AdminPassword string `yaml:"adminPass" envconfig:"ADMIN_PASS"`

		EditableKeys            bool   `yaml:"editableKeys" envconfig:"EDITABLE_KEYS"`
		CreateDefaultPeer       bool   `yaml:"createDefaultPeer" envconfig:"CREATE_DEFAULT_PEER"`
		SelfProvisioningAllowed bool   `yaml:"selfProvisioning" envconfig:"SELF_PROVISIONING"`
		LdapEnabled             bool   `yaml:"ldapEnabled" envconfig:"LDAP_ENABLED"`
		LogoUrl                 string `yaml:"logoUrl" envconfig:"LOGO_URL"`
	} `yaml:"core"`

	Mail     portal.MailConfig          `yaml:"email"`
	Database persistence.DatabaseConfig `yaml:"database"`
}
