package main

import (
	"io/ioutil"
	"os"

	"github.com/h44z/wg-portal/internal/server"
	"github.com/sirupsen/logrus"
)

func main() {
	_ = setupLogger(logrus.StandardLogger())

	logrus.Infof("Starting WireGuard Portal Server...")

	service := server.Server{}
	if err := service.Setup(); err != nil {
		logrus.Fatalf("Setup failed: %v", err)
	}

	service.Run()

	logrus.Infof("Stopped WireGuard Portal Server...")
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
