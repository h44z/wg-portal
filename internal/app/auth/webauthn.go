package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/app/audit"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type WebAuthnUserManager interface {
	// GetUser returns a user by its identifier.
	GetUser(context.Context, domain.UserIdentifier) (*domain.User, error)
	// GetUserByWebAuthnCredential returns a user by its WebAuthn ID.
	GetUserByWebAuthnCredential(ctx context.Context, credentialIdBase64 string) (*domain.User, error)
	// UpdateUser updates an existing user in the database.
	UpdateUser(ctx context.Context, user *domain.User) (*domain.User, error)
}

type WebAuthnAuthenticator struct {
	webAuthn *webauthn.WebAuthn
	users    WebAuthnUserManager
	bus      EventBus
}

func NewWebAuthnAuthenticator(cfg *config.Config, bus EventBus, users WebAuthnUserManager) (
	*WebAuthnAuthenticator,
	error,
) {
	if !cfg.Auth.WebAuthn.Enabled {
		return nil, nil
	}

	extUrl, err := url.Parse(cfg.Web.ExternalUrl)
	if err != nil {
		return nil, errors.New("failed to parse external URL - required for WebAuthn RP ID")
	}

	rpId := extUrl.Hostname()
	if rpId == "" {
		return nil, errors.New("failed to determine Webauthn RPID")
	}

	// Initialize the WebAuthn authenticator with the provided configuration
	awCfg := &webauthn.Config{
		RPID:          rpId,
		RPDisplayName: cfg.Web.SiteTitle,
		RPOrigins:     []string{cfg.Web.ExternalUrl},
	}

	webAuthn, err := webauthn.New(awCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create Webauthn instance: %w", err)
	}

	return &WebAuthnAuthenticator{
		webAuthn: webAuthn,
		users:    users,
		bus:      bus,
	}, nil
}

func (a *WebAuthnAuthenticator) Enabled() bool {
	return a != nil && a.webAuthn != nil
}

func (a *WebAuthnAuthenticator) StartWebAuthnRegistration(ctx context.Context, userId domain.UserIdentifier) (
	optionsAsJSON []byte,
	sessionDataAsJSON []byte,
	err error,
) {
	user, err := a.users.GetUser(ctx, userId)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user.IsLocked() || user.IsDisabled() {
		return nil, nil, errors.New("user is locked") // adding passkey to locked user is not allowed
	}

	if user.WebAuthnId == "" {
		user.GenerateWebAuthnId()
		user, err = a.users.UpdateUser(ctx, user)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to store webauthn id to user: %w", err)
		}
	}

	options, sessionData, err := a.webAuthn.BeginRegistration(user,
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to begin WebAuthn registration: %w", err)
	}

	optionsAsJSON, err = json.Marshal(options)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal webauthn options to JSON: %w", err)
	}
	sessionDataAsJSON, err = json.Marshal(sessionData)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal webauthn session data to JSON: %w", err)
	}

	return optionsAsJSON, sessionDataAsJSON, nil
}

func (a *WebAuthnAuthenticator) FinishWebAuthnRegistration(
	ctx context.Context,
	userId domain.UserIdentifier,
	name string,
	sessionDataAsJSON []byte,
	r *http.Request,
) ([]domain.UserWebauthnCredential, error) {
	user, err := a.users.GetUser(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user.IsLocked() || user.IsDisabled() {
		return nil, errors.New("user is locked") // adding passkey to locked user is not allowed
	}

	var webAuthnData webauthn.SessionData
	err = json.Unmarshal(sessionDataAsJSON, &webAuthnData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal webauthn session data: %w", err)
	}

	credential, err := a.webAuthn.FinishRegistration(user, webAuthnData, r)
	if err != nil {
		return nil, err
	}

	if name == "" {
		name = fmt.Sprintf("Passkey %d", len(user.WebAuthnCredentialList)+1) // fallback name
	}

	// Add the credential to the user
	err = user.AddCredential(userId, name, *credential)
	if err != nil {
		return nil, err
	}

	user, err = a.users.UpdateUser(ctx, user)
	if err != nil {
		return nil, err
	}

	return user.WebAuthnCredentialList, nil
}

func (a *WebAuthnAuthenticator) GetCredentials(
	ctx context.Context,
	userId domain.UserIdentifier,
) ([]domain.UserWebauthnCredential, error) {
	user, err := a.users.GetUser(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user.WebAuthnCredentialList, nil
}

func (a *WebAuthnAuthenticator) RemoveCredential(
	ctx context.Context,
	userId domain.UserIdentifier,
	credentialIdBase64 string,
) ([]domain.UserWebauthnCredential, error) {
	user, err := a.users.GetUser(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user.RemoveCredential(credentialIdBase64)
	user, err = a.users.UpdateUser(ctx, user)
	if err != nil {
		return nil, err
	}

	return user.WebAuthnCredentialList, nil
}

func (a *WebAuthnAuthenticator) UpdateCredential(
	ctx context.Context,
	userId domain.UserIdentifier,
	credentialIdBase64 string,
	name string,
) ([]domain.UserWebauthnCredential, error) {
	user, err := a.users.GetUser(ctx, userId)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	err = user.UpdateCredential(credentialIdBase64, name)
	if err != nil {
		return nil, err
	}

	user, err = a.users.UpdateUser(ctx, user)
	if err != nil {
		return nil, err
	}

	return user.WebAuthnCredentialList, nil
}

func (a *WebAuthnAuthenticator) StartWebAuthnLogin(_ context.Context) (
	optionsAsJSON []byte,
	sessionDataAsJSON []byte,
	err error,
) {
	options, sessionData, err := a.webAuthn.BeginDiscoverableLogin()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to begin WebAuthn login: %w", err)
	}

	optionsAsJSON, err = json.Marshal(options)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal webauthn options to JSON: %w", err)
	}
	sessionDataAsJSON, err = json.Marshal(sessionData)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal webauthn session data to JSON: %w", err)
	}

	return optionsAsJSON, sessionDataAsJSON, nil
}

func (a *WebAuthnAuthenticator) FinishWebAuthnLogin(
	ctx context.Context,
	sessionDataAsJSON []byte,
	r *http.Request,
) (*domain.User, error) {

	var webAuthnData webauthn.SessionData
	err := json.Unmarshal(sessionDataAsJSON, &webAuthnData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal webauthn session data: %w", err)
	}

	// switch to admin context for user lookup
	ctx = domain.SetUserInfo(ctx, domain.SystemAdminContextUserInfo())

	credential, err := a.webAuthn.FinishDiscoverableLogin(a.findUserForWebAuthnSecretFn(ctx), webAuthnData, r)
	if err != nil {
		return nil, err
	}

	// Find the user by the WebAuthn ID
	user, err := a.users.GetUserByWebAuthnCredential(ctx,
		base64.StdEncoding.EncodeToString(credential.ID))
	if err != nil {
		return nil, fmt.Errorf("failed to get user by webauthn credential: %w", err)
	}

	if user.IsLocked() || user.IsDisabled() {
		a.bus.Publish(app.TopicAuditLoginFailed, domain.AuditEventWrapper[audit.AuthEvent]{
			Ctx:    ctx,
			Source: "passkey",
			Event: audit.AuthEvent{
				Username: string(user.Identifier), Error: "User is locked",
			},
		})
		return nil, errors.New("user is locked") // login with passkey is not allowed
	}

	a.bus.Publish(app.TopicAuthLogin, user.Identifier)
	a.bus.Publish(app.TopicAuditLoginSuccess, domain.AuditEventWrapper[audit.AuthEvent]{
		Ctx:    ctx,
		Source: "passkey",
		Event: audit.AuthEvent{
			Username: string(user.Identifier),
		},
	})

	return user, nil
}

func (a *WebAuthnAuthenticator) findUserForWebAuthnSecretFn(ctx context.Context) func(rawID, userHandle []byte) (
	user webauthn.User,
	err error,
) {
	return func(rawID, userHandle []byte) (webauthn.User, error) {
		// Find the user by the WebAuthn ID
		user, err := a.users.GetUserByWebAuthnCredential(ctx, base64.StdEncoding.EncodeToString(rawID))
		if err != nil {
			return nil, fmt.Errorf("failed to get user by webauthn credential: %w", err)
		}

		return user, nil
	}
}
