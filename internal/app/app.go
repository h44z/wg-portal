package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// region dependencies

type WireGuardManager interface {
	ImportNewInterfaces(ctx context.Context, filter ...domain.InterfaceIdentifier) (int, error)
	RestoreInterfaceState(ctx context.Context, updateDbOnError bool, filter ...domain.InterfaceIdentifier) error
}

type UserManager interface {
	GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
	CreateUser(ctx context.Context, user *domain.User) (*domain.User, error)
}

// endregion dependencies

// App is the main application struct.
type App struct {
	cfg *config.Config

	wg    WireGuardManager
	users UserManager
}

// Initialize creates a new App instance and initializes it.
func Initialize(
	cfg *config.Config,
	wg WireGuardManager,
	users UserManager,
) error {
	a := &App{
		cfg: cfg,

		wg:    wg,
		users: users,
	}

	startupContext, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Switch to admin user context
	startupContext = domain.SetUserInfo(startupContext, domain.SystemAdminContextUserInfo())

	if !cfg.Core.AdminUserDisabled {
		if err := a.createDefaultUser(startupContext); err != nil {
			return fmt.Errorf("failed to create default user: %w", err)
		}
	} else {
		slog.Info("Local Admin user disabled!")
	}

	if err := a.importNewInterfaces(startupContext); err != nil {
		return fmt.Errorf("failed to import new interfaces: %w", err)
	}

	if err := a.restoreInterfaceState(startupContext); err != nil {
		return fmt.Errorf("failed to restore interface state: %w", err)
	}

	return nil
}

func (a *App) importNewInterfaces(ctx context.Context) error {
	if !a.cfg.Core.ImportExisting {
		slog.Debug("skipping interface import - feature disabled")
		return nil // feature disabled
	}

	importedCount, err := a.wg.ImportNewInterfaces(ctx)
	if err != nil {
		return err
	}

	if importedCount > 0 {
		slog.Info("new interfaces imported", "count", importedCount)
	}
	return nil
}

func (a *App) restoreInterfaceState(ctx context.Context) error {
	if !a.cfg.Core.RestoreState {
		slog.Debug("skipping interface state restore - feature disabled")
		return nil // feature disabled
	}

	err := a.wg.RestoreInterfaceState(ctx, true)
	if err != nil {
		return err
	}

	slog.Info("interface state restored")
	return nil
}

func (a *App) createDefaultUser(ctx context.Context) error {
	adminUserId := domain.UserIdentifier(a.cfg.Core.AdminUser)
	if adminUserId == "" {
		slog.Debug("skipping default user creation - admin user is blank")
		return nil // empty admin user - do not create
	}

	_, err := a.users.GetUser(ctx, adminUserId)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	if err == nil {
		slog.Debug("skipping default user creation - admin user already exists")
		return nil // admin user already exists
	}

	now := time.Now()
	defaultAdmin := &domain.User{
		BaseModel: domain.BaseModel{
			CreatedBy: domain.CtxSystemAdminId,
			UpdatedBy: domain.CtxSystemAdminId,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Identifier:      adminUserId,
		Email:           "admin@wgportal.local",
		Source:          domain.UserSourceDatabase,
		ProviderName:    "",
		IsAdmin:         true,
		Firstname:       "WireGuard Portal",
		Lastname:        "Admin",
		Phone:           "",
		Department:      "",
		Notes:           "default administrator user",
		Password:        domain.PrivateString(a.cfg.Core.AdminPassword),
		Disabled:        nil,
		DisabledReason:  "",
		Locked:          nil,
		LockedReason:    "",
		LinkedPeerCount: 0,
	}
	if a.cfg.Core.AdminApiToken != "" {
		if len(a.cfg.Core.AdminApiToken) < 18 {
			slog.Warn("admin API token is too short, should be at least 18 characters long")
		}
		defaultAdmin.ApiToken = a.cfg.Core.AdminApiToken
		defaultAdmin.ApiTokenCreated = &now
	}

	admin, err := a.users.CreateUser(ctx, defaultAdmin)
	if err != nil {
		return err
	}

	slog.Info("admin user created", "identifier", admin.Identifier)

	return nil
}
