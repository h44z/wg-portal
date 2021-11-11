package common

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/h44z/wg-portal/internal/persistence"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

type AuthenticatorType string

const (
	AuthenticatorTypeOAuth AuthenticatorType = "oauth"
	AuthenticatorTypeOidc  AuthenticatorType = "oidc"
)

type AuthenticatorUserInfo struct {
	Identifier persistence.UserIdentifier
	Email      string
	Firstname  string
	Lastname   string
	Phone      string
	Department string
	IsAdmin    bool
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
	userInfoMapping  OauthFields
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
	authenticator.userInfoMapping = getOauthFieldMapping(cfg.FieldMap)

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
	isAdmin, _ := strconv.ParseBool(mapDefaultString(raw, p.userInfoMapping.IsAdmin, ""))
	userInfo := &AuthenticatorUserInfo{
		Identifier: persistence.UserIdentifier(mapDefaultString(raw, p.userInfoMapping.UserIdentifier, "")),
		Email:      mapDefaultString(raw, p.userInfoMapping.Email, ""),
		Firstname:  mapDefaultString(raw, p.userInfoMapping.Firstname, ""),
		Lastname:   mapDefaultString(raw, p.userInfoMapping.Lastname, ""),
		Phone:      mapDefaultString(raw, p.userInfoMapping.Phone, ""),
		Department: mapDefaultString(raw, p.userInfoMapping.Department, ""),
		IsAdmin:    isAdmin,
	}

	return userInfo, nil
}

type oidcAuthenticator struct {
	name            string
	provider        *oidc.Provider
	verifier        *oidc.IDTokenVerifier
	cfg             *oauth2.Config
	userInfoMapping OauthFields
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
	authenticator.userInfoMapping = getOauthFieldMapping(cfg.FieldMap)

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
	isAdmin, _ := strconv.ParseBool(mapDefaultString(raw, o.userInfoMapping.IsAdmin, ""))
	userInfo := &AuthenticatorUserInfo{
		Identifier: persistence.UserIdentifier(mapDefaultString(raw, o.userInfoMapping.UserIdentifier, "")),
		Email:      mapDefaultString(raw, o.userInfoMapping.Email, ""),
		Firstname:  mapDefaultString(raw, o.userInfoMapping.Firstname, ""),
		Lastname:   mapDefaultString(raw, o.userInfoMapping.Lastname, ""),
		Phone:      mapDefaultString(raw, o.userInfoMapping.Phone, ""),
		Department: mapDefaultString(raw, o.userInfoMapping.Department, ""),
		IsAdmin:    isAdmin,
	}

	return userInfo, nil
}

func getOauthFieldMapping(f OauthFields) OauthFields {
	defaultMap := OauthFields{
		UserIdentifier: "sub",
		Email:          "email",
		Firstname:      "given_name",
		Lastname:       "family_name",
		Phone:          "phone",
		Department:     "department",
		IsAdmin:        "admin_flag",
	}
	switch {
	case f.UserIdentifier != "":
		defaultMap.UserIdentifier = f.UserIdentifier
	case f.Email != "":
		defaultMap.Email = f.Email
	case f.Firstname != "":
		defaultMap.Firstname = f.Firstname
	case f.Lastname != "":
		defaultMap.Lastname = f.Lastname
	case f.Phone != "":
		defaultMap.Phone = f.Phone
	case f.Department != "":
		defaultMap.Department = f.Department
	case f.IsAdmin != "":
		defaultMap.IsAdmin = f.IsAdmin
	}

	return defaultMap
}

func mapDefaultString(m map[string]interface{}, key string, dflt string) string {
	if tmp, ok := m[key]; !ok {
		return dflt
	} else {
		switch v := tmp.(type) {
		case string:
			return v
		default:
			return fmt.Sprintf("%v", v)
		}
	}
}
