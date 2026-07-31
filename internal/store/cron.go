package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
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
	row := CronMonitorRow{
		ProjectID:      in.ProjectID,
		Slug:           slug,
		Name:           in.Name,
		ScheduleSec:    in.ScheduleSec,
		GraceSec:       in.GraceSec,
		Environment:    strPtrOrNil(in.Environment),
		Status:         "unknown",
		NextExpectedAt: &next,
		Token:          token,
	}
	if err := s.DB.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	return s.GetCronMonitor(ctx, row.ID)
}

func (s *Store) GetCronMonitor(ctx context.Context, id int64) (*CronMonitor, error) {
	var row CronMonitorRow
	err := s.DB.WithContext(ctx).First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return cronMonitorFromRow(&row), nil
}

func (s *Store) GetCronMonitorByToken(ctx context.Context, token string) (*CronMonitor, error) {
	var row CronMonitorRow
	err := s.DB.WithContext(ctx).Where("token = ?", token).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return cronMonitorFromRow(&row), nil
}

func (s *Store) ListCronMonitors(ctx context.Context, projectID int64) ([]CronMonitor, error) {
	q := s.DB.WithContext(ctx).Model(&CronMonitorRow{})
	if projectID > 0 {
		q = q.Where("project_id = ?", projectID)
	}
	var rows []CronMonitorRow
	if err := q.Order("name ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]CronMonitor, 0, len(rows))
	for i := range rows {
		out = append(out, *cronMonitorFromRow(&rows[i]))
	}
	return out, nil
}

func (s *Store) DeleteCronMonitor(ctx context.Context, id int64) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("monitor_id = ?", id).Delete(&CronCheckinRow{}).Error; err != nil {
			return err
		}
		return tx.Delete(&CronMonitorRow{}, id).Error
	})
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
	err = s.DB.WithContext(ctx).Model(&CronMonitorRow{}).Where("id = ?", id).Updates(map[string]any{
		"name":         name,
		"schedule_sec": scheduleSec,
		"grace_sec":    graceSec,
		"environment":  strPtrOrNil(env),
	}).Error
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

	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var mon CronMonitorRow
		if err := tx.First(&mon, monitorID).Error; err != nil {
			return err
		}
		next := now.Add(time.Duration(mon.ScheduleSec) * time.Second).Format(time.RFC3339Nano)
		checkin := CronCheckinRow{
			MonitorID:  monitorID,
			Status:     status,
			DurationMS: durationMS,
			Timestamp:  ts,
		}
		if err := tx.Create(&checkin).Error; err != nil {
			return err
		}
		monStatus := "ok"
		return tx.Model(&CronMonitorRow{}).Where("id = ?", monitorID).Updates(map[string]any{
			"status":           monStatus,
			"last_checkin_at":  ts,
			"next_expected_at": next,
		}).Error
	})
	if err != nil {
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
	var rows []CronMonitorRow
	err := s.DB.WithContext(ctx).Where("next_expected_at IS NOT NULL").Find(&rows).Error
	if err != nil {
		return nil, err
	}

	var changes []CronStatusChange
	for i := range rows {
		m := cronMonitorFromRow(&rows[i])
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
			if err := s.DB.WithContext(ctx).Model(&CronMonitorRow{}).Where("id = ?", m.ID).Update("status", newStatus).Error; err != nil {
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

func cronMonitorFromRow(row *CronMonitorRow) *CronMonitor {
	return &CronMonitor{
		ID:             row.ID,
		ProjectID:      row.ProjectID,
		Slug:           row.Slug,
		Name:           row.Name,
		ScheduleSec:    row.ScheduleSec,
		GraceSec:       row.GraceSec,
		Environment:    row.Environment,
		Status:         row.Status,
		LastCheckinAt:  row.LastCheckinAt,
		NextExpectedAt: row.NextExpectedAt,
		Token:          row.Token,
		CreatedAt:      row.CreatedAt,
	}
}
