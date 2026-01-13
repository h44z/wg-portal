package users

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// region dependencies

type UserDatabaseRepo interface {
	// GetUser returns the user with the given identifier.
	GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
	// GetUserByEmail returns the user with the given email address.
	GetUserByEmail(ctx context.Context, email string) (*domain.User, error)
	// GetUserByWebAuthnCredential returns the user for the given WebAuthn credential ID.
	GetUserByWebAuthnCredential(ctx context.Context, credentialIdBase64 string) (*domain.User, error)
	// GetAllUsers returns all users.
	GetAllUsers(ctx context.Context) ([]domain.User, error)
	// FindUsers returns all users matching the search string.
	FindUsers(ctx context.Context, search string) ([]domain.User, error)
	// SaveUser saves the user with the given identifier.
	SaveUser(ctx context.Context, id domain.UserIdentifier, updateFunc func(u *domain.User) (*domain.User, error)) error
	// DeleteUser deletes the user with the given identifier.
	DeleteUser(ctx context.Context, id domain.UserIdentifier) error
}

type PeerDatabaseRepo interface {
	// GetUserPeers returns all peers linked to the given user.
	GetUserPeers(ctx context.Context, id domain.UserIdentifier) ([]domain.Peer, error)
}

type EventBus interface {
	// Publish sends a message to the message bus.
	Publish(topic string, args ...any)
}

// endregion dependencies

// Manager is the user manager.
type Manager struct {
	cfg *config.Config

	bus   EventBus
	users UserDatabaseRepo
	peers PeerDatabaseRepo
}

// NewUserManager creates a new user manager instance.
func NewUserManager(cfg *config.Config, bus EventBus, users UserDatabaseRepo, peers PeerDatabaseRepo) (
	*Manager,
	error,
) {
	m := &Manager{
		cfg: cfg,
		bus: bus,

		users: users,
		peers: peers,
	}
	return m, nil
}

// RegisterUser registers a new user.
func (m Manager) RegisterUser(ctx context.Context, user *domain.User) error {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return err
	}

	createdUser, err := m.create(ctx, user)
	if err != nil {
		return err
	}

	m.bus.Publish(app.TopicUserRegistered, *createdUser)

	return nil
}

// StartBackgroundJobs starts the background jobs.
// This method is non-blocking and returns immediately.
func (m Manager) StartBackgroundJobs(ctx context.Context) {
	go m.runLdapSynchronizationService(ctx)
}

// GetUser returns the user with the given identifier.
func (m Manager) GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error) {
	if err := domain.ValidateUserAccessRights(ctx, id); err != nil {
		return nil, err
	}

	return m.getUser(ctx, id)
}

// GetUserByEmail returns the user with the given email address.
func (m Manager) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	user, err := m.users.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("unable to load user for email %s: %w", email, err)
	}

	if err := domain.ValidateUserAccessRights(ctx, user.Identifier); err != nil {
		return nil, err
	}

	return m.enrichUser(ctx, user), nil
}

// GetUserByWebAuthnCredential returns the user for the given WebAuthn credential.
func (m Manager) GetUserByWebAuthnCredential(ctx context.Context, credentialIdBase64 string) (*domain.User, error) {
	user, err := m.users.GetUserByWebAuthnCredential(ctx, credentialIdBase64)
	if err != nil {
		return nil, fmt.Errorf("unable to load user for webauthn credential %s: %w", credentialIdBase64, err)
	}

	if err := domain.ValidateUserAccessRights(ctx, user.Identifier); err != nil {
		return nil, err
	}

	return m.enrichUser(ctx, user), nil
}

// GetAllUsers returns all users.
func (m Manager) GetAllUsers(ctx context.Context) ([]domain.User, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	users, err := m.users.GetAllUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load users: %w", err)
	}

	ch := make(chan *domain.User)
	wg := sync.WaitGroup{}
	workers := int(math.Min(float64(len(users)), 10))
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for user := range ch {
				m.enrichUser(ctx, user)
			}
		}()
	}
	for i := range users {
		ch <- &users[i]
	}
	close(ch)
	wg.Wait()

	return users, nil
}

// UpdateUser updates the user with the given identifier.
func (m Manager) UpdateUser(ctx context.Context, user *domain.User) (*domain.User, error) {
	if err := domain.ValidateUserAccessRights(ctx, user.Identifier); err != nil {
		return nil, err
	}

	existingUser, err := m.users.GetUser(ctx, user.Identifier)
	if err != nil {
		return nil, fmt.Errorf("unable to load existing user %s: %w", user.Identifier, err)
	}

	user.CopyCalculatedAttributes(existingUser, true) // ensure that crucial attributes stay the same

	return m.update(ctx, existingUser, user, true)
}

// UpdateUserInternal updates the user with the given identifier. This function must never be called from external.
// This function allows to override authentications and webauthn credentials.
func (m Manager) UpdateUserInternal(ctx context.Context, user *domain.User) (*domain.User, error) {
	existingUser, err := m.users.GetUser(ctx, user.Identifier)
	if err != nil {
		return nil, fmt.Errorf("unable to load existing user %s: %w", user.Identifier, err)
	}

	return m.update(ctx, existingUser, user, false)
}

// CreateUser creates a new user.
func (m Manager) CreateUser(ctx context.Context, user *domain.User) (*domain.User, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	return m.create(ctx, user)
}

// DeleteUser deletes the user with the given identifier.
func (m Manager) DeleteUser(ctx context.Context, id domain.UserIdentifier) error {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return err
	}

	existingUser, err := m.users.GetUser(ctx, id)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("unable to find user %s: %w", id, err)
	}

	if err := m.validateDeletion(ctx, existingUser); err != nil {
		return fmt.Errorf("deletion not allowed: %w", err)
	}

	err = m.users.DeleteUser(ctx, id)
	if err != nil {
		return fmt.Errorf("deletion failure: %w", err)
	}

	m.bus.Publish(app.TopicUserDeleted, *existingUser)

	return nil
}

// ActivateApi activates the API access for the user with the given identifier.
func (m Manager) ActivateApi(ctx context.Context, id domain.UserIdentifier) (*domain.User, error) {
	user, err := m.users.GetUser(ctx, id)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("unable to find user %s: %w", id, err)
	}

	if err := m.validateApiChange(ctx, user); err != nil {
		return nil, err
	}

	now := time.Now()
	user.ApiToken = uuid.New().String()
	user.ApiTokenCreated = &now

	user, err = m.update(ctx, user, user, true) // self-update
	if err != nil {
		return nil, err
	}
	m.bus.Publish(app.TopicUserApiEnabled, *user)

	return user, nil
}

// DeactivateApi deactivates the API access for the user with the given identifier.
func (m Manager) DeactivateApi(ctx context.Context, id domain.UserIdentifier) (*domain.User, error) {
	user, err := m.users.GetUser(ctx, id)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("unable to find user %s: %w", id, err)
	}

	if err := m.validateApiChange(ctx, user); err != nil {
		return nil, err
	}

	user.ApiToken = ""
	user.ApiTokenCreated = nil

	user, err = m.update(ctx, user, user, true) // self-update
	if err != nil {
		return nil, err
	}
	m.bus.Publish(app.TopicUserApiDisabled, *user)

	return user, nil
}

func (m Manager) validateModifications(ctx context.Context, old, new *domain.User) error {
	currentUser := domain.GetUserInfo(ctx)

	if currentUser.Id != new.Identifier && !currentUser.IsAdmin {
		return fmt.Errorf("insufficient permissions")
	}

	if err := old.EditAllowed(new); err != nil && currentUser.Id != domain.SystemAdminContextUserInfo().Id {
		return errors.Join(fmt.Errorf("no access: %w", err), domain.ErrInvalidData)
	}

	if err := old.CanChangePassword(); err != nil && string(new.Password) != "" {
		return errors.Join(fmt.Errorf("no access: %w", err), domain.ErrInvalidData)
	}

	if err := new.HasWeakPassword(m.cfg.Auth.MinPasswordLength); err != nil {
		return errors.Join(fmt.Errorf("password too weak: %w", err), domain.ErrInvalidData)
	}

	if currentUser.Id == old.Identifier && old.IsAdmin && !new.IsAdmin {
		return fmt.Errorf("cannot remove own admin rights: %w", domain.ErrInvalidData)
	}

	if currentUser.Id == old.Identifier && new.IsDisabled() {
		return fmt.Errorf("cannot disable own user: %w", domain.ErrInvalidData)
	}

	if currentUser.Id == old.Identifier && new.IsLocked() {
		return fmt.Errorf("cannot lock own user: %w", domain.ErrInvalidData)
	}

	return nil
}

func (m Manager) validateCreation(ctx context.Context, new *domain.User) error {
	currentUser := domain.GetUserInfo(ctx)

	if !currentUser.IsAdmin {
		return fmt.Errorf("insufficient permissions")
	}

	if new.Identifier == "" {
		return fmt.Errorf("invalid user identifier: %w", domain.ErrInvalidData)
	}

	if new.Identifier == "all" { // the 'all' user identifier collides with the rest api routes
		return fmt.Errorf("reserved user identifier: %w", domain.ErrInvalidData)
	}

	if new.Identifier == "new" { // the 'new' user identifier collides with the rest api routes
		return fmt.Errorf("reserved user identifier: %w", domain.ErrInvalidData)
	}

	if new.Identifier == "id" { // the 'id' user identifier collides with the rest api routes
		return fmt.Errorf("reserved user identifier: %w", domain.ErrInvalidData)
	}

	if new.Identifier == domain.CtxSystemAdminId || new.Identifier == domain.CtxUnknownUserId {
		return fmt.Errorf("reserved user identifier: %w", domain.ErrInvalidData)
	}

	if len(new.Authentications) != 1 {
		return fmt.Errorf("invalid number of authentications: %d, expected 1: %w",
			len(new.Authentications), domain.ErrInvalidData)
	}

	// Admins are allowed to create users for arbitrary sources.
	if new.Authentications[0].Source != domain.UserSourceDatabase && !currentUser.IsAdmin {
		return fmt.Errorf("invalid user source: %s, only %s is allowed: %w",
			new.Authentications[0].Source, domain.UserSourceDatabase, domain.ErrInvalidData)
	}

	// database users must have a password
	if new.Authentications[0].Source == domain.UserSourceDatabase && string(new.Password) == "" {
		return fmt.Errorf("missing password: %w", domain.ErrInvalidData)
	}

	if err := new.HasWeakPassword(m.cfg.Auth.MinPasswordLength); err != nil {
		return errors.Join(fmt.Errorf("password too weak: %w", err), domain.ErrInvalidData)
	}

	return nil
}

func (m Manager) validateDeletion(ctx context.Context, del *domain.User) error {
	currentUser := domain.GetUserInfo(ctx)

	if !currentUser.IsAdmin {
		return domain.ErrNoPermission
	}

	if err := del.DeleteAllowed(); err != nil {
		return errors.Join(fmt.Errorf("no access: %w", err), domain.ErrInvalidData)
	}

	if currentUser.Id == del.Identifier {
		return fmt.Errorf("cannot delete own user: %w", domain.ErrInvalidData)
	}

	return nil
}

func (m Manager) validateApiChange(ctx context.Context, user *domain.User) error {
	currentUser := domain.GetUserInfo(ctx)

	if currentUser.Id != user.Identifier {
		return fmt.Errorf("cannot change API access of user: %w", domain.ErrNoPermission)
	}

	return nil
}

// region internal-modifiers

func (m Manager) enrichUser(ctx context.Context, user *domain.User) *domain.User {
	if user == nil {
		return nil
	}
	peers, _ := m.peers.GetUserPeers(ctx, user.Identifier) // ignore error, list will be empty in error case
	user.LinkedPeerCount = len(peers)
	return user
}

func (m Manager) getUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error) {
	user, err := m.users.GetUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("unable to load user %s: %w", id, err)
	}
	return m.enrichUser(ctx, user), nil
}

func (m Manager) update(ctx context.Context, existingUser, user *domain.User, keepAuthentications bool) (
	*domain.User,
	error,
) {
	if err := m.validateModifications(ctx, existingUser, user); err != nil {
		return nil, fmt.Errorf("update not allowed: %w", err)
	}

	err := user.HashPassword()
	if err != nil {
		return nil, err
	}
	if user.Password == "" { // keep old password
		user.Password = existingUser.Password
	}

	err = m.users.SaveUser(ctx, existingUser.Identifier, func(u *domain.User) (*domain.User, error) {
		user.CopyCalculatedAttributes(u, keepAuthentications)
		return user, nil
	})
	if err != nil {
		return nil, fmt.Errorf("update failure: %w", err)
	}

	m.bus.Publish(app.TopicUserUpdated, *user)

	switch {
	case !existingUser.IsDisabled() && user.IsDisabled():
		m.bus.Publish(app.TopicUserDisabled, *user)
	case existingUser.IsDisabled() && !user.IsDisabled():
		m.bus.Publish(app.TopicUserEnabled, *user)
	}

	return user, nil
}

func (m Manager) create(ctx context.Context, user *domain.User) (*domain.User, error) {
	if user.Identifier == "" {
		return nil, errors.New("missing user identifier")
	}

	existingUser, err := m.users.GetUser(ctx, user.Identifier)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("unable to load existing user %s: %w", user.Identifier, err)
	}
	if existingUser != nil {
		return nil, errors.Join(fmt.Errorf("user %s already exists", user.Identifier), domain.ErrDuplicateEntry)
	}

	// Add default authentication if missing
	if len(user.Authentications) == 0 {
		ctxUserInfo := domain.GetUserInfo(ctx)
		now := time.Now()
		user.Authentications = []domain.UserAuthentication{
			{
				BaseModel: domain.BaseModel{
					CreatedBy: ctxUserInfo.UserId(),
					UpdatedBy: ctxUserInfo.UserId(),
					CreatedAt: now,
					UpdatedAt: now,
				},
				UserIdentifier: user.Identifier,
				Source:         domain.UserSourceDatabase,
				ProviderName:   "",
			},
		}
	}

	if err := m.validateCreation(ctx, user); err != nil {
		return nil, fmt.Errorf("creation not allowed: %w", err)
	}

	err = user.HashPassword()
	if err != nil {
		return nil, err
	}

	err = m.users.SaveUser(ctx, user.Identifier, func(u *domain.User) (*domain.User, error) {
		return user, nil
	})
	if err != nil {
		return nil, fmt.Errorf("creation failure: %w", err)
	}

	m.bus.Publish(app.TopicUserCreated, *user)

	return user, nil
}

// endregion internal-modifiers
