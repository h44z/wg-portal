package main

import (
	"context"
	"github.com/h44z/wg-portal/internal/app/api/core"
	handlersV0 "github.com/h44z/wg-portal/internal/app/api/v0/handlers"
	"github.com/h44z/wg-portal/internal/app/auth"
	"github.com/h44z/wg-portal/internal/app/filetemplate"
	"github.com/h44z/wg-portal/internal/app/users"
	"github.com/h44z/wg-portal/internal/app/wireguard"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/adapters"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/sirupsen/logrus"
	evbus "github.com/vardius/message-bus"
)

func main() {
	ctx := internal.SignalAwareContext(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	logrus.Infof("Starting WireGuard Portal...")

	cfg, err := config.GetConfig()
	internal.AssertNoError(err)
	setupLogging(cfg)

	rawDb, err := adapters.NewDatabase(cfg.Database)
	internal.AssertNoError(err)

	database, err := adapters.NewSqlRepository(rawDb)
	internal.AssertNoError(err)

	wireGuard := adapters.NewWireGuardRepository()

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

	authenticator, err := auth.NewAuthenticator(&cfg.Auth, eventBus, userManager)
	internal.AssertNoError(err)

	wireGuardManager, err := wireguard.NewWireGuardManager(cfg, eventBus, wireGuard, database)
	internal.AssertNoError(err)

	statisticsCollector, err := wireguard.NewStatisticsCollector(cfg, database, wireGuard)
	internal.AssertNoError(err)

	templateManager, err := filetemplate.NewTemplateManager(cfg, database, database)
	internal.AssertNoError(err)

	backend, err := app.New(cfg, eventBus, authenticator, userManager, wireGuardManager,
		statisticsCollector, templateManager)
	internal.AssertNoError(err)
	err = backend.Startup(ctx)
	internal.AssertNoError(err)

	apiFrontend := handlersV0.NewRestApi(cfg, backend)

	webSrv, err := core.NewServer(cfg, apiFrontend)
	internal.AssertNoError(err)

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
		logrus.SetLevel(logrus.WarnLevel)
	}
}
