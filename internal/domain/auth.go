package domain

import (
	"context"

	"golang.org/x/oauth2"
)

type LoginProvider string

type LoginProviderInfo struct {
	Identifier  string
	Name        string
	ProviderUrl string
	CallbackUrl string
}

type AuthenticatorUserInfo struct {
	Identifier UserIdentifier
	Email      string
	Firstname  string
	Lastname   string
	Phone      string
	Department string
	IsAdmin    bool
}

type AuthenticatorType string

const (
	AuthenticatorTypeOAuth AuthenticatorType = "oauth"
	AuthenticatorTypeOidc  AuthenticatorType = "oidc"
)

type OauthAuthenticator interface {
	GetName() string
	GetType() AuthenticatorType
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	GetUserInfo(ctx context.Context, token *oauth2.Token, nonce string) (map[string]interface{}, error)
	ParseUserInfo(raw map[string]interface{}) (*AuthenticatorUserInfo, error)
	RegistrationEnabled() bool
}

type LdapAuthenticator interface {
	GetName() string
	PlaintextAuthentication(userId UserIdentifier, plainPassword string) error
	GetUserInfo(ctx context.Context, username UserIdentifier) (map[string]interface{}, error)
	ParseUserInfo(raw map[string]interface{}) (*AuthenticatorUserInfo, error)
	RegistrationEnabled() bool
}
