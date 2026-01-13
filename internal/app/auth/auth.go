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
	// UpdateUserInternal updates an existing user in the database.
	UpdateUserInternal(ctx context.Context, user *domain.User) (*domain.User, error)
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

	callbackUrl *url.URL

	users UserManager
}

// NewAuthenticator creates a new Authenticator instance.
func NewAuthenticator(cfg *config.Auth, extUrl, basePath string, bus EventBus, users UserManager) (
	*Authenticator,
	error,
) {
	a := &Authenticator{
		cfg:                 cfg,
		bus:                 bus,
		users:               users,
		callbackUrlPrefix:   fmt.Sprintf("%s%s/api/v0", extUrl, basePath),
		oauthAuthenticators: make(map[string]AuthenticatorOauth, len(cfg.OpenIDConnect)+len(cfg.OAuth)),
		ldapAuthenticators:  make(map[string]AuthenticatorLdap, len(cfg.Ldap)),
	}

	parsedExtUrl, err := url.Parse(a.callbackUrlPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to parse external URL: %w", err)
	}
	a.callbackUrl = parsedExtUrl

	return a, nil
}

// StartBackgroundJobs starts the background jobs for the authenticator.
// It sets up the external authentication providers (OIDC, OAuth, LDAP) and retries in case of errors.
func (a *Authenticator) StartBackgroundJobs(ctx context.Context) {
	go func() {
		slog.Debug("setting up external auth providers...")

		// Initialize local copies of authentication providers to allow retry in case of errors
		oidcQueue := a.cfg.OpenIDConnect
		oauthQueue := a.cfg.OAuth
		ldapQueue := a.cfg.Ldap

		// Immediate attempt
		failedOidc, failedOauth, failedLdap := a.setupExternalAuthProviders(oidcQueue, oauthQueue, ldapQueue)
		if len(failedOidc) == 0 && len(failedOauth) == 0 && len(failedLdap) == 0 {
			slog.Info("successfully setup all external auth providers")
			return
		}

		// Prepare for retries with only the failed ones
		oidcQueue = failedOidc
		oauthQueue = failedOauth
		ldapQueue = failedLdap
		slog.Warn("failed to setup some external auth providers, retrying in 30 seconds",
			"failedOidc", len(failedOidc), "failedOauth", len(failedOauth), "failedLdap", len(failedLdap))

		ticker := time.NewTicker(30 * time.Second) // Ticker for delay between retries
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				failedOidc, failedOauth, failedLdap := a.setupExternalAuthProviders(oidcQueue, oauthQueue, ldapQueue)
				if len(failedOidc) > 0 || len(failedOauth) > 0 || len(failedLdap) > 0 {
					slog.Warn("failed to setup some external auth providers, retrying in 30 seconds",
						"failedOidc", len(failedOidc), "failedOauth", len(failedOauth), "failedLdap", len(failedLdap))
					// Retry failed providers
					oidcQueue = failedOidc
					oauthQueue = failedOauth
					ldapQueue = failedLdap
				} else {
					slog.Info("successfully setup all external auth providers")
					return // Exit goroutine if all providers are set up successfully
				}
			case <-ctx.Done():
				slog.Info("context cancelled, stopping setup of external auth providers")
				return // Exit goroutine if context is cancelled
			}
		}
	}()
}

func (a *Authenticator) setupExternalAuthProviders(
	oidc []config.OpenIDConnectProvider,
	oauth []config.OAuthProvider,
	ldap []config.LdapProvider,
) (
	[]config.OpenIDConnectProvider,
	[]config.OAuthProvider,
	[]config.LdapProvider,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var failedOidc []config.OpenIDConnectProvider
	var failedOauth []config.OAuthProvider
	var failedLdap []config.LdapProvider

	for i := range oidc { // OIDC
		providerCfg := &oidc[i]
		providerId := strings.ToLower(providerCfg.ProviderName)

		if _, exists := a.oauthAuthenticators[providerId]; exists {
			// this is an unrecoverable error, we cannot register the same provider twice
			slog.Error("OIDC auth provider is already registered", "name", providerId)
			continue // skip this provider
		}

		redirectUrl := *a.callbackUrl
		redirectUrl.Path = path.Join(redirectUrl.Path, "/auth/login/", providerId, "/callback")

		provider, err := newOidcAuthenticator(ctx, redirectUrl.String(), providerCfg)
		if err != nil {
			failedOidc = append(failedOidc, oidc[i])
			slog.Error("failed to setup oidc authentication provider", "name", providerId, "error", err)
			continue
		}
		a.oauthAuthenticators[providerId] = provider
	}
	for i := range oauth { // PLAIN OAUTH
		providerCfg := &oauth[i]
		providerId := strings.ToLower(providerCfg.ProviderName)

		if _, exists := a.oauthAuthenticators[providerId]; exists {
			// this is an unrecoverable error, we cannot register the same provider twice
			slog.Error("OAUTH auth provider is already registered", "name", providerId)
			continue // skip this provider
		}

		redirectUrl := *a.callbackUrl
		redirectUrl.Path = path.Join(redirectUrl.Path, "/auth/login/", providerId, "/callback")

		provider, err := newPlainOauthAuthenticator(ctx, redirectUrl.String(), providerCfg)
		if err != nil {
			failedOauth = append(failedOauth, oauth[i])
			slog.Error("failed to setup oauth authentication provider", "name", providerId, "error", err)
			continue
		}
		a.oauthAuthenticators[providerId] = provider
	}
	for i := range ldap { // LDAP
		providerCfg := &ldap[i]
		providerId := strings.ToLower(providerCfg.ProviderName)

		if _, exists := a.ldapAuthenticators[providerId]; exists {
			// this is an unrecoverable error, we cannot register the same provider twice
			slog.Error("LDAP auth provider is already registered", "name", providerId)
			continue // skip this provider
		}

		provider, err := newLdapAuthenticator(ctx, providerCfg)
		if err != nil {
			failedLdap = append(failedLdap, ldap[i])
			slog.Error("failed to setup ldap authentication provider", "name", providerId, "error", err)
			continue
		}
		a.ldapAuthenticators[providerId] = provider
	}

	return failedOidc, failedOauth, failedLdap
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
	existingUser, err := a.users.GetUser(ctx, identifier)
	if err == nil {
		userInDatabase = true
	}
	if userInDatabase && (existingUser.IsLocked() || existingUser.IsDisabled()) {
		return nil, errors.New("user is locked")
	}

	authOK := false
	if userInDatabase {
		// User is already in db, search for authentication sources which support password authentication and
		// validate the password.
		for _, authentication := range existingUser.Authentications {
			if authentication.Source == domain.UserSourceDatabase {
				err := existingUser.CheckPassword(password)
				if err == nil {
					authOK = true
					break
				}
			}

			if authentication.Source == domain.UserSourceLdap {
				ldapProvider, ok := a.ldapAuthenticators[strings.ToLower(authentication.ProviderName)]
				if !ok {
					continue // ldap provider not found, skip further checks
				}
				err := ldapProvider.PlaintextAuthentication(identifier, password)
				if err == nil {
					authOK = true
					break
				}
			}
		}
	} else {
		// User is not yet in the db, check ldap providers which have registration enabled.
		// If the user is found, check the password - on success, sync it to the db.
		for _, ldapAuth := range a.ldapAuthenticators {
			if !ldapAuth.RegistrationEnabled() {
				continue // ldap provider does not support registration, skip further checks
			}

			rawUserInfo, err := ldapAuth.GetUserInfo(context.Background(), identifier)
			if err != nil {
				if !errors.Is(err, domain.ErrNotFound) {
					slog.Warn("failed to fetch ldap user info",
						"source", ldapAuth.GetName(), "identifier", identifier, "error", err)
				}
				continue // user not found / other ldap error
			}

			// user found, check if the password is correct
			err = ldapAuth.PlaintextAuthentication(identifier, password)
			if err != nil {
				continue // password is incorrect, skip further checks
			}

			// create a new user in the db
			ldapUserInfo, err = ldapAuth.ParseUserInfo(rawUserInfo)
			if err != nil {
				slog.Error("failed to parse ldap user info",
					"source", ldapAuth.GetName(), "identifier", identifier, "error", err)
				continue
			}
			user, err := a.processUserInfo(ctx, ldapUserInfo, domain.UserSourceLdap, ldapProvider.GetName(), true)
			if err != nil {
				return nil, fmt.Errorf("unable to process user information: %w", err)
			}

			existingUser = user
			slog.Debug("created new LDAP user in db",
				"identifier", user.Identifier, "provider", ldapProvider.GetName())

			authOK = true
			break
		}
	}

	if !authOK {
		return nil, errors.New("failed to authenticate user")
	}

	return existingUser, nil
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

	if !isDomainAllowed(userInfo.Email, oauthProvider.GetAllowedDomains()) {
		return nil, fmt.Errorf("user %s is not in allowed domains", userInfo.Email)
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
	ctxUserInfo := domain.GetUserInfo(ctx)
	now := time.Now()

	// convert user info to domain.User
	user := &domain.User{
		Identifier: userInfo.Identifier,
		Email:      userInfo.Email,
		IsAdmin:    userInfo.IsAdmin,
		Firstname:  userInfo.Firstname,
		Lastname:   userInfo.Lastname,
		Phone:      userInfo.Phone,
		Department: userInfo.Department,
		Authentications: []domain.UserAuthentication{
			{
				BaseModel: domain.BaseModel{
					CreatedBy: ctxUserInfo.UserId(),
					UpdatedBy: ctxUserInfo.UserId(),
					CreatedAt: now,
					UpdatedAt: now,
				},
				UserIdentifier: userInfo.Identifier,
				Source:         source,
				ProviderName:   provider,
			},
		},
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

	// Update authentication sources
	foundAuthSource := false
	for _, auth := range existingUser.Authentications {
		if auth.Source == source && auth.ProviderName == provider {
			foundAuthSource = true
			break
		}
	}
	if !foundAuthSource {
		ctxUserInfo := domain.GetUserInfo(ctx)
		now := time.Now()
		existingUser.Authentications = append(existingUser.Authentications, domain.UserAuthentication{
			BaseModel: domain.BaseModel{
				CreatedBy: ctxUserInfo.UserId(),
				UpdatedBy: ctxUserInfo.UserId(),
				CreatedAt: now,
				UpdatedAt: now,
			},
			UserIdentifier: existingUser.Identifier,
			Source:         source,
			ProviderName:   provider,
		})
	}

	if existingUser.PersistLocalChanges {
		if !foundAuthSource {
			// Even if local changes are persisted, we need to save the new authentication source
			_, err := a.users.UpdateUserInternal(ctx, existingUser)
			return err
		}
		return nil
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

	if isChanged || !foundAuthSource {
		_, err := a.users.UpdateUserInternal(ctx, existingUser)
		if err != nil {
			return fmt.Errorf("failed to update user: %w", err)
		}

		slog.Debug("updated user with data from external authentication provider",
			"user", existingUser.Identifier,
			"isAdmin", existingUser.IsAdmin,
			"provider", source)
	}

	return nil
}

// endregion oauth authentication
