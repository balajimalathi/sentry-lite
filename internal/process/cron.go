package process

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/skndan/sentry-lite/internal/alerts"
	"github.com/skndan/sentry-lite/internal/store"
)

// CronWatcher evaluates missed/late monitors and fires alerts.
type CronWatcher struct {
	Store  *store.Store
	Alerts *alerts.Dispatcher
}

func (w *CronWatcher) Run(ctx context.Context) {
	log.Println("cron watcher started")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	w.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *CronWatcher) tick(ctx context.Context) {
	changes, err := w.Store.EvaluateCronMonitors(ctx)
	if err != nil {
		log.Printf("cron evaluate: %v", err)
		return
	}
	if w.Alerts == nil {
		return
	}
	for _, ch := range changes {
		if ch.NewStatus != "missed" && ch.NewStatus != "late" {
			continue
		}
		// Fire cron_missed for both late and missed (plan: same channels)
		trigger := "cron_missed"
		summary := fmt.Sprintf("Cron %q is %s (expected by %s, grace %ds)",
			ch.Monitor.Name, ch.NewStatus,
			strOr(ch.Monitor.NextExpectedAt, "?"), ch.Monitor.GraceSec)
		w.Alerts.Handle(ctx, alerts.Event{
			Trigger:   trigger,
			ProjectID: ch.Monitor.ProjectID,
			IssueID:   0,
			Title:     "Cron: " + ch.Monitor.Name,
			Culprit:   ch.Monitor.Slug,
			Summary:   summary,
		})
	}
}

func strOr(p *string, fallback string) string {
	if p != nil {
		return *p
	}
	return fallback
}
