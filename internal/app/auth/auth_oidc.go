package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// OidcAuthenticator is an authenticator for OpenID Connect providers.
type OidcAuthenticator struct {
	name                 string
	provider             *oidc.Provider
	verifier             *oidc.IDTokenVerifier
	cfg                  *oauth2.Config
	userInfoMapping      config.OauthFields
	userAdminMapping     *config.OauthAdminMapping
	registrationEnabled  bool
	userInfoLogging      bool
	sensitiveInfoLogging bool
	allowedDomains       []string
	allowedUserGroups    []string
	endSessionEndpoint   string
	logoutIdpSession     bool
	usePKCE              bool
	pkceMethod           string
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
	provider.sensitiveInfoLogging = cfg.LogSensitiveInfo
	provider.allowedDomains = cfg.AllowedDomains
	provider.allowedUserGroups = cfg.AllowedUserGroups
	provider.logoutIdpSession = cfg.LogoutIdpSession == nil || *cfg.LogoutIdpSession
	provider.usePKCE = cfg.UsePKCE == nil || *cfg.UsePKCE
	provider.pkceMethod = cfg.PKCEMethod
	if provider.pkceMethod == "" {
		provider.pkceMethod = pkceMethodS256
	}
	if provider.usePKCE && provider.pkceMethod != pkceMethodS256 && provider.pkceMethod != pkceMethodPlain {
		return nil, fmt.Errorf("unsupported PKCE method %q, allowed: S256, plain", provider.pkceMethod)
	}

	var providerMetadata struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err = provider.provider.Claims(&providerMetadata); err != nil {
		slog.Debug("OIDC: failed to parse provider metadata", "provider", cfg.ProviderName, "error", err)
	} else {
		provider.endSessionEndpoint = providerMetadata.EndSessionEndpoint
	}

	return provider, nil
}

// GetName returns the name of the authenticator.
func (o OidcAuthenticator) GetName() string {
	return o.name
}

func (o OidcAuthenticator) GetAllowedDomains() []string {
	return o.allowedDomains
}

func (o OidcAuthenticator) GetAllowedUserGroups() []string {
	return o.allowedUserGroups
}

func (o OidcAuthenticator) GetLogoutUrl(idTokenHint, postLogoutRedirectUri string) (string, bool) {
	if !o.logoutIdpSession {
		return "", false
	}
	if o.endSessionEndpoint == "" {
		slog.Debug("OIDC logout URL generation disabled: provider has no end_session_endpoint", "provider", o.name)
		return "", false
	}

	logoutUrl, err := url.Parse(o.endSessionEndpoint)
	if err != nil {
		slog.Debug("OIDC logout URL generation failed, unable to parse end_session_endpoint url",
			"provider", o.name, "error", err)
		return "", false
	}

	params := logoutUrl.Query()
	if idTokenHint != "" {
		params.Set("id_token_hint", idTokenHint)
	}
	if postLogoutRedirectUri != "" {
		params.Set("post_logout_redirect_uri", postLogoutRedirectUri)
	}
	logoutUrl.RawQuery = params.Encode()

	return logoutUrl.String(), true
}

// PKCEAuthCodeOptions returns PKCE options for the authorization request and the verifier for the token exchange.
func (o OidcAuthenticator) PKCEAuthCodeOptions() ([]oauth2.AuthCodeOption, string) {
	if !o.usePKCE {
		return nil, ""
	}

	verifier := oauth2.GenerateVerifier()
	if o.pkceMethod == pkceMethodPlain {
		return []oauth2.AuthCodeOption{
			oauth2.SetAuthURLParam("code_challenge", verifier),
			oauth2.SetAuthURLParam("code_challenge_method", pkceMethodPlain),
		}, verifier
	}

	return []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(verifier)}, verifier
}

// PKCETokenOptions returns PKCE options for the token exchange.
func (o OidcAuthenticator) PKCETokenOptions(verifier string) []oauth2.AuthCodeOption {
	if !o.usePKCE || verifier == "" {
		return nil
	}

	return []oauth2.AuthCodeOption{oauth2.VerifierOption(verifier)}
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

// GetUserInfo retrieves the user info from the token and the userinfo endpoint.
func (o OidcAuthenticator) GetUserInfo(ctx context.Context, token *oauth2.Token, nonce string) (
	map[string]any,
	error,
) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		if o.sensitiveInfoLogging {
			slog.Debug("OIDC: token does not contain id_token", "token", token, "nonce", nonce)
		}
		return nil, errors.New("token does not contain id_token")
	}
	idToken, err := o.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		if o.sensitiveInfoLogging {
			slog.Debug("OIDC: failed to validate id_token", "token", token, "id_token", rawIDToken, "nonce", nonce,
				"error",
				err)
		}
		return nil, fmt.Errorf("failed to validate id_token: %w", err)
	}
	if idToken.Nonce != nonce {
		if o.sensitiveInfoLogging {
			slog.Debug("OIDC: id_token nonce mismatch", "token", token, "id_token", idToken, "nonce", nonce)
		}
		return nil, errors.New("nonce mismatch")
	}

	var tokenFields map[string]any
	if err = idToken.Claims(&tokenFields); err != nil {
		if o.sensitiveInfoLogging {
			slog.Debug("OIDC: failed to parse extra claims", "token", token, "id_token", idToken, "nonce", nonce,
				"error",
				err)
		}
		return nil, fmt.Errorf("failed to parse extra claims: %w", err)
	}

	// Fetch additional user information from the userinfo endpoint
	userInfo, err := o.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		if o.sensitiveInfoLogging {
			slog.Debug("OIDC: failed to fetch user info from endpoint", "provider", o.name, "error", err)
		}
		// Don't fail the entire flow if userinfo endpoint is unavailable;
		// ID token claims may be sufficient
		slog.Debug("OIDC: proceeding with ID token claims only", "provider", o.name)
	} else {
		// Parse claims from userinfo endpoint response
		var userInfoFields map[string]any
		if err = userInfo.Claims(&userInfoFields); err != nil {
			if o.sensitiveInfoLogging {
				slog.Debug("OIDC: failed to parse userinfo claims", "provider", o.name, "error", err)
			}
			// Don't fail if we can't parse userinfo; continue with ID token claims
			slog.Debug("OIDC: proceeding with ID token claims only", "provider", o.name)
		} else {
			// Merge userinfo fields into tokenFields, preferring ID token claims
			for key, value := range userInfoFields {
				if _, exists := tokenFields[key]; !exists {
					tokenFields[key] = value
				}
			}

			if o.userInfoLogging {
				contents, _ := json.Marshal(userInfoFields)
				slog.Debug("OIDC: user info from endpoint",
					"source", o.name,
					"info", string(contents))
			}
		}
	}

	if o.userInfoLogging {
		contents, _ := json.Marshal(tokenFields)
		slog.Debug("OIDC: user info debug",
			"source", o.name,
			"info", string(contents))
	}

	return tokenFields, nil
}

// ParseUserInfo parses the user info.
func (o OidcAuthenticator) ParseUserInfo(raw map[string]any) (*domain.AuthenticatorUserInfo, error) {
	return parseOauthUserInfo(o.userInfoMapping, o.userAdminMapping, raw, "oidc", o.name)
}
