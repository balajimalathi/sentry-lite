package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestDashboardStats(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Hour)
	from := now.Add(-3 * time.Hour)
	to := now.Add(time.Hour)

	if err := s.DB.Create(&Organization{Slug: "org", Name: "Org"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Create(&ProjectRow{
		OrganizationID: 1,
		Slug:           "demo",
		Name:           "Demo",
		AllowedOrigins: "[]",
	}).Error; err != nil {
		t.Fatal(err)
	}

	ts := now.Format(time.RFC3339Nano)
	older := now.Add(-2 * time.Hour).Format(time.RFC3339Nano)
	issues := []IssueRow{
		{ProjectID: 1, Fingerprint: "a", Title: "Hot", Status: "open", Level: "error", Count: 50, FirstSeen: older, LastSeen: ts, Regressed: 1},
		{ProjectID: 1, Fingerprint: "b", Title: "Cold", Status: "open", Level: "error", Count: 5, FirstSeen: older, LastSeen: ts},
		{ProjectID: 1, Fingerprint: "c", Title: "Done", Status: "resolved", Level: "error", Count: 3, FirstSeen: older, LastSeen: older},
	}
	for i := range issues {
		if err := s.DB.Create(&issues[i]).Error; err != nil {
			t.Fatal(err)
		}
	}

	events := []EventRow{
		{EventID: "e1", IssueID: 1, ProjectID: 1, Timestamp: now.Add(-90 * time.Minute).Format(time.RFC3339Nano), RawPath: "x", PayloadJSON: "{}"},
		{EventID: "e2", IssueID: 1, ProjectID: 1, Timestamp: now.Add(-30 * time.Minute).Format(time.RFC3339Nano), RawPath: "x", PayloadJSON: "{}"},
		{EventID: "e3", IssueID: 2, ProjectID: 1, Timestamp: now.Add(-30 * time.Minute).Format(time.RFC3339Nano), RawPath: "x", PayloadJSON: "{}"},
	}
	for i := range events {
		if err := s.DB.Create(&events[i]).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := s.DB.Create(&CronMonitorRow{
		ProjectID:   1,
		Slug:        "job",
		Name:        "Job",
		ScheduleSec: 60,
		GraceSec:    10,
		Status:      "missed",
		Token:       "tok",
	}).Error; err != nil {
		t.Fatal(err)
	}

	stats, err := s.DashboardStats(ctx, DashboardStatsFilter{
		ProjectID: 1,
		From:      from,
		To:        to,
		Interval:  time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if stats.Unresolved != 2 {
		t.Fatalf("unresolved=%d want 2", stats.Unresolved)
	}
	if stats.Events != 3 {
		t.Fatalf("events=%d want 3", stats.Events)
	}
	if stats.Regressions != 1 {
		t.Fatalf("regressions=%d want 1", stats.Regressions)
	}
	if stats.CronsUnhealthy != 1 {
		t.Fatalf("crons_unhealthy=%d want 1", stats.CronsUnhealthy)
	}
	if stats.ByStatus["open"] != 2 || stats.ByStatus["resolved"] != 1 {
		t.Fatalf("by_status=%v", stats.ByStatus)
	}
	if len(stats.TopIssues) == 0 || stats.TopIssues[0].Title != "Hot" {
		t.Fatalf("top_issues=%v", stats.TopIssues)
	}
	if len(stats.Series) == 0 {
		t.Fatal("expected non-empty series")
	}
	var seriesSum int64
	for _, b := range stats.Series {
		seriesSum += b.Events
	}
	if seriesSum != 3 {
		t.Fatalf("series sum=%d want 3", seriesSum)
	}
}
