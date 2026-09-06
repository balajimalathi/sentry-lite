package process

import (
	"encoding/json"
	"strings"
	"time"
)

const (
	maxLogItems = 100
	maxLogBody  = 8 * 1024
)

type NormalizedLog struct {
	Timestamp      time.Time
	Level          string
	Body           string
	TraceID        string
	SpanID         string
	SeverityNumber int
	Environment    string
	Release        string
	Origin         string
	PayloadJSON    string
}

func NormalizeLogs(payload []byte, fallbackEnv, fallbackRelease string) ([]NormalizedLog, error) {
	var raw struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, err
	}
	items := raw.Items
	if len(items) > maxLogItems {
		items = items[:maxLogItems]
	}
	out := make([]NormalizedLog, 0, len(items))
	for _, item := range items {
		var entry map[string]any
		if err := json.Unmarshal(item, &entry); err != nil {
			continue
		}
		attrs, _ := entry["attributes"].(map[string]any)
		env := attrString(attrs, "sentry.environment")
		if env == "" {
			env = fallbackEnv
		}
		release := attrString(attrs, "sentry.release")
		if release == "" {
			release = fallbackRelease
		}
		body := asString(entry["body"])
		n := NormalizedLog{
			Timestamp:      parseTimestamp(entry["timestamp"]),
			Level:          normalizeLogLevel(asString(entry["level"])),
			Body:           truncate(body, maxLogBody),
			TraceID:        strings.ReplaceAll(asString(entry["trace_id"]), "-", ""),
			SpanID:         asString(entry["span_id"]),
			SeverityNumber: asInt(entry["severity_number"]),
			Environment:    env,
			Release:        release,
			Origin:         attrString(attrs, "sentry.origin"),
			PayloadJSON:    string(item),
		}
		out = append(out, n)
	}
	return out, nil
}

func normalizeLogLevel(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "trace", "debug", "info", "warn", "error", "fatal":
		return s
	case "warning":
		return "warn"
	case "log", "":
		return "info"
	default:
		return "info"
	}
}

func attrString(attrs map[string]any, key string) string {
	if attrs == nil {
		return ""
	}
	v, ok := attrs[key]
	if !ok {
		return ""
	}
	if m, ok := v.(map[string]any); ok {
		if val, exists := m["value"]; exists {
			return asString(val)
		}
	}
	return asString(v)
}
