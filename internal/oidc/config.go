package oidc

import (
	"fmt"

	"github.com/h44z/wg-portal/internal/oauth"
	"github.com/h44z/wg-portal/internal/oauth/oauthproviders"
	"github.com/pkg/errors"
)

type IconType string

const (
	IconTypeOpenID   = "openid"
	IconTypeKeycloak = "keycloak"
)

const (
	defaultLabel = "Sign In with OIDC"
)

type Config []ConfigItem

func (c Config) Parse(redirectURL string) (err error) {
	for i := range c {
		config := ProviderConfig{
			DiscoveryURL: c[i].DiscoveryURL,
			ClientID:     c[i].ClientID,
			ClientSecret: c[i].ClientSecret,
			RedirectURL:  redirectURL,
			CreateUsers:  c[i].CreateUsers,
			VerifyEmail:  c[i].VerifyEmail,
		}

		c[i].provider, err = New(config)
		if err != nil {
			return err
		}
	}

	return
}

func (c Config) IsEnabled() bool {
	return len(c) > 0
}

func (c Config) ProviderByID(providerID string) (oauthproviders.Provider, error) {
	for i := range c {
		if c[i].provider.ID() == providerID {
			return c[i].provider, nil
		}
	}

	return nil, errors.New(fmt.Sprintf("oauth: the providerID was not found in the configuration: %s", providerID))
}

func (c Config) ToFrontendButtons() (fc []oauth.FrontendButtonConfig) {
	for i := range c {
		fc = append(fc, c[i].ToFrontendButton())
	}

	return
}

type ConfigItem struct {
	DiscoveryURL string `yaml:"discoveryURL" envconfig:"DISCOVERY_URL"`
	ClientID     string `yaml:"clientID" envconfig:"CLIENT_ID"`
	ClientSecret string `yaml:"clientSecret" envconfig:"CLIENT_SECRET"`
	CreateUsers  bool   `yaml:"createUsers" envconfig:"CREATE_USERS"`
	VerifyEmail  bool   `yaml:"verifyEmail" envconfig:"VERIFY_EMAIL"`
	provider     oauthproviders.Provider
	Button       struct {
		Icon  IconType `yaml:"icon,omitempty" envconfig:"BUTTON_ICON"`
		Label string   `yaml:"label,omitempty" envconfig:"BUTTON_LABEL"`
	} `yaml:"button,omitempty"`
}

func (ci ConfigItem) ToFrontendButton() oauth.FrontendButtonConfig {
	var style string

	switch ci.Button.Icon {
	case IconTypeKeycloak:
		style = "logo-keycloak"
	case IconTypeOpenID:
		style = "logo-openid"
	default:
		style = "logo-openid"
	}

	if ci.Button.Label == "" {
		ci.Button.Label = defaultLabel
	}

	return oauth.FrontendButtonConfig{
		ProviderID:  ci.provider.ID(),
		ButtonStyle: "btn-openid",
		IconStyle:   style,
		Label:       ci.Button.Label,
	}
}
