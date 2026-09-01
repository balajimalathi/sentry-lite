package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	ID                 int64   `json:"id"`
	ProjectID          int64   `json:"project_id"`
	Name               string  `json:"name"`
	Trigger            string  `json:"trigger"`
	Channel            string  `json:"channel"`
	Target             string  `json:"target"`
	Threshold          int64   `json:"threshold"`
	WindowSec          int64   `json:"window_sec"`
	Enabled            bool    `json:"enabled"`
	Secret             string  `json:"secret,omitempty"`
	CreatedAt          string  `json:"created_at"`
	LastDeliveredAt    *string `json:"last_delivered_at,omitempty"`
	LastDeliveryStatus *string `json:"last_delivery_status,omitempty"`
}

func (s *Store) UpsertRelease(ctx context.Context, projectID int64, version, ref, url string) (*Release, error) {
	if version == "" {
		return nil, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	row := ReleaseRow{
		ProjectID:    projectID,
		Version:      version,
		Ref:          strPtrOrNil(ref),
		URL:          strPtrOrNil(url),
		DateReleased: &now,
	}
	err := s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "project_id"}, {Name: "version"}},
		DoUpdates: clause.Assignments(map[string]any{
			"ref": gorm.Expr("COALESCE(excluded.ref, releases.ref)"),
			"url": gorm.Expr("COALESCE(excluded.url, releases.url)"),
		}),
	}).Create(&row).Error
	if err != nil {
		return nil, err
	}
	return s.GetRelease(ctx, projectID, version)
}

func (s *Store) GetRelease(ctx context.Context, projectID int64, version string) (*Release, error) {
	var row ReleaseRow
	err := s.DB.WithContext(ctx).
		Where("project_id = ? AND version = ?", projectID, version).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return releaseFromRow(&row), nil
}

func (s *Store) ListReleases(ctx context.Context, projectID int64) ([]Release, error) {
	type releaseScan struct {
		ID           int64
		ProjectID    int64
		Version      string
		Ref          *string
		URL          *string
		DateReleased *string
		CreatedAt    string
		IssueCount   int64
		EventCount   int64
	}
	var rows []releaseScan
	err := s.DB.WithContext(ctx).Raw(`
		SELECT r.id, r.project_id, r.version, r.ref, r.url, r.date_released, r.created_at,
		       COALESCE((SELECT COUNT(DISTINCT e.issue_id) FROM events e WHERE e.project_id = r.project_id AND e.release = r.version), 0) AS issue_count,
		       COALESCE((SELECT COUNT(*) FROM events e WHERE e.project_id = r.project_id AND e.release = r.version), 0) AS event_count
		FROM releases r
		WHERE r.project_id = ?
		ORDER BY r.created_at DESC
	`, projectID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]Release, 0, len(rows))
	for _, r := range rows {
		out = append(out, Release{
			ID:           r.ID,
			ProjectID:    r.ProjectID,
			Version:      r.Version,
			Ref:          r.Ref,
			URL:          r.URL,
			DateReleased: r.DateReleased,
			CreatedAt:    r.CreatedAt,
			IssueCount:   r.IssueCount,
			EventCount:   r.EventCount,
		})
	}
	return out, nil
}

func releaseFromRow(row *ReleaseRow) *Release {
	return &Release{
		ID:           row.ID,
		ProjectID:    row.ProjectID,
		Version:      row.Version,
		Ref:          row.Ref,
		URL:          row.URL,
		DateReleased: row.DateReleased,
		CreatedAt:    row.CreatedAt,
	}
}

func (s *Store) CreateAlertRule(ctx context.Context, rule AlertRule) (*AlertRule, error) {
	en := 0
	if rule.Enabled {
		en = 1
	}
	if rule.WindowSec <= 0 {
		rule.WindowSec = 300
	}
	row := AlertRuleRow{
		ProjectID: rule.ProjectID,
		Name:      rule.Name,
		Trigger:   rule.Trigger,
		Channel:   rule.Channel,
		Target:    rule.Target,
		Threshold: rule.Threshold,
		WindowSec: rule.WindowSec,
		Enabled:   en,
		Secret:    strPtrOrNil(rule.Secret),
	}
	if err := s.DB.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	return s.GetAlertRule(ctx, row.ID)
}

func (s *Store) GetAlertRule(ctx context.Context, id int64) (*AlertRule, error) {
	var row AlertRuleRow
	err := s.DB.WithContext(ctx).First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return alertRuleFromRow(&row), nil
}

func (s *Store) ListAlertRules(ctx context.Context, projectID int64) ([]AlertRule, error) {
	q := s.DB.WithContext(ctx).Model(&AlertRuleRow{})
	if projectID > 0 {
		q = q.Where("project_id = ?", projectID)
	}
	var rows []AlertRuleRow
	if err := q.Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]AlertRule, 0, len(rows))
	for i := range rows {
		out = append(out, *alertRuleFromRow(&rows[i]))
	}
	if err := s.attachAlertDeliveries(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) attachAlertDeliveries(ctx context.Context, rules []AlertRule) error {
	if len(rules) == 0 {
		return nil
	}
	type lastDel struct {
		RuleID    int64
		Status    string
		CreatedAt string
	}
	var rows []lastDel
	err := s.DB.WithContext(ctx).Raw(`
		SELECT rule_id, status, created_at
		FROM alert_deliveries
		WHERE id IN (SELECT MAX(id) FROM alert_deliveries GROUP BY rule_id)
	`).Scan(&rows).Error
	if err != nil {
		return err
	}
	byRule := map[int64]lastDel{}
	for _, r := range rows {
		byRule[r.RuleID] = r
	}
	for i := range rules {
		if d, ok := byRule[rules[i].ID]; ok {
			status := d.Status
			at := d.CreatedAt
			rules[i].LastDeliveryStatus = &status
			rules[i].LastDeliveredAt = &at
		}
	}
	return nil
}

func (s *Store) ListEnabledAlertRules(ctx context.Context, projectID int64, trigger string) ([]AlertRule, error) {
	var rows []AlertRuleRow
	err := s.DB.WithContext(ctx).
		Where("project_id = ? AND trigger = ? AND enabled = 1", projectID, trigger).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]AlertRule, 0, len(rows))
	for i := range rows {
		out = append(out, *alertRuleFromRow(&rows[i]))
	}
	return out, nil
}

func (s *Store) CountEventsSince(ctx context.Context, projectID int64, since time.Time) (int64, error) {
	var n int64
	err := s.DB.WithContext(ctx).Model(&EventRow{}).
		Where("project_id = ? AND timestamp >= ?", projectID, since.UTC().Format(time.RFC3339Nano)).
		Count(&n).Error
	return n, err
}

func (s *Store) RecordAlertDelivery(ctx context.Context, ruleID, issueID int64, status, detail string) error {
	row := AlertDeliveryRow{
		RuleID: ruleID,
		Status: status,
		Detail: strPtrOrNil(detail),
	}
	if issueID != 0 {
		row.IssueID = &issueID
	}
	return s.DB.WithContext(ctx).Create(&row).Error
}

func (s *Store) DeleteAlertRule(ctx context.Context, id int64) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("rule_id = ?", id).Delete(&AlertDeliveryRow{}).Error; err != nil {
			return err
		}
		return tx.Delete(&AlertRuleRow{}, id).Error
	})
}

type AlertRulePatch struct {
	Name      *string `json:"name"`
	Trigger   *string `json:"trigger"`
	Channel   *string `json:"channel"`
	Target    *string `json:"target"`
	Threshold *int64  `json:"threshold"`
	WindowSec *int64  `json:"window_sec"`
	Enabled   *bool   `json:"enabled"`
	Secret    *string `json:"secret"`
}

func (s *Store) UpdateAlertRule(ctx context.Context, id int64, patch AlertRulePatch) (*AlertRule, error) {
	existing, err := s.GetAlertRule(ctx, id)
	if err != nil || existing == nil {
		return existing, err
	}
	updates := map[string]any{}
	if patch.Name != nil && strings.TrimSpace(*patch.Name) != "" {
		updates["name"] = strings.TrimSpace(*patch.Name)
	}
	if patch.Trigger != nil && *patch.Trigger != "" {
		updates["trigger"] = *patch.Trigger
	}
	if patch.Channel != nil && *patch.Channel != "" {
		updates["channel"] = *patch.Channel
	}
	if patch.Target != nil && strings.TrimSpace(*patch.Target) != "" {
		updates["target"] = strings.TrimSpace(*patch.Target)
	}
	if patch.Threshold != nil {
		updates["threshold"] = *patch.Threshold
	}
	if patch.WindowSec != nil && *patch.WindowSec > 0 {
		updates["window_sec"] = *patch.WindowSec
	}
	if patch.Enabled != nil {
		en := 0
		if *patch.Enabled {
			en = 1
		}
		updates["enabled"] = en
	}
	if patch.Secret != nil {
		updates["secret"] = strPtrOrNil(*patch.Secret)
	}
	if len(updates) == 0 {
		return existing, nil
	}
	if err := s.DB.WithContext(ctx).Model(&AlertRuleRow{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetAlertRule(ctx, id)
}

func (s *Store) HasRecentDelivery(ctx context.Context, ruleID, issueID int64, within time.Duration) (bool, error) {
	sec := int(within.Seconds())
	if sec < 1 {
		sec = 1
	}
	q := s.DB.WithContext(ctx).Model(&AlertDeliveryRow{}).
		Where("rule_id = ? AND status = ? AND created_at >= datetime('now', ?)", ruleID, "ok", fmt.Sprintf("-%d seconds", sec))
	if issueID > 0 {
		q = q.Where("issue_id = ?", issueID)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

func alertRuleFromRow(row *AlertRuleRow) *AlertRule {
	rule := &AlertRule{
		ID:        row.ID,
		ProjectID: row.ProjectID,
		Name:      row.Name,
		Trigger:   row.Trigger,
		Channel:   row.Channel,
		Target:    row.Target,
		Threshold: row.Threshold,
		WindowSec: row.WindowSec,
		Enabled:   row.Enabled == 1,
		CreatedAt: row.CreatedAt,
	}
	if row.Secret != nil {
		rule.Secret = *row.Secret
	}
	return rule
}
