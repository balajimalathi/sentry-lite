package process

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/skndan/sentry-lite/internal/ingest"
	"github.com/skndan/sentry-lite/internal/store"
)

func TestNormalizeLogsFlattensAttributesAndLevels(t *testing.T) {
	payload := []byte(`{
		"items": [
			{
				"timestamp": 1544719860.0,
				"trace_id": "5b8efff798038103d269b633813fc60c",
				"level": "log",
				"body": "User John has logged in!",
				"attributes": {
					"sentry.environment": {"value": "production", "type": "string"},
					"sentry.release": {"value": "app@1.0.0", "type": "string"},
					"sentry.origin": {"value": "auto.log.console", "type": "string"},
					"sentry.message.parameter.0": {"value": "John", "type": "string"}
				}
			},
			{
				"timestamp": 1544719861.0,
				"level": "warning",
				"body": "careful",
				"trace_id": "5b8efff798038103d269b633813fc60c"
			}
		]
	}`)
	logs, err := NormalizeLogs(payload, "fallback-env", "fallback-rel")
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 {
		t.Fatalf("len=%d", len(logs))
	}
	if logs[0].Level != "info" {
		t.Fatalf("console log level=%q", logs[0].Level)
	}
	if logs[0].Environment != "production" || logs[0].Release != "app@1.0.0" {
		t.Fatalf("env/release from attrs: %+v", logs[0])
	}
	if logs[0].Origin != "auto.log.console" {
		t.Fatalf("origin=%q", logs[0].Origin)
	}
	if logs[1].Level != "warn" {
		t.Fatalf("warning mapped to %q", logs[1].Level)
	}
	if logs[1].Environment != "fallback-env" {
		t.Fatalf("fallback env=%q", logs[1].Environment)
	}
}

func TestHandleLogDoesNotCreateIssue(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	if _, err := s.CreateProject(ctx, "p", "p", "http://localhost:8080", nil); err != nil {
		t.Fatal(err)
	}

	payload := json.RawMessage(`{"items":[{"timestamp":1544719860.0,"level":"error","body":"boom","trace_id":"abc"}]}`)
	msg, err := json.Marshal(ingest.IngestMessage{
		ProjectID: 1,
		EventID:   "logbatch1",
		Kind:      "log",
		Payload:   payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	w := &Worker{Store: s, DataDir: t.TempDir()}
	if err := w.handle(ctx, msg); err != nil {
		t.Fatal(err)
	}

	issues, err := s.ListIssues(ctx, store.IssueListFilter{ProjectID: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected no issues, got %d", len(issues))
	}
	logs, err := s.ListLogs(ctx, store.LogListFilter{ProjectID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].Body != "boom" || logs[0].Level != "error" {
		t.Fatalf("logs=%+v", logs)
	}
}
