package users

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/h44z/wg-portal/internal/app"

	"github.com/h44z/wg-portal/internal"

	"github.com/go-ldap/ldap/v3"

	"github.com/sirupsen/logrus"

	evbus "github.com/vardius/message-bus"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type Manager struct {
	cfg *config.Config
	bus evbus.MessageBus

	syncInterval time.Duration
	users        UserDatabaseRepo
	peers        PeerDatabaseRepo
}

func NewUserManager(cfg *config.Config, bus evbus.MessageBus, users UserDatabaseRepo, peers PeerDatabaseRepo) (*Manager, error) {
	m := &Manager{
		cfg: cfg,
		bus: bus,

		syncInterval: 10 * time.Second,
		users:        users,
		peers:        peers,
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
		u.IsAdmin = user.IsAdmin
		u.Firstname = user.Firstname
		u.Lastname = user.Lastname
		u.Phone = user.Phone
		u.Department = user.Department
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
		return nil, fmt.Errorf("unable to load peer %s: %w", id, err)
	}
	peers, _ := m.peers.GetUserPeers(ctx, id) // ignore error, list will be empty in error case

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
		return nil, fmt.Errorf("user %s already exists", user.Identifier)
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

	m.bus.Publish(app.TopicUserDeleted, existingUser)

	return nil
}

func (m Manager) validateModifications(ctx context.Context, old, new *domain.User) error {
	currentUser := domain.GetUserInfo(ctx)

	if currentUser.Id != new.Identifier && !currentUser.IsAdmin {
		return fmt.Errorf("insufficient permissions")
	}

	if err := old.EditAllowed(new); err != nil {
		return fmt.Errorf("no access: %w", err)
	}

	if err := old.CanChangePassword(); err != nil && string(new.Password) != "" {
		return fmt.Errorf("no access: %w", err)
	}

	if currentUser.Id == old.Identifier && old.IsAdmin && !new.IsAdmin {
		return fmt.Errorf("cannot remove own admin rights")
	}

	if currentUser.Id == old.Identifier && new.IsDisabled() {
		return fmt.Errorf("cannot disable own user")
	}

	if currentUser.Id == old.Identifier && new.IsLocked() {
		return fmt.Errorf("cannot lock own user")
	}

	if old.Source != new.Source {
		return fmt.Errorf("cannot change user source")
	}

	return nil
}

func (m Manager) validateCreation(ctx context.Context, new *domain.User) error {
	currentUser := domain.GetUserInfo(ctx)

	if !currentUser.IsAdmin {
		return fmt.Errorf("insufficient permissions")
	}

	if new.Identifier == "" {
		return fmt.Errorf("invalid user identifier")
	}

	if new.Identifier == "all" { // the all user identifier collides with the rest api routes
		return fmt.Errorf("reserved user identifier")
	}

	if new.Source != domain.UserSourceDatabase {
		return fmt.Errorf("invalid user source: %s", new.Source)
	}

	if string(new.Password) == "" {
		return fmt.Errorf("invalid password")
	}

	return nil
}

func (m Manager) validateDeletion(ctx context.Context, del *domain.User) error {
	currentUser := domain.GetUserInfo(ctx)

	if !currentUser.IsAdmin {
		return fmt.Errorf("insufficient permissions")
	}

	if err := del.DeleteAllowed(); err != nil {
		return fmt.Errorf("no access: %w", err)
	}

	if currentUser.Id == del.Identifier {
		return fmt.Errorf("cannot delete own user")
	}

	return nil
}

func (m Manager) runLdapSynchronizationService(ctx context.Context) {
	running := true
	for running {
		select {
		case <-ctx.Done():
			running = false
			continue
		case <-time.After(m.syncInterval):
			// select blocks until one of the cases evaluate to true
		}

		for _, ldapCfg := range m.cfg.Auth.Ldap { // LDAP Auth providers
			if !ldapCfg.Synchronize {
				continue // sync disabled
			}
			//logrus.Tracef(&ldapCfg)
			err := m.synchronizeLdapUsers(ctx, &ldapCfg)
			if err != nil {
				logrus.Errorf("failed to synchronize LDAP users for %s: %v", ldapCfg.ProviderName, err)
			}
		}
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

	logrus.Tracef("fetched %d raw ldap users...", len(rawUsers))

	// Update existing LDAP users
	err = m.updateLdapUsers(ctx, provider.ProviderName, rawUsers, &provider.FieldMap, provider.ParsedAdminGroupDN)
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

func (m Manager) updateLdapUsers(ctx context.Context, providerName string, rawUsers []internal.RawLdapUser, fields *config.LdapFields, adminGroupDN *ldap.DN) error {
	for _, rawUser := range rawUsers {
		user, err := convertRawLdapUser(providerName, rawUser, fields, adminGroupDN)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("failed to convert LDAP data for %v: %w", rawUser["dn"], err)
		}

		existingUser, err := m.users.GetUser(ctx, user.Identifier)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("find error for user id %s: %w", user.Identifier, err)
		}

		tctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		tctx = domain.SetUserInfo(tctx, domain.SystemAdminContextUserInfo())
		
		if existingUser == nil {
			err := m.NewUser(tctx, user)
			if err != nil {
				return fmt.Errorf("create error for user id %s: %w", user.Identifier, err)
			}
		}

		if existingUser != nil && existingUser.Source == domain.UserSourceLdap && userChangedInLdap(existingUser, user) {
		
			err := m.users.SaveUser(tctx, user.Identifier, func(u *domain.User) (*domain.User, error) {
				u.UpdatedAt = time.Now()
				u.UpdatedBy = "ldap_sync"
				u.Email = user.Email
				u.Firstname = user.Firstname
				u.Lastname = user.Lastname
				u.Phone = user.Phone
				u.Department = user.Department
				u.IsAdmin = user.IsAdmin
				u.Disabled = user.Disabled

				return u, nil
			})
			if err != nil {
				return fmt.Errorf("update error for user id %s: %w", user.Identifier, err)
			}
		}
	}

	return nil
}

func (m Manager) disableMissingLdapUsers(ctx context.Context, providerName string, rawUsers []internal.RawLdapUser, fields *config.LdapFields) error {
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

		err := m.users.SaveUser(ctx, user.Identifier, func(u *domain.User) (*domain.User, error) {
			now := time.Now()
			u.Disabled = &now
			u.DisabledReason = "missing in ldap"
			return u, nil
		})
		if err != nil {
			return fmt.Errorf("disable error for user id %s: %w", user.Identifier, err)
		}

		m.bus.Publish(app.TopicUserDisabled, user)
	}

	return nil
}
