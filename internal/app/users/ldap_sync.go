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

	// Update interface allowed users based on LDAP filters. The flag says whether
	// the directory answered usefully, so it counts entries we could extract an
	// identifier from: a field map naming an attribute the server does not return
	// yields entries with empty identifiers, which tells us nothing about who is
	// entitled and must not be read as "the tier is empty".
	err = m.updateInterfaceLdapFilters(ctx, conn, provider, hasUsableIdentifiers(rawUsers, &provider.FieldMap))
	if err != nil {
		return err
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
			if errors.Is(err, domain.ErrInvalidData) {
				slog.Warn("skipping LDAP user with invalid data after sanitization",
					"raw-dn", rawUser["dn"], "error", err)
				continue
			}
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
	// Collect the identifiers the directory actually returned. A hard LDAP
	// failure is already handled by the caller, but a search that *succeeds*
	// and yields nothing usable is indistinguishable here from "every user was
	// removed from the directory". In practice that means a wrong base DN, a
	// sync_filter that no longer matches, a replica that is up but not yet
	// populated, a field map pointing at an attribute the server does not
	// return, or a bind account that has lost read access to the user subtree.
	// LDAP offers nothing to tell those apart: a search that matches nobody and
	// one the client is not allowed to answer both come back as success with
	// zero entries, and a directory server may reply that way for a base DN
	// that does not exist rather than disclose it to an unprivileged client.
	//
	// Acting on it disables every LDAP-sourced user at once, and through
	// TopicUserDisabled that removes each of their peers from the WireGuard
	// device rather than only flagging them. The search succeeded, so no error
	// is logged, and every message on this path is Debug: at the default log
	// level the whole thing is silent. It also repeats every sync interval.
	// Refuse instead. The cost is that a directory intentionally emptied of all
	// users disables nobody; leaving a single account in scope restores the
	// normal behaviour for everyone else.
	ldapUserIds := make(map[domain.UserIdentifier]struct{}, len(rawUsers))
	for _, rawUser := range rawUsers {
		if userId := ldapUserIdentifier(rawUser, fields.UserIdentifier); userId != "" {
			ldapUserIds[userId] = struct{}{}
		}
	}
	if len(ldapUserIds) == 0 {
		slog.Error("refusing to disable missing LDAP users: directory returned no usable user identifiers",
			"provider", providerName,
			"raw-entries", len(rawUsers),
			"identifier-field", fields.UserIdentifier,
			"hint", "check base_dn, sync_filter and the bind account's read permissions; "+
				"if the directory is intentionally empty, keep one account matching sync_filter")
		return nil
	}

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

		if _, existsInLDAP := ldapUserIds[user.Identifier]; existsInLDAP {
			continue
		}

		// Warn, not Debug: this removes the user's peers from the device, and at
		// the default log level a Debug line would make a mass disable silent.
		slog.Warn("user is missing in ldap provider, disabling",
			"user", user.Identifier, "provider", providerName)

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

func (m Manager) updateInterfaceLdapFilters(
	ctx context.Context,
	conn *ldap.Conn,
	provider *config.LdapProvider,
	directoryReturnedUsers bool,
) error {
	if len(provider.InterfaceFilter) == 0 {
		return nil // nothing to do if no interfaces are configured for this provider
	}

	for ifaceName, groupFilter := range provider.InterfaceFilter {
		ifaceId := domain.InterfaceIdentifier(ifaceName)

		// Combined filter: user must match the provider's base SyncFilter AND the interface's LdapGroupFilter
		combinedFilter := fmt.Sprintf("(&(%s)(%s))", provider.SyncFilter, groupFilter)

		rawUsers, err := internal.LdapFindAllUsers(conn, provider.BaseDN, combinedFilter, &provider.FieldMap)
		if err != nil {
			slog.Error("failed to find users for interface filter",
				"interface", ifaceId,
				"provider", provider.ProviderName,
				"error", err)
			continue
		}

		matchedUserIds := make([]domain.UserIdentifier, 0, len(rawUsers))
		for _, rawUser := range rawUsers {
			userId := ldapUserIdentifier(rawUser, provider.FieldMap.UserIdentifier)
			if userId != "" {
				matchedUserIds = append(matchedUserIds, userId)
			}
		}

		m.applyInterfaceLdapFilter(ctx, ifaceId, provider, matchedUserIds, directoryReturnedUsers)
	}

	return nil
}

// applyInterfaceLdapFilter stores the users an LDAP filter matched onto an
// existing interface.
//
// It deliberately does not create the interface. SaveInterface would, which is
// wrong twice over: an interface_filter entry is a statement about who may use
// an interface, not a reason to bring one into existence, and the row it
// creates has no backend. It also races the startup importer, which snapshots
// the interface list before its device round-trips and then fails with
// "interface already exists" once it finds the row.
func (m Manager) applyInterfaceLdapFilter(
	ctx context.Context,
	ifaceId domain.InterfaceIdentifier,
	provider *config.LdapProvider,
	matchedUserIds []domain.UserIdentifier,
	directoryReturnedUsers bool,
) {
	providerName := provider.ProviderName
	if _, err := m.interfaces.GetInterface(ctx, ifaceId); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			slog.Warn("skipping interface filter for unknown interface",
				"interface", ifaceId, "provider", providerName)
		} else {
			slog.Error("failed to look up interface for ldap filter",
				"interface", ifaceId, "provider", providerName, "error", err)
		}
		return
	}

	err := m.interfaces.SaveInterface(ctx, ifaceId, func(i *domain.Interface) (*domain.Interface, error) {
		if i.LdapAllowedUsers == nil {
			i.LdapAllowedUsers = make(map[string][]domain.UserIdentifier)
		}
		i.LdapAllowedUsers[providerName] = matchedUserIds
		return i, nil
	})
	if err != nil {
		slog.Error("failed to save interface ldap allowed users",
			"interface", ifaceId, "provider", providerName, "error", err)
		return
	}

	slog.Debug("updated interface ldap allowed users",
		"interface", ifaceId, "provider", providerName, "matched_count", len(matchedUserIds))

	if !provider.RevokeOnFilterChange {
		return
	}

	// An interface filter matching nobody is ambiguous: the group may genuinely
	// have emptied, or the filter may be broken, the group renamed, or a replica
	// unpopulated. The provider-level sync result disambiguates it. If the
	// directory returned users at all it is answering correctly, so an empty
	// filter means the tier really is empty. If it returned nothing the whole
	// sync is suspect and nothing should be revoked on the strength of it.
	if len(matchedUserIds) == 0 && !directoryReturnedUsers {
		slog.Error("refusing to reconcile interface access: the directory returned no users at all",
			"interface", ifaceId, "provider", providerName)
		return
	}

	// Publish the resulting entitlement rather than a diff against the previous
	// one. A diff only fires on the sync that happens to observe the change, so
	// a demotion while wg-portal is stopped would never be acted on. Handing
	// over the whole allowed set lets the consumer reconcile from current state,
	// which is idempotent and repairs existing drift.
	m.bus.Publish(app.TopicInterfaceLdapFilterApplied, ifaceId, matchedUserIds)
}

// hasUsableIdentifiers reports whether at least one entry carried an identifier.
// Entries alone are not enough: they can come back with the identifier attribute
// missing, in which case the sync knows nothing about who is entitled.
func hasUsableIdentifiers(rawUsers []internal.RawLdapUser, fields *config.LdapFields) bool {
	for _, rawUser := range rawUsers {
		if ldapUserIdentifier(rawUser, fields.UserIdentifier) != "" {
			return true
		}
	}
	return false
}

func ldapUserIdentifier(rawUser map[string]any, field string) domain.UserIdentifier {
	identifier := internal.MapDefaultString(rawUser, field, "")
	identifier = domain.SanitizeIdentifier(identifier, 256)
	if identifier == "" {
		return ""
	}
	return domain.UserIdentifier(identifier)
}
