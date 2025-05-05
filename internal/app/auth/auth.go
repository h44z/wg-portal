package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/app/audit"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// region dependencies

type UserManager interface {
	// GetUser returns a user by its identifier.
	GetUser(context.Context, domain.UserIdentifier) (*domain.User, error)
	// RegisterUser creates a new user in the database.
	RegisterUser(ctx context.Context, user *domain.User) error
	// UpdateUser updates an existing user in the database.
	UpdateUser(ctx context.Context, user *domain.User) (*domain.User, error)
}

type EventBus interface {
	// Publish sends a message to the message bus.
	Publish(topic string, args ...any)
}

// endregion dependencies

type AuthenticatorType string

const (
	AuthenticatorTypeOAuth AuthenticatorType = "oauth"
	AuthenticatorTypeOidc  AuthenticatorType = "oidc"
)

// AuthenticatorOauth is the interface for all OAuth authenticators.
type AuthenticatorOauth interface {
	// GetName returns the name of the authenticator.
	GetName() string
	// GetType returns the type of the authenticator. It can be either AuthenticatorTypeOAuth or AuthenticatorTypeOidc.
	GetType() AuthenticatorType
	// AuthCodeURL returns the URL for the authentication flow.
	AuthCodeURL(state string, opts ...oauth2.AuthCodeOption) string
	// Exchange exchanges the OAuth code for an access token.
	Exchange(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	// GetUserInfo fetches the user information from the OAuth or OIDC provider.
	GetUserInfo(ctx context.Context, token *oauth2.Token, nonce string) (map[string]any, error)
	// ParseUserInfo parses the raw user information into a domain.AuthenticatorUserInfo struct.
	ParseUserInfo(raw map[string]any) (*domain.AuthenticatorUserInfo, error)
	// RegistrationEnabled returns whether registration is enabled for the OAuth authenticator.
	RegistrationEnabled() bool
	// GetAllowedDomains returns the list of whitelisted domains
	GetAllowedDomains() []string
}

// AuthenticatorLdap is the interface for all LDAP authenticators.
type AuthenticatorLdap interface {
	// GetName returns the name of the authenticator.
	GetName() string
	// PlaintextAuthentication performs a plaintext authentication against the LDAP server.
	PlaintextAuthentication(userId domain.UserIdentifier, plainPassword string) error
	// GetUserInfo fetches the user information from the LDAP server.
	GetUserInfo(ctx context.Context, username domain.UserIdentifier) (map[string]any, error)
	// ParseUserInfo parses the raw user information into a domain.AuthenticatorUserInfo struct.
	ParseUserInfo(raw map[string]any) (*domain.AuthenticatorUserInfo, error)
	// RegistrationEnabled returns whether registration is enabled for the LDAP authenticator.
	RegistrationEnabled() bool
}

// Authenticator is the main entry point for all authentication related tasks.
// This includes password authentication and external authentication providers (OIDC, OAuth, LDAP).
type Authenticator struct {
	cfg *config.Auth
	bus EventBus

	oauthAuthenticators map[string]AuthenticatorOauth
	ldapAuthenticators  map[string]AuthenticatorLdap

	// URL prefix for the callback endpoints, this is a combination of the external URL and the API prefix
	callbackUrlPrefix string

	users UserManager
}

// NewAuthenticator creates a new Authenticator instance.
func NewAuthenticator(cfg *config.Auth, extUrl string, bus EventBus, users UserManager) (
	*Authenticator,
	error,
) {
	a := &Authenticator{
		cfg:               cfg,
		bus:               bus,
		users:             users,
		callbackUrlPrefix: fmt.Sprintf("%s/api/v0", extUrl),
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
	extUrl, err := url.Parse(a.callbackUrlPrefix)
	if err != nil {
		return fmt.Errorf("failed to parse external url: %w", err)
	}

	a.oauthAuthenticators = make(map[string]AuthenticatorOauth, len(a.cfg.OpenIDConnect)+len(a.cfg.OAuth))
	a.ldapAuthenticators = make(map[string]AuthenticatorLdap, len(a.cfg.Ldap))

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

// GetExternalLoginProviders returns a list of all available external login providers.
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
			ProviderUrl: fmt.Sprintf("/auth/login/%s/init", providerId),
			CallbackUrl: fmt.Sprintf("/auth/login/%s/callback", providerId),
		})
	}

	return authProviders
}

// IsUserValid checks if a user is valid and not locked or disabled.
func (a *Authenticator) IsUserValid(ctx context.Context, id domain.UserIdentifier) bool {
	ctx = domain.SetUserInfo(ctx, domain.SystemAdminContextUserInfo()) // switch to admin user context
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

// PlainLogin performs a password authentication for a user. The username and password are trimmed before usage.
// If the login is successful, the user is returned, otherwise an error.
func (a *Authenticator) PlainLogin(ctx context.Context, username, password string) (*domain.User, error) {
	// Validate form input
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return nil, fmt.Errorf("missing username or password")
	}

	user, err := a.passwordAuthentication(ctx, domain.UserIdentifier(username), password)
	if err != nil {
		a.bus.Publish(app.TopicAuditLoginFailed, domain.AuditEventWrapper[audit.AuthEvent]{
			Ctx:    ctx,
			Source: "plain",
			Event: audit.AuthEvent{
				Username: username, Error: err.Error(),
			},
		})
		return nil, fmt.Errorf("login failed: %w", err)
	}

	a.bus.Publish(app.TopicAuthLogin, user.Identifier)
	a.bus.Publish(app.TopicAuditLoginSuccess, domain.AuditEventWrapper[audit.AuthEvent]{
		Ctx:    ctx,
		Source: "plain",
		Event: audit.AuthEvent{
			Username: string(user.Identifier),
		},
	})

	return user, nil
}

func (a *Authenticator) passwordAuthentication(
	ctx context.Context,
	identifier domain.UserIdentifier,
	password string,
) (*domain.User, error) {
	ctx = domain.SetUserInfo(ctx,
		domain.SystemAdminContextUserInfo()) // switch to admin user context to check if user exists

	var ldapUserInfo *domain.AuthenticatorUserInfo
	var ldapProvider AuthenticatorLdap

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
					slog.Warn("failed to fetch ldap user info", "identifier", identifier, "error", err)
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

	if userSource == domain.UserSourceLdap && ldapProvider == nil {
		return nil, errors.New("ldap provider not found")
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
		user, err := a.processUserInfo(ctx, ldapUserInfo, domain.UserSourceLdap, ldapProvider.GetName(),
			ldapProvider.RegistrationEnabled())
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

// OauthLoginStep1 starts the oauth authentication flow by returning the authentication URL, state and nonce.
func (a *Authenticator) OauthLoginStep1(_ context.Context, providerId string) (
	authCodeUrl, state, nonce string,
	err error,
) {
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
	case AuthenticatorTypeOAuth:
		authCodeUrl = oauthProvider.AuthCodeURL(state)
	case AuthenticatorTypeOidc:
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

func isDomainAllowed(email string, allowedDomains []string) bool {
	if len(allowedDomains) == 0 {
		return true
	}
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	domain := strings.ToLower(parts[1])
	for _, allowed := range allowedDomains {
		if domain == strings.ToLower(allowed) {
			return true
		}
	}
	return false
}

// OauthLoginStep2 finishes the oauth authentication flow by exchanging the code for an access token and
// fetching the user information.
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

	ctx = domain.SetUserInfo(ctx,
		domain.SystemAdminContextUserInfo()) // switch to admin user context to check if user exists
	user, err := a.processUserInfo(ctx, userInfo, domain.UserSourceOauth, oauthProvider.GetName(),
		oauthProvider.RegistrationEnabled())
	if err != nil {
		a.bus.Publish(app.TopicAuditLoginFailed, domain.AuditEventWrapper[audit.AuthEvent]{
			Ctx:    ctx,
			Source: "oauth " + providerId,
			Event: audit.AuthEvent{
				Username: string(userInfo.Identifier),
				Error:    err.Error(),
			},
		})
		return nil, fmt.Errorf("unable to process user information: %w", err)
	}

	if !isDomainAllowed(userInfo.Email, oauthProvider.GetAllowedDomains()) {
		return nil, fmt.Errorf("user is not in allowed domains: %w", err)
	}

	if user.IsLocked() || user.IsDisabled() {
		a.bus.Publish(app.TopicAuditLoginFailed, domain.AuditEventWrapper[audit.AuthEvent]{
			Ctx:    ctx,
			Source: "oauth " + providerId,
			Event: audit.AuthEvent{
				Username: string(user.Identifier),
				Error:    "user is locked",
			},
		})
		return nil, errors.New("user is locked")
	}

	a.bus.Publish(app.TopicAuthLogin, user.Identifier)
	a.bus.Publish(app.TopicAuditLoginSuccess, domain.AuditEventWrapper[audit.AuthEvent]{
		Ctx:    ctx,
		Source: "oauth " + providerId,
		Event: audit.AuthEvent{
			Username: string(user.Identifier),
		},
	})

	return user, nil
}

func (a *Authenticator) processUserInfo(
	ctx context.Context,
	userInfo *domain.AuthenticatorUserInfo,
	source domain.UserSource,
	provider string,
	withReg bool,
) (*domain.User, error) {
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
	default:
		err = a.updateExternalUser(ctx, user, userInfo, source, provider)
		if err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}
	}

	return user, nil
}

func (a *Authenticator) registerNewUser(
	ctx context.Context,
	userInfo *domain.AuthenticatorUserInfo,
	source domain.UserSource,
	provider string,
) (*domain.User, error) {
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

	slog.Debug("registered user from external authentication provider",
		"user", user.Identifier,
		"isAdmin", user.IsAdmin,
		"provider", source)

	return user, nil
}

func (a *Authenticator) getAuthenticatorConfig(id string) (any, error) {
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

func (a *Authenticator) updateExternalUser(
	ctx context.Context,
	existingUser *domain.User,
	userInfo *domain.AuthenticatorUserInfo,
	source domain.UserSource,
	provider string,
) error {
	if existingUser.IsLocked() || existingUser.IsDisabled() {
		return nil // user is locked or disabled, do not update
	}

	isChanged := false
	if existingUser.Email != userInfo.Email {
		existingUser.Email = userInfo.Email
		isChanged = true
	}
	if existingUser.Firstname != userInfo.Firstname {
		existingUser.Firstname = userInfo.Firstname
		isChanged = true
	}
	if existingUser.Lastname != userInfo.Lastname {
		existingUser.Lastname = userInfo.Lastname
		isChanged = true
	}
	if existingUser.Phone != userInfo.Phone {
		existingUser.Phone = userInfo.Phone
		isChanged = true
	}
	if existingUser.Department != userInfo.Department {
		existingUser.Department = userInfo.Department
		isChanged = true
	}
	if existingUser.IsAdmin != userInfo.IsAdmin {
		existingUser.IsAdmin = userInfo.IsAdmin
		isChanged = true
	}
	if existingUser.Source != source {
		existingUser.Source = source
		isChanged = true
	}
	if existingUser.ProviderName != provider {
		existingUser.ProviderName = provider
		isChanged = true
	}

	if !isChanged {
		return nil // nothing to update
	}

	_, err := a.users.UpdateUser(ctx, existingUser)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	slog.Debug("updated user with data from external authentication provider",
		"user", existingUser.Identifier,
		"isAdmin", existingUser.IsAdmin,
		"provider", source)

	return nil
}

// endregion oauth authentication
