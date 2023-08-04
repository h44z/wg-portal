package config

type WebConfig struct {
	RequestLogging    bool   `yaml:"request_logging"`
	ExternalUrl       string `yaml:"external_url"`
	ListeningAddress  string `yaml:"listening_address"`
	SessionIdentifier string `yaml:"session_identifier"`
	SessionSecret     string `yaml:"session_secret"`
	CsrfSecret        string `yaml:"csrf_secret"`
	SiteTitle         string `yaml:"site_title"`
	SiteCompanyName   string `yaml:"site_company_name"`
}
