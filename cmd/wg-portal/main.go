package main

import (
	"context"
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	evbus "github.com/vardius/message-bus"
	"gorm.io/gorm/schema"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/adapters"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/app/api/core"
	backendV0 "github.com/h44z/wg-portal/internal/app/api/v0/backend"
	handlersV0 "github.com/h44z/wg-portal/internal/app/api/v0/handlers"
	backendV1 "github.com/h44z/wg-portal/internal/app/api/v1/backend"
	handlersV1 "github.com/h44z/wg-portal/internal/app/api/v1/handlers"
	"github.com/h44z/wg-portal/internal/app/audit"
	"github.com/h44z/wg-portal/internal/app/auth"
	"github.com/h44z/wg-portal/internal/app/configfile"
	"github.com/h44z/wg-portal/internal/app/mail"
	"github.com/h44z/wg-portal/internal/app/route"
	"github.com/h44z/wg-portal/internal/app/users"
	"github.com/h44z/wg-portal/internal/app/webhooks"
	"github.com/h44z/wg-portal/internal/app/wireguard"
	"github.com/h44z/wg-portal/internal/config"
)

// main entry point for WireGuard Portal
func main() {
	ctx := internal.SignalAwareContext(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	slog.Info("Starting WireGuard Portal V2...", "version", internal.Version)

	cfg, err := config.GetConfig()
	internal.AssertNoError(err)
	internal.SetupLogging(cfg.Advanced.LogLevel, cfg.Advanced.LogPretty, cfg.Advanced.LogJson)

	cfg.LogStartupValues()

	dbEncryptedSerializer := app.NewGormEncryptedStringSerializer(cfg.Database.EncryptionPassphrase)
	schema.RegisterSerializer("encstr", dbEncryptedSerializer)
	rawDb, err := adapters.NewDatabase(cfg.Database)
	internal.AssertNoError(err)

	database, err := adapters.NewSqlRepository(rawDb)
	internal.AssertNoError(err)

	wireGuard := adapters.NewWireGuardRepository()

	wgQuick := adapters.NewWgQuickRepo()

	mailer := adapters.NewSmtpMailRepo(cfg.Mail)

	metricsServer := adapters.NewMetricsServer(cfg)

	cfgFileSystem, err := adapters.NewFileSystemRepository(cfg.Advanced.ConfigStoragePath)
	internal.AssertNoError(err)

	shouldExit, err := app.HandleProgramArgs(rawDb)
	switch {
	case shouldExit && err == nil:
		return
	case shouldExit:
		slog.Error("Failed to process program args", "error", err)
		os.Exit(1)
	default:
		internal.AssertNoError(err)
	}

	queueSize := 100
	eventBus := evbus.New(queueSize)

	auditManager := audit.NewManager(database)

	auditRecorder, err := audit.NewAuditRecorder(cfg, eventBus, database)
	internal.AssertNoError(err)
	auditRecorder.StartBackgroundJobs(ctx)

	userManager, err := users.NewUserManager(cfg, eventBus, database, database)
	internal.AssertNoError(err)
	userManager.StartBackgroundJobs(ctx)

	authenticator, err := auth.NewAuthenticator(&cfg.Auth, cfg.Web.ExternalUrl, eventBus, userManager)
	internal.AssertNoError(err)

	webAuthn, err := auth.NewWebAuthnAuthenticator(cfg, eventBus, userManager)
	internal.AssertNoError(err)

	wireGuardManager, err := wireguard.NewWireGuardManager(cfg, eventBus, wireGuard, wgQuick, database)
	internal.AssertNoError(err)
	wireGuardManager.StartBackgroundJobs(ctx)

	statisticsCollector, err := wireguard.NewStatisticsCollector(cfg, eventBus, database, wireGuard, metricsServer)
	internal.AssertNoError(err)
	statisticsCollector.StartBackgroundJobs(ctx)

	cfgFileManager, err := configfile.NewConfigFileManager(cfg, eventBus, database, database, cfgFileSystem)
	internal.AssertNoError(err)

	mailManager, err := mail.NewMailManager(cfg, mailer, cfgFileManager, database, database)
	internal.AssertNoError(err)

	routeManager, err := route.NewRouteManager(cfg, eventBus, database)
	internal.AssertNoError(err)
	routeManager.StartBackgroundJobs(ctx)

	webhookManager, err := webhooks.NewManager(cfg, eventBus)
	internal.AssertNoError(err)
	webhookManager.StartBackgroundJobs(ctx)

	err = app.Initialize(cfg, wireGuardManager, userManager)
	internal.AssertNoError(err)

	validatorManager := validator.New()

	// region API v0 (SPA frontend)

	apiV0Session := handlersV0.NewSessionWrapper(cfg)
	apiV0Auth := handlersV0.NewAuthenticationHandler(authenticator, apiV0Session)

	apiV0BackendUsers := backendV0.NewUserService(cfg, userManager, wireGuardManager)
	apiV0BackendInterfaces := backendV0.NewInterfaceService(cfg, wireGuardManager, cfgFileManager)
	apiV0BackendPeers := backendV0.NewPeerService(cfg, wireGuardManager, cfgFileManager, mailManager)

	apiV0EndpointAuth := handlersV0.NewAuthEndpoint(cfg, apiV0Auth, apiV0Session, validatorManager, authenticator,
		webAuthn)
	apiV0EndpointAudit := handlersV0.NewAuditEndpoint(cfg, apiV0Auth, auditManager)
	apiV0EndpointUsers := handlersV0.NewUserEndpoint(cfg, apiV0Auth, validatorManager, apiV0BackendUsers)
	apiV0EndpointInterfaces := handlersV0.NewInterfaceEndpoint(cfg, apiV0Auth, validatorManager, apiV0BackendInterfaces)
	apiV0EndpointPeers := handlersV0.NewPeerEndpoint(cfg, apiV0Auth, validatorManager, apiV0BackendPeers)
	apiV0EndpointConfig := handlersV0.NewConfigEndpoint(cfg, apiV0Auth)
	apiV0EndpointTest := handlersV0.NewTestEndpoint(apiV0Auth)

	apiFrontend := handlersV0.NewRestApi(apiV0Session,
		apiV0EndpointAuth,
		apiV0EndpointAudit,
		apiV0EndpointUsers,
		apiV0EndpointInterfaces,
		apiV0EndpointPeers,
		apiV0EndpointConfig,
		apiV0EndpointTest,
	)

	// endregion API v0 (SPA frontend)

	// region API v1 (User REST API)

	apiV1Auth := handlersV1.NewAuthenticationHandler(userManager)
	apiV1BackendUsers := backendV1.NewUserService(cfg, userManager)
	apiV1BackendPeers := backendV1.NewPeerService(cfg, wireGuardManager, userManager)
	apiV1BackendInterfaces := backendV1.NewInterfaceService(cfg, wireGuardManager)
	apiV1BackendProvisioning := backendV1.NewProvisioningService(cfg, userManager, wireGuardManager, cfgFileManager)
	apiV1BackendMetrics := backendV1.NewMetricsService(cfg, database, userManager, wireGuardManager)

	apiV1EndpointUsers := handlersV1.NewUserEndpoint(apiV1Auth, validatorManager, apiV1BackendUsers)
	apiV1EndpointPeers := handlersV1.NewPeerEndpoint(apiV1Auth, validatorManager, apiV1BackendPeers)
	apiV1EndpointInterfaces := handlersV1.NewInterfaceEndpoint(apiV1Auth, validatorManager, apiV1BackendInterfaces)
	apiV1EndpointProvisioning := handlersV1.NewProvisioningEndpoint(apiV1Auth, validatorManager,
		apiV1BackendProvisioning)
	apiV1EndpointMetrics := handlersV1.NewMetricsEndpoint(apiV1Auth, validatorManager, apiV1BackendMetrics)

	apiV1 := handlersV1.NewRestApi(
		apiV1EndpointUsers,
		apiV1EndpointPeers,
		apiV1EndpointInterfaces,
		apiV1EndpointProvisioning,
		apiV1EndpointMetrics,
	)

	// endregion API v1 (User REST API)

	webSrv, err := core.NewServer(cfg, apiFrontend, apiV1)
	internal.AssertNoError(err)

	go metricsServer.Run(ctx)
	go webSrv.Run(ctx, cfg.Web.ListeningAddress)

	slog.Info("Application startup complete")

	// wait until context gets cancelled
	<-ctx.Done()

	slog.Info("Stopping WireGuard Portal")

	time.Sleep(5 * time.Second) // wait for (most) goroutines to finish gracefully

	slog.Info("Stopped WireGuard Portal")
}
