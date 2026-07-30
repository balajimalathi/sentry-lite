package process

import (
	"context"
	"log"
	"time"

	"github.com/skndan/sentry-lite/internal/store"
)

// RollupWorker periodically recomputes transaction latency stats.
type RollupWorker struct {
	Store *store.Store
}

func (w *RollupWorker) Run(ctx context.Context) {
	log.Println("perf rollup started")
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	// Run once soon after start
	if err := w.Store.RecomputeTransactionStats(ctx, store.DefaultPerfWindowSec); err != nil {
		log.Printf("perf rollup: %v", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.Store.RecomputeTransactionStats(ctx, store.DefaultPerfWindowSec); err != nil {
				log.Printf("perf rollup: %v", err)
			}
		}
	}
}
