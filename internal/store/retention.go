package store

import (
	"context"
	"os"
	"time"
)

type PurgeResult struct {
	Events       int64
	Transactions int64
	Files        int
}

func (s *Store) PurgeBefore(ctx context.Context, cutoff time.Time) (*PurgeResult, error) {
	out := &PurgeResult{}
	if cutoff.IsZero() {
		return out, nil
	}
	cutoffStr := cutoff.UTC().Format(time.RFC3339Nano)
	db := s.DB.WithContext(ctx)

	var eventPaths []string
	if err := db.Model(&EventRow{}).Where("timestamp < ?", cutoffStr).Pluck("raw_path", &eventPaths).Error; err != nil {
		return nil, err
	}
	var eventIDs []string
	if err := db.Model(&EventRow{}).Where("timestamp < ?", cutoffStr).Pluck("event_id", &eventIDs).Error; err != nil {
		return nil, err
	}
	var txPaths []string
	if err := db.Model(&TransactionRow{}).Where("timestamp < ?", cutoffStr).Pluck("raw_path", &txPaths).Error; err != nil {
		return nil, err
	}
	var txEventIDs []string
	if err := db.Model(&TransactionRow{}).Where("timestamp < ?", cutoffStr).Pluck("event_id", &txEventIDs).Error; err != nil {
		return nil, err
	}

	if len(eventIDs) > 0 {
		if err := db.Where("event_id IN ?", eventIDs).Delete(&EventTagRow{}).Error; err != nil {
			return nil, err
		}
	}
	res := db.Where("timestamp < ?", cutoffStr).Delete(&EventRow{})
	if res.Error != nil {
		return nil, res.Error
	}
	out.Events = res.RowsAffected

	if len(txEventIDs) > 0 {
		if err := db.Where("transaction_event_id IN ?", txEventIDs).Delete(&SpanRow{}).Error; err != nil {
			return nil, err
		}
	}
	res = db.Where("timestamp < ?", cutoffStr).Delete(&TransactionRow{})
	if res.Error != nil {
		return nil, res.Error
	}
	out.Transactions = res.RowsAffected

	seen := map[string]bool{}
	for _, p := range append(eventPaths, txPaths...) {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		if err := os.Remove(p); err == nil || os.IsNotExist(err) {
			out.Files++
		}
	}
	return out, nil
}
