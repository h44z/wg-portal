package config

import "strings"

// WebConfig contains the configuration for the web server.
type WebConfig struct {
	// RequestLogging enables logging of all HTTP requests.
	RequestLogging bool `yaml:"request_logging"`
	// ExposeHostInfo sets whether the host information should be exposed in a response header.
	ExposeHostInfo bool `yaml:"expose_host_info"`
	// ExternalUrl is the URL where a client can access WireGuard Portal.
	// This is used for the callback URL of the OAuth providers.
	ExternalUrl string `yaml:"external_url"`
	// ListeningAddress is the address and port for the web server.
	ListeningAddress string `yaml:"listening_address"`
	// SessionIdentifier is the session identifier for the web frontend.
	SessionIdentifier string `yaml:"session_identifier"`
	// SessionSecret is the session secret for the web frontend.
	SessionSecret string `yaml:"session_secret"`
	// CsrfSecret is the CSRF secret.
	CsrfSecret string `yaml:"csrf_secret"`
	// SiteTitle is the title that is shown in the web frontend.
	SiteTitle string `yaml:"site_title"`
	// SiteCompanyName is the company name that is shown at the bottom of the web frontend.
	SiteCompanyName string `yaml:"site_company_name"`
	// CertFile is the path to the TLS certificate file.
	CertFile string `yaml:"cert_file"`
	// KeyFile is the path to the TLS certificate key file.
	KeyFile string `yaml:"key_file"`
}

func (c *WebConfig) Sanitize() {
	c.ExternalUrl = strings.TrimRight(c.ExternalUrl, "/")
}
