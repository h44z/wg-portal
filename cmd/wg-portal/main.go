package main

import (
	"context"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/h44z/wg-portal/internal/server"
	"github.com/sirupsen/logrus"
)

var Version = "unknown (local build)"

func main() {
	_ = setupLogger(logrus.StandardLogger())

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	logrus.Infof("starting WireGuard Portal Server [%s]...", Version)

	// Context for clean shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service := server.Server{}
	if err := service.Setup(ctx); err != nil {
		logrus.Fatalf("setup failed: %v", err)
	}

	// Attach signal handlers to context
	go func() {
		osCall := <-c
		logrus.Tracef("received system call: %v", osCall)
		cancel() // cancel the context
	}()

	// Start main process in background
	go service.Run()

	<-ctx.Done() // Wait until the context gets canceled

	// Give goroutines some time to stop gracefully
	logrus.Info("stopping WireGuard Portal Server...")
	time.Sleep(2 * time.Second)

	logrus.Infof("stopped WireGuard Portal Server...")
	logrus.Exit(0)
}

func setupLogger(logger *logrus.Logger) error {
	// Check environment variables for logrus settings
	level, ok := os.LookupEnv("LOG_LEVEL")
	if !ok {
		level = "debug" // Default logrus level
	}

	useJSON, ok := os.LookupEnv("LOG_JSON")
	if !ok {
		useJSON = "false" // Default use human readable logging
	}

	useColor, ok := os.LookupEnv("LOG_COLOR")
	if !ok {
		useColor = "true"
	}

	switch level {
	case "off":
		logger.SetOutput(ioutil.Discard)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "trace":
		logger.SetLevel(logrus.TraceLevel)
	}

	var formatter logrus.Formatter
	if useJSON == "false" {
		f := new(logrus.TextFormatter)
		f.TimestampFormat = "2006-01-02 15:04:05"
		f.FullTimestamp = true
		if useColor == "true" {
			f.ForceColors = true
		}
		formatter = f
	} else {
		f := new(logrus.JSONFormatter)
		f.TimestampFormat = "2006-01-02 15:04:05"
		formatter = f
	}

	logger.SetFormatter(formatter)

	return nil
}
