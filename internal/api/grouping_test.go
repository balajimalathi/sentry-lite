package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/skndan/sentry-lite/internal/fingerprint"
	"github.com/skndan/sentry-lite/internal/store"
)

func TestGroupingAPI(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	ctx := t.Context()
	if _, err := st.CreateProject(ctx, "p", "p", "http://localhost:8080", nil); err != nil {
		t.Fatal(err)
	}

	h := &Handler{Store: st, PublicURL: "http://localhost:8080"}
	r := chi.NewRouter()
	h.Routes(r)

	rules := "error.type:TimeoutError -> timeout\n"
	body, _ := json.Marshal(map[string]any{
		"grouping_config":   fingerprint.Config20260901,
		"fingerprint_rules": rules,
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/internal/projects/1", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("patch project %d %s", rec.Code, rec.Body.String())
	}

	bad := httptest.NewRequest(http.MethodPatch, "/api/internal/projects/1", bytes.NewReader([]byte(`{"fingerprint_rules":"no-arrow"}`)))
	badRec := httptest.NewRecorder()
	r.ServeHTTP(badRec, bad)
	if badRec.Code != 400 {
		t.Fatalf("expected 400 for bad rules, got %d %s", badRec.Code, badRec.Body.String())
	}

	ts := time.Now().UTC()
	a, err := st.UpsertEvent(ctx, store.UpsertEventInput{
		EventID: "e1", ProjectID: 1, Fingerprint: "fp-a", Title: "A",
		Hashes:       []store.IssueHashInput{{Hash: "fp-a", Variant: "v1"}},
		GroupingHash: "fp-a", GroupingVariant: "v1",
		Timestamp: ts, RawPath: "x", PayloadJSON: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := st.UpsertEvent(ctx, store.UpsertEventInput{
		EventID: "e2", ProjectID: 1, Fingerprint: "fp-b", Title: "B",
		Hashes:       []store.IssueHashInput{{Hash: "fp-b", Variant: "v1"}},
		GroupingHash: "fp-b", GroupingVariant: "v1",
		Timestamp: ts, RawPath: "x", PayloadJSON: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}

	mergeBody, _ := json.Marshal(map[string]any{"ids": []int64{b.IssueID}})
	mergeReq := httptest.NewRequest(http.MethodPost, "/api/internal/issues/"+itoa(a.IssueID)+"/merge", bytes.NewReader(mergeBody))
	mergeRec := httptest.NewRecorder()
	r.ServeHTTP(mergeRec, mergeReq)
	if mergeRec.Code != 200 {
		t.Fatalf("merge %d %s", mergeRec.Code, mergeRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/internal/issues/"+itoa(a.IssueID), nil)
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, getReq)
	if getRec.Code != 200 {
		t.Fatalf("get issue %d %s", getRec.Code, getRec.Body.String())
	}
	var got struct {
		Hashes []store.IssueHash `json:"hashes"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Hashes) < 2 {
		t.Fatalf("hashes=%+v", got.Hashes)
	}

	unmergeBody, _ := json.Marshal(map[string]any{"hashes": []string{"fp-b"}})
	unmergeReq := httptest.NewRequest(http.MethodPost, "/api/internal/issues/"+itoa(a.IssueID)+"/unmerge", bytes.NewReader(unmergeBody))
	unmergeRec := httptest.NewRecorder()
	r.ServeHTTP(unmergeRec, unmergeReq)
	if unmergeRec.Code != 200 {
		t.Fatalf("unmerge %d %s", unmergeRec.Code, unmergeRec.Body.String())
	}
}

func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}
