package users

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/h44z/wg-portal/internal/domain"
)

// keyedLocks provides one mutex per key.
type keyedLocks struct {
	locks sync.Map // map[any]*sync.Mutex
}

// lock locks the given key and returns the matching unlock function.
func (l *keyedLocks) lock(key any) func() {
	mutex, _ := l.locks.LoadOrStore(key, &sync.Mutex{})
	mutex.(*sync.Mutex).Lock()
	return mutex.(*sync.Mutex).Unlock
}

// UpdateOauthInterfaceAccess stores the result of evaluating the interface_filter of an OAuth or OIDC provider
// for the user: a grant for each interface whose value in access is true, none for the others.
func (m Manager) UpdateOauthInterfaceAccess(
	ctx context.Context,
	providerName string,
	userId domain.UserIdentifier,
	access map[domain.InterfaceIdentifier]bool,
) error {
	if err := domain.ValidateAdminAccessRights(ctx); err != nil {
		return err
	}

	// Serializes with DeleteUser, so a deleted user cannot regain a grant through a login that already started.
	defer m.userLocks.lock(userId)()

	now := time.Now()
	var errs []error
	for ifaceId, allowed := range access {
		var err error
		if allowed {
			err = m.interfaces.SaveInterfaceAccessGrant(ctx, domain.InterfaceAccessGrant{
				InterfaceIdentifier: ifaceId,
				ProviderName:        providerName,
				UserIdentifier:      userId,
				EvaluatedAt:         now,
			})
		} else {
			err = m.interfaces.DeleteInterfaceAccessGrant(ctx, ifaceId, providerName, userId)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to update access to interface %s: %w", ifaceId, err))
			continue
		}

		slog.Debug("updated interface access grant",
			"interface", ifaceId, "provider", providerName, "user", userId, "allowed", allowed)
	}

	return errors.Join(errs...)
}
