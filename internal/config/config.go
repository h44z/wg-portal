package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/a8m/envsubst"
	"gopkg.in/yaml.v3"
)

type FanoutConfig struct {
    Enabled     bool          `yaml:"enabled"`
    Peers       []string      `yaml:"peers"`
    AuthHeader  string        `yaml:"auth_header"`
    AuthValue   string        `yaml:"auth_value"`
    Timeout     time.Duration `yaml:"timeout"`
    Debounce    time.Duration `yaml:"debounce"`
    SelfURL     string        `yaml:"self_url"`
    Origin      string        `yaml:"origin" mapstructure:"origin"`
    KickOnStart bool          `yaml:"kick_on_start" mapstructure:"kick_on_start"`
    Topics      []string      `yaml:"topics" mapstructure:"topics"`
}

// Config is the main configuration struct.
type Config struct {
	Core struct {
		// AdminUser defines the default administrator account that will be created
		AdminUser     string `yaml:"admin_user"`
		AdminPassword string `yaml:"admin_password"`
		AdminApiToken string `yaml:"admin_api_token"` // if set, the API access is enabled automatically

		EditableKeys                bool `yaml:"editable_keys"`
		CreateDefaultPeer           bool `yaml:"create_default_peer"`
		CreateDefaultPeerOnCreation bool `yaml:"create_default_peer_on_creation"`
		ReEnablePeerAfterUserEnable bool `yaml:"re_enable_peer_after_user_enable"`
		DeletePeerAfterUserDeleted  bool `yaml:"delete_peer_after_user_deleted"`
		SelfProvisioningAllowed     bool `yaml:"self_provisioning_allowed"`
		ImportExisting              bool `yaml:"import_existing"`
		RestoreState                bool `yaml:"restore_state"`
		SyncOnStartup               bool `mapstructure:"sync_on_startup" yaml:"sync_on_startup" env:"WG_SYNC_ON_STARTUP"`

		Fanout              FanoutConfig `yaml:"fanout"`
	} `yaml:"core"`

	Advanced struct {
		LogLevel                 string        `yaml:"log_level"`
		LogPretty                bool          `yaml:"log_pretty"`
		LogJson                  bool          `yaml:"log_json"`
		StartListenPort          int           `yaml:"start_listen_port"`
		StartCidrV4              string        `yaml:"start_cidr_v4"`
		StartCidrV6              string        `yaml:"start_cidr_v6"`
		UseIpV6                  bool          `yaml:"use_ip_v6"`
		ConfigStoragePath        string        `yaml:"config_storage_path"` // keep empty to disable config export to file
		ExpiryCheckInterval      time.Duration `yaml:"expiry_check_interval"`
		RulePrioOffset           int           `yaml:"rule_prio_offset"`
		RouteTableOffset         int           `yaml:"route_table_offset"`
		ApiAdminOnly             bool          `yaml:"api_admin_only"` // if true, only admin users can access the API
		LimitAdditionalUserPeers int           `yaml:"limit_additional_user_peers"`
	} `yaml:"advanced"`

	Backend Backend `yaml:"backend"`

	Statistics struct {
		UsePingChecks          bool          `yaml:"use_ping_checks"`
		PingCheckWorkers       int           `yaml:"ping_check_workers"`
		PingUnprivileged       bool          `yaml:"ping_unprivileged"`
		PingCheckInterval      time.Duration `yaml:"ping_check_interval"`
		DataCollectionInterval time.Duration `yaml:"data_collection_interval"`
		CollectInterfaceData   bool          `yaml:"collect_interface_data"`
		CollectPeerData        bool          `yaml:"collect_peer_data"`
		CollectAuditData       bool          `yaml:"collect_audit_data"`
		ListeningAddress       string        `yaml:"listening_address"`
	} `yaml:"statistics"`

	Mail MailConfig `yaml:"mail"`

	Auth Auth `yaml:"auth"`

	Database DatabaseConfig `yaml:"database"`

	Web WebConfig `yaml:"web"`

	Webhook WebhookConfig `yaml:"webhook"`
}

// LogStartupValues logs the startup values of the configuration in debug level
func (c *Config) LogStartupValues() {
	slog.Info("Configuration loaded!", "logLevel", c.Advanced.LogLevel)

	slog.Debug("Config Features",
		"editableKeys", c.Core.EditableKeys,
		"createDefaultPeerOnCreation", c.Core.CreateDefaultPeerOnCreation,
		"reEnablePeerAfterUserEnable", c.Core.ReEnablePeerAfterUserEnable,
		"deletePeerAfterUserDeleted", c.Core.DeletePeerAfterUserDeleted,
		"selfProvisioningAllowed", c.Core.SelfProvisioningAllowed,
		"limitAdditionalUserPeers", c.Advanced.LimitAdditionalUserPeers,
		"importExisting", c.Core.ImportExisting,
		"restoreState", c.Core.RestoreState,
		"useIpV6", c.Advanced.UseIpV6,
		"collectInterfaceData", c.Statistics.CollectInterfaceData,
		"collectPeerData", c.Statistics.CollectPeerData,
		"collectAuditData", c.Statistics.CollectAuditData,
	)

	slog.Debug("Config Settings",
		"configStoragePath", c.Advanced.ConfigStoragePath,
		"externalUrl", c.Web.ExternalUrl,
	)

	slog.Debug("Config Authentication",
		"oidcProviders", len(c.Auth.OpenIDConnect),
		"oauthProviders", len(c.Auth.OAuth),
		"ldapProviders", len(c.Auth.Ldap),
		"webauthnEnabled", c.Auth.WebAuthn.Enabled,
		"minPasswordLength", c.Auth.MinPasswordLength,
		"hideLoginForm", c.Auth.HideLoginForm,
	)

	slog.Debug("Config Backend",
		"defaultBackend", c.Backend.Default,
		"extraBackends", len(c.Backend.Mikrotik),
	)

}

// defaultConfig returns the default configuration
func defaultConfig() *Config {
	cfg := &Config{}

	cfg.Core.AdminUser = "admin@wgportal.local"
	cfg.Core.AdminPassword = "wgportal-default"
	cfg.Core.AdminApiToken = "" // by default, the API access is disabled
	cfg.Core.ImportExisting = true
	cfg.Core.RestoreState = true
	cfg.Core.CreateDefaultPeer = false
	cfg.Core.CreateDefaultPeerOnCreation = false
	cfg.Core.EditableKeys = true
	cfg.Core.SelfProvisioningAllowed = false
	cfg.Core.ReEnablePeerAfterUserEnable = true
	cfg.Core.DeletePeerAfterUserDeleted = false

	cfg.Database = DatabaseConfig{
		Type: "sqlite",
		DSN:  "data/sqlite.db",
	}

	cfg.Backend = Backend{
		Default: LocalBackendName, // local backend is the default (using wgcrtl)
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

	cfg.Advanced.LogLevel = "info"
	cfg.Advanced.StartListenPort = 51820
	cfg.Advanced.StartCidrV4 = "10.11.12.0/24"
	cfg.Advanced.StartCidrV6 = "fdfd:d3ad:c0de:1234::0/64"
	cfg.Advanced.UseIpV6 = true
	cfg.Advanced.ExpiryCheckInterval = 15 * time.Minute
	cfg.Advanced.RulePrioOffset = 20000
	cfg.Advanced.RouteTableOffset = 20000
	cfg.Advanced.ApiAdminOnly = true
	cfg.Advanced.LimitAdditionalUserPeers = 0

	cfg.Statistics.UsePingChecks = true
	cfg.Statistics.PingCheckWorkers = 10
	cfg.Statistics.PingUnprivileged = false
	cfg.Statistics.PingCheckInterval = 1 * time.Minute
	cfg.Statistics.DataCollectionInterval = 1 * time.Minute
	cfg.Statistics.CollectInterfaceData = true
	cfg.Statistics.CollectPeerData = true
	cfg.Statistics.CollectAuditData = true
	cfg.Statistics.ListeningAddress = ":8787"

	cfg.Mail = MailConfig{
		Host:           "127.0.0.1",
		Port:           25,
		Encryption:     MailEncryptionNone,
		CertValidation: true,
		Username:       "",
		Password:       "",
		AuthType:       MailAuthPlain,
		From:           "Wireguard Portal <noreply@wireguard.local>",
		LinkOnly:       false,
	}

	cfg.Webhook.Url = "" // no webhook by default
	cfg.Webhook.Authentication = ""
	cfg.Webhook.Timeout = 10 * time.Second

	cfg.Auth.WebAuthn.Enabled = true
	cfg.Auth.MinPasswordLength = 16
	cfg.Auth.HideLoginForm = false

	return cfg
}

// GetConfig returns the configuration from the config file.
// Environment variable substitution is supported.
func GetConfig() (*Config, error) {
	cfg := defaultConfig()

	// override config values from YAML file

	cfgFileName := "config/config.yaml"
	cfgFileNameFallback := "config/config.yml"
	if envCfgFileName := os.Getenv("WG_PORTAL_CONFIG"); envCfgFileName != "" {
		cfgFileName = envCfgFileName
		cfgFileNameFallback = envCfgFileName
	}

	// check if the config file exists, otherwise use the fallback file name
	if _, err := os.Stat(cfgFileName); os.IsNotExist(err) {
		cfgFileName = cfgFileNameFallback
	}

	if err := loadConfigFile(cfg, cfgFileName); err != nil {
		return nil, fmt.Errorf("failed to load config from yaml: %w", err)
	}

	cfg.Web.Sanitize()
	err := cfg.Backend.Validate()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// loadConfigFile loads the configuration from a YAML file into the given cfg struct.
func loadConfigFile(cfg any, filename string) error {
	data, err := envsubst.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("Config file not found, using default values", "filename", filename)
			return nil
		}
		return fmt.Errorf("envsubst error: %v", err)
	}

	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return fmt.Errorf("yaml error: %v", err)
	}

	return nil
}
