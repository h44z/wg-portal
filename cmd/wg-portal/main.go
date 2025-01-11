package main

import (
	"context"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/h44z/wg-portal/internal/app/api/core"
	handlersV0 "github.com/h44z/wg-portal/internal/app/api/v0/handlers"
	backendV1 "github.com/h44z/wg-portal/internal/app/api/v1/backend"
	handlersV1 "github.com/h44z/wg-portal/internal/app/api/v1/handlers"
	"github.com/h44z/wg-portal/internal/app/audit"
	"github.com/h44z/wg-portal/internal/app/auth"
	"github.com/h44z/wg-portal/internal/app/configfile"
	"github.com/h44z/wg-portal/internal/app/mail"
	"github.com/h44z/wg-portal/internal/app/route"
	"github.com/h44z/wg-portal/internal/app/users"
	"github.com/h44z/wg-portal/internal/app/wireguard"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/adapters"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/sirupsen/logrus"
	evbus "github.com/vardius/message-bus"
)

// main entry point for WireGuard Portal
func main() {
	ctx := internal.SignalAwareContext(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	logrus.Infof("Starting WireGuard Portal V2...")
	logrus.Infof("WireGuard Portal version: %s", internal.Version)

	cfg, err := config.GetConfig()
	internal.AssertNoError(err)
	setupLogging(cfg)

	cfg.LogStartupValues()

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

	shouldExit, err := app.HandleProgramArgs(cfg, rawDb)
	switch {
	case shouldExit && err == nil:
		return
	case shouldExit && err != nil:
		logrus.Errorf("Failed to process program args: %v", err)
		os.Exit(1)
	case !shouldExit:
		internal.AssertNoError(err)
	}

	queueSize := 100
	eventBus := evbus.New(queueSize)

	userManager, err := users.NewUserManager(cfg, eventBus, database, database)
	internal.AssertNoError(err)

	authenticator, err := auth.NewAuthenticator(&cfg.Auth, cfg.Web.ExternalUrl, eventBus, userManager)
	internal.AssertNoError(err)

	wireGuardManager, err := wireguard.NewWireGuardManager(cfg, eventBus, wireGuard, wgQuick, database)
	internal.AssertNoError(err)

	statisticsCollector, err := wireguard.NewStatisticsCollector(cfg, eventBus, database, wireGuard, metricsServer)
	internal.AssertNoError(err)

	cfgFileManager, err := configfile.NewConfigFileManager(cfg, eventBus, database, database, cfgFileSystem)
	internal.AssertNoError(err)

	mailManager, err := mail.NewMailManager(cfg, mailer, cfgFileManager, database, database)
	internal.AssertNoError(err)

	auditRecorder, err := audit.NewAuditRecorder(cfg, eventBus, database)
	internal.AssertNoError(err)
	auditRecorder.StartBackgroundJobs(ctx)

	routeManager, err := route.NewRouteManager(cfg, eventBus, database)
	internal.AssertNoError(err)
	routeManager.StartBackgroundJobs(ctx)

	backend, err := app.New(cfg, eventBus, authenticator, userManager, wireGuardManager,
		statisticsCollector, cfgFileManager, mailManager)
	internal.AssertNoError(err)
	err = backend.Startup(ctx)
	internal.AssertNoError(err)

	apiFrontend := handlersV0.NewRestApi(cfg, backend)

	apiV1BackendUsers := backendV1.NewUserService(cfg, userManager)
	apiV1BackendPeers := backendV1.NewPeerService(cfg, wireGuardManager, userManager)
	apiV1BackendInterfaces := backendV1.NewInterfaceService(cfg, wireGuardManager)
	apiV1BackendProvisioning := backendV1.NewProvisioningService(cfg, userManager, wireGuardManager, cfgFileManager)
	apiV1EndpointUsers := handlersV1.NewUserEndpoint(apiV1BackendUsers)
	apiV1EndpointPeers := handlersV1.NewPeerEndpoint(apiV1BackendPeers)
	apiV1EndpointInterfaces := handlersV1.NewInterfaceEndpoint(apiV1BackendInterfaces)
	apiV1EndpointProvisioning := handlersV1.NewProvisioningEndpoint(apiV1BackendProvisioning)
	apiV1 := handlersV1.NewRestApi(
		userManager,
		apiV1EndpointUsers,
		apiV1EndpointPeers,
		apiV1EndpointInterfaces,
		apiV1EndpointProvisioning,
	)

	webSrv, err := core.NewServer(cfg, apiFrontend, apiV1)
	internal.AssertNoError(err)

	go metricsServer.Run(ctx)
	go webSrv.Run(ctx, cfg.Web.ListeningAddress)

	// wait until context gets cancelled
	<-ctx.Done()

	logrus.Infof("Stopping WireGuard Portal")

	time.Sleep(5 * time.Second) // wait for (most) goroutines to finish gracefully

	logrus.Infof("Stopped WireGuard Portal")
}

func setupLogging(cfg *config.Config) {
	switch strings.ToLower(cfg.Advanced.LogLevel) {
	case "trace":
		logrus.SetLevel(logrus.TraceLevel)
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info", "information":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn", "warning":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}

	switch {
	case cfg.Advanced.LogJson:
		logrus.SetFormatter(&logrus.JSONFormatter{
			PrettyPrint: cfg.Advanced.LogPretty,
		})
	case cfg.Advanced.LogPretty:
		logrus.SetFormatter(&logrus.TextFormatter{
			ForceColors:   true,
			DisableColors: false,
		})
	}
}
