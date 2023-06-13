package auth

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/coreos/go-oidc"
	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"golang.org/x/oauth2"
)

type OidcAuthenticator struct {
	name                string
	provider            *oidc.Provider
	verifier            *oidc.IDTokenVerifier
	cfg                 *oauth2.Config
	userInfoMapping     config.OauthFields
	registrationEnabled bool
}

func newOidcAuthenticator(ctx context.Context, callbackUrl string, cfg *config.OpenIDConnectProvider) (*OidcAuthenticator, error) {
	var err error
	var provider = &OidcAuthenticator{}

	provider.name = cfg.ProviderName
	provider.provider, err = oidc.NewProvider(context.Background(), cfg.BaseUrl) // use new context here, see https://github.com/coreos/go-oidc/issues/339
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
	provider.registrationEnabled = cfg.RegistrationEnabled

	return provider, nil
}

func (o OidcAuthenticator) GetName() string {
	return o.name
}

func (o OidcAuthenticator) RegistrationEnabled() bool {
	return o.registrationEnabled
}

func (o OidcAuthenticator) GetType() domain.AuthenticatorType {
	return domain.AuthenticatorTypeOidc
}

func (o OidcAuthenticator) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	return o.cfg.AuthCodeURL(state, opts...)
}

func (o OidcAuthenticator) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return o.cfg.Exchange(ctx, code, opts...)
}

func (o OidcAuthenticator) GetUserInfo(ctx context.Context, token *oauth2.Token, nonce string) (map[string]interface{}, error) {
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

	var tokenFields map[string]interface{}
	if err = idToken.Claims(&tokenFields); err != nil {
		return nil, fmt.Errorf("failed to parse extra claims: %w", err)
	}

	return tokenFields, nil
}

func (o OidcAuthenticator) ParseUserInfo(raw map[string]interface{}) (*domain.AuthenticatorUserInfo, error) {
	isAdmin, _ := strconv.ParseBool(internal.MapDefaultString(raw, o.userInfoMapping.IsAdmin, ""))
	userInfo := &domain.AuthenticatorUserInfo{
		Identifier: domain.UserIdentifier(internal.MapDefaultString(raw, o.userInfoMapping.UserIdentifier, "")),
		Email:      internal.MapDefaultString(raw, o.userInfoMapping.Email, ""),
		Firstname:  internal.MapDefaultString(raw, o.userInfoMapping.Firstname, ""),
		Lastname:   internal.MapDefaultString(raw, o.userInfoMapping.Lastname, ""),
		Phone:      internal.MapDefaultString(raw, o.userInfoMapping.Phone, ""),
		Department: internal.MapDefaultString(raw, o.userInfoMapping.Department, ""),
		IsAdmin:    isAdmin,
	}

	return userInfo, nil
}
