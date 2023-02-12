package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Core struct {
		// AdminUser defines the default administrator account that will be created
		AdminUser     string `yaml:"admin_user"`
		AdminPassword string `yaml:"admin_password"`

		EditableKeys            bool `yaml:"editable_keys"`
		CreateDefaultPeer       bool `yaml:"create_default_peer"`
		SelfProvisioningAllowed bool `yaml:"self_provisioning_allowed"`
		LdapSyncEnabled         bool `yaml:"ldap_enabled"`
		ImportExisting          bool `yaml:"import_existing"`
		RestoreState            bool `yaml:"restore_state"`
	} `yaml:"core"`

	Advanced struct {
		LogLevel         string        `yaml:"log_level"`
		StartupTimeout   time.Duration `yaml:"startup_timeout"`
		LdapSyncInterval time.Duration `yaml:"ldap_sync_interval"`
	} `yaml:"advanced"`

	Mail MailConfig `yaml:"mail"`

	Auth Auth `yaml:"auth"`

	Database DatabaseConfig `yaml:"database"`

	Web WebConfig `yaml:"web"`
}

func GetConfig() (*Config, error) {
	cfg := &Config{}

	// default config

	cfg.Core.ImportExisting = true
	cfg.Core.RestoreState = true

	cfg.Database = DatabaseConfig{
		Type: "sqlite",
		DSN:  "sqlite.db",
	}

	cfg.Web = WebConfig{
		ListeningAddress:  ":8888",
		SessionSecret:     "verysecret",
		SessionIdentifier: "wgPortalSession",
	}

	cfg.Auth.CallbackUrlPrefix = "/api/v0"

	// override config values from YAML file

	cfgFileName := "config.yml"
	if envCfgFileName := os.Getenv("WG_PORTAL_CONFIG"); envCfgFileName != "" {
		cfgFileName = envCfgFileName
	}

	if err := loadConfigFile(cfg, cfgFileName); err != nil {
		return nil, fmt.Errorf("failed to load config from yaml: %w", err)
	}

	return cfg, nil
}

func loadConfigFile(cfg any, filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(cfg)
	if err != nil {
		return err
	}

	return nil
}
