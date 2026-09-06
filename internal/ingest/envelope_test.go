package ingest

import (
	"encoding/json"
	"testing"
)

func TestParseEnvelopeLogOnly(t *testing.T) {
	body := buildEnvelope(
		map[string]any{
			"event_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"trace": map[string]any{
				"environment": "development",
				"release":     "sample@0.1.0",
			},
		},
		envelopePart{
			header: map[string]any{
				"type":         "log",
				"item_count":   2,
				"content_type": "application/vnd.sentry.items.log+json",
			},
			payload: map[string]any{
				"items": []any{
					map[string]any{"timestamp": 1544719860.0, "level": "info", "body": "hello", "trace_id": "5b8efff798038103d269b633813fc60c"},
					map[string]any{"timestamp": 1544719861.0, "level": "warn", "body": "careful", "trace_id": "5b8efff798038103d269b633813fc60c"},
				},
			},
		},
	)
	parsed, err := parseEnvelope(body)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.EventID != "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("event_id=%q", parsed.EventID)
	}
	if parsed.Environment != "development" || parsed.Release != "sample@0.1.0" {
		t.Fatalf("env=%q release=%q", parsed.Environment, parsed.Release)
	}
	kinds := ingestibleKinds(parsed)
	if len(kinds) != 1 || kinds[0] != "log" {
		t.Fatalf("kinds=%v", kinds)
	}
	var batch map[string]any
	if err := json.Unmarshal(parsed.Items[0].Payload, &batch); err != nil {
		t.Fatal(err)
	}
	items, _ := batch["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items=%d", len(items))
	}
}

func TestParseEnvelopeMixedEventAndLog(t *testing.T) {
	body := buildEnvelope(
		map[string]any{"event_id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
		envelopePart{
			header:  map[string]any{"type": "event"},
			payload: map[string]any{"event_id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "message": "boom"},
		},
		envelopePart{
			header: map[string]any{"type": "log", "item_count": 1, "content_type": "application/vnd.sentry.items.log+json"},
			payload: map[string]any{
				"items": []any{
					map[string]any{"timestamp": 1.0, "level": "error", "body": "logged", "trace_id": "cccccccccccccccccccccccccccccccc"},
				},
			},
		},
	)
	parsed, err := parseEnvelope(body)
	if err != nil {
		t.Fatal(err)
	}
	kinds := ingestibleKinds(parsed)
	if len(kinds) != 2 || kinds[0] != "event" || kinds[1] != "log" {
		t.Fatalf("kinds=%v", kinds)
	}
}

func TestParseEnvelopeSessionOnly(t *testing.T) {
	body := buildEnvelope(
		map[string]any{},
		envelopePart{
			header:  map[string]any{"type": "session"},
			payload: map[string]any{"status": "ok", "sid": "x"},
		},
	)
	parsed, err := parseEnvelope(body)
	if err != nil {
		t.Fatal(err)
	}
	if kinds := ingestibleKinds(parsed); len(kinds) != 0 {
		t.Fatalf("expected skip, kinds=%v", kinds)
	}
}

type envelopePart struct {
	header  map[string]any
	payload any
}

func buildEnvelope(header map[string]any, parts ...envelopePart) []byte {
	var b []byte
	appendJSON := func(v any) {
		raw, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		b = append(b, raw...)
		b = append(b, '\n')
	}
	appendJSON(header)
	for _, p := range parts {
		payload, err := json.Marshal(p.payload)
		if err != nil {
			panic(err)
		}
		hdr := map[string]any{}
		for k, v := range p.header {
			hdr[k] = v
		}
		hdr["length"] = len(payload)
		appendJSON(hdr)
		b = append(b, payload...)
		b = append(b, '\n')
	}
	return b
}

func ingestibleKinds(p *parsedEnvelope) []string {
	var out []string
	for _, it := range p.Items {
		switch it.Type {
		case "event", "transaction", "log":
			out = append(out, it.Type)
		}
	}
	return out
}
