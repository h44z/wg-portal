package oauth

import (
	"fmt"

	"github.com/h44z/wg-portal/internal/oauth/oauthproviders"
	"github.com/h44z/wg-portal/internal/oauth/oauthproviders/github"
	"github.com/h44z/wg-portal/internal/oauth/oauthproviders/gitlab"
	"github.com/h44z/wg-portal/internal/oauth/oauthproviders/google"
	"github.com/pkg/errors"
)

type Config struct {
	Github struct {
		ClientID     string `yaml:"clientID" envconfig:"OAUTH_GITHUB_CLIENT_ID"`
		ClientSecret string `yaml:"clientSecret" envconfig:"OAUTH_GITHUB_CLIENT_SECRET"`
		CreateUsers  bool   `yaml:"createUsers" envconfig:"OAUTH_GITHUB_CREATE_USERS"`
		Enabled      bool   `yaml:"enabled" envconfig:"OAUTH_GITHUB_ENABLED"`
		provider     oauthproviders.Provider
	} `yaml:"github"`
	Google struct {
		ClientID     string `yaml:"clientID" envconfig:"OAUTH_GOOGLE_CLIENT_ID"`
		ClientSecret string `yaml:"clientSecret" envconfig:"OAUTH_GOOGLE_CLIENT_SECRET"`
		CreateUsers  bool   `yaml:"createUsers" envconfig:"OAUTH_GOOGLE_CREATE_USERS"`
		Enabled      bool   `yaml:"enabled" envconfig:"OAUTH_GOOGLE_ENABLED"`
		provider     oauthproviders.Provider
	} `yaml:"google"`
	Gitlab struct {
		ClientID     string `yaml:"clientID" envconfig:"OAUTH_GITLAB_CLIENT_ID"`
		ClientSecret string `yaml:"clientSecret" envconfig:"OAUTH_GITLAB_CLIENT_SECRET"`
		CreateUsers  bool   `yaml:"createUsers" envconfig:"OAUTH_GITLAB_CREATE_USERS"`
		Enabled      bool   `yaml:"enabled" envconfig:"OAUTH_GITLAB_ENABLED"`
		provider     oauthproviders.Provider
	} `yaml:"gitlab"`
	RedirectURL string `yaml:"redirectURL" envconfig:"OAUTH_REDIRECT_URL"`
	enabled     bool
}

func (c *Config) Parse(redirectURL string) {
	if c.Github.Enabled {
		c.Github.provider = github.New(oauthproviders.ProviderConfig{
			ClientID:     c.Github.ClientID,
			ClientSecret: c.Github.ClientSecret,
			RedirectURL:  redirectURL,
			CreateUsers:  c.Github.CreateUsers,
		})
		c.enabled = true
	}

	if c.Google.Enabled {
		c.Google.provider = google.New(oauthproviders.ProviderConfig{
			ClientID:     c.Google.ClientID,
			ClientSecret: c.Google.ClientSecret,
			RedirectURL:  redirectURL,
			CreateUsers:  c.Google.CreateUsers,
		})
		c.enabled = true
	}

	if c.Gitlab.Enabled {
		c.Gitlab.provider = gitlab.New(oauthproviders.ProviderConfig{
			ClientID:     c.Gitlab.ClientID,
			ClientSecret: c.Gitlab.ClientSecret,
			RedirectURL:  redirectURL,
			CreateUsers:  c.Gitlab.CreateUsers,
		})
		c.enabled = true
	}
}

func (c Config) IsEnabled() bool {
	return c.enabled
}

func (c Config) ProviderByID(providerID string) (oauthproviders.Provider, error) {
	switch oauthproviders.ProviderType(providerID) {
	case github.ProviderGithub:
		return c.Github.provider, nil
	case google.ProviderGoogle:
		return c.Google.provider, nil
	case gitlab.ProviderGitlab:
		return c.Gitlab.provider, nil
	}

	return nil, errors.New(fmt.Sprintf("oauth: the providerID was not found in the configuration: %s", providerID))
}

type FrontendButtonConfig struct {
	ProviderID  string
	ButtonStyle string
	IconStyle   string
	Label       string
}

func (c Config) ToFrontendButtons() (fc []FrontendButtonConfig) {
	if c.Github.Enabled {
		fc = append(fc, FrontendButtonConfig{
			ProviderID:  c.Github.provider.ID(),
			ButtonStyle: "btn-github",
			IconStyle:   "fa-github",
			Label:       "Sign in with GitHub",
		})
	}

	if c.Google.Enabled {
		fc = append(fc, FrontendButtonConfig{
			ProviderID:  c.Google.provider.ID(),
			ButtonStyle: "btn-google",
			IconStyle:   "fa-google",
			Label:       "Sign in with Google",
		})
	}

	if c.Gitlab.Enabled {
		fc = append(fc, FrontendButtonConfig{
			ProviderID:  c.Github.provider.ID(),
			ButtonStyle: "btn-gitlab",
			IconStyle:   "fa-gitlab",
			Label:       "Sign in with Gitlab",
		})
	}

	return
}
