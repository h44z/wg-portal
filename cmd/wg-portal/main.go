package main

import (
	"context"
	"fmt"
	"os"
	"syscall"

	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/adapters"
	"github.com/h44z/wg-portal/internal/app"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/ports/api/core"
	handlersV0 "github.com/h44z/wg-portal/internal/ports/api/v0/handlers"
	"github.com/sirupsen/logrus"
	evbus "github.com/vardius/message-bus"
)

func main() {
	ctx := internal.SignalAwareContext(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	logrus.Infof("Starting web portal...")

	cfg, err := config.GetConfig()
	internal.AssertNoError(err)

	rawDb, err := adapters.NewDatabase(cfg.Database)
	internal.AssertNoError(err)

	database, err := adapters.NewSqlRepository(rawDb)
	internal.AssertNoError(err)

	wireGuard := adapters.NewWireGuardRepository()

	shouldExit, err := app.HandleProgramArgs(cfg, rawDb, wireGuard)
	switch {
	case shouldExit && err == nil:
		return
	case shouldExit && err != nil:
		logrus.Errorf("failed to process program args: %v", err)
		os.Exit(1)
	case !shouldExit:
		internal.AssertNoError(err)
	}

	queueSize := 100
	eventBus := evbus.New(queueSize)

	backend, err := app.New(cfg, eventBus, database, wireGuard)
	internal.AssertNoError(err)
	backend.Users.StartBackgroundJobs(ctx)

	apiFrontend := handlersV0.NewRestApi(cfg, backend)

	webSrv, err := core.NewServer(cfg, apiFrontend)
	internal.AssertNoError(err)

	go webSrv.Run(ctx, cfg.Web.ListeningAddress)
	fmt.Println(backend) // TODO: Remove

	// wait until context gets cancelled
	<-ctx.Done()

	logrus.Infof("Stopped web portal")
}
