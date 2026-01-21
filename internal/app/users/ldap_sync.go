package users

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-ldap/ldap/v3"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

func (m Manager) runLdapSynchronizationService(ctx context.Context) {
	ctx = domain.SetUserInfo(ctx, domain.LdapSyncContextUserInfo()) // switch to service context for LDAP sync

	for _, ldapCfg := range m.cfg.Auth.Ldap { // LDAP Auth providers
		go func(cfg config.LdapProvider) {
			syncInterval := cfg.SyncInterval
			if syncInterval == 0 {
				slog.Debug("sync disabled for LDAP server", "provider", cfg.ProviderName)
				return
			}

			// perform initial sync
			err := m.synchronizeLdapUsers(ctx, &cfg)
			if err != nil {
				slog.Error("failed to synchronize LDAP users", "provider", cfg.ProviderName, "error", err)
			} else {
				slog.Debug("initial LDAP user sync completed", "provider", cfg.ProviderName)
			}

			// start periodic sync
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
					slog.Error("failed to synchronize LDAP users", "provider", cfg.ProviderName, "error", err)
				}
			}
		}(ldapCfg)
	}
}

func (m Manager) synchronizeLdapUsers(ctx context.Context, provider *config.LdapProvider) error {
	slog.Debug("starting to synchronize users", "provider", provider.ProviderName)

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

	slog.Debug("fetched raw ldap users", "count", len(rawUsers), "provider", provider.ProviderName)

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

		if provider.SyncLogUserInfo {
			slog.Debug("ldap user data",
				"raw-user", rawUser, "user", user.Identifier,
				"is-admin", user.IsAdmin, "provider", provider.ProviderName)
		}

		existingUser, err := m.users.GetUser(ctx, user.Identifier)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("find error for user id %s: %w", user.Identifier, err)
		}

		tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		tctx = domain.SetUserInfo(tctx, domain.SystemAdminContextUserInfo())

		if existingUser == nil {
			// create new user
			slog.Debug("creating new user from provider", "user", user.Identifier, "provider", provider.ProviderName)

			_, err := m.create(tctx, user)
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

			if existingUser.PersistLocalChanges {
				cancel()
				continue // skip synchronization for this user
			}

			if userChangedInLdap(existingUser, user) {
				syncedUser, err := m.users.GetUser(ctx, user.Identifier)
				if err != nil && !errors.Is(err, domain.ErrNotFound) {
					cancel()
					return fmt.Errorf("find error for user id %s: %w", user.Identifier, err)
				}
				syncedUser.UpdatedAt = time.Now()
				syncedUser.UpdatedBy = domain.CtxSystemLdapSyncer
				syncedUser.MergeAuthSources(user.Authentications...)
				syncedUser.Email = user.Email
				syncedUser.Firstname = user.Firstname
				syncedUser.Lastname = user.Lastname
				syncedUser.Phone = user.Phone
				syncedUser.Department = user.Department
				syncedUser.IsAdmin = user.IsAdmin
				syncedUser.Disabled = user.Disabled
				syncedUser.DisabledReason = user.DisabledReason

				_, err = m.update(tctx, existingUser, syncedUser, false)
				if err != nil {
					cancel()
					return fmt.Errorf("update error for user id %s: %w", user.Identifier, err)
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
		userHasAuthSource := false
		for _, auth := range user.Authentications {
			if auth.Source == domain.UserSourceLdap && auth.ProviderName == providerName {
				userHasAuthSource = true
				break
			}
		}
		if !userHasAuthSource {
			continue // ignore non ldap users
		}
		if user.IsDisabled() {
			continue // ignore deactivated
		}
		if user.PersistLocalChanges {
			continue // skip sync for this user
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

		slog.Debug("user is missing in ldap provider, disabling", "user", user.Identifier, "provider", providerName)

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
