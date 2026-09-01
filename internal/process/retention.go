package process

import (
	"context"
	"log"
	"time"

	"github.com/skndan/sentry-lite/internal/store"
)

type RetentionWorker struct {
	Store *store.Store
	Days  int
}

func (w *RetentionWorker) Run(ctx context.Context) {
	if w.Days <= 0 {
		log.Println("retention disabled (EVENT_RETENTION_DAYS<=0)")
		return
	}
	log.Printf("retention worker started (%d days)", w.Days)
	w.purge(ctx)
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.purge(ctx)
		}
	}
}

func (w *RetentionWorker) purge(ctx context.Context) {
	cutoff := time.Now().UTC().Add(-time.Duration(w.Days) * 24 * time.Hour)
	res, err := w.Store.PurgeBefore(ctx, cutoff)
	if err != nil {
		log.Printf("retention purge: %v", err)
		return
	}
	if res.Events > 0 || res.Transactions > 0 {
		log.Printf("retention purged events=%d transactions=%d files=%d", res.Events, res.Transactions, res.Files)
	}
}
