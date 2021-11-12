package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/h44z/wg-portal/internal/persistence"

	"github.com/h44z/wg-portal/cmd/wg-portal/common"

	"github.com/sirupsen/logrus"
)

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	logrus.Info("starting WireGuard Portal...")

	// Context for clean shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Attach signal handlers to context
	go func() {
		osCall := <-c
		logrus.Tracef("received system call: %v", osCall)
		cancel() // cancel the context
	}()

	go entrypoint(ctx, cancel) // main entry point

	<-ctx.Done() // Wait until the context gets canceled

	logrus.Info("stopped WireGuard Portal")
	logrus.Exit(0)
}

func entrypoint(ctx context.Context, cancel context.CancelFunc) {
	defer cancel() // quit program if main entrypoint ends

	// default config, TODO: implement
	cfg := &common.Config{
		Database: persistence.DatabaseConfig{
			Type: "sqlite",
			DSN:  "sqlite.db",
		},
	}
	cfg.Core.ListeningAddress = ":8080"
	cfg.Core.ExternalUrl = "http://localhost:8080"
	cfg.Core.GinDebug = true
	cfg.Core.LogLevel = "trace"
	cfg.Core.CompanyName = "Test Company"
	cfg.Core.LogoUrl = "/img/header-logo.png"

	err := common.LoadConfigFile(&cfg, "config.yml")
	if err != nil {
		logrus.Errorf("failed to load config file: %v", err)
		return
	}

	srv, err := NewServer(cfg)
	if err != nil {
		logrus.Errorf("failed to setup server: %v", err)
		return
	}
	defer srv.Shutdown()

	// Run is blocking
	srv.Run(ctx)
}
