package adapters

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// newInterfaceAccessTestRepo opens a file based SQLite database configured like the production one.
func newInterfaceAccessTestRepo(t *testing.T) *SqlRepo {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "sqlite.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&domain.Interface{}, &domain.Cidr{}, &domain.InterfaceAccessGrant{}))
	return &SqlRepo{db: db, cfg: &config.Config{}}
}

func TestSqlRepo_InterfaceAccessGrants(t *testing.T) {
	repo := newInterfaceAccessTestRepo(t)
	ctx := context.Background()

	first := time.Date(2026, 1, 2, 3, 4, 5, 123456789, time.FixedZone("UTC+3", 3*60*60))
	require.NoError(t, repo.SaveInterfaceAccessGrant(ctx, domain.InterfaceAccessGrant{
		InterfaceIdentifier: "wg-data", ProviderName: "oidc1", UserIdentifier: "alice", EvaluatedAt: first,
	}))
	require.NoError(t, repo.SaveInterfaceAccessGrant(ctx, domain.InterfaceAccessGrant{
		InterfaceIdentifier: "wg-ops", ProviderName: "oidc1", UserIdentifier: "alice", EvaluatedAt: first,
	}))
	require.NoError(t, repo.SaveInterfaceAccessGrant(ctx, domain.InterfaceAccessGrant{
		InterfaceIdentifier: "wg-data", ProviderName: "oidc1", UserIdentifier: "bob", EvaluatedAt: first,
	}))

	grants, err := repo.GetUserInterfaceAccessGrants(ctx, "alice")
	require.NoError(t, err)
	require.Len(t, grants, 2)
	assert.WithinDuration(t, first, grants[0].EvaluatedAt, time.Millisecond,
		"the evaluation time must survive the round trip, independent of its time zone")

	// A new login updates the existing grant instead of adding a second one.
	second := first.Add(time.Hour).UTC()
	require.NoError(t, repo.SaveInterfaceAccessGrant(ctx, domain.InterfaceAccessGrant{
		InterfaceIdentifier: "wg-data", ProviderName: "oidc1", UserIdentifier: "alice", EvaluatedAt: second,
	}))
	grants, err = repo.GetUserInterfaceAccessGrants(ctx, "alice")
	require.NoError(t, err)
	require.Len(t, grants, 2)
	for _, grant := range grants {
		if grant.InterfaceIdentifier == "wg-data" {
			assert.WithinDuration(t, second, grant.EvaluatedAt, time.Millisecond)
		}
	}

	require.NoError(t, repo.DeleteInterfaceAccessGrant(ctx, "wg-data", "oidc1", "alice"))
	grants, err = repo.GetUserInterfaceAccessGrants(ctx, "alice")
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Equal(t, domain.InterfaceIdentifier("wg-ops"), grants[0].InterfaceIdentifier)

	require.NoError(t, repo.DeleteUserInterfaceAccessGrants(ctx, "alice"))
	grants, err = repo.GetUserInterfaceAccessGrants(ctx, "alice")
	require.NoError(t, err)
	assert.Empty(t, grants)

	grants, err = repo.GetUserInterfaceAccessGrants(ctx, "bob")
	require.NoError(t, err)
	assert.Len(t, grants, 1, "other users' grants must be left alone")
}

// An admin saving the interface while a login revokes access must not bring the grant back.
func TestSqlRepo_InterfaceSaveDoesNotResurrectRevokedGrant(t *testing.T) {
	repo := newInterfaceAccessTestRepo(t)
	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
	require.NoError(t, repo.db.Create(&domain.Interface{Identifier: "wg-data"}).Error)
	require.NoError(t, repo.SaveInterfaceAccessGrant(ctx, domain.InterfaceAccessGrant{
		InterfaceIdentifier: "wg-data", ProviderName: "oidc1", UserIdentifier: "alice", EvaluatedAt: time.Now(),
	}))

	readDone := make(chan struct{})
	adminDone := make(chan error)
	go func() {
		adminDone <- repo.SaveInterface(ctx, "wg-data", func(i *domain.Interface) (*domain.Interface, error) {
			close(readDone)
			time.Sleep(50 * time.Millisecond) // the backend round trip of a real interface save
			i.DisplayName = "renamed by admin"
			return i, nil
		})
	}()
	<-readDone
	require.NoError(t, repo.DeleteInterfaceAccessGrant(ctx, "wg-data", "oidc1", "alice"))
	require.NoError(t, <-adminDone)

	grants, err := repo.GetUserInterfaceAccessGrants(ctx, "alice")
	require.NoError(t, err)
	assert.Empty(t, grants)
	iface, err := repo.GetInterface(ctx, "wg-data")
	require.NoError(t, err)
	assert.Equal(t, "renamed by admin", iface.DisplayName)
}

func TestSqlRepo_DeleteInterfaceRemovesGrants(t *testing.T) {
	repo := newInterfaceAccessTestRepo(t)
	require.NoError(t, repo.db.AutoMigrate(&domain.InterfaceStatus{}))
	require.NoError(t, repo.db.Exec(
		"CREATE TABLE peers (identifier text, interface_identifier text)").Error) // DeleteInterface also deletes peers
	ctx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
	require.NoError(t, repo.db.Create(&domain.Interface{Identifier: "wg-data"}).Error)
	require.NoError(t, repo.SaveInterfaceAccessGrant(ctx, domain.InterfaceAccessGrant{
		InterfaceIdentifier: "wg-data", ProviderName: "oidc1", UserIdentifier: "alice", EvaluatedAt: time.Now(),
	}))

	require.NoError(t, repo.DeleteInterface(ctx, "wg-data"))

	grants, err := repo.GetUserInterfaceAccessGrants(ctx, "alice")
	require.NoError(t, err)
	assert.Empty(t, grants, "a re-created interface with the same name must not inherit old grants")
}

func TestSqlRepo_DeleteInterfaceAccessGrantOnlyMatchingProvider(t *testing.T) {
	repo := newInterfaceAccessTestRepo(t)
	ctx := context.Background()
	for _, provider := range []string{"oidc-a", "oidc-b"} {
		require.NoError(t, repo.SaveInterfaceAccessGrant(ctx, domain.InterfaceAccessGrant{
			InterfaceIdentifier: "wg-data", ProviderName: provider, UserIdentifier: "alice", EvaluatedAt: time.Now(),
		}))
	}

	require.NoError(t, repo.DeleteInterfaceAccessGrant(ctx, "wg-data", "oidc-b", "alice"))

	grants, err := repo.GetUserInterfaceAccessGrants(ctx, "alice")
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Equal(t, "oidc-a", grants[0].ProviderName)
}
