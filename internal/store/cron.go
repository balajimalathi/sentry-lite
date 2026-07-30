package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type CronMonitor struct {
	ID             int64   `json:"id"`
	ProjectID      int64   `json:"project_id"`
	Slug           string  `json:"slug"`
	Name           string  `json:"name"`
	ScheduleSec    int64   `json:"schedule_sec"`
	GraceSec       int64   `json:"grace_sec"`
	Environment    *string `json:"environment"`
	Status         string  `json:"status"`
	LastCheckinAt  *string `json:"last_checkin_at"`
	NextExpectedAt *string `json:"next_expected_at"`
	Token          string  `json:"token"`
	CreatedAt      string  `json:"created_at"`
}

type CronCheckin struct {
	ID         int64    `json:"id"`
	MonitorID  int64    `json:"monitor_id"`
	Status     string   `json:"status"`
	DurationMS *float64 `json:"duration_ms,omitempty"`
	Timestamp  string   `json:"timestamp"`
}

type CreateCronInput struct {
	ProjectID   int64
	Name        string
	Slug        string
	ScheduleSec int64
	GraceSec    int64
	Environment string
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash && b.Len() > 0 {
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "monitor"
	}
	return out
}

func (s *Store) CreateCronMonitor(ctx context.Context, in CreateCronInput) (*CronMonitor, error) {
	if in.ProjectID <= 0 || in.Name == "" {
		return nil, fmt.Errorf("project_id and name required")
	}
	if in.ScheduleSec <= 0 {
		return nil, fmt.Errorf("schedule_sec must be > 0")
	}
	if in.GraceSec <= 0 {
		in.GraceSec = 60
	}
	slug := in.Slug
	if slug == "" {
		slug = slugify(in.Name)
	}
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	next := now.Add(time.Duration(in.ScheduleSec) * time.Second).Format(time.RFC3339Nano)
	res, err := s.DB.ExecContext(ctx, `
		INSERT INTO cron_monitors (
			project_id, slug, name, schedule_sec, grace_sec, environment,
			status, next_expected_at, token
		) VALUES (?, ?, ?, ?, ?, ?, 'unknown', ?, ?)
	`, in.ProjectID, slug, in.Name, in.ScheduleSec, in.GraceSec, nullOrNil(in.Environment), next, token)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetCronMonitor(ctx, id)
}

func (s *Store) GetCronMonitor(ctx context.Context, id int64) (*CronMonitor, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, project_id, slug, name, schedule_sec, grace_sec, environment, status,
		       last_checkin_at, next_expected_at, token, created_at
		FROM cron_monitors WHERE id = ?
	`, id)
	return scanCronMonitor(row)
}

func (s *Store) GetCronMonitorByToken(ctx context.Context, token string) (*CronMonitor, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, project_id, slug, name, schedule_sec, grace_sec, environment, status,
		       last_checkin_at, next_expected_at, token, created_at
		FROM cron_monitors WHERE token = ?
	`, token)
	return scanCronMonitor(row)
}

func (s *Store) ListCronMonitors(ctx context.Context, projectID int64) ([]CronMonitor, error) {
	q := `
		SELECT id, project_id, slug, name, schedule_sec, grace_sec, environment, status,
		       last_checkin_at, next_expected_at, token, created_at
		FROM cron_monitors`
	args := []any{}
	if projectID > 0 {
		q += ` WHERE project_id = ?`
		args = append(args, projectID)
	}
	q += ` ORDER BY name ASC`
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CronMonitor
	for rows.Next() {
		m, err := scanCronMonitor(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func (s *Store) DeleteCronMonitor(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM cron_monitors WHERE id = ?`, id)
	return err
}

func (s *Store) UpdateCronMonitor(ctx context.Context, id int64, name string, scheduleSec, graceSec int64, env string) (*CronMonitor, error) {
	m, err := s.GetCronMonitor(ctx, id)
	if err != nil || m == nil {
		return m, err
	}
	if name == "" {
		name = m.Name
	}
	if scheduleSec <= 0 {
		scheduleSec = m.ScheduleSec
	}
	if graceSec <= 0 {
		graceSec = m.GraceSec
	}
	_, err = s.DB.ExecContext(ctx, `
		UPDATE cron_monitors SET name = ?, schedule_sec = ?, grace_sec = ?, environment = ? WHERE id = ?
	`, name, scheduleSec, graceSec, nullOrNil(env), id)
	if err != nil {
		return nil, err
	}
	return s.GetCronMonitor(ctx, id)
}

// RecordCronCheckin records a heartbeat and advances next_expected_at.
func (s *Store) RecordCronCheckin(ctx context.Context, monitorID int64, status string, durationMS *float64) (*CronMonitor, error) {
	if status == "" {
		status = "ok"
	}
	now := time.Now().UTC()
	ts := now.Format(time.RFC3339Nano)

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var scheduleSec int64
	err = tx.QueryRowContext(ctx, `SELECT schedule_sec FROM cron_monitors WHERE id = ?`, monitorID).Scan(&scheduleSec)
	if err != nil {
		return nil, err
	}
	next := now.Add(time.Duration(scheduleSec) * time.Second).Format(time.RFC3339Nano)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO cron_checkins (monitor_id, status, duration_ms, timestamp) VALUES (?, ?, ?, ?)
	`, monitorID, status, durationMS, ts)
	if err != nil {
		return nil, err
	}

	monStatus := "ok"
	if status == "error" {
		monStatus = "ok" // reported, even if job failed — miss/late is about heartbeat absence
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE cron_monitors SET status = ?, last_checkin_at = ?, next_expected_at = ? WHERE id = ?
	`, monStatus, ts, next, monitorID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetCronMonitor(ctx, monitorID)
}

type CronStatusChange struct {
	Monitor    CronMonitor
	PrevStatus string
	NewStatus  string
}

// EvaluateCronMonitors marks late/missed monitors. Missed = past next+grace; late = past next but within grace.
func (s *Store) EvaluateCronMonitors(ctx context.Context) ([]CronStatusChange, error) {
	now := time.Now().UTC()
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, project_id, slug, name, schedule_sec, grace_sec, environment, status,
		       last_checkin_at, next_expected_at, token, created_at
		FROM cron_monitors
		WHERE next_expected_at IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}

	var monitors []CronMonitor
	for rows.Next() {
		m, err := scanCronMonitor(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		monitors = append(monitors, *m)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	var changes []CronStatusChange
	for i := range monitors {
		m := &monitors[i]
		if m.NextExpectedAt == nil {
			continue
		}
		next, err := parseFlexibleTime(*m.NextExpectedAt)
		if err != nil {
			continue
		}
		graceEnd := next.Add(time.Duration(m.GraceSec) * time.Second)
		prev := m.Status
		newStatus := prev
		if now.After(graceEnd) {
			newStatus = "missed"
		} else if now.After(next) {
			newStatus = "late"
		}
		if newStatus != prev && (newStatus == "late" || newStatus == "missed") {
			_, err = s.DB.ExecContext(ctx, `UPDATE cron_monitors SET status = ? WHERE id = ?`, newStatus, m.ID)
			if err != nil {
				return nil, err
			}
			m.Status = newStatus
			changes = append(changes, CronStatusChange{Monitor: *m, PrevStatus: prev, NewStatus: newStatus})
		}
	}
	return changes, nil
}

func parseFlexibleTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

func scanCronMonitor(row scannable) (*CronMonitor, error) {
	var m CronMonitor
	var env, last, next sql.NullString
	err := row.Scan(
		&m.ID, &m.ProjectID, &m.Slug, &m.Name, &m.ScheduleSec, &m.GraceSec, &env, &m.Status,
		&last, &next, &m.Token, &m.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m.Environment = nullStr(env)
	m.LastCheckinAt = nullStr(last)
	m.NextExpectedAt = nullStr(next)
	return &m, nil
}
