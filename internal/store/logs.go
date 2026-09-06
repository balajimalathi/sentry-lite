package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const (
	DefaultLogLimit = 200
	MaxLogLimit     = 500
)

type Log struct {
	ID             int64          `json:"id"`
	ProjectID      int64          `json:"project_id"`
	Timestamp      string         `json:"timestamp"`
	Level          string         `json:"level"`
	Body           string         `json:"body"`
	TraceID        string         `json:"trace_id"`
	SpanID         string         `json:"span_id"`
	SeverityNumber int            `json:"severity_number"`
	Environment    *string        `json:"environment"`
	Release        *string        `json:"release"`
	Origin         string         `json:"origin"`
	PayloadJSON    string         `json:"payload_json,omitempty"`
	Attributes     map[string]any `json:"attributes,omitempty"`
}

type LogListFilter struct {
	ProjectID int64
	Level     string
	Q         string
	Limit     int
}

func (s *Store) InsertLogs(ctx context.Context, rows []LogRow) error {
	if len(rows) == 0 {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i := range rows {
		if rows[i].CreatedAt == "" {
			rows[i].CreatedAt = now
		}
	}
	return s.DB.WithContext(ctx).CreateInBatches(rows, 100).Error
}

func (s *Store) ListLogs(ctx context.Context, f LogListFilter) ([]Log, error) {
	if f.ProjectID <= 0 {
		return nil, fmt.Errorf("project_id required")
	}
	limit := f.Limit
	if limit <= 0 {
		limit = DefaultLogLimit
	}
	if limit > MaxLogLimit {
		limit = MaxLogLimit
	}
	q := s.DB.WithContext(ctx).Where("project_id = ?", f.ProjectID)
	if f.Level != "" {
		q = q.Where("level = ?", f.Level)
	}
	if f.Q != "" {
		q = q.Where("body LIKE ?", "%"+f.Q+"%")
	}
	var rows []LogRow
	err := q.Order("timestamp DESC").Limit(limit).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]Log, 0, len(rows))
	for i := range rows {
		out = append(out, logFromRow(&rows[i]))
	}
	return out, nil
}

func (s *Store) ListLogsForTrace(ctx context.Context, traceID string, limit int) ([]Log, error) {
	if traceID == "" {
		return []Log{}, nil
	}
	if limit <= 0 {
		limit = DefaultLogLimit
	}
	if limit > MaxLogLimit {
		limit = MaxLogLimit
	}
	var rows []LogRow
	err := s.DB.WithContext(ctx).
		Where("trace_id = ?", traceID).
		Order("timestamp ASC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]Log, 0, len(rows))
	for i := range rows {
		out = append(out, logFromRow(&rows[i]))
	}
	return out, nil
}

func logFromRow(row *LogRow) Log {
	out := Log{
		ID:             row.ID,
		ProjectID:      row.ProjectID,
		Timestamp:      row.Timestamp,
		Level:          row.Level,
		Body:           row.Body,
		TraceID:        row.TraceID,
		SpanID:         row.SpanID,
		SeverityNumber: row.SeverityNumber,
		Environment:    row.Environment,
		Release:        row.Release,
		Origin:         row.Origin,
		PayloadJSON:    row.PayloadJSON,
	}
	if row.PayloadJSON != "" {
		var raw map[string]any
		if json.Unmarshal([]byte(row.PayloadJSON), &raw) == nil {
			if attrs, ok := raw["attributes"].(map[string]any); ok {
				out.Attributes = flattenLogAttributes(attrs)
			}
		}
	}
	return out
}

func flattenLogAttributes(attrs map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range attrs {
		if m, ok := v.(map[string]any); ok {
			if val, exists := m["value"]; exists {
				out[k] = val
				continue
			}
		}
		out[k] = v
	}
	return out
}
