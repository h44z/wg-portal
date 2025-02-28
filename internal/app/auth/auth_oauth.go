package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type PlainOauthAuthenticator struct {
	name                string
	cfg                 *oauth2.Config
	userInfoEndpoint    string
	client              *http.Client
	userInfoMapping     config.OauthFields
	userAdminMapping    *config.OauthAdminMapping
	registrationEnabled bool
	userInfoLogging     bool
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

	return provider, nil
}

func (p PlainOauthAuthenticator) GetName() string {
	return p.name
}

func (p PlainOauthAuthenticator) RegistrationEnabled() bool {
	return p.registrationEnabled
}

func (p PlainOauthAuthenticator) GetType() domain.AuthenticatorType {
	return domain.AuthenticatorTypeOAuth
}

func (p PlainOauthAuthenticator) AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string {
	return p.cfg.AuthCodeURL(state, opts...)
}

func (p PlainOauthAuthenticator) Exchange(
	ctx context.Context,
	code string,
	opts ...oauth2.AuthCodeOption,
) (*oauth2.Token, error) {
	return p.cfg.Exchange(ctx, code, opts...)
}

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
		logrus.Tracef("User info from OAuth source %s: %v", p.name, string(contents))
	}

	return userFields, nil
}

func (p PlainOauthAuthenticator) ParseUserInfo(raw map[string]any) (*domain.AuthenticatorUserInfo, error) {
	return parseOauthUserInfo(p.userInfoMapping, p.userAdminMapping, raw)
}
