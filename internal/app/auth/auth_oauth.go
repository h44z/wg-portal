package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// PlainOauthAuthenticator is an authenticator that uses OAuth for authentication.
// User information is retrieved from the specified user info endpoint.
type PlainOauthAuthenticator struct {
	name                string
	cfg                 *oauth2.Config
	userInfoEndpoint    string
	client              *http.Client
	userInfoMapping     config.OauthFields
	userAdminMapping    *config.OauthAdminMapping
	registrationEnabled bool
	userInfoLogging     bool
	allowedDomains      []string
}

func newPlainOauthAuthenticator(
	_ context.Context,
	callbackUrl string,
	cfg *config.OAuthProvider,
) (*PlainOauthAuthenticator, error) {
	var provider = &PlainOauthAuthenticator{}

	provider.name = cfg.ProviderName
	provider.client = &http.Client{
		Timeout: time.Second * 10,
	}
	provider.cfg = &oauth2.Config{
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
	provider.userInfoEndpoint = cfg.UserInfoURL
	provider.userInfoMapping = getOauthFieldMapping(cfg.FieldMap)
	provider.userAdminMapping = &cfg.AdminMapping
	provider.registrationEnabled = cfg.RegistrationEnabled
	provider.userInfoLogging = cfg.LogUserInfo
	provider.allowedDomains = cfg.AllowedDomains

	return provider, nil
}

// GetName returns the name of the OAuth authenticator.
func (p PlainOauthAuthenticator) GetName() string {
	return p.name
}

func (p PlainOauthAuthenticator) GetAllowedDomains() []string {
	return p.allowedDomains
}

// RegistrationEnabled returns whether registration is enabled for the OAuth authenticator.
func (p PlainOauthAuthenticator) RegistrationEnabled() bool {
	return p.registrationEnabled
}

// GetType returns the type of the authenticator.
func (p PlainOauthAuthenticator) GetType() AuthenticatorType {
	return AuthenticatorTypeOAuth
}

// AuthCodeURL returns the URL to redirect the user to for authentication.
func (p PlainOauthAuthenticator) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	return p.cfg.AuthCodeURL(state, opts...)
}

// Exchange exchanges the OAuth code for a token.
func (p PlainOauthAuthenticator) Exchange(
	ctx context.Context,
	code string,
	opts ...oauth2.AuthCodeOption,
) (*oauth2.Token, error) {
	return p.cfg.Exchange(ctx, code, opts...)
}

// GetUserInfo retrieves the user information from the user info endpoint.
func (p PlainOauthAuthenticator) GetUserInfo(
	ctx context.Context,
	token *oauth2.Token,
	_ string,
) (map[string]any, error) {
	req, err := http.NewRequest("GET", p.userInfoEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create user info get request: %w", err)
	}
	req.Header.Add("Authorization", "Bearer "+token.AccessToken)
	req.WithContext(ctx)

	response, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer internal.LogClose(response.Body)
	contents, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var userFields map[string]any
	err = json.Unmarshal(contents, &userFields)
	if err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	if p.userInfoLogging {
		slog.Debug("OAuth user info",
			"source", p.name,
			"info", string(contents))
	}

	return userFields, nil
}

// ParseUserInfo parses the user information from the raw data.
func (p PlainOauthAuthenticator) ParseUserInfo(raw map[string]any) (*domain.AuthenticatorUserInfo, error) {
	return parseOauthUserInfo(p.userInfoMapping, p.userAdminMapping, raw)
}
