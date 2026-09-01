package store

import (
	"context"
	"math"
	"sort"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const MaxSpansPerTransaction = 50
const DefaultPerfWindowSec = 86400 // 24h

type Transaction struct {
	ID          int64   `json:"id"`
	EventID     string  `json:"event_id"`
	ProjectID   int64   `json:"project_id"`
	Name        string  `json:"name"`
	Op          string  `json:"op"`
	TraceID     string  `json:"trace_id"`
	SpanID      string  `json:"span_id"`
	DurationMS  float64 `json:"duration_ms"`
	Status      string  `json:"status"`
	Environment *string `json:"environment"`
	Release     *string `json:"release"`
	Timestamp   string  `json:"timestamp"`
	RawPath     string  `json:"raw_path,omitempty"`
	PayloadJSON string  `json:"payload_json,omitempty"`
	Spans       []Span  `json:"spans,omitempty"`
}

type Span struct {
	ID                 int64   `json:"id,omitempty"`
	TransactionEventID string  `json:"transaction_event_id,omitempty"`
	SpanID             string  `json:"span_id"`
	ParentSpanID       string  `json:"parent_span_id"`
	Op                 string  `json:"op"`
	Description        string  `json:"description"`
	DurationMS         float64 `json:"duration_ms"`
	Status             string  `json:"status"`
	StartOffsetMS      float64 `json:"start_offset_ms"`
}

type TransactionSummary struct {
	Name      string  `json:"name"`
	Count     int64   `json:"count"`
	P95MS     float64 `json:"p95_ms"`
	P99MS     float64 `json:"p99_ms"`
	ProjectID int64   `json:"project_id"`
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
	ts := in.Timestamp.UTC().Format(time.RFC3339Nano)
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := TransactionRow{
			EventID:     in.EventID,
			ProjectID:   in.ProjectID,
			Name:        in.Name,
			Op:          in.Op,
			TraceID:     in.TraceID,
			SpanID:      in.SpanID,
			DurationMS:  in.DurationMS,
			Status:      in.Status,
			Environment: strPtrOrNil(in.Environment),
			Release:     strPtrOrNil(in.Release),
			Timestamp:   ts,
			RawPath:     in.RawPath,
			PayloadJSON: in.PayloadJSON,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}

		spans := in.Spans
		if len(spans) > MaxSpansPerTransaction {
			spans = spans[:MaxSpansPerTransaction]
		}
		for _, sp := range spans {
			spanRow := SpanRow{
				TransactionEventID: in.EventID,
				SpanID:             sp.SpanID,
				ParentSpanID:       sp.ParentSpanID,
				Op:                 sp.Op,
				Description:        sp.Description,
				DurationMS:         sp.DurationMS,
				StartOffsetMS:      sp.StartOffsetMS,
				Status:             sp.Status,
			}
			if err := tx.Create(&spanRow).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) ListTransactionSummaries(ctx context.Context, projectID int64, from time.Time) ([]TransactionSummary, error) {
	if projectID <= 0 {
		return nil, nil
	}
	return s.computeTransactionSummaries(ctx, projectID, from)
}

func (s *Store) computeTransactionSummaries(ctx context.Context, projectID int64, from time.Time) ([]TransactionSummary, error) {
	fromStr := from.UTC().Format(time.RFC3339Nano)
	type durRow struct {
		Name       string
		DurationMS float64 `gorm:"column:duration_ms"`
	}
	var rows []durRow
	err := s.DB.WithContext(ctx).Raw(`
		SELECT name, duration_ms FROM transactions
		WHERE project_id = ? AND timestamp >= ?
		ORDER BY name
	`, projectID, fromStr).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	byName := map[string][]float64{}
	var order []string
	for _, r := range rows {
		if _, ok := byName[r.Name]; !ok {
			order = append(order, r.Name)
		}
		byName[r.Name] = append(byName[r.Name], r.DurationMS)
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
	var rows []TransactionRow
	err := s.DB.WithContext(ctx).
		Where("project_id = ? AND name = ?", projectID, name).
		Order("timestamp DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]Transaction, 0, len(rows))
	for i := range rows {
		out = append(out, *transactionFromRow(&rows[i]))
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
	var rows []SpanRow
	err := s.DB.WithContext(ctx).
		Where("transaction_event_id = ?", transactionEventID).
		Order("id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]Span, 0, len(rows))
	for _, r := range rows {
		out = append(out, Span{
			ID:                 r.ID,
			TransactionEventID: r.TransactionEventID,
			SpanID:             r.SpanID,
			ParentSpanID:       r.ParentSpanID,
			Op:                 r.Op,
			Description:        r.Description,
			DurationMS:         r.DurationMS,
			StartOffsetMS:      r.StartOffsetMS,
			Status:             r.Status,
		})
	}
	return out, nil
}

func (s *Store) GetTrace(ctx context.Context, traceID string) (*TraceDetail, error) {
	if traceID == "" {
		return nil, nil
	}
	var rows []TransactionRow
	err := s.DB.WithContext(ctx).
		Where("trace_id = ?", traceID).
		Order("timestamp ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	txs := make([]Transaction, 0, len(rows))
	for i := range rows {
		txs = append(txs, *transactionFromRow(&rows[i]))
	}
	for i := range txs {
		spans, err := s.ListSpans(ctx, txs[i].EventID)
		if err != nil {
			return nil, err
		}
		txs[i].Spans = spans
	}

	type issueScan struct {
		IssueID int64
		Title   string
		EventID string
	}
	var issueRows []issueScan
	err = s.DB.WithContext(ctx).Raw(`
		SELECT e.issue_id, i.title, e.event_id
		FROM events e
		JOIN issues i ON i.id = e.issue_id
		WHERE e.trace_id = ?
		ORDER BY e.timestamp DESC
	`, traceID).Scan(&issueRows).Error
	if err != nil {
		return nil, err
	}
	issues := []TraceIssue{}
	seen := map[int64]bool{}
	for _, r := range issueRows {
		if seen[r.IssueID] {
			continue
		}
		seen[r.IssueID] = true
		issues = append(issues, TraceIssue{IssueID: r.IssueID, Title: r.Title, EventID: r.EventID})
	}
	if txs == nil {
		txs = []Transaction{}
	}
	return &TraceDetail{TraceID: traceID, Transactions: txs, Issues: issues}, nil
}

func (s *Store) RecomputeTransactionStats(ctx context.Context, windowSec int) error {
	if windowSec <= 0 {
		windowSec = DefaultPerfWindowSec
	}
	from := time.Now().UTC().Add(-time.Duration(windowSec) * time.Second)
	windowStart := from.Format(time.RFC3339Nano)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	var projectIDs []int64
	err := s.DB.WithContext(ctx).Raw(`
		SELECT DISTINCT project_id FROM transactions WHERE timestamp >= ?
	`, windowStart).Scan(&projectIDs).Error
	if err != nil {
		return err
	}

	for _, pid := range projectIDs {
		summaries, err := s.computeTransactionSummaries(ctx, pid, from)
		if err != nil {
			return err
		}
		for _, sum := range summaries {
			row := TransactionStatRow{
				ProjectID:   pid,
				Name:        sum.Name,
				WindowStart: windowStart,
				WindowSec:   windowSec,
				Count:       sum.Count,
				P95MS:       sum.P95MS,
				P99MS:       sum.P99MS,
				UpdatedAt:   now,
			}
			err = s.DB.WithContext(ctx).Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "project_id"}, {Name: "name"}, {Name: "window_start"}, {Name: "window_sec"},
				},
				DoUpdates: clause.AssignmentColumns([]string{"count", "p95_ms", "p99_ms", "updated_at"}),
			}).Create(&row).Error
			if err != nil {
				return err
			}
		}
	}
	cutoff := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339Nano)
	_ = s.DB.WithContext(ctx).Where("window_start < ?", cutoff).Delete(&TransactionStatRow{}).Error
	return nil
}

func transactionFromRow(row *TransactionRow) *Transaction {
	return &Transaction{
		ID:          row.ID,
		EventID:     row.EventID,
		ProjectID:   row.ProjectID,
		Name:        row.Name,
		Op:          row.Op,
		TraceID:     row.TraceID,
		SpanID:      row.SpanID,
		DurationMS:  row.DurationMS,
		Status:      row.Status,
		Environment: row.Environment,
		Release:     row.Release,
		Timestamp:   row.Timestamp,
		RawPath:     row.RawPath,
		PayloadJSON: row.PayloadJSON,
	}
}
