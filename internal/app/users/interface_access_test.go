package users

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

type grantKey struct {
	iface    domain.InterfaceIdentifier
	provider string
	user     domain.UserIdentifier
}

// fakeGrantRepo stores interface access grants in memory.
type fakeGrantRepo struct {
	fakeInterfaceRepo
	mu        sync.Mutex
	grants    map[grantKey]time.Time
	deleteErr error // returned when purging a user
	writeErr  error // returned when saving or deleting a single grant
}

func (f *fakeGrantRepo) SaveInterfaceAccessGrant(_ context.Context, grant domain.InterfaceAccessGrant) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writeErr != nil {
		return f.writeErr
	}
	if f.grants == nil {
		f.grants = make(map[grantKey]time.Time)
	}
	f.grants[grantKey{grant.InterfaceIdentifier, grant.ProviderName, grant.UserIdentifier}] = grant.EvaluatedAt
	return nil
}

func (f *fakeGrantRepo) DeleteInterfaceAccessGrant(
	_ context.Context,
	ifaceId domain.InterfaceIdentifier,
	providerName string,
	userId domain.UserIdentifier,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writeErr != nil {
		return f.writeErr
	}
	delete(f.grants, grantKey{ifaceId, providerName, userId})
	return nil
}

func (f *fakeGrantRepo) DeleteUserInterfaceAccessGrants(_ context.Context, userId domain.UserIdentifier) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleteErr != nil {
		return f.deleteErr
	}
	for key := range f.grants {
		if key.user == userId {
			delete(f.grants, key)
		}
	}
	return nil
}

func (f *fakeGrantRepo) grant(iface domain.InterfaceIdentifier, user domain.UserIdentifier) (time.Time, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	at, ok := f.grants[grantKey{iface, "oidc1", user}]
	return at, ok
}

func (f *fakeGrantRepo) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.grants)
}

func newAccessTestManager(repo InterfaceDatabaseRepo) Manager {
	return Manager{cfg: &config.Config{}, bus: &fakeBus{}, interfaces: repo, userLocks: &keyedLocks{}}
}

func adminCtx() context.Context {
	return domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
}

func TestUpdateOauthInterfaceAccess_GrantAndRevoke(t *testing.T) {
	repo := &fakeGrantRepo{}
	m := newAccessTestManager(repo)

	before := time.Now()
	require.NoError(t, m.UpdateOauthInterfaceAccess(adminCtx(), "oidc1", "alice",
		map[domain.InterfaceIdentifier]bool{"wg-data": true, "wg-ops": false}))
	at, ok := repo.grant("wg-data", "alice")
	require.True(t, ok)
	assert.False(t, at.Before(before), "the grant must carry the evaluation time")
	_, ok = repo.grant("wg-ops", "alice")
	assert.False(t, ok)

	// The user lost the group: the next login revokes access.
	require.NoError(t, m.UpdateOauthInterfaceAccess(adminCtx(), "oidc1", "alice",
		map[domain.InterfaceIdentifier]bool{"wg-data": false}))
	_, ok = repo.grant("wg-data", "alice")
	assert.False(t, ok)
}

func TestUpdateOauthInterfaceAccess_ReloginRefreshesGrant(t *testing.T) {
	expired := time.Now().Add(-48 * time.Hour)
	repo := &fakeGrantRepo{grants: map[grantKey]time.Time{{"wg-data", "oidc1", "alice"}: expired}}
	m := newAccessTestManager(repo)

	require.NoError(t, m.UpdateOauthInterfaceAccess(adminCtx(), "oidc1", "alice",
		map[domain.InterfaceIdentifier]bool{"wg-data": true}))

	at, _ := repo.grant("wg-data", "alice")
	assert.True(t, at.After(expired))
}

func TestUpdateOauthInterfaceAccess_RequiresAdminContext(t *testing.T) {
	repo := &fakeGrantRepo{}
	m := newAccessTestManager(repo)
	ctx := domain.SetUserInfo(context.Background(), &domain.ContextUserInfo{Id: "alice"})

	err := m.UpdateOauthInterfaceAccess(ctx, "oidc1", "alice", map[domain.InterfaceIdentifier]bool{"wg-data": true})
	require.ErrorIs(t, err, domain.ErrNoPermission)
	assert.Zero(t, repo.count())
}

func TestUpdateOauthInterfaceAccess_ConcurrentLogins(t *testing.T) {
	repo := &fakeGrantRepo{}
	m := newAccessTestManager(repo)

	const logins = 25
	var wg sync.WaitGroup
	for i := range logins {
		wg.Go(func() {
			userId := domain.UserIdentifier(fmt.Sprintf("user%d", i))
			assert.NoError(t, m.UpdateOauthInterfaceAccess(adminCtx(), "oidc1", userId,
				map[domain.InterfaceIdentifier]bool{"wg-data": true}))
		})
	}
	wg.Wait()

	assert.Equal(t, logins, repo.count())
}

// existingUserRepo returns a single existing user, so DeleteUser can run.
type existingUserRepo struct {
	fakeUserRepo
	user         domain.User
	caseIgnoring bool
	deleted      bool
	onDelete     func()
}

func (r *existingUserRepo) GetUser(_ context.Context, id domain.UserIdentifier) (*domain.User, error) {
	if id != r.user.Identifier && !r.caseIgnoring {
		return nil, domain.ErrNotFound
	}
	user := r.user
	return &user, nil
}

func (r *existingUserRepo) DeleteUser(_ context.Context, _ domain.UserIdentifier) error {
	if r.onDelete != nil {
		r.onDelete()
	}
	r.deleted = true
	return nil
}

func TestDeleteUser_RevokesBeforeDeleting(t *testing.T) {
	repo := &fakeGrantRepo{grants: map[grantKey]time.Time{
		{"wg-data", "oidc1", "alice"}: time.Now(),
		{"wg-data", "oidc1", "bob"}:   time.Now(),
	}}
	users := &existingUserRepo{user: domain.User{Identifier: "alice"}}
	users.onDelete = func() {
		_, ok := repo.grant("wg-data", "alice")
		assert.False(t, ok, "the grant must be gone before the user is deleted")
	}
	m := newAccessTestManager(repo)
	m.users = users

	require.NoError(t, m.DeleteUser(adminCtx(), "alice"))

	assert.True(t, users.deleted)
	_, ok := repo.grant("wg-data", "bob")
	assert.True(t, ok, "other users' grants must be left alone")
}

// With a case-insensitive user lookup, the grants are stored under the identifier from the database.
func TestDeleteUser_RevokesByStoredIdentifier(t *testing.T) {
	repo := &fakeGrantRepo{grants: map[grantKey]time.Time{{"wg-data", "oidc1", "alice"}: time.Now()}}
	users := &existingUserRepo{user: domain.User{Identifier: "alice"}, caseIgnoring: true}
	m := newAccessTestManager(repo)
	m.users = users

	require.NoError(t, m.DeleteUser(adminCtx(), "Alice"))

	assert.True(t, users.deleted)
	assert.Zero(t, repo.count())
}

func TestDeleteUser_AbortsWhenRevokeFails(t *testing.T) {
	users := &existingUserRepo{user: domain.User{Identifier: "alice"}}
	m := newAccessTestManager(&fakeGrantRepo{deleteErr: assert.AnError})
	m.users = users

	err := m.DeleteUser(adminCtx(), "alice")

	assert.ErrorIs(t, err, assert.AnError)
	assert.False(t, users.deleted, "the user must not be deleted while a grant may remain")
}

// A login racing a deletion must not leave a grant for the deleted user behind.
func TestDeleteUser_SerializedWithLoginGrant(t *testing.T) {
	repo := &fakeGrantRepo{}
	m := newAccessTestManager(repo)
	users := &existingUserRepo{user: domain.User{Identifier: "alice"}}
	m.users = users

	grantDone := make(chan struct{})
	users.onDelete = func() {
		go func() {
			assert.NoError(t, m.UpdateOauthInterfaceAccess(adminCtx(), "oidc1", "alice",
				map[domain.InterfaceIdentifier]bool{"wg-data": true}))
			close(grantDone)
		}()
		select {
		case <-grantDone:
			t.Error("the login grant must not run while the user is being deleted")
		case <-time.After(50 * time.Millisecond):
		}
	}

	require.NoError(t, m.DeleteUser(adminCtx(), "alice"))
	<-grantDone
}

func TestUpdateOauthInterfaceAccess_PropagatesWriteErrors(t *testing.T) {
	m := newAccessTestManager(&fakeGrantRepo{writeErr: assert.AnError})

	for _, allowed := range []bool{true, false} {
		err := m.UpdateOauthInterfaceAccess(adminCtx(), "oidc1", "alice",
			map[domain.InterfaceIdentifier]bool{"wg-data": allowed})
		assert.ErrorIs(t, err, assert.AnError, "allowed=%v", allowed)
	}
}
