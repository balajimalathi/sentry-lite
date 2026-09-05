package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/skndan/sentry-lite/internal/fingerprint"
)

func TestCreateProjectUsesNewGroupingConfig(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	created, err := s.CreateProject(ctx, "p", "p", "http://localhost:8080", nil)
	if err != nil {
		t.Fatal(err)
	}
	if created.Project.GroupingConfig != fingerprint.Config20260901 {
		t.Fatalf("grouping_config=%s", created.Project.GroupingConfig)
	}
}

func TestHashAliasLookup(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if _, err := s.CreateProject(ctx, "p", "p", "http://localhost:8080", nil); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().UTC()
	first, err := s.UpsertEvent(ctx, UpsertEventInput{
		EventID: "e1", ProjectID: 1, Fingerprint: "app-hash", Title: "A",
		Hashes: []IssueHashInput{{Hash: "app-hash", Variant: "app"}, {Hash: "sys-hash", Variant: "system"}},
		GroupingHash: "app-hash", GroupingVariant: "app",
		Timestamp: ts, RawPath: "x", PayloadJSON: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.UpsertEvent(ctx, UpsertEventInput{
		EventID: "e2", ProjectID: 1, Fingerprint: "sys-hash", Title: "B",
		Hashes: []IssueHashInput{{Hash: "sys-hash", Variant: "system"}},
		GroupingHash: "sys-hash", GroupingVariant: "system",
		Timestamp: ts.Add(time.Second), RawPath: "x", PayloadJSON: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.IsNew || second.IssueID != first.IssueID {
		t.Fatalf("expected alias hit, first=%+v second=%+v", first, second)
	}
	iss, err := s.GetIssue(ctx, first.IssueID)
	if err != nil {
		t.Fatal(err)
	}
	if iss.Count != 2 {
		t.Fatalf("count=%d", iss.Count)
	}
}

func TestNewConfigSameStackDifferentMessages(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if _, err := s.CreateProject(ctx, "p", "p", "http://localhost:8080", nil); err != nil {
		t.Fatal(err)
	}
	frames := []fingerprint.Frame{{Filename: "app.js", Function: "handle", InApp: true}}
	a := fingerprint.Group(fingerprint.Input{Config: fingerprint.Config20260901, ExceptionType: "TypeError", Message: "one", Frames: frames})
	b := fingerprint.Group(fingerprint.Input{Config: fingerprint.Config20260901, ExceptionType: "TypeError", Message: "two", Frames: frames})
	if a.Primary != b.Primary {
		t.Fatal("expected same grouping hash")
	}
	ts := time.Now().UTC()
	first, err := s.UpsertEvent(ctx, upsertFromGroup("e1", a, ts))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.UpsertEvent(ctx, upsertFromGroup("e2", b, ts.Add(time.Second)))
	if err != nil {
		t.Fatal(err)
	}
	if second.IssueID != first.IssueID || second.IsNew {
		t.Fatalf("expected same issue, first=%+v second=%+v", first, second)
	}
}

func TestMergeAndUnmerge(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if _, err := s.CreateProject(ctx, "p", "p", "http://localhost:8080", nil); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().UTC()
	a, err := s.UpsertEvent(ctx, UpsertEventInput{
		EventID: "e1", ProjectID: 1, Fingerprint: "fp-a", Title: "A",
		Hashes: []IssueHashInput{{Hash: "fp-a", Variant: "v1"}},
		GroupingHash: "fp-a", GroupingVariant: "v1",
		Timestamp: ts, RawPath: "x", PayloadJSON: "{}", Release: "1.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.UpsertEvent(ctx, UpsertEventInput{
		EventID: "e2", ProjectID: 1, Fingerprint: "fp-b", Title: "B",
		Hashes: []IssueHashInput{{Hash: "fp-b", Variant: "v1"}},
		GroupingHash: "fp-b", GroupingVariant: "v1",
		Timestamp: ts.Add(time.Minute), RawPath: "x", PayloadJSON: "{}", Release: "1.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	merged, err := s.MergeIssues(ctx, a.IssueID, []int64{b.IssueID})
	if err != nil {
		t.Fatal(err)
	}
	if merged.Count != 2 {
		t.Fatalf("merged count=%d", merged.Count)
	}
	if merged.LastRelease == nil || *merged.LastRelease != "1.1" {
		t.Fatalf("last_release=%v", merged.LastRelease)
	}

	list, err := s.ListIssues(ctx, IssueListFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != a.IssueID {
		t.Fatalf("list=%+v", list)
	}

	src, err := s.GetIssue(ctx, b.IssueID)
	if err != nil {
		t.Fatal(err)
	}
	if src.Status != "merged" || src.MergedInto == nil || *src.MergedInto != a.IssueID {
		t.Fatalf("source=%+v", src)
	}

	follow, err := s.UpsertEvent(ctx, UpsertEventInput{
		EventID: "e3", ProjectID: 1, Fingerprint: "fp-b", Title: "B again",
		Hashes: []IssueHashInput{{Hash: "fp-b", Variant: "v1"}},
		GroupingHash: "fp-b", GroupingVariant: "v1",
		Timestamp: ts.Add(2 * time.Minute), RawPath: "x", PayloadJSON: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if follow.IssueID != a.IssueID {
		t.Fatalf("future event should follow merge alias, got %d", follow.IssueID)
	}

	created, err := s.UnmergeIssueHashes(ctx, a.IssueID, []string{"fp-b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 {
		t.Fatalf("unmerged=%+v", created)
	}
	dest, err := s.GetIssue(ctx, a.IssueID)
	if err != nil {
		t.Fatal(err)
	}
	if dest.Count != 1 {
		t.Fatalf("dest count=%d", dest.Count)
	}
	if created[0].Count != 2 {
		t.Fatalf("split count=%d (events with fp-b)", created[0].Count)
	}

	list, err = s.ListIssues(ctx, IssueListFilter{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("after unmerge list=%d", len(list))
	}
}

func TestUnmergeRejectsSingleHash(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if _, err := s.CreateProject(ctx, "p", "p", "http://localhost:8080", nil); err != nil {
		t.Fatal(err)
	}
	first, err := s.UpsertEvent(ctx, UpsertEventInput{
		EventID: "e1", ProjectID: 1, Fingerprint: "only", Title: "A",
		Hashes: []IssueHashInput{{Hash: "only", Variant: "v1"}},
		GroupingHash: "only", GroupingVariant: "v1",
		Timestamp: time.Now().UTC(), RawPath: "x", PayloadJSON: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.UnmergeIssueHashes(ctx, first.IssueID, []string{"only"}); err == nil {
		t.Fatal("expected error")
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func upsertFromGroup(eventID string, g fingerprint.Result, ts time.Time) UpsertEventInput {
	hashes := make([]IssueHashInput, 0, len(g.Variants))
	for _, v := range g.Variants {
		hashes = append(hashes, IssueHashInput{Hash: v.Hash, Variant: v.Kind})
	}
	return UpsertEventInput{
		EventID: eventID, ProjectID: 1, Fingerprint: g.Primary, Title: "err",
		Hashes: hashes, GroupingHash: g.Primary, GroupingVariant: g.Variant,
		Timestamp: ts, RawPath: "x", PayloadJSON: "{}",
	}
}
