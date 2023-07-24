package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/sirupsen/logrus"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	evbus "github.com/vardius/message-bus"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type UserManager interface {
	GetUser(context.Context, domain.UserIdentifier) (*domain.User, error)
	RegisterUser(ctx context.Context, user *domain.User) error
}

type Authenticator struct {
	cfg *config.Auth
	bus evbus.MessageBus

	oauthAuthenticators map[string]domain.OauthAuthenticator
	ldapAuthenticators  map[string]domain.LdapAuthenticator

	users UserManager
}

func NewAuthenticator(cfg *config.Auth, bus evbus.MessageBus, users UserManager) (*Authenticator, error) {
	a := &Authenticator{
		cfg:   cfg,
		bus:   bus,
		users: users,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := a.setupExternalAuthProviders(ctx)
	if err != nil {
		return nil, err
	}

	return a, nil
}

func (a *Authenticator) setupExternalAuthProviders(ctx context.Context) error {
	extUrl, err := url.Parse(a.cfg.CallbackUrlPrefix)
	if err != nil {
		return fmt.Errorf("failed to parse external url: %w", err)
	}

	a.oauthAuthenticators = make(map[string]domain.OauthAuthenticator, len(a.cfg.OpenIDConnect)+len(a.cfg.OAuth))
	a.ldapAuthenticators = make(map[string]domain.LdapAuthenticator, len(a.cfg.Ldap))

	for i := range a.cfg.OpenIDConnect { // OIDC
		providerCfg := &a.cfg.OpenIDConnect[i]
		providerId := strings.ToLower(providerCfg.ProviderName)

		if _, exists := a.oauthAuthenticators[providerId]; exists {
			return fmt.Errorf("auth provider with name %s is already registerd", providerId)
		}

		redirectUrl := *extUrl
		redirectUrl.Path = path.Join(redirectUrl.Path, "/auth/login/", providerId, "/callback")

		provider, err := newOidcAuthenticator(ctx, redirectUrl.String(), providerCfg)
		if err != nil {
			return fmt.Errorf("failed to setup oidc authentication provider %s: %w", providerCfg.ProviderName, err)
		}
		a.oauthAuthenticators[providerId] = provider
	}
	for i := range a.cfg.OAuth { // PLAIN OAUTH
		providerCfg := &a.cfg.OAuth[i]
		providerId := strings.ToLower(providerCfg.ProviderName)

		if _, exists := a.oauthAuthenticators[providerId]; exists {
			return fmt.Errorf("auth provider with name %s is already registerd", providerId)
		}

		redirectUrl := *extUrl
		redirectUrl.Path = path.Join(redirectUrl.Path, "/auth/login/", providerId, "/callback")

		provider, err := newPlainOauthAuthenticator(ctx, redirectUrl.String(), providerCfg)
		if err != nil {
			return fmt.Errorf("failed to setup oauth authentication provider %s: %w", providerId, err)
		}
		a.oauthAuthenticators[providerId] = provider
	}
	for i := range a.cfg.Ldap { // LDAP
		providerCfg := &a.cfg.Ldap[i]
		providerId := strings.ToLower(providerCfg.URL)

		if _, exists := a.ldapAuthenticators[providerId]; exists {
			return fmt.Errorf("auth provider with name %s is already registerd", providerId)
		}

		provider, err := newLdapAuthenticator(ctx, providerCfg)
		if err != nil {
			return fmt.Errorf("failed to setup ldap authentication provider %s: %w", providerId, err)
		}
		a.ldapAuthenticators[providerId] = provider
	}

	return nil
}

func (a *Authenticator) GetExternalLoginProviders(_ context.Context) []domain.LoginProviderInfo {
	authProviders := make([]domain.LoginProviderInfo, 0, len(a.cfg.OAuth)+len(a.cfg.OpenIDConnect))

	for _, provider := range a.cfg.OpenIDConnect {
		providerId := strings.ToLower(provider.ProviderName)
		providerName := provider.DisplayName
		if providerName == "" {
			providerName = provider.ProviderName
		}
		authProviders = append(authProviders, domain.LoginProviderInfo{
			Identifier:  providerId,
			Name:        providerName,
			ProviderUrl: fmt.Sprintf("/auth/login/%s/init", providerId),
			CallbackUrl: fmt.Sprintf("/auth/login/%s/callback", providerId),
		})
	}

	for _, provider := range a.cfg.OAuth {
		providerId := strings.ToLower(provider.ProviderName)
		providerName := provider.DisplayName
		if providerName == "" {
			providerName = provider.ProviderName
		}
		authProviders = append(authProviders, domain.LoginProviderInfo{
			Identifier:  providerId,
			Name:        providerName,
			ProviderUrl: fmt.Sprintf("%s/%s/init", a.cfg.CallbackUrlPrefix, providerId),
			CallbackUrl: fmt.Sprintf("%s/%s/callback", a.cfg.CallbackUrlPrefix, providerId),
		})
	}

	return authProviders
}

func (a *Authenticator) IsUserValid(ctx context.Context, id domain.UserIdentifier) bool {
	user, err := a.users.GetUser(ctx, id)
	if err != nil {
		return false
	}

	if user.IsDisabled() {
		return false
	}

	if user.IsLocked() {
		return false
	}

	return true
}

// region password authentication

func (a *Authenticator) PlainLogin(ctx context.Context, username, password string) (*domain.User, error) {
	// Validate form input
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return nil, fmt.Errorf("missing username or password")
	}

	user, err := a.passwordAuthentication(ctx, domain.UserIdentifier(username), password)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}

	a.bus.Publish(app.TopicAuthLogin, user.Identifier)

	return user, nil
}

func (a *Authenticator) passwordAuthentication(ctx context.Context, identifier domain.UserIdentifier, password string) (*domain.User, error) {
	var ldapUserInfo *domain.AuthenticatorUserInfo
	var ldapProvider domain.LdapAuthenticator

	var userInDatabase = false
	var userSource domain.UserSource
	existingUser, err := a.users.GetUser(ctx, identifier)
	if err == nil {
		userInDatabase = true
		userSource = existingUser.Source
	}
	if userInDatabase && (existingUser.IsLocked() || existingUser.IsDisabled()) {
		return nil, errors.New("user is locked")
	}

	if !userInDatabase || userSource == domain.UserSourceLdap {
		// search user in ldap if registration is enabled
		for _, ldapAuth := range a.ldapAuthenticators {
			if !userInDatabase && !ldapAuth.RegistrationEnabled() {
				continue
			}

			rawUserInfo, err := ldapAuth.GetUserInfo(context.Background(), identifier)
			if err != nil {
				if !errors.Is(err, domain.ErrNotFound) {
					logrus.Warnf("failed to fetch ldap user info for %s: %v", identifier, err)
				}
				continue // user not found / other ldap error
			}
			ldapUserInfo, err = ldapAuth.ParseUserInfo(rawUserInfo)
			if err != nil {
				continue
			}

			// ldap user found
			userSource = domain.UserSourceLdap
			ldapProvider = ldapAuth

			break
		}
	}

	if userSource == "" {
		return nil, errors.New("user not found")
	}

	switch userSource {
	case domain.UserSourceDatabase:
		err = existingUser.CheckPassword(password)
	case domain.UserSourceLdap:
		err = ldapProvider.PlaintextAuthentication(identifier, password)
	default:
		err = errors.New("no authentication backend available")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate: %w", err)
	}

	if !userInDatabase {
		user, err := a.processUserInfo(ctx, ldapUserInfo, domain.UserSourceLdap, ldapProvider.GetName(), ldapProvider.RegistrationEnabled())
		if err != nil {
			return nil, fmt.Errorf("unable to process user information: %w", err)
		}
		return user, nil
	} else {
		return existingUser, nil
	}
}

// endregion password authentication

// region oauth authentication

func (a *Authenticator) OauthLoginStep1(_ context.Context, providerId string) (authCodeUrl, state, nonce string, err error) {
	oauthProvider, ok := a.oauthAuthenticators[providerId]
	if !ok {
		return "", "", "", fmt.Errorf("missing oauth provider %s", providerId)
	}

	// Prepare authentication flow, set state cookies
	state, err = a.randString(16)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate state: %w", err)
	}

	switch oauthProvider.GetType() {
	case domain.AuthenticatorTypeOAuth:
		authCodeUrl = oauthProvider.AuthCodeURL(state)
	case domain.AuthenticatorTypeOidc:
		nonce, err = a.randString(16)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to generate nonce: %w", err)
		}

		authCodeUrl = oauthProvider.AuthCodeURL(state, oidc.Nonce(nonce))
	}

	return
}

func (a *Authenticator) randString(nByte int) (string, error) {
	b := make([]byte, nByte)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (a *Authenticator) OauthLoginStep2(ctx context.Context, providerId, nonce, code string) (*domain.User, error) {
	oauthProvider, ok := a.oauthAuthenticators[providerId]
	if !ok {
		return nil, fmt.Errorf("missing oauth provider %s", providerId)
	}

	oauth2Token, err := oauthProvider.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("unable to exchange code: %w", err)
	}

	rawUserInfo, err := oauthProvider.GetUserInfo(ctx, oauth2Token, nonce)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch user information: %w", err)
	}

	userInfo, err := oauthProvider.ParseUserInfo(rawUserInfo)
	if err != nil {
		return nil, fmt.Errorf("unable to parse user information: %w", err)
	}

	user, err := a.processUserInfo(ctx, userInfo, domain.UserSourceOauth, oauthProvider.GetName(), oauthProvider.RegistrationEnabled())
	if err != nil {
		return nil, fmt.Errorf("unable to process user information: %w", err)
	}

	if user.IsLocked() || user.IsDisabled() {
		return nil, errors.New("user is locked")
	}

	a.bus.Publish(app.TopicAuthLogin, user.Identifier)

	return user, nil
}

func (a *Authenticator) processUserInfo(ctx context.Context, userInfo *domain.AuthenticatorUserInfo, source domain.UserSource, provider string, withReg bool) (*domain.User, error) {
	// Search user in backend
	user, err := a.users.GetUser(ctx, userInfo.Identifier)
	switch {
	case err != nil && withReg:
		user, err = a.registerNewUser(ctx, userInfo, source, provider)
		if err != nil {
			return nil, fmt.Errorf("failed to register user: %w", err)
		}
	case err != nil:
		return nil, fmt.Errorf("registration disabled, cannot create missing user: %w", err)
	}

	return user, nil
}

func (a *Authenticator) registerNewUser(ctx context.Context, userInfo *domain.AuthenticatorUserInfo, source domain.UserSource, provider string) (*domain.User, error) {
	// convert user info to domain.User
	user := &domain.User{
		Identifier:   userInfo.Identifier,
		Email:        userInfo.Email,
		Source:       source,
		ProviderName: provider,
		IsAdmin:      userInfo.IsAdmin,
		Firstname:    userInfo.Firstname,
		Lastname:     userInfo.Lastname,
		Phone:        userInfo.Phone,
		Department:   userInfo.Department,
	}

	err := a.users.RegisterUser(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to register new user: %w", err)
	}

	return user, nil
}

func (a *Authenticator) getAuthenticatorConfig(id string) (interface{}, error) {
	for i := range a.cfg.OpenIDConnect {
		if a.cfg.OpenIDConnect[i].ProviderName == id {
			return a.cfg.OpenIDConnect[i], nil
		}
	}

	for i := range a.cfg.OAuth {
		if a.cfg.OAuth[i].ProviderName == id {
			return a.cfg.OAuth[i], nil
		}
	}

	return nil, fmt.Errorf("no configuration for Authenticator id %s", id)
}

// endregion oauth authentication
