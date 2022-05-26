package server

import (
	"os"
	"reflect"
	"runtime"

	"github.com/h44z/wg-portal/internal/common"
	"github.com/h44z/wg-portal/internal/ldap"
	"github.com/h44z/wg-portal/internal/wireguard"
	"github.com/kelseyhightower/envconfig"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	gldap "github.com/go-ldap/ldap/v3"
)

var ErrInvalidSpecification = errors.New("specification must be a struct pointer")

// loadConfigFile parses yaml files. It uses yaml annotation to store the data in a struct.
func loadConfigFile(cfg interface{}, filename string) error {
	s := reflect.ValueOf(cfg)

	if s.Kind() != reflect.Ptr {
		return ErrInvalidSpecification
	}
	s = s.Elem()
	if s.Kind() != reflect.Struct {
		return ErrInvalidSpecification
	}

	f, err := os.Open(filename)
	if err != nil {
		return errors.Wrapf(err, "failed to open config file %s", filename)
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(cfg)
	if err != nil {
		return errors.Wrapf(err, "failed to decode config file %s", filename)
	}

	return nil
}

// loadConfigEnv processes envconfig annotations and loads environment variables to the given configuration struct.
func loadConfigEnv(cfg interface{}) error {
	err := envconfig.Process("", cfg)
	if err != nil {
		return errors.Wrap(err, "failed to process environment config")
	}

	return nil
}

type Config struct {
	Core struct {
		ListeningAddress        string `yaml:"listeningAddress" envconfig:"LISTENING_ADDRESS"`
		ExternalUrl             string `yaml:"externalUrl" envconfig:"EXTERNAL_URL"`
		Title                   string `yaml:"title" envconfig:"WEBSITE_TITLE"`
		CompanyName             string `yaml:"company" envconfig:"COMPANY_NAME"`
		MailFrom                string `yaml:"mailFrom" envconfig:"MAIL_FROM"`
		AdminUser               string `yaml:"adminUser" envconfig:"ADMIN_USER"` // must be an email address
		AdminPassword           string `yaml:"adminPass" envconfig:"ADMIN_PASS"`
		EditableKeys            bool   `yaml:"editableKeys" envconfig:"EDITABLE_KEYS"`
		CreateDefaultPeer       bool   `yaml:"createDefaultPeer" envconfig:"CREATE_DEFAULT_PEER"`
		SelfProvisioningAllowed bool   `yaml:"selfProvisioning" envconfig:"SELF_PROVISIONING"`
		WGExoprterFriendlyNames bool   `yaml:"wgExporterFriendlyNames" envconfig:"WG_EXPORTER_FRIENDLY_NAMES"`
		LdapEnabled             bool   `yaml:"ldapEnabled" envconfig:"LDAP_ENABLED"`
		SessionSecret           string `yaml:"sessionSecret" envconfig:"SESSION_SECRET"`
		LogoUrl                 string `yaml:"logoUrl" envconfig:"LOGO_URL"`
	} `yaml:"core"`
	Database common.DatabaseConfig `yaml:"database"`
	Email    common.MailConfig     `yaml:"email"`
	LDAP     ldap.Config           `yaml:"ldap"`
	WG       wireguard.Config      `yaml:"wg"`
}

func NewConfig() *Config {
	cfg := &Config{}

	// Default config
	cfg.Core.ListeningAddress = ":8123"
	cfg.Core.Title = "WireGuard VPN"
	cfg.Core.CompanyName = "WireGuard Portal"
	cfg.Core.LogoUrl = "/img/header-logo.png"
	cfg.Core.ExternalUrl = "http://localhost:8123"
	cfg.Core.MailFrom = "WireGuard VPN <noreply@company.com>"
	cfg.Core.AdminUser = "admin@wgportal.local"
	cfg.Core.AdminPassword = "wgportal"
	cfg.Core.LdapEnabled = false
	cfg.Core.EditableKeys = true
	cfg.Core.WGExoprterFriendlyNames = false
	cfg.Core.SessionSecret = "secret"

	cfg.Database.Typ = "sqlite"
	cfg.Database.Database = "data/wg_portal.db"

	cfg.LDAP.URL = "ldap://srv-ad01.company.local:389"
	cfg.LDAP.BaseDN = "DC=COMPANY,DC=LOCAL"
	cfg.LDAP.StartTLS = true
	cfg.LDAP.BindUser = "company\\\\ldap_wireguard"
	cfg.LDAP.BindPass = "SuperSecret"
	cfg.LDAP.EmailAttribute = "mail"
	cfg.LDAP.FirstNameAttribute = "givenName"
	cfg.LDAP.LastNameAttribute = "sn"
	cfg.LDAP.PhoneAttribute = "telephoneNumber"
	cfg.LDAP.GroupMemberAttribute = "memberOf"
	cfg.LDAP.AdminLdapGroup = "CN=WireGuardAdmins,OU=_O_IT,DC=COMPANY,DC=LOCAL"
	cfg.LDAP.LoginFilter = "(&(objectClass=organizationalPerson)(mail={{login_identifier}})(!userAccountControl:1.2.840.113556.1.4.803:=2))"
	cfg.LDAP.SyncFilter = "(&(objectClass=organizationalPerson)(!userAccountControl:1.2.840.113556.1.4.803:=2)(mail=*))"

	cfg.WG.DeviceNames = []string{"wg0"}
	cfg.WG.DefaultDeviceName = "wg0"
	cfg.WG.ConfigDirectoryPath = "/etc/wireguard"
	cfg.WG.ManageIPAddresses = true
	cfg.WG.UserManagePeers = false
	cfg.Email.Host = "127.0.0.1"
	cfg.Email.Port = 25
	cfg.Email.Encryption = common.MailEncryptionNone
	cfg.Email.AuthType = common.MailAuthPlain

	// Load config from file and environment
	cfgFile, ok := os.LookupEnv("CONFIG_FILE")
	if !ok {
		cfgFile = "config.yml" // Default config file
	}
	err := loadConfigFile(cfg, cfgFile)
	if err != nil {
		logrus.Warnf("unable to load config.yml file: %v, using default configuration...", err)
	}
	err = loadConfigEnv(cfg)
	if err != nil {
		logrus.Warnf("unable to load environment config: %v", err)
	}
	cfg.LDAP.AdminLdapGroup_, err = gldap.ParseDN(cfg.LDAP.AdminLdapGroup)
	if err != nil {
		logrus.Warnf("Parsing AdminLDAPGroup failed: %v", err)
	}

	if cfg.WG.ManageIPAddresses && runtime.GOOS != "linux" {
		logrus.Warnf("managing IP addresses only works on linux, feature disabled...")
		cfg.WG.ManageIPAddresses = false
	}

	return cfg
}
