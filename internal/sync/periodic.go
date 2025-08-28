package sync

import (
	"context"
	"log"
	"time"
)

func startPeriodicSync(ctx context.Context, wgManager WireguardSynchronizer, interval time.Duration) {
	log.Printf("✅ Starting periodic WireGuard sync every %s", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Println("Running initial sync on startup...")
	if err := wgManager.SyncDevice(); err != nil {
		log.Printf("ERROR during initial sync: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			log.Println("⚙️ Ticker fired: running periodic sync...")
			if err := wgManager.SyncDevice(); err != nil {
				log.Printf("ERROR during periodic sync: %v", err)
			}
		case <-ctx.Done():
			log.Println("Stopping periodic sync.")
			return
		}
	}
}

type WireguardSynchronizer interface {
	SyncDevice() error
}