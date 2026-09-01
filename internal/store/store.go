package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type Store struct {
	DB *gorm.DB
}

type ProjectKey struct {
	ProjectID int64
	PublicKey string
	SecretKey string
}

type Project struct {
	ID               int64    `json:"id"`
	Slug             string   `json:"slug"`
	Name             string   `json:"name"`
	AllowedOrigins   []string `json:"allowed_origins"`
	IssueCount       int64    `json:"issue_count"`
	LatestActivityAt *string  `json:"latest_activity_at"`
	CreatedAt        string   `json:"created_at"`
}

type Issue struct {
	ID           int64    `json:"id"`
	ProjectID    int64    `json:"project_id"`
	Fingerprint  string   `json:"fingerprint"`
	Title        string   `json:"title"`
	Culprit      string   `json:"culprit"`
	Status       string   `json:"status"`
	Level        string   `json:"level"`
	Count        int64    `json:"count"`
	FirstSeen    string   `json:"first_seen"`
	LastSeen     string   `json:"last_seen"`
	FirstRelease *string  `json:"first_release"`
	LastRelease  *string  `json:"last_release"`
	Regressed    bool     `json:"regressed"`
	Assignee     *string  `json:"assignee,omitempty"`
	Environments []string `json:"environments,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

type Event struct {
	ID            int64             `json:"id"`
	EventID       string            `json:"event_id"`
	IssueID       int64             `json:"issue_id"`
	ProjectID     int64             `json:"project_id"`
	Timestamp     string            `json:"timestamp"`
	Environment   *string           `json:"environment"`
	Release       *string           `json:"release"`
	Platform      *string           `json:"platform"`
	Message       *string           `json:"message"`
	ExceptionType *string           `json:"exception_type"`
	Culprit       *string           `json:"culprit"`
	UserID        *string           `json:"user_id"`
	UserEmail     *string           `json:"user_email"`
	TraceID       *string           `json:"trace_id,omitempty"`
	RawPath       string            `json:"raw_path"`
	PayloadJSON   string            `json:"payload_json,omitempty"`
	Tags          map[string]string `json:"tags,omitempty"`
}

type IssueListFilter struct {
	ProjectID   int64
	Environment string
	Release     string
	Query       string
	TagKey      string
	TagValue    string
	From        string // RFC3339
	To          string // RFC3339
	Limit       int
}

const QuietWindow = 24 * time.Hour

func Open(path string) (*Store, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)", path)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)

	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (s *Store) migrate() error {
	return s.DB.AutoMigrate(allModels()...)
}

func (s *Store) LookupProjectKey(ctx context.Context, publicKey string, projectID int64) (*ProjectKey, error) {
	var row ProjectKeyRow
	err := s.DB.WithContext(ctx).
		Where("public_key = ? AND project_id = ?", publicKey, projectID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &ProjectKey{
		ProjectID: row.ProjectID,
		PublicKey: row.PublicKey,
		SecretKey: row.SecretKey,
	}, nil
}

func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	type projectScan struct {
		ID             int64
		Slug           string
		Name           string
		AllowedOrigins string
		CreatedAt      string
		IssueCount     int64
		LatestActivity *string
	}
	var rows []projectScan
	err := s.DB.WithContext(ctx).Raw(`
		SELECT p.id, p.slug, p.name, p.allowed_origins, p.created_at,
		       COALESCE((SELECT COUNT(*) FROM issues i WHERE i.project_id = p.id), 0) AS issue_count,
		       (SELECT MAX(i.last_seen) FROM issues i WHERE i.project_id = p.id) AS latest_activity
		FROM projects p
		ORDER BY p.id ASC
	`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]Project, 0, len(rows))
	for _, r := range rows {
		out = append(out, Project{
			ID:               r.ID,
			Slug:             r.Slug,
			Name:             r.Name,
			AllowedOrigins:   decodeOriginsJSON(r.AllowedOrigins),
			IssueCount:       r.IssueCount,
			LatestActivityAt: r.LatestActivity,
			CreatedAt:        r.CreatedAt,
		})
	}
	return out, nil
}

func (s *Store) ListIssues(ctx context.Context, f IssueListFilter) ([]Issue, error) {
	if f.Limit <= 0 {
		f.Limit = 100
	}
	conds := []string{"1=1"}
	args := []any{}
	if f.ProjectID > 0 {
		conds = append(conds, "i.project_id = ?")
		args = append(args, f.ProjectID)
	}
	if f.Environment != "" {
		conds = append(conds, `EXISTS (SELECT 1 FROM event_tags et WHERE et.issue_id = i.id AND et.key = 'environment' AND et.value = ?)`)
		args = append(args, f.Environment)
	}
	if f.Release != "" {
		conds = append(conds, `EXISTS (SELECT 1 FROM event_tags et WHERE et.issue_id = i.id AND et.key = 'release' AND et.value = ?)`)
		args = append(args, f.Release)
	}
	if f.TagKey != "" && f.TagValue != "" {
		conds = append(conds, `EXISTS (SELECT 1 FROM event_tags et WHERE et.issue_id = i.id AND et.key = ? AND et.value = ?)`)
		args = append(args, f.TagKey, f.TagValue)
	}
	if f.From != "" {
		conds = append(conds, "i.last_seen >= ?")
		args = append(args, f.From)
	}
	if f.To != "" {
		conds = append(conds, "i.last_seen <= ?")
		args = append(args, f.To)
	}
	if f.Query != "" {
		conds = append(conds, `(i.title LIKE ? OR i.culprit LIKE ? OR EXISTS (
			SELECT 1 FROM events e WHERE e.issue_id = i.id AND (e.message LIKE ? OR e.exception_type LIKE ?)
		))`)
		q := "%" + f.Query + "%"
		args = append(args, q, q, q, q)
	}
	args = append(args, f.Limit)
	query := `SELECT i.id, i.project_id, i.fingerprint, i.title, i.culprit, i.status, i.level,
		i.count, i.first_seen, i.last_seen, i.first_release, i.last_release, i.regressed, i.assignee
		FROM issues i WHERE ` + strings.Join(conds, " AND ") + ` ORDER BY i.last_seen DESC LIMIT ?`

	var rows []IssueRow
	if err := s.DB.WithContext(ctx).Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Issue, 0, len(rows))
	for i := range rows {
		out = append(out, issueFromRow(&rows[i]))
	}
	if err := s.attachIssueTagFacets(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) attachIssueTagFacets(ctx context.Context, issues []Issue) error {
	if len(issues) == 0 {
		return nil
	}
	byID := make(map[int64]int, len(issues))
	ids := make([]int64, len(issues))
	for i := range issues {
		byID[issues[i].ID] = i
		ids[i] = issues[i].ID
	}
	type tagRow struct {
		IssueID int64
		Key     string
		Value   string
	}
	var tags []tagRow
	err := s.DB.WithContext(ctx).Raw(
		`SELECT DISTINCT issue_id, key, value FROM event_tags WHERE issue_id IN ? ORDER BY key, value`,
		ids,
	).Scan(&tags).Error
	if err != nil {
		return err
	}
	for _, t := range tags {
		idx, ok := byID[t.IssueID]
		if !ok || t.Value == "" {
			continue
		}
		switch t.Key {
		case "environment":
			issues[idx].Environments = append(issues[idx].Environments, t.Value)
		case "release":
			// Releases are already on first_release / last_release.
		default:
			issues[idx].Tags = append(issues[idx].Tags, t.Key+":"+t.Value)
		}
	}
	return nil
}

func issueFromRow(row *IssueRow) Issue {
	return Issue{
		ID:           row.ID,
		ProjectID:    row.ProjectID,
		Fingerprint:  row.Fingerprint,
		Title:        row.Title,
		Culprit:      row.Culprit,
		Status:       row.Status,
		Level:        row.Level,
		Count:        row.Count,
		FirstSeen:    row.FirstSeen,
		LastSeen:     row.LastSeen,
		FirstRelease: row.FirstRelease,
		LastRelease:  row.LastRelease,
		Regressed:    row.Regressed == 1,
		Assignee:     row.Assignee,
	}
}

func (s *Store) GetIssue(ctx context.Context, id int64) (*Issue, error) {
	var row IssueRow
	err := s.DB.WithContext(ctx).First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	iss := issueFromRow(&row)
	list := []Issue{iss}
	if err := s.attachIssueTagFacets(ctx, list); err != nil {
		return &iss, nil
	}
	return &list[0], nil
}

func (s *Store) UpdateIssueStatus(ctx context.Context, id int64, status string) error {
	updates := map[string]any{
		"status":    status,
		"regressed": 0,
	}
	if status == "resolved" {
		updates["resolved_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	} else {
		updates["resolved_at"] = nil
	}
	return s.DB.WithContext(ctx).Model(&IssueRow{}).Where("id = ?", id).Updates(updates).Error
}

func (s *Store) UpdateIssueAssignee(ctx context.Context, id int64, assignee string) error {
	var val any
	if assignee == "" {
		val = nil
	} else {
		val = assignee
	}
	return s.DB.WithContext(ctx).Model(&IssueRow{}).Where("id = ?", id).Update("assignee", val).Error
}

func (s *Store) ListEventsForIssue(ctx context.Context, issueID int64, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []EventRow
	err := s.DB.WithContext(ctx).
		Where("issue_id = ?", issueID).
		Order("timestamp DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return eventsFromRows(rows), nil
}

func (s *Store) GetLatestEvent(ctx context.Context, issueID int64) (*Event, error) {
	events, err := s.ListEventsForIssue(ctx, issueID, 1)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	ev := events[0]
	tags, err := s.GetEventTags(ctx, ev.EventID)
	if err != nil {
		return &ev, nil
	}
	ev.Tags = tags
	return &ev, nil
}

func (s *Store) GetEvent(ctx context.Context, eventID string) (*Event, error) {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return nil, nil
	}
	var row EventRow
	err := s.DB.WithContext(ctx).Where("event_id = ?", eventID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ev := eventsFromRows([]EventRow{row})[0]
	tags, err := s.GetEventTags(ctx, ev.EventID)
	if err != nil {
		return &ev, nil
	}
	ev.Tags = tags
	return &ev, nil
}

func (s *Store) GetEventTags(ctx context.Context, eventID string) (map[string]string, error) {
	var rows []EventTagRow
	err := s.DB.WithContext(ctx).Where("event_id = ?", eventID).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, r := range rows {
		out[r.Key] = r.Value
	}
	return out, nil
}

func eventsFromRows(rows []EventRow) []Event {
	out := make([]Event, 0, len(rows))
	for _, r := range rows {
		out = append(out, Event{
			ID:            r.ID,
			EventID:       r.EventID,
			IssueID:       r.IssueID,
			ProjectID:     r.ProjectID,
			Timestamp:     r.Timestamp,
			Environment:   r.Environment,
			Release:       r.Release,
			Platform:      r.Platform,
			Message:       r.Message,
			ExceptionType: r.ExceptionType,
			Culprit:       r.Culprit,
			UserID:        r.UserID,
			UserEmail:     r.UserEmail,
			TraceID:       r.TraceID,
			RawPath:       r.RawPath,
			PayloadJSON:   r.PayloadJSON,
		})
	}
	return out
}

type UpsertEventInput struct {
	EventID       string
	ProjectID     int64
	Fingerprint   string
	Title         string
	Culprit       string
	Level         string
	Timestamp     time.Time
	Environment   string
	Release       string
	Platform      string
	Message       string
	ExceptionType string
	UserID        string
	UserEmail     string
	TraceID       string
	RawPath       string
	PayloadJSON   string
	Tags          map[string]string
}

type UpsertResult struct {
	IssueID   int64
	IsNew     bool
	Regressed bool
	Ignored   bool
}

func (s *Store) UpsertEvent(ctx context.Context, in UpsertEventInput) (*UpsertResult, error) {
	result := &UpsertResult{}
	ts := in.Timestamp.UTC().Format(time.RFC3339Nano)

	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var issue IssueRow
		q := tx.Where("project_id = ? AND fingerprint = ?", in.ProjectID, in.Fingerprint).Limit(1).Find(&issue)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected == 0 {
			issue = IssueRow{
				ProjectID:    in.ProjectID,
				Fingerprint:  in.Fingerprint,
				Title:        in.Title,
				Culprit:      in.Culprit,
				Status:       "open",
				Level:        in.Level,
				Count:        1,
				FirstSeen:    ts,
				LastSeen:     ts,
				FirstRelease: strPtrOrNil(in.Release),
				LastRelease:  strPtrOrNil(in.Release),
				Regressed:    0,
			}
			if err := tx.Create(&issue).Error; err != nil {
				return err
			}
			result.IsNew = true
			result.IssueID = issue.ID
		} else {
			result.IssueID = issue.ID
			newStatus := issue.Status
			regressed := issue.Regressed
			if issue.Status == "ignored" {
				result.Ignored = true
			} else if issue.Status == "resolved" && shouldRegress(in.Release, issue.LastRelease, issue.ResolvedAt, in.Timestamp) {
				newStatus = "open"
				regressed = 1
				result.Regressed = true
			}
			firstRel := issue.FirstRelease
			lastRel := issue.LastRelease
			if in.Release != "" {
				if firstRel == nil {
					firstRel = strPtrOrNil(in.Release)
				}
				lastRel = strPtrOrNil(in.Release)
			}
			updates := map[string]any{
				"count":         gorm.Expr("count + 1"),
				"last_seen":     ts,
				"status":        newStatus,
				"regressed":     regressed,
				"title":         in.Title,
				"culprit":       in.Culprit,
				"first_release": firstRel,
				"last_release":  lastRel,
			}
			if result.Regressed {
				updates["resolved_at"] = nil
				updates["regressed"] = 1
			}
			if err := tx.Model(&IssueRow{}).Where("id = ?", issue.ID).Updates(updates).Error; err != nil {
				return err
			}
		}

		ev := EventRow{
			EventID:       in.EventID,
			IssueID:       result.IssueID,
			ProjectID:     in.ProjectID,
			Timestamp:     ts,
			Environment:   strPtrOrNil(in.Environment),
			Release:       strPtrOrNil(in.Release),
			Platform:      strPtrOrNil(in.Platform),
			Message:       strPtrOrNil(in.Message),
			ExceptionType: strPtrOrNil(in.ExceptionType),
			Culprit:       strPtrOrNil(in.Culprit),
			UserID:        strPtrOrNil(in.UserID),
			UserEmail:     strPtrOrNil(in.UserEmail),
			TraceID:       strPtrOrNil(in.TraceID),
			RawPath:       in.RawPath,
			PayloadJSON:   in.PayloadJSON,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ev).Error; err != nil {
			return err
		}

		seen := map[string]bool{}
		addTag := func(k, v string) error {
			if k == "" || v == "" || seen[k] {
				return nil
			}
			seen[k] = true
			return tx.Create(&EventTagRow{
				EventID:   in.EventID,
				IssueID:   result.IssueID,
				ProjectID: in.ProjectID,
				Key:       k,
				Value:     v,
			}).Error
		}
		for k, v := range in.Tags {
			if err := addTag(k, v); err != nil {
				return err
			}
		}
		if err := addTag("environment", in.Environment); err != nil {
			return err
		}
		return addTag("release", in.Release)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullOrNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// shouldRegress: newer/different release than last_release, OR quiet window elapsed since resolve.
func shouldRegress(eventRelease string, lastRelease, resolvedAt *string, eventTS time.Time) bool {
	if eventRelease != "" && lastRelease != nil && eventRelease != *lastRelease {
		return true
	}
	if resolvedAt == nil || *resolvedAt == "" {
		return true // no resolved_at → treat as regression (legacy rows)
	}
	rt, err := time.Parse(time.RFC3339Nano, *resolvedAt)
	if err != nil {
		rt, err = time.Parse(time.RFC3339, *resolvedAt)
		if err != nil {
			return true
		}
	}
	return eventTS.Sub(rt) >= QuietWindow
}
