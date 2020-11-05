package main

import (
	"github.com/h44z/wg-portal/internal/server"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.Infof("Starting WireGuard Portal Server...")

	service := server.Server{}
	if err := service.Setup(); err != nil {
		log.Fatalf("Setup failed: %v", err)
	}

	service.Run()

	log.Infof("Stopped WireGuard Portal Server...")
}
