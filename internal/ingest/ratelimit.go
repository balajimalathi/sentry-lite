package ingest

import (
	"sync"
	"time"
)

type tokenBucket struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
	rps    float64
}

func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	if b.last.IsZero() {
		b.last = now
		b.tokens = b.rps
	}
	elapsed := now.Sub(b.last).Seconds()
	b.last = now
	b.tokens += elapsed * b.rps
	if b.tokens > b.rps {
		b.tokens = b.rps
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (h *Handler) allowIngest(projectID int64) bool {
	if h.IngestRPS <= 0 {
		return true
	}
	v, _ := h.limiters.LoadOrStore(projectID, &tokenBucket{
		rps:    float64(h.IngestRPS),
		tokens: float64(h.IngestRPS),
		last:   time.Now(),
	})
	return v.(*tokenBucket).allow()
}
