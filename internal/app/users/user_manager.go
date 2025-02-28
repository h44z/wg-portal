package users

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	evbus "github.com/vardius/message-bus"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type Manager struct {
	cfg *config.Config
	bus evbus.MessageBus

	users UserDatabaseRepo
	peers PeerDatabaseRepo
}

func NewUserManager(cfg *config.Config, bus evbus.MessageBus, users UserDatabaseRepo, peers PeerDatabaseRepo) (
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

func (m Manager) RegisterUser(ctx context.Context, user *domain.User) error {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return err
	}

	err := m.NewUser(ctx, user)
	if err != nil {
		return err
	}

	m.bus.Publish(app.TopicUserRegistered, user)

	return nil
}

func (m Manager) NewUser(ctx context.Context, user *domain.User) error {
	if user.Identifier == "" {
		return errors.New("missing user identifier")
	}

	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return err
	}

	err := m.users.SaveUser(ctx, user.Identifier, func(u *domain.User) (*domain.User, error) {
		u.Identifier = user.Identifier
		u.Email = user.Email
		u.Source = user.Source
		u.ProviderName = user.ProviderName
		u.IsAdmin = user.IsAdmin
		u.Firstname = user.Firstname
		u.Lastname = user.Lastname
		u.Phone = user.Phone
		u.Department = user.Department
		u.Notes = user.Notes
		u.ApiToken = user.ApiToken
		u.ApiTokenCreated = user.ApiTokenCreated

		return u, nil
	})
	if err != nil {
		return fmt.Errorf("failed to save user: %w", err)
	}

	m.bus.Publish(app.TopicUserCreated, user)

	return nil
}

func (m Manager) StartBackgroundJobs(ctx context.Context) {

	go m.runLdapSynchronizationService(ctx)

}

func (m Manager) GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error) {
	if err := domain.ValidateUserAccessRights(ctx, id); err != nil {
		return nil, err
	}

	user, err := m.users.GetUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("unable to load user %s: %w", id, err)
	}
	peers, _ := m.peers.GetUserPeers(ctx, id) // ignore error, list will be empty in error case

	user.LinkedPeerCount = len(peers)

	return user, nil
}

func (m Manager) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {

	user, err := m.users.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("unable to load user for email %s: %w", email, err)
	}

	if err := domain.ValidateUserAccessRights(ctx, user.Identifier); err != nil {
		return nil, err
	}

	peers, _ := m.peers.GetUserPeers(ctx, user.Identifier) // ignore error, list will be empty in error case

	user.LinkedPeerCount = len(peers)

	return user, nil
}

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
				peers, _ := m.peers.GetUserPeers(ctx, user.Identifier) // ignore error, list will be empty in error case
				user.LinkedPeerCount = len(peers)
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

func (m Manager) UpdateUser(ctx context.Context, user *domain.User) (*domain.User, error) {
	if err := domain.ValidateUserAccessRights(ctx, user.Identifier); err != nil {
		return nil, err
	}

	existingUser, err := m.users.GetUser(ctx, user.Identifier)
	if err != nil {
		return nil, fmt.Errorf("unable to load existing user %s: %w", user.Identifier, err)
	}

	if err := m.validateModifications(ctx, existingUser, user); err != nil {
		return nil, fmt.Errorf("update not allowed: %w", err)
	}

	user.CopyCalculatedAttributes(existingUser)
	err = user.HashPassword()
	if err != nil {
		return nil, err
	}
	if user.Password == "" { // keep old password
		user.Password = existingUser.Password
	}

	err = m.users.SaveUser(ctx, existingUser.Identifier, func(u *domain.User) (*domain.User, error) {
		user.CopyCalculatedAttributes(u)
		return user, nil
	})
	if err != nil {
		return nil, fmt.Errorf("update failure: %w", err)
	}

	switch {
	case !existingUser.IsDisabled() && user.IsDisabled():
		m.bus.Publish(app.TopicUserDisabled, *user)
	case existingUser.IsDisabled() && !user.IsDisabled():
		m.bus.Publish(app.TopicUserEnabled, *user)
	}

	return user, nil
}

func (m Manager) CreateUser(ctx context.Context, user *domain.User) (*domain.User, error) {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return nil, err
	}

	existingUser, err := m.users.GetUser(ctx, user.Identifier)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("unable to load existing user %s: %w", user.Identifier, err)
	}
	if existingUser != nil {
		return nil, errors.Join(fmt.Errorf("user %s already exists", user.Identifier), domain.ErrDuplicateEntry)
	}

	if err := m.validateCreation(ctx, user); err != nil {
		return nil, fmt.Errorf("creation not allowed: %w", err)
	}

	err = user.HashPassword()
	if err != nil {
		return nil, err
	}

	err = m.users.SaveUser(ctx, user.Identifier, func(u *domain.User) (*domain.User, error) {
		user.CopyCalculatedAttributes(u)
		return user, nil
	})
	if err != nil {
		return nil, fmt.Errorf("creation failure: %w", err)
	}

	return user, nil
}

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

	err = m.users.SaveUser(ctx, user.Identifier, func(u *domain.User) (*domain.User, error) {
		user.CopyCalculatedAttributes(u)
		return user, nil
	})
	if err != nil {
		return nil, fmt.Errorf("update failure: %w", err)
	}

	m.bus.Publish(app.TopicUserApiEnabled, user)

	return user, nil
}

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

	err = m.users.SaveUser(ctx, user.Identifier, func(u *domain.User) (*domain.User, error) {
		user.CopyCalculatedAttributes(u)
		return user, nil
	})
	if err != nil {
		return nil, fmt.Errorf("update failure: %w", err)
	}

	m.bus.Publish(app.TopicUserApiDisabled, user)

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

	if currentUser.Id == old.Identifier && old.IsAdmin && !new.IsAdmin {
		return fmt.Errorf("cannot remove own admin rights: %w", domain.ErrInvalidData)
	}

	if currentUser.Id == old.Identifier && new.IsDisabled() {
		return fmt.Errorf("cannot disable own user: %w", domain.ErrInvalidData)
	}

	if currentUser.Id == old.Identifier && new.IsLocked() {
		return fmt.Errorf("cannot lock own user: %w", domain.ErrInvalidData)
	}

	if old.Source != new.Source {
		return fmt.Errorf("cannot change user source: %w", domain.ErrInvalidData)
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

	if new.Source != domain.UserSourceDatabase {
		return fmt.Errorf("invalid user source: %s, only %s is allowed: %w",
			new.Source, domain.UserSourceDatabase, domain.ErrInvalidData)
	}

	if string(new.Password) == "" {
		return fmt.Errorf("invalid password: %w", domain.ErrInvalidData)
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

func (m Manager) runLdapSynchronizationService(ctx context.Context) {
	for _, ldapCfg := range m.cfg.Auth.Ldap { // LDAP Auth providers
		go func(cfg config.LdapProvider) {
			syncInterval := cfg.SyncInterval
			if syncInterval == 0 {
				logrus.Debugf("sync disabled for LDAP server: %s", cfg.ProviderName)
				return
			}

			running := true
			for running {
				select {
				case <-ctx.Done():
					running = false
					continue
				case <-time.After(syncInterval):
					// select blocks until one of the cases evaluate to true
				}

				err := m.synchronizeLdapUsers(ctx, &cfg)
				if err != nil {
					logrus.Errorf("failed to synchronize LDAP users for %s: %v", cfg.ProviderName, err)
				}
			}
		}(ldapCfg)
	}
}

func (m Manager) synchronizeLdapUsers(ctx context.Context, provider *config.LdapProvider) error {
	logrus.Tracef("starting to synchronize users for %s", provider.ProviderName)

	dn, err := ldap.ParseDN(provider.AdminGroupDN)
	if err != nil {
		return fmt.Errorf("failed to parse admin group DN: %w", err)
	}
	provider.ParsedAdminGroupDN = dn

	conn, err := internal.LdapConnect(provider)
	if err != nil {
		return fmt.Errorf("failed to setup LDAP connection: %w", err)
	}
	defer internal.LdapDisconnect(conn)

	rawUsers, err := internal.LdapFindAllUsers(conn, provider.BaseDN, provider.SyncFilter, &provider.FieldMap)
	if err != nil {
		return err
	}

	logrus.Tracef("fetched %d raw ldap users from provider %s...", len(rawUsers), provider.ProviderName)

	// Update existing LDAP users
	err = m.updateLdapUsers(ctx, provider, rawUsers, &provider.FieldMap, provider.ParsedAdminGroupDN)
	if err != nil {
		return err
	}

	// Disable missing LDAP users
	if provider.DisableMissing {
		err = m.disableMissingLdapUsers(ctx, provider.ProviderName, rawUsers, &provider.FieldMap)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m Manager) updateLdapUsers(
	ctx context.Context,
	provider *config.LdapProvider,
	rawUsers []internal.RawLdapUser,
	fields *config.LdapFields,
	adminGroupDN *ldap.DN,
) error {
	for _, rawUser := range rawUsers {
		user, err := convertRawLdapUser(provider.ProviderName, rawUser, fields, adminGroupDN)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("failed to convert LDAP data for %v: %w", rawUser["dn"], err)
		}

		existingUser, err := m.users.GetUser(ctx, user.Identifier)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("find error for user id %s: %w", user.Identifier, err)
		}

		tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		tctx = domain.SetUserInfo(tctx, domain.SystemAdminContextUserInfo())

		if existingUser == nil {
			// create new user
			logrus.Tracef("creating new user %s from provider %s...", user.Identifier, provider.ProviderName)

			err := m.NewUser(tctx, user)
			if err != nil {
				cancel()
				return fmt.Errorf("create error for user id %s: %w", user.Identifier, err)
			}
		} else {
			// update existing user
			if provider.AutoReEnable && existingUser.DisabledReason == domain.DisabledReasonLdapMissing {
				user.Disabled = nil
				user.DisabledReason = ""
			} else {
				user.Disabled = existingUser.Disabled
				user.DisabledReason = existingUser.DisabledReason
			}
			if existingUser.Source == domain.UserSourceLdap && userChangedInLdap(existingUser, user) {
				err := m.users.SaveUser(tctx, user.Identifier, func(u *domain.User) (*domain.User, error) {
					u.UpdatedAt = time.Now()
					u.UpdatedBy = domain.CtxSystemLdapSyncer
					u.Source = user.Source
					u.ProviderName = user.ProviderName
					u.Email = user.Email
					u.Firstname = user.Firstname
					u.Lastname = user.Lastname
					u.Phone = user.Phone
					u.Department = user.Department
					u.IsAdmin = user.IsAdmin
					u.Disabled = nil
					u.DisabledReason = ""

					return u, nil
				})
				if err != nil {
					cancel()
					return fmt.Errorf("update error for user id %s: %w", user.Identifier, err)
				}

				if existingUser.IsDisabled() && !user.IsDisabled() {
					m.bus.Publish(app.TopicUserEnabled, *user)
				}
			}
		}

		cancel()
	}

	return nil
}

func (m Manager) disableMissingLdapUsers(
	ctx context.Context,
	providerName string,
	rawUsers []internal.RawLdapUser,
	fields *config.LdapFields,
) error {
	allUsers, err := m.users.GetAllUsers(ctx)
	if err != nil {
		return err
	}
	for _, user := range allUsers {
		if user.Source != domain.UserSourceLdap {
			continue // ignore non ldap users
		}
		if user.ProviderName != providerName {
			continue // user was synchronized through different provider
		}
		if user.IsDisabled() {
			continue // ignore deactivated
		}

		existsInLDAP := false
		for _, rawUser := range rawUsers {
			userId := domain.UserIdentifier(internal.MapDefaultString(rawUser, fields.UserIdentifier, ""))
			if user.Identifier == userId {
				existsInLDAP = true
				break
			}
		}

		if existsInLDAP {
			continue
		}

		logrus.Tracef("user %s is missing in ldap provider %s, disabling", user.Identifier, providerName)

		now := time.Now()
		user.Disabled = &now
		user.DisabledReason = domain.DisabledReasonLdapMissing

		err := m.users.SaveUser(ctx, user.Identifier, func(u *domain.User) (*domain.User, error) {
			u.Disabled = user.Disabled
			u.DisabledReason = user.DisabledReason
			return u, nil
		})
		if err != nil {
			return fmt.Errorf("disable error for user id %s: %w", user.Identifier, err)
		}

		m.bus.Publish(app.TopicUserDisabled, user)
	}

	return nil
}
