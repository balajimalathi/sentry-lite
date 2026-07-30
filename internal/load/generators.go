package load

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Gen struct {
	rng *rand.Rand
}

func NewGen(seed int64) *Gen {
	return &Gen{rng: rand.New(rand.NewSource(seed))}
}

func (g *Gen) PickCategory(mix [catCount]int) Category {
	total := 0
	for _, w := range mix {
		total += w
	}
	if total == 0 {
		return CatError
	}
	n := g.rng.Intn(total)
	for i, w := range mix {
		n -= w
		if n < 0 {
			return Category(i)
		}
	}
	return CatError
}

func (g *Gen) Build(cat Category) ([]byte, error) {
	switch cat {
	case CatError:
		return g.errorEvent("sample client throw (load-test)", "client")
	case CatMessage:
		return g.messageEvent("sample captureMessage (load-test)")
	case CatContext:
		return g.contextEvent()
	case CatFingerprint:
		return g.fingerprintMessage()
	case CatTransaction:
		return g.transaction(g.rng.Intn(4))
	case CatCheckoutFail:
		return g.checkoutFail()
	case CatRelease:
		return g.releaseError()
	case CatCron:
		return nil, errSkipIngest
	default:
		return g.errorEvent("sample load-test error", "load")
	}
}

var errSkipIngest = fmt.Errorf("skip ingest")

func newEventID() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

func newTraceID() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

func spanID() string {
	id := strings.ReplaceAll(uuid.NewString(), "-", "")
	if len(id) > 16 {
		return id[:16]
	}
	return id
}

func nowTS() float64 {
	return float64(time.Now().UTC().UnixNano()) / 1e9
}

func (g *Gen) errorEvent(msg, surface string) ([]byte, error) {
	payload := map[string]any{
		"event_id":    newEventID(),
		"platform":    "javascript",
		"environment": "development",
		"release":     "sample@0.1.0",
		"tags": map[string]string{
			"service": "sample",
			"surface": surface,
			"source":  "load-test",
		},
		"exception": map[string]any{
			"values": []map[string]any{{
				"type":  "Error",
				"value": msg,
			}},
		},
	}
	return json.Marshal(payload)
}

func (g *Gen) messageEvent(msg string) ([]byte, error) {
	payload := map[string]any{
		"event_id":    newEventID(),
		"platform":    "javascript",
		"environment": "development",
		"level":       "info",
		"message":     msg,
		"tags": map[string]string{
			"service": "sample",
			"surface": "server-action",
			"source":  "load-test",
		},
	}
	return json.Marshal(payload)
}

func (g *Gen) contextEvent() ([]byte, error) {
	payload := map[string]any{
		"event_id":    newEventID(),
		"platform":    "javascript",
		"environment": "development",
		"user": map[string]string{
			"id":       "demo-user-1",
			"email":    "demo@example.com",
			"username": "demo",
		},
		"breadcrumbs": map[string]any{
			"values": []map[string]any{
				{"category": "auth", "message": "user opened demo context panel", "level": "info"},
				{"category": "ui.click", "message": "clicked Capture with context", "level": "info"},
			},
		},
		"tags": map[string]string{
			"service": "sample",
			"surface": "server-action",
			"feature": "context",
			"source":  "load-test",
		},
		"exception": map[string]any{
			"values": []map[string]any{{
				"type":  "Error",
				"value": "sample exception with user + breadcrumbs (load-test)",
			}},
		},
	}
	return json.Marshal(payload)
}

func (g *Gen) fingerprintMessage() ([]byte, error) {
	payload := map[string]any{
		"event_id":    newEventID(),
		"platform":    "javascript",
		"environment": "development",
		"message":     fmt.Sprintf("fingerprint demo message %c (load-test)", 'A'+g.rng.Intn(2)),
		"fingerprint": []string{"demo-group"},
		"tags": map[string]string{
			"service": "sample",
			"surface": "fingerprint",
			"source":  "load-test",
		},
	}
	return json.Marshal(payload)
}

func (g *Gen) releaseError() ([]byte, error) {
	release := "sample@0.1.0"
	if g.rng.Intn(2) == 1 {
		release = "sample@0.2.0"
	}
	payload := map[string]any{
		"event_id":    newEventID(),
		"platform":    "javascript",
		"environment": "development",
		"release":     release,
		"tags": map[string]string{
			"surface": "release-demo",
			"source":  "load-test",
		},
		"exception": map[string]any{
			"values": []map[string]any{{
				"type":  "Error",
				"value": fmt.Sprintf("sample release regression candidate %s (load-test)", release),
			}},
		},
	}
	return json.Marshal(payload)
}

func (g *Gen) transaction(kind int) ([]byte, error) {
	names := []string{
		"GET /api/mock/users",
		"GET /api/mock/users/1",
		"GET /api/mock/slow",
		"POST /api/mock/checkout",
	}
	name := names[kind%len(names)]
	start := nowTS()
	dur := 0.05 + g.rng.Float64()*0.15
	if strings.Contains(name, "slow") {
		dur = 1.2 + g.rng.Float64()*0.4
	}
	traceID := newTraceID()
	rootSpan := spanID()
	spans := []map[string]any{
		{
			"span_id":         spanID(),
			"parent_span_id":  rootSpan,
			"trace_id":        traceID,
			"op":              "cache",
			"description":     "cache.get users",
			"start_timestamp": start + 0.01,
			"timestamp":       start + 0.03,
		},
		{
			"span_id":         spanID(),
			"parent_span_id":  rootSpan,
			"trace_id":        traceID,
			"op":              "db",
			"description":     "SELECT * FROM users",
			"start_timestamp": start + 0.03,
			"timestamp":       start + dur*0.7,
		},
	}
	payload := map[string]any{
		"type":         "transaction",
		"event_id":     newEventID(),
		"platform":     "javascript",
		"environment":  "development",
		"release":      "sample@0.1.0",
		"transaction":  name,
		"start_timestamp": start,
		"timestamp":    start + dur,
		"contexts": map[string]any{
			"trace": map[string]any{
				"trace_id": traceID,
				"span_id":  rootSpan,
				"op":       "http.server",
				"status":   "ok",
			},
		},
		"spans": spans,
		"tags": map[string]string{
			"service": "sample",
			"surface": "mock-api",
			"source":  "load-test",
		},
	}
	return json.Marshal(payload)
}

func (g *Gen) checkoutFail() ([]byte, error) {
	// ~30% linked exception inside transaction path; here always emit error linked to checkout
	if g.rng.Float64() < 0.3 {
		return g.errorEvent("sample checkout payment declined (load-test)", "mock-api")
	}
	return g.transaction(3)
}
