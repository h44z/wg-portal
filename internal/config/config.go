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
		StartListenPort  int           `yaml:"start_listen_port"`
		StartCidrV4      string        `yaml:"start_cidr_v4"`
		StartCidrV6      string        `yaml:"start_cidr_v6"`
		UseIpV6          bool          `yaml:"use_ip_v6"`
	} `yaml:"advanced"`

	Statistics struct {
		UsePingChecks          bool          `yaml:"use_ping_checks"`
		PingCheckWorkers       int           `yaml:"ping_check_workers"`
		PingUnprivileged       bool          `yaml:"ping_unprivileged"`
		PingCheckInterval      time.Duration `yaml:"ping_check_interval"`
		CollectInterfaceData   bool          `yaml:"collect_interface_data"`
		CollectPeerData        bool          `yaml:"collect_peer_data"`
		DataCollectionInterval time.Duration `yaml:"data_collection_interval"`
	}

	Mail MailConfig `yaml:"mail"`

	Auth Auth `yaml:"auth"`

	Database DatabaseConfig `yaml:"database"`

	Web WebConfig `yaml:"web"`
}

func defaultConfig() *Config {
	cfg := &Config{}

	cfg.Core.ImportExisting = true
	cfg.Core.RestoreState = true

	cfg.Database = DatabaseConfig{
		Type: "sqlite",
		DSN:  "sqlite.db",
	}

	cfg.Web = WebConfig{
		RequestLogging:    false,
		ListeningAddress:  ":8888",
		SessionSecret:     "verysecret",
		SessionIdentifier: "wgPortalSession",
	}

	cfg.Auth.CallbackUrlPrefix = "/api/v0"

	cfg.Advanced.StartListenPort = 51820
	cfg.Advanced.StartCidrV4 = "10.6.6.1/24"
	cfg.Advanced.StartCidrV6 = "fdfd:d3ad:c0de:1234::1/64"
	cfg.Advanced.UseIpV6 = true

	cfg.Statistics.UsePingChecks = true
	cfg.Statistics.PingCheckWorkers = 10
	cfg.Statistics.PingUnprivileged = false
	cfg.Statistics.PingCheckInterval = 1 * time.Minute
	cfg.Statistics.CollectInterfaceData = true
	cfg.Statistics.CollectPeerData = true
	cfg.Statistics.DataCollectionInterval = 10 * time.Second

	return cfg
}

func GetConfig() (*Config, error) {
	cfg := defaultConfig()

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
