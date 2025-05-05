package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// OidcAuthenticator is an authenticator for OpenID Connect providers.
type OidcAuthenticator struct {
	name                string
	provider            *oidc.Provider
	verifier            *oidc.IDTokenVerifier
	cfg                 *oauth2.Config
	userInfoMapping     config.OauthFields
	userAdminMapping    *config.OauthAdminMapping
	registrationEnabled bool
	userInfoLogging     bool
	allowedDomains      []string
}

func newOidcAuthenticator(
	_ context.Context,
	callbackUrl string,
	cfg *config.OpenIDConnectProvider,
) (*OidcAuthenticator, error) {
	var err error
	var provider = &OidcAuthenticator{}

	provider.name = cfg.ProviderName
	provider.provider, err = oidc.NewProvider(context.Background(),
		cfg.BaseUrl) // use new context here, see https://github.com/coreos/go-oidc/issues/339
	if err != nil {
		return nil, fmt.Errorf("failed to create new oidc provider: %w", err)
	}
	provider.verifier = provider.provider.Verifier(&oidc.Config{
		ClientID: cfg.ClientID,
	})

	scopes := []string{oidc.ScopeOpenID}
	scopes = append(scopes, cfg.ExtraScopes...)
	provider.cfg = &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     provider.provider.Endpoint(),
		RedirectURL:  callbackUrl,
		Scopes:       scopes,
	}
	provider.userInfoMapping = getOauthFieldMapping(cfg.FieldMap)
	provider.userAdminMapping = &cfg.AdminMapping
	provider.registrationEnabled = cfg.RegistrationEnabled
	provider.userInfoLogging = cfg.LogUserInfo
	provider.allowedDomains = cfg.AllowedDomains

	return provider, nil
}

// GetName returns the name of the authenticator.
func (o OidcAuthenticator) GetName() string {
	return o.name
}

func (o OidcAuthenticator) GetAllowedDomains() []string {
	return o.allowedDomains
}

// RegistrationEnabled returns whether registration is enabled for this authenticator.
func (o OidcAuthenticator) RegistrationEnabled() bool {
	return o.registrationEnabled
}

// GetType returns the type of the authenticator.
func (o OidcAuthenticator) GetType() AuthenticatorType {
	return AuthenticatorTypeOidc
}

// AuthCodeURL returns the URL for the OAuth2 flow.
func (o OidcAuthenticator) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	return o.cfg.AuthCodeURL(state, opts...)
}

// Exchange exchanges the code for a token.
func (o OidcAuthenticator) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (
	*oauth2.Token,
	error,
) {
	return o.cfg.Exchange(ctx, code, opts...)
}

// GetUserInfo retrieves the user info from the token.
func (o OidcAuthenticator) GetUserInfo(ctx context.Context, token *oauth2.Token, nonce string) (
	map[string]any,
	error,
) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("token does not contain id_token")
	}
	idToken, err := o.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to validate id_token: %w", err)
	}
	if idToken.Nonce != nonce {
		return nil, errors.New("nonce mismatch")
	}

	var tokenFields map[string]any
	if err = idToken.Claims(&tokenFields); err != nil {
		return nil, fmt.Errorf("failed to parse extra claims: %w", err)
	}

	if o.userInfoLogging {
		contents, _ := json.Marshal(tokenFields)
		slog.Debug("OIDC user info",
			"source", o.name,
			"info", string(contents))
	}

	return tokenFields, nil
}

// ParseUserInfo parses the user info.
func (o OidcAuthenticator) ParseUserInfo(raw map[string]any) (*domain.AuthenticatorUserInfo, error) {
	return parseOauthUserInfo(o.userInfoMapping, o.userAdminMapping, raw)
}
