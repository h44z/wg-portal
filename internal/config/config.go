package config

import (
	"fmt"
	"os"
	"time"

	"github.com/a8m/envsubst"
	"github.com/sirupsen/logrus"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Core struct {
		// AdminUser defines the default administrator account that will be created
		AdminUser     string `yaml:"admin_user"`
		AdminPassword string `yaml:"admin_password"`

		EditableKeys                bool `yaml:"editable_keys"`
		CreateDefaultPeer           bool `yaml:"create_default_peer"`
		CreateDefaultPeerOnCreation bool `yaml:"create_default_peer_on_creation"`
		SelfProvisioningAllowed     bool `yaml:"self_provisioning_allowed"`
		ImportExisting              bool `yaml:"import_existing"`
		RestoreState                bool `yaml:"restore_state"`
	} `yaml:"core"`

	Advanced struct {
		LogLevel            string        `yaml:"log_level"`
		LogPretty           bool          `yaml:"log_pretty"`
		LogJson             bool          `yaml:"log_json"`
		StartListenPort     int           `yaml:"start_listen_port"`
		StartCidrV4         string        `yaml:"start_cidr_v4"`
		StartCidrV6         string        `yaml:"start_cidr_v6"`
		UseIpV6             bool          `yaml:"use_ip_v6"`
		ConfigStoragePath   string        `yaml:"config_storage_path"` // keep empty to disable config export to file
		ExpiryCheckInterval time.Duration `yaml:"expiry_check_interval"`
		RulePrioOffset      int           `yaml:"rule_prio_offset"`
		RouteTableOffset    int           `yaml:"route_table_offset"`
	} `yaml:"advanced"`

	Statistics struct {
		UsePingChecks          bool          `yaml:"use_ping_checks"`
		PingCheckWorkers       int           `yaml:"ping_check_workers"`
		PingUnprivileged       bool          `yaml:"ping_unprivileged"`
		PingCheckInterval      time.Duration `yaml:"ping_check_interval"`
		DataCollectionInterval time.Duration `yaml:"data_collection_interval"`
		CollectInterfaceData   bool          `yaml:"collect_interface_data"`
		CollectPeerData        bool          `yaml:"collect_peer_data"`
		CollectAuditData       bool          `yaml:"collect_audit_data"`
	} `yaml:"statistics"`

	Mail MailConfig `yaml:"mail"`

	Auth Auth `yaml:"auth"`

	Database DatabaseConfig `yaml:"database"`

	Web WebConfig `yaml:"web"`
}

func (c *Config) LogStartupValues() {
	logrus.Debug("WireGuard Portal Features:")
	logrus.Debugf("  - EditableKeys: %t", c.Core.EditableKeys)
	logrus.Debugf("  - CreateDefaultPeerOnCreation: %t", c.Core.CreateDefaultPeerOnCreation)
	logrus.Debugf("  - SelfProvisioningAllowed: %t", c.Core.SelfProvisioningAllowed)
	logrus.Debugf("  - ImportExisting: %t", c.Core.ImportExisting)
	logrus.Debugf("  - RestoreState: %t", c.Core.RestoreState)
	logrus.Debugf("  - UseIpV6: %t", c.Advanced.UseIpV6)
	logrus.Debugf("  - CollectInterfaceData: %t", c.Statistics.CollectInterfaceData)
	logrus.Debugf("  - CollectPeerData: %t", c.Statistics.CollectPeerData)
	logrus.Debugf("  - CollectAuditData: %t", c.Statistics.CollectAuditData)

	logrus.Debug("WireGuard Portal Settings:")
	logrus.Debugf("  - ConfigStoragePath: %s", c.Advanced.ConfigStoragePath)
	logrus.Debugf("  - ExternalUrl: %s", c.Web.ExternalUrl)

	logrus.Debug("WireGuard Portal Authentication:")
	logrus.Debugf("  - OIDC Providers: %d", len(c.Auth.OpenIDConnect))
	logrus.Debugf("  - OAuth Providers: %d", len(c.Auth.OAuth))
	logrus.Debugf("  - Ldap Providers: %d", len(c.Auth.Ldap))
}

func defaultConfig() *Config {
	cfg := &Config{}

	cfg.Core.ImportExisting = true
	cfg.Core.RestoreState = true

	cfg.Database = DatabaseConfig{
		Type: "sqlite",
		DSN:  "data/sqlite.db",
	}

	cfg.Web = WebConfig{
		RequestLogging:    false,
		ExternalUrl:       "http://localhost:8888",
		ListeningAddress:  ":8888",
		SessionIdentifier: "wgPortalSession",
		SessionSecret:     "very_secret",
		CsrfSecret:        "extremely_secret",
		SiteTitle:         "WireGuard Portal",
		SiteCompanyName:   "WireGuard Portal",
	}

	cfg.Auth.CallbackUrlPrefix = "/api/v0"

	cfg.Advanced.StartListenPort = 51820
	cfg.Advanced.StartCidrV4 = "10.11.12.0/24"
	cfg.Advanced.StartCidrV6 = "fdfd:d3ad:c0de:1234::0/64"
	cfg.Advanced.UseIpV6 = true
	cfg.Advanced.ExpiryCheckInterval = 15 * time.Minute
	cfg.Advanced.RulePrioOffset = 20000
	cfg.Advanced.RouteTableOffset = 20000

	cfg.Statistics.UsePingChecks = true
	cfg.Statistics.PingCheckWorkers = 10
	cfg.Statistics.PingUnprivileged = false
	cfg.Statistics.PingCheckInterval = 1 * time.Minute
	cfg.Statistics.DataCollectionInterval = 10 * time.Second
	cfg.Statistics.CollectInterfaceData = true
	cfg.Statistics.CollectPeerData = true
	cfg.Statistics.CollectAuditData = true

	cfg.Mail = MailConfig{
		Host:           "127.0.0.1",
		Port:           25,
		Encryption:     MailEncryptionNone,
		CertValidation: false,
		Username:       "",
		Password:       "",
		AuthType:       MailAuthPlain,
		From:           "Wireguard Portal <noreply@wireguard.local>",
		LinkOnly:       false,
	}

	return cfg
}

func GetConfig() (*Config, error) {
	cfg := defaultConfig()

	// override config values from YAML file

	cfgFileName := "config/config.yml"
	if envCfgFileName := os.Getenv("WG_PORTAL_CONFIG"); envCfgFileName != "" {
		cfgFileName = envCfgFileName
	}

	if err := loadConfigFile(cfg, cfgFileName); err != nil {
		return nil, fmt.Errorf("failed to load config from yaml: %w", err)
	}

	return cfg, nil
}

func loadConfigFile(cfg any, filename string) error {
	data, err := envsubst.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("envsubst error: %v", err)
	}

	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return fmt.Errorf("yaml error: %v", err)
	}

	return nil
}
