package oauth

import (
	"fmt"

	"github.com/h44z/wg-portal/internal/oauth/oauthproviders"
	"github.com/h44z/wg-portal/internal/oauth/oauthproviders/github"
	"github.com/h44z/wg-portal/internal/oauth/oauthproviders/gitlab"
	"github.com/h44z/wg-portal/internal/oauth/oauthproviders/google"
)

type Config struct {
	Github struct {
		ClientID     string `yaml:"clientID" envconfig:"OAUTH_GITHUB_CLIENT_ID"`
		ClientSecret string `yaml:"clientSecret" envconfig:"OAUTH_GITHUB_CLIENT_SECRET"`
		CreateUsers  bool   `yaml:"createUsers" envconfig:"OAUTH_GITHUB_CREATE_USERS"`
		Enabled      bool   `yaml:"enabled" envconfig:"OAUTH_GITHUB_ENABLED"`
	} `yaml:"github"`
	Google struct {
		ClientID     string `yaml:"clientID" envconfig:"OAUTH_GOOGLE_CLIENT_ID"`
		ClientSecret string `yaml:"clientSecret" envconfig:"OAUTH_GOOGLE_CLIENT_SECRET"`
		CreateUsers  bool   `yaml:"createUsers" envconfig:"OAUTH_GOOGLE_CREATE_USERS"`
		Enabled      bool   `yaml:"enabled" envconfig:"OAUTH_GOOGLE_ENABLED"`
	} `yaml:"google"`
	Gitlab struct {
		ClientID     string `yaml:"clientID" envconfig:"OAUTH_GITLAB_CLIENT_ID"`
		ClientSecret string `yaml:"clientSecret" envconfig:"OAUTH_GITLAB_CLIENT_SECRET"`
		CreateUsers  bool   `yaml:"createUsers" envconfig:"OAUTH_GITLAB_CREATE_USERS"`
		Enabled      bool   `yaml:"enabled" envconfig:"OAUTH_GITLAB_ENABLED"`
	} `yaml:"gitlab"`
	RedirectURL string `yaml:"redirectURL" envconfig:"OAUTH_REDIRECT_URL"`
}

func (c Config) IsEnabled() bool {
	return c.Github.Enabled ||
		c.Google.Enabled ||
		c.Gitlab.Enabled
}

func (c Config) NewProviderFromID(providerID oauthproviders.ProviderType, redirectURL string) (oauthproviders.Provider, error) {
	switch providerID {
	case oauthproviders.ProviderGithub:
		config := oauthproviders.ProviderConfig{
			ClientID:     c.Github.ClientID,
			ClientSecret: c.Github.ClientSecret,
			RedirectURL:  redirectURL,
			CreateUsers:  c.Github.CreateUsers,
		}

		return github.New(config), nil
	case oauthproviders.ProviderGoogle:
		config := oauthproviders.ProviderConfig{
			ClientID:     c.Google.ClientID,
			ClientSecret: c.Google.ClientSecret,
			RedirectURL:  redirectURL,
			CreateUsers:  c.Google.CreateUsers,
		}

		return google.New(config), nil
	case oauthproviders.ProviderGitlab:
		config := oauthproviders.ProviderConfig{
			ClientID:     c.Gitlab.ClientID,
			ClientSecret: c.Gitlab.ClientSecret,
			RedirectURL:  redirectURL,
			CreateUsers:  c.Gitlab.CreateUsers,
		}

		return gitlab.New(config), nil
	}

	return nil, fmt.Errorf("cannot create oauth provider from ID %s", providerID)
}
