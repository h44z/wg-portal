package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"golang.org/x/oauth2"
)

type plainOauthAuthenticator struct {
	name                string
	cfg                 *oauth2.Config
	userInfoEndpoint    string
	client              *http.Client
	userInfoMapping     config.OauthFields
	registrationEnabled bool
}

func newPlainOauthAuthenticator(_ context.Context, callbackUrl string, cfg *config.OAuthProvider) (*plainOauthAuthenticator, error) {
	var provider = &plainOauthAuthenticator{}

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
	provider.registrationEnabled = cfg.RegistrationEnabled

	return provider, nil
}

func (p plainOauthAuthenticator) GetName() string {
	return p.name
}

func (p plainOauthAuthenticator) RegistrationEnabled() bool {
	return p.registrationEnabled
}

func (p plainOauthAuthenticator) GetType() domain.AuthenticatorType {
	return domain.AuthenticatorTypeOAuth
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
		return nil, fmt.Errorf("failed to create user info get request: %w", err)
	}
	req.Header.Add("Authorization", "Bearer "+token.AccessToken)
	req.WithContext(ctx)

	response, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer response.Body.Close()
	contents, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var userFields map[string]interface{}
	err = json.Unmarshal(contents, &userFields)
	if err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	return userFields, nil
}

func (p plainOauthAuthenticator) ParseUserInfo(raw map[string]interface{}) (*domain.AuthenticatorUserInfo, error) {
	isAdmin, _ := strconv.ParseBool(internal.MapDefaultString(raw, p.userInfoMapping.IsAdmin, ""))
	userInfo := &domain.AuthenticatorUserInfo{
		Identifier: domain.UserIdentifier(internal.MapDefaultString(raw, p.userInfoMapping.UserIdentifier, "")),
		Email:      internal.MapDefaultString(raw, p.userInfoMapping.Email, ""),
		Firstname:  internal.MapDefaultString(raw, p.userInfoMapping.Firstname, ""),
		Lastname:   internal.MapDefaultString(raw, p.userInfoMapping.Lastname, ""),
		Phone:      internal.MapDefaultString(raw, p.userInfoMapping.Phone, ""),
		Department: internal.MapDefaultString(raw, p.userInfoMapping.Department, ""),
		IsAdmin:    isAdmin,
	}

	return userInfo, nil
}

func getOauthFieldMapping(f config.OauthFields) config.OauthFields {
	defaultMap := config.OauthFields{
		BaseFields: config.BaseFields{
			UserIdentifier: "sub",
			Email:          "email",
			Firstname:      "given_name",
			Lastname:       "family_name",
			Phone:          "phone",
			Department:     "department",
		},
		IsAdmin: "admin_flag",
	}
	if f.UserIdentifier != "" {
		defaultMap.UserIdentifier = f.UserIdentifier
	}
	if f.Email != "" {
		defaultMap.Email = f.Email
	}
	if f.Firstname != "" {
		defaultMap.Firstname = f.Firstname
	}
	if f.Lastname != "" {
		defaultMap.Lastname = f.Lastname
	}
	if f.Phone != "" {
		defaultMap.Phone = f.Phone
	}
	if f.Department != "" {
		defaultMap.Department = f.Department
	}
	if f.IsAdmin != "" {
		defaultMap.IsAdmin = f.IsAdmin
	}

	return defaultMap
}
