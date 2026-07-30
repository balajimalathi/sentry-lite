package load

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

const latencyRingSize = 8192

type Counters struct {
	Sent     atomic.Int64
	OK       atomic.Int64
	Err4xx   atomic.Int64
	Err5xx   atomic.Int64
	Timeout  atomic.Int64
	OtherErr atomic.Int64
	InFlight atomic.Int64
	ByCat    [catCount]atomic.Int64

	latIdx atomic.Uint64
	latBuf [latencyRingSize]int64 // nanoseconds
}

func (c *Counters) Record(cat Category, r SendResult) {
	c.Sent.Add(1)
	c.ByCat[cat].Add(1)
	if r.Latency > 0 {
		i := c.latIdx.Add(1) % latencyRingSize
		c.latBuf[i] = r.Latency.Nanoseconds()
	}
	switch {
	case r.Err != nil:
		if isTimeout(r.Err) {
			c.Timeout.Add(1)
		} else {
			c.OtherErr.Add(1)
		}
	case r.StatusCode >= 200 && r.StatusCode < 300:
		c.OK.Add(1)
	case r.StatusCode >= 400 && r.StatusCode < 500:
		c.Err4xx.Add(1)
	case r.StatusCode >= 500:
		c.Err5xx.Add(1)
	default:
		c.OtherErr.Add(1)
	}
}

func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "timeout") || strings.Contains(s, "deadline") || strings.Contains(s, "context canceled")
}

func (c *Counters) Percentiles() (p50, p95, p99 time.Duration) {
	samples := make([]int64, 0, latencyRingSize)
	sent := c.Sent.Load()
	n := sent
	if n > latencyRingSize {
		n = latencyRingSize
	}
	start := c.latIdx.Load()
	for i := uint64(0); i < uint64(n); i++ {
		idx := (start - i - 1 + latencyRingSize) % latencyRingSize
		v := c.latBuf[idx]
		if v > 0 {
			samples = append(samples, v)
		}
	}
	if len(samples) == 0 {
		return 0, 0, 0
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })
	p50 = time.Duration(samples[len(samples)*50/100])
	p95 = time.Duration(samples[len(samples)*95/100])
	p99 = time.Duration(samples[len(samples)*99/100])
	return p50, p95, p99
}

type HealthSnapshot struct {
	At         time.Time
	OK         bool
	Latency    time.Duration
	Err        string
	WasHealthy bool
	Crashed    bool
	DataBytes  uint64
	SQLiteBytes uint64
	EventFiles int
}

func CollectDisk(dataDir string) (dataBytes, sqliteBytes uint64, eventFiles int) {
	dataDir = filepath.Clean(dataDir)
	_ = filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		size := uint64(info.Size())
		dataBytes += size
		base := filepath.Base(path)
		if base == "sentry-lite.db" || base == "sentry-lite.db-wal" || base == "sentry-lite.db-shm" {
			sqliteBytes += size
		}
		if filepath.Base(filepath.Dir(path)) != "" && filepath.Base(filepath.Dir(path)) != "data" {
			if filepath.Ext(path) == ".json" {
				eventFiles++
			}
		}
		return nil
	})
	eventsDir := filepath.Join(dataDir, "events")
	_ = filepath.Walk(eventsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".json" {
			eventFiles++
		}
		return nil
	})
	return dataBytes, sqliteBytes, eventFiles
}

type RateTracker struct {
	lastAt    time.Time
	lastSent  int64
	instant   float64
}

func (r *RateTracker) Update(sent int64) float64 {
	now := time.Now()
	if r.lastAt.IsZero() {
		r.lastAt = now
		r.lastSent = sent
		return 0
	}
	dt := now.Sub(r.lastAt).Seconds()
	if dt <= 0 {
		return r.instant
	}
	r.instant = float64(sent-r.lastSent) / dt
	r.lastAt = now
	r.lastSent = sent
	return r.instant
}

func (r *RateTracker) Avg(sent int64, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	return float64(sent) / elapsed.Seconds()
}
