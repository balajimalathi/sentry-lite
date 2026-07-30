package store

import (
	"context"
	"database/sql"
	"math"
	"sort"
	"time"
)

const MaxSpansPerTransaction = 50
const DefaultPerfWindowSec = 86400 // 24h

type Transaction struct {
	ID          int64    `json:"id"`
	EventID     string   `json:"event_id"`
	ProjectID   int64    `json:"project_id"`
	Name        string   `json:"name"`
	Op          string   `json:"op"`
	TraceID     string   `json:"trace_id"`
	SpanID      string   `json:"span_id"`
	DurationMS  float64  `json:"duration_ms"`
	Status      string   `json:"status"`
	Environment *string  `json:"environment"`
	Release     *string  `json:"release"`
	Timestamp   string   `json:"timestamp"`
	RawPath     string   `json:"raw_path,omitempty"`
	PayloadJSON string   `json:"payload_json,omitempty"`
	Spans       []Span   `json:"spans,omitempty"`
}

type Span struct {
	ID                  int64   `json:"id,omitempty"`
	TransactionEventID  string  `json:"transaction_event_id,omitempty"`
	SpanID              string  `json:"span_id"`
	ParentSpanID        string  `json:"parent_span_id"`
	Op                  string  `json:"op"`
	Description         string  `json:"description"`
	DurationMS          float64 `json:"duration_ms"`
	Status              string  `json:"status"`
}

type TransactionSummary struct {
	Name       string  `json:"name"`
	Count      int64   `json:"count"`
	P95MS      float64 `json:"p95_ms"`
	P99MS      float64 `json:"p99_ms"`
	ProjectID  int64   `json:"project_id"`
}

type TraceIssue struct {
	IssueID int64  `json:"issue_id"`
	Title   string `json:"title"`
	EventID string `json:"event_id"`
}

type TraceDetail struct {
	TraceID      string        `json:"trace_id"`
	Transactions []Transaction `json:"transactions"`
	Issues       []TraceIssue  `json:"issues"`
}

type InsertTransactionInput struct {
	EventID     string
	ProjectID   int64
	Name        string
	Op          string
	TraceID     string
	SpanID      string
	DurationMS  float64
	Status      string
	Environment string
	Release     string
	Timestamp   time.Time
	RawPath     string
	PayloadJSON string
	Spans       []Span
}

func (s *Store) InsertTransaction(ctx context.Context, in InsertTransactionInput) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ts := in.Timestamp.UTC().Format(time.RFC3339Nano)
	_, err = tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO transactions (
			event_id, project_id, name, op, trace_id, span_id, duration_ms, status,
			environment, release, timestamp, raw_path, payload_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, in.EventID, in.ProjectID, in.Name, in.Op, in.TraceID, in.SpanID, in.DurationMS, in.Status,
		nullOrNil(in.Environment), nullOrNil(in.Release), ts, in.RawPath, in.PayloadJSON)
	if err != nil {
		return err
	}

	spans := in.Spans
	if len(spans) > MaxSpansPerTransaction {
		spans = spans[:MaxSpansPerTransaction]
	}
	for _, sp := range spans {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO spans (transaction_event_id, span_id, parent_span_id, op, description, duration_ms, status)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, in.EventID, sp.SpanID, sp.ParentSpanID, sp.Op, sp.Description, sp.DurationMS, sp.Status)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ListTransactionSummaries(ctx context.Context, projectID int64, from time.Time) ([]TransactionSummary, error) {
	if projectID <= 0 {
		return nil, nil
	}
	return s.computeTransactionSummaries(ctx, projectID, from)
}

func (s *Store) computeTransactionSummaries(ctx context.Context, projectID int64, from time.Time) ([]TransactionSummary, error) {
	fromStr := from.UTC().Format(time.RFC3339Nano)
	rows, err := s.DB.QueryContext(ctx, `
		SELECT name, duration_ms FROM transactions
		WHERE project_id = ? AND timestamp >= ?
		ORDER BY name
	`, projectID, fromStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byName := map[string][]float64{}
	var order []string
	for rows.Next() {
		var name string
		var dur float64
		if err := rows.Scan(&name, &dur); err != nil {
			return nil, err
		}
		if _, ok := byName[name]; !ok {
			order = append(order, name)
		}
		byName[name] = append(byName[name], dur)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]TransactionSummary, 0, len(order))
	for _, name := range order {
		durs := byName[name]
		sort.Float64s(durs)
		out = append(out, TransactionSummary{
			Name:      name,
			Count:     int64(len(durs)),
			P95MS:     percentile(durs, 0.95),
			P99MS:     percentile(durs, 0.99),
			ProjectID: projectID,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Name < out[j].Name
		}
		return out[i].Count > out[j].Count
	})
	return out, nil
}

func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	idx := p * float64(n-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	w := idx - float64(lo)
	return sorted[lo]*(1-w) + sorted[hi]*w
}

func (s *Store) GetTransactionDetail(ctx context.Context, projectID int64, name string, limit int) ([]Transaction, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, event_id, project_id, name, op, trace_id, span_id, duration_ms, status,
		       environment, release, timestamp, raw_path, payload_json
		FROM transactions
		WHERE project_id = ? AND name = ?
		ORDER BY timestamp DESC LIMIT ?
	`, projectID, name, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Transaction
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		spans, err := s.ListSpans(ctx, out[i].EventID)
		if err != nil {
			return nil, err
		}
		out[i].Spans = spans
	}
	return out, nil
}

func (s *Store) ListSpans(ctx context.Context, transactionEventID string) ([]Span, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, transaction_event_id, span_id, parent_span_id, op, description, duration_ms, status
		FROM spans WHERE transaction_event_id = ? ORDER BY id ASC
	`, transactionEventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Span
	for rows.Next() {
		var sp Span
		if err := rows.Scan(&sp.ID, &sp.TransactionEventID, &sp.SpanID, &sp.ParentSpanID, &sp.Op, &sp.Description, &sp.DurationMS, &sp.Status); err != nil {
			return nil, err
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}

func (s *Store) GetTrace(ctx context.Context, traceID string) (*TraceDetail, error) {
	if traceID == "" {
		return nil, nil
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, event_id, project_id, name, op, trace_id, span_id, duration_ms, status,
		       environment, release, timestamp, raw_path, payload_json
		FROM transactions WHERE trace_id = ? ORDER BY timestamp ASC
	`, traceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var txs []Transaction
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		txs = append(txs, *t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range txs {
		spans, err := s.ListSpans(ctx, txs[i].EventID)
		if err != nil {
			return nil, err
		}
		txs[i].Spans = spans
	}

	issueRows, err := s.DB.QueryContext(ctx, `
		SELECT e.issue_id, i.title, e.event_id
		FROM events e
		JOIN issues i ON i.id = e.issue_id
		WHERE e.trace_id = ?
		ORDER BY e.timestamp DESC
	`, traceID)
	if err != nil {
		return nil, err
	}
	defer issueRows.Close()
	var issues []TraceIssue
	seen := map[int64]bool{}
	for issueRows.Next() {
		var ti TraceIssue
		if err := issueRows.Scan(&ti.IssueID, &ti.Title, &ti.EventID); err != nil {
			return nil, err
		}
		if seen[ti.IssueID] {
			continue
		}
		seen[ti.IssueID] = true
		issues = append(issues, ti)
	}
	if issues == nil {
		issues = []TraceIssue{}
	}
	if txs == nil {
		txs = []Transaction{}
	}
	return &TraceDetail{TraceID: traceID, Transactions: txs, Issues: issues}, issueRows.Err()
}

func (s *Store) RecomputeTransactionStats(ctx context.Context, windowSec int) error {
	if windowSec <= 0 {
		windowSec = DefaultPerfWindowSec
	}
	from := time.Now().UTC().Add(-time.Duration(windowSec) * time.Second)
	windowStart := from.Format(time.RFC3339Nano)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	rows, err := s.DB.QueryContext(ctx, `
		SELECT DISTINCT project_id FROM transactions WHERE timestamp >= ?
	`, windowStart)
	if err != nil {
		return err
	}
	defer rows.Close()
	var projectIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		projectIDs = append(projectIDs, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, pid := range projectIDs {
		summaries, err := s.computeTransactionSummaries(ctx, pid, from)
		if err != nil {
			return err
		}
		for _, sum := range summaries {
			_, err = s.DB.ExecContext(ctx, `
				INSERT INTO transaction_stats (project_id, name, window_start, window_sec, count, p95_ms, p99_ms, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT(project_id, name, window_start, window_sec) DO UPDATE SET
					count = excluded.count,
					p95_ms = excluded.p95_ms,
					p99_ms = excluded.p99_ms,
					updated_at = excluded.updated_at
			`, pid, sum.Name, windowStart, windowSec, sum.Count, sum.P95MS, sum.P99MS, now)
			if err != nil {
				return err
			}
		}
	}
	// Prune old rollup rows (keep last 2 windows worth of starts loosely)
	cutoff := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339Nano)
	_, _ = s.DB.ExecContext(ctx, `DELETE FROM transaction_stats WHERE window_start < ?`, cutoff)
	return nil
}

func scanTransaction(row scannable) (*Transaction, error) {
	var t Transaction
	var env, rel sql.NullString
	err := row.Scan(
		&t.ID, &t.EventID, &t.ProjectID, &t.Name, &t.Op, &t.TraceID, &t.SpanID, &t.DurationMS, &t.Status,
		&env, &rel, &t.Timestamp, &t.RawPath, &t.PayloadJSON,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	t.Environment = nullStr(env)
	t.Release = nullStr(rel)
	return &t, nil
}
