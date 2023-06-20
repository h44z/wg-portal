package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/sirupsen/logrus"
	evbus "github.com/vardius/message-bus"
	"time"
)

type App struct {
	Config *config.Config
	bus    evbus.MessageBus

	Authenticator
	UserManager
	WireGuardManager
	StatisticsCollector
	TemplateManager
}

func New(cfg *config.Config, bus evbus.MessageBus, authenticator Authenticator, users UserManager, wireGuard WireGuardManager, stats StatisticsCollector, templates TemplateManager) (*App, error) {

	a := &App{
		Config: cfg,
		bus:    bus,

		Authenticator:       authenticator,
		UserManager:         users,
		WireGuardManager:    wireGuard,
		StatisticsCollector: stats,
		TemplateManager:     templates,
	}

	startupContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// The first user in the DB is admin.
	startupContext = context.WithValue(startupContext, domain.CtxUserInfo, domain.GetAdminInfo())

	if err := a.createDefaultUser(startupContext); err != nil {
		return nil, fmt.Errorf("failed to create default user: %w", err)
	}

	if err := a.importNewInterfaces(startupContext); err != nil {
		return nil, fmt.Errorf("failed to import new interfaces: %w", err)
	}

	if err := a.restoreInterfaceState(startupContext); err != nil {
		return nil, fmt.Errorf("failed to restore interface state: %w", err)
	}

	return a, nil
}

func (a *App) Startup(ctx context.Context) error {
	a.UserManager.StartBackgroundJobs(ctx)
	a.StatisticsCollector.StartBackgroundJobs(ctx)

	return nil
}

func (a *App) importNewInterfaces(ctx context.Context) error {
	if !a.Config.Core.ImportExisting {
		logrus.Trace("skipping interface import - feature disabled")
		return nil // feature disabled
	}

	err := a.ImportNewInterfaces(ctx)
	if err != nil {
		return err
	}

	logrus.Trace("potential new interfaces imported")
	return nil
}

func (a *App) restoreInterfaceState(ctx context.Context) error {
	if !a.Config.Core.RestoreState {
		logrus.Trace("skipping interface state restore - feature disabled")
		return nil // feature disabled
	}

	err := a.RestoreInterfaceState(ctx, true)
	if err != nil {
		return err
	}

	logrus.Trace("interface state restored")
	return nil
}

func (a *App) createDefaultUser(ctx context.Context) error {
	adminUserId := domain.UserIdentifier(a.Config.Core.AdminUser)
	if adminUserId == "" {
		logrus.Trace("skipping default user creation - admin user is blank")
		return nil // empty admin user - do not create
	}

	_, err := a.GetUser(ctx, adminUserId)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	if err == nil {
		logrus.Trace("skipping default user creation - admin user already exists")
		return nil // admin user already exists
	}

	now := time.Now()
	admin, err := a.CreateUser(ctx, &domain.User{
		BaseModel: domain.BaseModel{
			CreatedBy: "system",
			UpdatedBy: "system",
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
		Password:        domain.PrivateString(a.Config.Core.AdminPassword),
		Disabled:        nil,
		DisabledReason:  "",
		Locked:          nil,
		LockedReason:    "",
		LinkedPeerCount: 0,
	})
	if err != nil {
		return err
	}

	logrus.Tracef("admin user %s created", admin.Identifier)

	return nil
}
