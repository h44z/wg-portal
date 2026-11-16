package wireguard

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"

	"github.com/h44z/wg-portal/internal/adapters"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/app/users"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// A login stores a grant through the user manager, as OauthLoginStep2 does, and the WireGuard manager enforces
// it from the same SQLite database until it is revoked or expires.
func TestInterfaceAccessGrants_SqliteEndToEnd(t *testing.T) {
	schema.RegisterSerializer("encstr", app.NewGormEncryptedStringSerializer(""))
	maxAge := time.Hour
	cfg := &config.Config{}
	cfg.Database = config.DatabaseConfig{Type: config.DatabaseSQLite, DSN: filepath.Join(t.TempDir(), "sqlite.db")}
	cfg.Core.SelfProvisioningAllowed = true
	cfg.Core.EditableKeys = true
	cfg.Auth.OpenIDConnect = []config.OpenIDConnectProvider{{
		ProviderName:          "oidc1",
		InterfaceFilter:       map[string]string{"wg-data": "^vpn-data$"},
		InterfaceFilterMaxAge: &maxAge,
	}}

	rawDb, err := adapters.NewDatabase(cfg.Database)
	require.NoError(t, err)
	repo, err := adapters.NewSqlRepository(rawDb, cfg)
	require.NoError(t, err)

	adminCtx := domain.SetUserInfo(context.Background(), domain.SystemAdminContextUserInfo())
	require.NoError(t, repo.SaveInterface(adminCtx, "wg-data", func(i *domain.Interface) (*domain.Interface, error) {
		i.Type = domain.InterfaceTypeServer
		return i, nil
	}))
	require.NoError(t, repo.SaveUser(adminCtx, "alice", func(u *domain.User) (*domain.User, error) {
		return u, nil
	}))

	userManager, err := users.NewUserManager(cfg, &mockBus{}, repo, repo, repo)
	require.NoError(t, err)
	wgManager := Manager{
		cfg: cfg,
		bus: &mockBus{},
		db:  repo,
		wg: &ControllerManager{
			controllers: map[domain.InterfaceBackend]backendInstance{
				config.LocalBackendName: {Implementation: &mockController{}},
			},
		},
	}
	aliceCtx := domain.SetUserInfo(context.Background(), &domain.ContextUserInfo{Id: "alice"})
	createPeer := func(key string) error {
		_, err := wgManager.CreatePeer(aliceCtx, &domain.Peer{
			UserIdentifier:      "alice",
			InterfaceIdentifier: "wg-data",
			Interface:           domain.PeerInterfaceConfig{KeyPair: domain.KeyPair{PublicKey: key}},
		})
		return err
	}
	visible := func() bool {
		ifaces, err := wgManager.GetUserInterfaces(context.Background(), "alice")
		require.NoError(t, err)
		return len(ifaces) == 1
	}
	login := func(allowed bool) {
		require.NoError(t, userManager.UpdateOauthInterfaceAccess(adminCtx, "oidc1", "alice",
			map[domain.InterfaceIdentifier]bool{"wg-data": allowed}))
	}

	assert.False(t, visible(), "without a grant the interface is hidden")
	assert.ErrorIs(t, createPeer("KEY_1"), domain.ErrNoPermission)

	login(true)
	assert.True(t, visible())
	require.NoError(t, createPeer("KEY_2"))

	login(false)
	assert.False(t, visible(), "a revoked grant must deny access")
	assert.ErrorIs(t, createPeer("KEY_3"), domain.ErrNoPermission)

	login(true)
	require.NoError(t, rawDb.Model(&domain.InterfaceAccessGrant{}).Where("user_identifier = ?", "alice").
		Update("evaluated_at", time.Now().Add(-2*maxAge)).Error)
	assert.False(t, visible(), "an expired grant must deny access")
	assert.ErrorIs(t, createPeer("KEY_4"), domain.ErrNoPermission)
}
