package common

import (
	"errors"
	"os"
	"reflect"

	"github.com/h44z/wg-portal/internal/wireguard"

	"github.com/h44z/wg-portal/internal/ldap"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var ErrInvalidSpecification = errors.New("specification must be a struct pointer")

// LoadConfigFile parses yaml files. It uses to yaml annotation to store the data in a struct.
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

// LoadConfigEnv processes envconfig annotations and loads environment variables to the given configuration struct.
func loadConfigEnv(cfg interface{}) error {
	err := envconfig.Process("", cfg)
	if err != nil {
		return err
	}

	return nil
}

type Config struct {
	Core struct {
		ListeningAddress string `yaml:"listeningAddress" envconfig:"LISTENING_ADDRESS"`
		Title            string `yaml:"title" envconfig:"WEBSITE_TITLE"`
	} `yaml:"core"`

	LDAP               ldap.Config      `yaml:"ldap"`
	WG                 wireguard.Config `yaml:"wg"`
	AdminLdapGroup     string           `yaml:"adminLdapGroup" envconfig:"ADMIN_LDAP_GROUP"`
	LogoutRedirectPath string           `yaml:"logoutRedirectPath" envconfig:"LOGOUT_REDIRECT_PATH"`
	AuthRoutePrefix    string           `yaml:"authRoutePrefix" envconfig:"AUTH_ROUTE_PREFIX"`
}

func NewConfig() *Config {
	cfg := &Config{}

	// Default config
	cfg.Core.ListeningAddress = ":8080"
	cfg.Core.Title = "WireGuard VPN"
	cfg.LDAP.URL = "ldap://srv-ad01.company.local:389"
	cfg.LDAP.BaseDN = "DC=COMPANY,DC=LOCAL"
	cfg.LDAP.StartTLS = true
	cfg.LDAP.BindUser = "company\\ldap_wireguard"
	cfg.LDAP.BindPass = "SuperSecret"
	cfg.WG.DeviceName = "wg0"
	cfg.AdminLdapGroup = "CN=WireGuardAdmins,OU=_O_IT,DC=COMPANY,DC=LOCAL"
	cfg.LogoutRedirectPath = "/"
	cfg.AuthRoutePrefix = "/auth"

	// Load config from file and environment
	cfgFile, ok := os.LookupEnv("CONFIG_FILE")
	if !ok {
		cfgFile = "config.yml" // Default config file
	}
	err := loadConfigFile(cfg, cfgFile)
	if err != nil {
		log.Warnf("unable to load config.yml file: %v, using default configuration...", err)
	}
	err = loadConfigEnv(cfg)
	if err != nil {
		log.Warnf("unable to load environment config: %v", err)
	}

	return cfg
}
