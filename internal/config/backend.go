package config

import (
	"fmt"
	"time"
)

const LocalBackendName = "local"

type Backend struct {
	Default string `yaml:"default"` // The default backend to use (defaults to the internal backend)

	ReKeyTimeoutInterval time.Duration `yaml:"rekey_timeout_interval"` // Interval after which a connection is assumed dead

	// Local Backend-specific configuration

	IgnoredLocalInterfaces []string `yaml:"ignored_local_interfaces"` // A list of interface names that should be ignored by this backend (e.g., "wg0")
	LocalResolvconfPrefix  string   `yaml:"local_resolvconf_prefix"`  // The prefix to use for interface names when passing them to resolvconf.

	// External Backend-specific configuration

	Mikrotik []BackendMikrotik `yaml:"mikrotik"`
	Pfsense  []BackendPfsense  `yaml:"pfsense"`
	Opnsense []BackendOpnsense `yaml:"opnsense"`
}

// Validate checks the backend configuration for errors.
func (b *Backend) Validate() error {
	if b.Default == "" {
		b.Default = LocalBackendName
	}

	uniqueMap := make(map[string]struct{})
	checkBackendId := func(id string) error {
		if id == LocalBackendName {
			return fmt.Errorf("backend ID %q is a reserved keyword", LocalBackendName)
		}
		if _, exists := uniqueMap[id]; exists {
			return fmt.Errorf("backend ID %q is not unique", id)
		}
		uniqueMap[id] = struct{}{}
		return nil
	}

	for _, backend := range b.Mikrotik {
		if err := checkBackendId(backend.Id); err != nil {
			return err
		}
	}
	for _, backend := range b.Pfsense {
		if err := checkBackendId(backend.Id); err != nil {
			return err
		}
	}
	for _, backend := range b.Opnsense {
		if err := checkBackendId(backend.Id); err != nil {
			return err
		}
	}

	if b.Default != LocalBackendName {
		if _, ok := uniqueMap[b.Default]; !ok {
			return fmt.Errorf("default backend %q is not defined in the configuration", b.Default)
		}
	}

	return nil
}

type BackendBase struct {
	Id          string `yaml:"id"`           // A unique id for the backend
	DisplayName string `yaml:"display_name"` // A display name for the backend
	Type        string `yaml:"-"`            // The type of the backend (e.g. local, mikrotik, pfsense)

	IgnoredInterfaces []string `yaml:"ignored_interfaces"` // A list of interface names that should be ignored by this backend (e.g., "wg0")
}

// GetDisplayName returns the display name of the backend.
// If no display name is set, it falls back to the ID.
func (b BackendBase) GetDisplayName() string {
	if b.DisplayName == "" {
		return b.Id // Fallback to ID if no display name is set
	}
	return b.DisplayName
}

type BackendMikrotik struct {
	BackendBase `yaml:",inline"` // Embed the base fields

	ApiUrl       string        `yaml:"api_url"` // The base URL of the Mikrotik API (e.g., "https://10.10.10.10:8729/rest")
	ApiUser      string        `yaml:"api_user"`
	ApiPassword  string        `yaml:"api_password"`
	ApiVerifyTls bool          `yaml:"api_verify_tls"` // Whether to verify the TLS certificate of the Mikrotik API
	ApiTimeout   time.Duration `yaml:"api_timeout"`    // Timeout for API requests (default: 30 seconds)

	// Concurrency controls the maximum number of concurrent API requests that this backend will issue
	// when enumerating interfaces and their details. If 0 or negative, a default of 5 is used.
	Concurrency int `yaml:"concurrency"`

	Debug bool `yaml:"debug"` // Enable debug logging for the Mikrotik backend
}

// GetConcurrency returns the configured concurrency for this backend or a sane default (5)
// when the configured value is zero or negative.
func (b *BackendMikrotik) GetConcurrency() int {
	if b == nil {
		return 5
	}
	if b.Concurrency <= 0 {
		return 5
	}
	return b.Concurrency
}

// GetApiTimeout returns the configured API timeout or a sane default (30 seconds)
// when the configured value is zero or negative.
func (b *BackendMikrotik) GetApiTimeout() time.Duration {
	if b == nil {
		return 30 * time.Second
	}
	if b.ApiTimeout <= 0 {
		return 30 * time.Second
	}
	return b.ApiTimeout
}

type BackendPfsense struct {
	BackendBase `yaml:",inline"` // Embed the base fields

	ApiUrl       string        `yaml:"api_url"`        // The base URL of the pfSense REST API (e.g., "https://pfsense.example.com/api/v2")
	ApiKey       string        `yaml:"api_key"`        // API key for authentication (generated in pfSense under 'System' -> 'REST API' -> 'Keys')
	ApiVerifyTls bool          `yaml:"api_verify_tls"` // Whether to verify the TLS certificate of the pfSense API
	ApiTimeout   time.Duration `yaml:"api_timeout"`    // Timeout for API requests (default: 30 seconds)

	// Concurrency controls the maximum number of concurrent API requests that this backend will issue
	// when enumerating interfaces and their details. If 0 or negative, a default of 5 is used.
	Concurrency int `yaml:"concurrency"`

	Debug bool `yaml:"debug"` // Enable debug logging for the pfSense backend
}

// GetConcurrency returns the configured concurrency for this backend or a sane default (5)
// when the configured value is zero or negative.
func (b *BackendPfsense) GetConcurrency() int {
	if b == nil {
		return 5
	}
	if b.Concurrency <= 0 {
		return 5
	}
	return b.Concurrency
}

// GetApiTimeout returns the configured API timeout or a sane default (30 seconds)
// when the configured value is zero or negative.
func (b *BackendPfsense) GetApiTimeout() time.Duration {
	if b == nil {
		return 30 * time.Second
	}
	if b.ApiTimeout <= 0 {
		return 30 * time.Second
	}
	return b.ApiTimeout
}

type BackendOpnsense struct {
	BackendBase `yaml:",inline"` // Embed the base fields

	// The base URL of the OPNsense API (e.g., "https://opnsense.example.com").
	// Unlike the pfSense backend this is the host root, not an /api prefix: the
	// controller paths already start with /api/wireguard/.
	ApiUrl string `yaml:"api_url"`
	// OPNsense authenticates with an API key/secret pair sent as HTTP Basic
	// credentials, generated under 'System' -> 'Access' -> 'Users' -> 'API keys'.
	// This differs from pfSense, which uses a single X-API-Key header value.
	ApiKey       string        `yaml:"api_key"`
	ApiSecret    string        `yaml:"api_secret"`
	ApiVerifyTls bool          `yaml:"api_verify_tls"` // Whether to verify the TLS certificate of the OPNsense API
	ApiTimeout   time.Duration `yaml:"api_timeout"`    // Timeout for API requests (default: 30 seconds)

	Debug bool `yaml:"debug"` // Enable debug logging for the OPNsense backend

	// Note: there is deliberately no concurrency setting here. The Mikrotik and
	// pfSense backends fan out one request per interface to collect details;
	// OPNsense's searchXxx endpoints return every record with its fields already
	// populated, so enumeration costs a fixed three calls regardless of size.
}

// GetApiTimeout returns the configured API timeout or a sane default (30 seconds)
// when the configured value is zero or negative.
func (b *BackendOpnsense) GetApiTimeout() time.Duration {
	if b == nil {
		return 30 * time.Second
	}
	if b.ApiTimeout <= 0 {
		return 30 * time.Second
	}
	return b.ApiTimeout
}
