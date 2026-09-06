package store

import (
	"context"
	"testing"
	"time"
)

func TestInsertAndListLogs(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if _, err := s.CreateProject(ctx, "p", "p", "http://localhost:8080", nil); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	err := s.InsertLogs(ctx, []LogRow{
		{
			ProjectID:   1,
			Timestamp:   now.Format(time.RFC3339Nano),
			Level:       "info",
			Body:        "hello",
			TraceID:     "trace-a",
			Origin:      "auto.log.console",
			PayloadJSON: `{"body":"hello","attributes":{"sentry.origin":{"value":"auto.log.console","type":"string"}}}`,
		},
		{
			ProjectID:   1,
			Timestamp:   now.Add(time.Second).Format(time.RFC3339Nano),
			Level:       "error",
			Body:        "boom",
			TraceID:     "trace-a",
			PayloadJSON: `{"body":"boom"}`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	all, err := s.ListLogs(ctx, LogListFilter{ProjectID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("len=%d", len(all))
	}
	if all[0].Body != "boom" {
		t.Fatalf("expected newest first, got %q", all[0].Body)
	}
	if all[1].Attributes["sentry.origin"] != "auto.log.console" {
		t.Fatalf("attrs=%v", all[1].Attributes)
	}

	errors, err := s.ListLogs(ctx, LogListFilter{ProjectID: 1, Level: "error"})
	if err != nil {
		t.Fatal(err)
	}
	if len(errors) != 1 || errors[0].Body != "boom" {
		t.Fatalf("level filter=%+v", errors)
	}

	hello, err := s.ListLogs(ctx, LogListFilter{ProjectID: 1, Q: "hel"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hello) != 1 || hello[0].Body != "hello" {
		t.Fatalf("q filter=%+v", hello)
	}

	traceLogs, err := s.ListLogsForTrace(ctx, "trace-a", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(traceLogs) != 2 {
		t.Fatalf("trace logs=%d", len(traceLogs))
	}

	cutoff := now.Add(30 * time.Second)
	res, err := s.PurgeBefore(ctx, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if res.Logs != 2 {
		t.Fatalf("purged logs=%d", res.Logs)
	}
	left, err := s.ListLogs(ctx, LogListFilter{ProjectID: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Fatalf("expected empty after purge, got %d", len(left))
	}
}

func TestGetTraceIncludesLogsWithoutTransactions(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if _, err := s.CreateProject(ctx, "p", "p", "http://localhost:8080", nil); err != nil {
		t.Fatal(err)
	}
	traceID := "ffffffffffffffffffffffffffffffff"
	if err := s.InsertLogs(ctx, []LogRow{{
		ProjectID:   1,
		Timestamp:   time.Now().UTC().Format(time.RFC3339Nano),
		Level:       "info",
		Body:        "only logs",
		TraceID:     traceID,
		PayloadJSON: `{"body":"only logs"}`,
	}}); err != nil {
		t.Fatal(err)
	}
	detail, err := s.GetTrace(ctx, traceID)
	if err != nil {
		t.Fatal(err)
	}
	if detail == nil {
		t.Fatal("expected trace detail")
	}
	if len(detail.Transactions) != 0 {
		t.Fatalf("txs=%d", len(detail.Transactions))
	}
	if len(detail.Logs) != 1 || detail.Logs[0].Body != "only logs" {
		t.Fatalf("logs=%+v", detail.Logs)
	}
}
