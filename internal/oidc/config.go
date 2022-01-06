package oidc

import (
	"context"
	"fmt"

	"github.com/h44z/wg-portal/internal/oauth/oauthproviders"
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

type ConfigItem struct {
	DiscoveryURL string `yaml:"discoveryURL" envconfig:"DISCOVERY_URL"`
	ClientID     string `yaml:"clientID" envconfig:"CLIENT_ID"`
	ClientSecret string `yaml:"clientSecret" envconfig:"CLIENT_SECRET"`
	CreateUsers  bool   `yaml:"createUsers" envconfig:"CREATE_USERS"`
	VerifyEmail  bool   `yaml:"verifyEmail" envconfig:"VERIFY_EMAIL"`
	LoginURL     string
	Button       struct {
		Icon  IconType `yaml:"icon,omitempty" envconfig:"BUTTON_ICON"`
		Label string   `yaml:"label,omitempty" envconfig:"BUTTON_LABEL"`
	} `yaml:"button,omitempty"`
}

func (c Config) IsEnabled() bool {
	return len(c) > 0
}

func (c Config) getByLoginURL(loginURL string) (*ConfigItem, error) {
	for i := range c {
		if c[i].LoginURL == loginURL {
			return &c[i], nil
		}
	}

	return &ConfigItem{}, fmt.Errorf("the loginURL was not found in the configuration: %s", loginURL)
}

func (c Config) ToFrontendButtons() (fc []FrontendButton) {
	for i := range c {
		fc = append(fc, c[i].ToFrontendButton())
	}

	return
}

func (c Config) NewProviderFromID(ctx context.Context, loginURL, redirectURL string) (oauthproviders.Provider, error) {
	item, err := c.getByLoginURL(loginURL)
	if err != nil {
		return nil, err
	}

	config := ProviderConfig{
		DiscoveryURL: item.DiscoveryURL,
		ClientID:     item.ClientID,
		ClientSecret: item.ClientSecret,
		RedirectURL:  redirectURL,
		CreateUsers:  item.CreateUsers,
		VerifyEmail:  item.VerifyEmail,
	}

	return New(ctx, config)
}

type FrontendButton struct {
	LoginURL string
	Style    string
	Label    string
}

func (c ConfigItem) ToFrontendButton() FrontendButton {
	var style string

	switch c.Button.Icon {
	case IconTypeKeycloak:
		style = "logo-keycloak"
	case IconTypeOpenID:
		style = "logo-openid"
	default:
		style = "logo-openid"
	}

	if c.Button.Label == "" {
		c.Button.Label = defaultLabel
	}

	return FrontendButton{
		LoginURL: c.LoginURL,
		Style:    style,
		Label:    c.Button.Label,
	}
}
