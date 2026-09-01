package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSeedDemoProjectOnce(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()

	first, err := s.SeedDemoProject(ctx, "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	if first == nil || first.PublicKey != DemoPublicKey {
		t.Fatalf("expected demo public key, got %+v", first)
	}
	if first.DSN != "http://a1b2c3d4e5f6789012345678abcdef01@localhost:8080/1" {
		t.Fatalf("dsn=%s", first.DSN)
	}

	second, err := s.SeedDemoProject(ctx, "http://localhost:8080")
	if err != nil {
		t.Fatal(err)
	}
	if second != nil {
		t.Fatal("second seed should be a no-op")
	}
}

func TestIgnoredIssueDoesNotRegress(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()

	if _, err := s.CreateProject(ctx, "p", "p", "http://localhost:8080", nil); err != nil {
		t.Fatal(err)
	}

	first, err := s.UpsertEvent(ctx, UpsertEventInput{
		EventID:     "e1",
		ProjectID:   1,
		Fingerprint: "fp",
		Title:       "Boom",
		Timestamp:   time.Now().UTC(),
		RawPath:     "x",
		PayloadJSON: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !first.IsNew {
		t.Fatal("expected new issue")
	}
	if err := s.UpdateIssueStatus(ctx, first.IssueID, "ignored"); err != nil {
		t.Fatal(err)
	}

	again, err := s.UpsertEvent(ctx, UpsertEventInput{
		EventID:     "e2",
		ProjectID:   1,
		Fingerprint: "fp",
		Title:       "Boom",
		Timestamp:   time.Now().UTC(),
		RawPath:     "x",
		PayloadJSON: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !again.Ignored || again.Regressed {
		t.Fatalf("ignored=%v regressed=%v", again.Ignored, again.Regressed)
	}
	iss, err := s.GetIssue(ctx, first.IssueID)
	if err != nil {
		t.Fatal(err)
	}
	if iss.Status != "ignored" {
		t.Fatalf("status=%s", iss.Status)
	}
}

func TestPurgeBeforeRemovesOldEvents(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if _, err := s.CreateProject(ctx, "p", "p", "http://localhost:8080", nil); err != nil {
		t.Fatal(err)
	}

	oldFile := filepath.Join(dir, "old.json")
	if err := os.WriteFile(oldFile, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-48 * time.Hour)
	fresh := time.Now().UTC()
	if _, err := s.UpsertEvent(ctx, UpsertEventInput{
		EventID: "old", ProjectID: 1, Fingerprint: "a", Title: "old",
		Timestamp: old, RawPath: oldFile, PayloadJSON: "{}",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertEvent(ctx, UpsertEventInput{
		EventID: "new", ProjectID: 1, Fingerprint: "b", Title: "new",
		Timestamp: fresh, RawPath: "keep", PayloadJSON: "{}",
	}); err != nil {
		t.Fatal(err)
	}

	res, err := s.PurgeBefore(ctx, time.Now().UTC().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if res.Events != 1 {
		t.Fatalf("purged events=%d", res.Events)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("expected old file removed: %v", err)
	}
	ev, err := s.GetEvent(ctx, "new")
	if err != nil || ev == nil {
		t.Fatalf("fresh event missing: %v", err)
	}
}

func TestHasRecentDelivery(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	if _, err := s.CreateProject(ctx, "p", "p", "http://localhost:8080", nil); err != nil {
		t.Fatal(err)
	}
	rule, err := s.CreateAlertRule(ctx, AlertRule{
		ProjectID: 1, Name: "n", Trigger: "error_volume", Channel: "webhook",
		Target: "https://example.com/hook", Enabled: true, WindowSec: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	recent, err := s.HasRecentDelivery(ctx, rule.ID, 0, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if recent {
		t.Fatal("expected no deliveries")
	}
	if err := s.RecordAlertDelivery(ctx, rule.ID, 0, "ok", ""); err != nil {
		t.Fatal(err)
	}
	recent, err = s.HasRecentDelivery(ctx, rule.ID, 0, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !recent {
		t.Fatal("expected recent delivery")
	}
}
