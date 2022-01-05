package oidc

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/h44z/wg-portal/internal/oauth/oauthproviders"
	"github.com/h44z/wg-portal/internal/oauth/userprofile"
	"golang.org/x/oauth2"
)

type oidcProvider struct {
	oauth2.Config
	p           *oidc.Provider
	createUsers bool
}

type ProviderConfig struct {
	DiscoveryURL string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	CreateUsers  bool
}

func New(ctx context.Context, c ProviderConfig) (oauthproviders.Provider, error) {
	p, err := oidc.NewProvider(ctx, c.DiscoveryURL)
	if err != nil {
		return nil, err
	}

	config := oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Endpoint:     p.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		RedirectURL:  c.RedirectURL,
	}

	return &oidcProvider{
		Config:      config,
		p:           p,
		createUsers: c.CreateUsers,
	}, nil
}

func (p oidcProvider) UserInfo(ctx context.Context, ts oauth2.TokenSource) (userprofile.Profile, error) {
	userInfo, err := p.p.UserInfo(ctx, ts)
	if err != nil {
		return userprofile.Profile{}, err
	}

	if !userInfo.EmailVerified {
		return userprofile.Profile{}, fmt.Errorf("oidc: user email not verified")
	}

	t, err := ts.Token()
	if err != nil {
		return userprofile.Profile{}, err
	}

	rawIDToken, ok := t.Extra("id_token").(string)
	if !ok {
		return userprofile.Profile{}, fmt.Errorf("oidc: missing id_token")
	}

	verifier := p.p.Verifier(&oidc.Config{ClientID: p.Config.ClientID})

	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return userprofile.Profile{}, err
	}

	// Extract custom claims
	var claims struct {
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Email         string `json:"email"`
	}

	if err := idToken.Claims(&claims); err != nil {
		return userprofile.Profile{}, err
	}

	return userprofile.Profile{
		FirstName: claims.GivenName,
		LastName:  claims.FamilyName,
		Email:     userInfo.Email,
	}, nil
}

func (p oidcProvider) CanCreateUsers() bool {
	return p.createUsers
}
