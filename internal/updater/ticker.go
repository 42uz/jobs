package updater

import (
	"log"
	"time"

	"faangjobs/internal/httpapi"
	"faangjobs/internal/store"
)

// StartTickerSync performs an initial sync on launch, then triggers every hour.
func StartTickerSync(downloadURL, targetDir string, idx *httpapi.Index, st *store.Store) {
	go func() {
		log.Println("[Ticker] Running initial startup data sync check...")
		if err := SyncData(downloadURL, targetDir, idx, st); err != nil {
			log.Printf("[Ticker] Initial sync attempt notice: %v", err)
		}

		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			log.Println("[Ticker] 30 minutes elapsed. Checking GitHub Releases for updates...")
			if err := SyncData(downloadURL, targetDir, idx, st); err != nil {
				log.Printf("[Ticker] Scheduled background sync failed: %v", err)
			}
		}
	}()
}