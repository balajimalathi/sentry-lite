package store

import (
	"context"
	"database/sql"
	"time"
)

type Release struct {
	ID           int64   `json:"id"`
	ProjectID    int64   `json:"project_id"`
	Version      string  `json:"version"`
	Ref          *string `json:"ref,omitempty"`
	URL          *string `json:"url,omitempty"`
	DateReleased *string `json:"date_released,omitempty"`
	CreatedAt    string  `json:"created_at"`
	IssueCount   int64   `json:"issue_count"`
	EventCount   int64   `json:"event_count"`
}

type AlertRule struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"project_id"`
	Name      string `json:"name"`
	Trigger   string `json:"trigger"`
	Channel   string `json:"channel"`
	Target    string `json:"target"`
	Threshold int64  `json:"threshold"`
	WindowSec int64  `json:"window_sec"`
	Enabled   bool   `json:"enabled"`
	Secret    string `json:"secret,omitempty"`
	CreatedAt string `json:"created_at"`
}

func (s *Store) UpsertRelease(ctx context.Context, projectID int64, version, ref, url string) (*Release, error) {
	if version == "" {
		return nil, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO releases (project_id, version, ref, url, date_released)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(project_id, version) DO UPDATE SET
			ref = COALESCE(excluded.ref, releases.ref),
			url = COALESCE(excluded.url, releases.url)
	`, projectID, version, nullOrNil(ref), nullOrNil(url), now)
	if err != nil {
		return nil, err
	}
	return s.GetRelease(ctx, projectID, version)
}

func (s *Store) GetRelease(ctx context.Context, projectID int64, version string) (*Release, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, project_id, version, ref, url, date_released, created_at FROM releases
		WHERE project_id = ? AND version = ?
	`, projectID, version)
	return scanRelease(row)
}

func (s *Store) ListReleases(ctx context.Context, projectID int64) ([]Release, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT r.id, r.project_id, r.version, r.ref, r.url, r.date_released, r.created_at,
		       COALESCE((SELECT COUNT(DISTINCT e.issue_id) FROM events e WHERE e.project_id = r.project_id AND e.release = r.version), 0),
		       COALESCE((SELECT COUNT(*) FROM events e WHERE e.project_id = r.project_id AND e.release = r.version), 0)
		FROM releases r
		WHERE r.project_id = ?
		ORDER BY r.created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Release
	for rows.Next() {
		var rel Release
		var ref, url, dateRel sql.NullString
		if err := rows.Scan(&rel.ID, &rel.ProjectID, &rel.Version, &ref, &url, &dateRel, &rel.CreatedAt, &rel.IssueCount, &rel.EventCount); err != nil {
			return nil, err
		}
		if ref.Valid {
			rel.Ref = &ref.String
		}
		if url.Valid {
			rel.URL = &url.String
		}
		if dateRel.Valid {
			rel.DateReleased = &dateRel.String
		}
		out = append(out, rel)
	}
	return out, rows.Err()
}

func scanRelease(row scannable) (*Release, error) {
	var rel Release
	var ref, url, dateRel sql.NullString
	err := row.Scan(&rel.ID, &rel.ProjectID, &rel.Version, &ref, &url, &dateRel, &rel.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if ref.Valid {
		rel.Ref = &ref.String
	}
	if url.Valid {
		rel.URL = &url.String
	}
	if dateRel.Valid {
		rel.DateReleased = &dateRel.String
	}
	return &rel, nil
}

func (s *Store) CreateAlertRule(ctx context.Context, rule AlertRule) (*AlertRule, error) {
	en := 0
	if rule.Enabled {
		en = 1
	}
	if rule.WindowSec <= 0 {
		rule.WindowSec = 300
	}
	res, err := s.DB.ExecContext(ctx, `
		INSERT INTO alert_rules (project_id, name, trigger, channel, target, threshold, window_sec, enabled, secret)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, rule.ProjectID, rule.Name, rule.Trigger, rule.Channel, rule.Target, rule.Threshold, rule.WindowSec, en, nullOrNil(rule.Secret))
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetAlertRule(ctx, id)
}

func (s *Store) GetAlertRule(ctx context.Context, id int64) (*AlertRule, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, project_id, name, trigger, channel, target, threshold, window_sec, enabled, secret, created_at
		FROM alert_rules WHERE id = ?
	`, id)
	return scanAlertRule(row)
}

func (s *Store) ListAlertRules(ctx context.Context, projectID int64) ([]AlertRule, error) {
	q := `SELECT id, project_id, name, trigger, channel, target, threshold, window_sec, enabled, secret, created_at FROM alert_rules`
	args := []any{}
	if projectID > 0 {
		q += ` WHERE project_id = ?`
		args = append(args, projectID)
	}
	q += ` ORDER BY id ASC`
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AlertRule
	for rows.Next() {
		rule, err := scanAlertRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rule)
	}
	return out, rows.Err()
}

func (s *Store) ListEnabledAlertRules(ctx context.Context, projectID int64, trigger string) ([]AlertRule, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, project_id, name, trigger, channel, target, threshold, window_sec, enabled, secret, created_at
		FROM alert_rules WHERE project_id = ? AND trigger = ? AND enabled = 1
	`, projectID, trigger)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AlertRule
	for rows.Next() {
		rule, err := scanAlertRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rule)
	}
	return out, rows.Err()
}

func (s *Store) CountEventsSince(ctx context.Context, projectID int64, since time.Time) (int64, error) {
	var n int64
	err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM events WHERE project_id = ? AND timestamp >= ?`,
		projectID, since.UTC().Format(time.RFC3339Nano),
	).Scan(&n)
	return n, err
}

func (s *Store) RecordAlertDelivery(ctx context.Context, ruleID, issueID int64, status, detail string) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO alert_deliveries (rule_id, issue_id, status, detail) VALUES (?, ?, ?, ?)`,
		ruleID, nullOrNilInt(issueID), status, detail,
	)
	return err
}

func scanAlertRule(row scannable) (*AlertRule, error) {
	var rule AlertRule
	var en int
	var secret sql.NullString
	err := row.Scan(&rule.ID, &rule.ProjectID, &rule.Name, &rule.Trigger, &rule.Channel, &rule.Target,
		&rule.Threshold, &rule.WindowSec, &en, &secret, &rule.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rule.Enabled = en == 1
	if secret.Valid {
		rule.Secret = secret.String
	}
	return &rule, nil
}

func nullOrNilInt(n int64) any {
	if n == 0 {
		return nil
	}
	return n
}
