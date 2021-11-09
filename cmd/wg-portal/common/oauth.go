package common

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/pkg/errors"

	"golang.org/x/oauth2"
)

type AuthenticatorType string

const (
	AuthenticatorTypeOAuth AuthenticatorType = "oauth"
	AuthenticatorTypeOidc  AuthenticatorType = "oidc"
)

type AuthenticatorUserInfo struct {
}

type Authenticator interface {
	GetType() AuthenticatorType
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	GetUserInfo(ctx context.Context, token *oauth2.Token, nonce string) (map[string]interface{}, error)
	ParseUserInfo(raw map[string]interface{}) (*AuthenticatorUserInfo, error)
}

type plainOauthAuthenticator struct {
	name             string
	cfg              *oauth2.Config
	userInfoEndpoint string
	client           *http.Client
	userInfoMapping  map[string]string
}

func NewPlainOauthAuthenticator(_ context.Context, callbackUrl string, cfg *OAuthProvider) (*plainOauthAuthenticator, error) {
	var authenticator = &plainOauthAuthenticator{}

	authenticator.name = cfg.ProviderName
	authenticator.client = &http.Client{
		Timeout: time.Second * 10,
	}
	authenticator.cfg = &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:   cfg.AuthURL,
			TokenURL:  cfg.TokenURL,
			AuthStyle: oauth2.AuthStyleAutoDetect,
		},
		RedirectURL: callbackUrl,
		Scopes:      cfg.Scopes,
	}
	authenticator.userInfoEndpoint = cfg.UserInfoURL

	return authenticator, nil
}

func (p plainOauthAuthenticator) GetType() AuthenticatorType {
	return AuthenticatorTypeOAuth
}

func (p plainOauthAuthenticator) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	return p.cfg.AuthCodeURL(state, opts...)
}

func (p plainOauthAuthenticator) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return p.cfg.Exchange(ctx, code, opts...)
}

func (p plainOauthAuthenticator) GetUserInfo(ctx context.Context, token *oauth2.Token, _ string) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", p.userInfoEndpoint, nil)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create user info get request")
	}
	req.Header.Add("Authorization", "Bearer "+token.AccessToken)
	req.WithContext(ctx)

	response, err := p.client.Do(req)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get user info")
	}
	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to read response body")
	}

	var userFields map[string]interface{}
	err = json.Unmarshal(contents, &userFields)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to parse user info")
	}

	return userFields, nil
}

func (p plainOauthAuthenticator) ParseUserInfo(raw map[string]interface{}) (*AuthenticatorUserInfo, error) {
	return nil, nil // TODO: implement
}

type oidcAuthenticator struct {
	name            string
	provider        *oidc.Provider
	verifier        *oidc.IDTokenVerifier
	cfg             *oauth2.Config
	userInfoMapping map[string]string
}

func NewOidcAuthenticator(ctx context.Context, callbackUrl string, cfg *OpenIDConnectProvider) (*oidcAuthenticator, error) {
	var err error
	var authenticator = &oidcAuthenticator{}

	authenticator.name = cfg.ProviderName
	authenticator.provider, err = oidc.NewProvider(ctx, cfg.BaseUrl)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create new oidc provider")
	}
	authenticator.verifier = authenticator.provider.Verifier(&oidc.Config{
		ClientID: cfg.ClientID,
	})

	scopes := []string{oidc.ScopeOpenID}
	scopes = append(scopes, cfg.ExtraScopes...)
	authenticator.cfg = &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     authenticator.provider.Endpoint(),
		RedirectURL:  callbackUrl,
		Scopes:       scopes,
	}

	return authenticator, nil
}

func (o oidcAuthenticator) GetType() AuthenticatorType {
	return AuthenticatorTypeOidc
}

func (o oidcAuthenticator) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	return o.cfg.AuthCodeURL(state, opts...)
}

func (o oidcAuthenticator) Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return o.cfg.Exchange(ctx, code, opts...)
}

func (o oidcAuthenticator) GetUserInfo(ctx context.Context, token *oauth2.Token, nonce string) (map[string]interface{}, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("token does not contain id_token")
	}
	idToken, err := o.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to validate id_token")
	}
	if idToken.Nonce != nonce {
		return nil, errors.New("nonce mismatch")
	}

	var tokenFields map[string]interface{}
	if err = idToken.Claims(&tokenFields); err != nil {
		return nil, errors.WithMessage(err, "failed to parse extra claims")
	}

	return tokenFields, nil
}

func (o oidcAuthenticator) ParseUserInfo(raw map[string]interface{}) (*AuthenticatorUserInfo, error) {
	return nil, nil // TODO: implement
}
