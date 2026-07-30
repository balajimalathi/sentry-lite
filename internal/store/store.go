package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type Store struct {
	DB *sql.DB
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
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.seed(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}

func (s *Store) migrate() error {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	// Sort by name so 001 then 002
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		b, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		for _, stmt := range splitSQL(string(b)) {
			if _, err := s.DB.Exec(stmt); err != nil {
				msg := err.Error()
				if strings.Contains(msg, "duplicate column") || strings.Contains(msg, "already exists") {
					continue
				}
				return fmt.Errorf("migrate %s: %w", name, err)
			}
		}
	}
	return nil
}

func splitSQL(s string) []string {
	parts := strings.Split(s, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// drop leading/inline comment-only lines
		lines := strings.Split(p, "\n")
		var kept []string
		for _, line := range lines {
			trim := strings.TrimSpace(line)
			if trim == "" || strings.HasPrefix(trim, "--") {
				continue
			}
			kept = append(kept, line)
		}
		p = strings.TrimSpace(strings.Join(kept, "\n"))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

const (
	SeedPublicKey = "a1b2c3d4e5f6789012345678abcdef01"
	SeedSecretKey = "deadbeefdeadbeefdeadbeefdeadbeef"
)

func (s *Store) seed() error {
	var n int
	if err := s.DB.QueryRow(`SELECT COUNT(*) FROM organizations`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec(`INSERT INTO organizations (slug, name) VALUES (?, ?)`, "default", "Default")
	if err != nil {
		return err
	}
	orgID, _ := res.LastInsertId()
	seedOrigins := `["http://localhost:5173","http://localhost:3000","http://localhost:8080"]`
	res, err = tx.Exec(
		`INSERT INTO projects (organization_id, slug, name, allowed_origins) VALUES (?, ?, ?, ?)`,
		orgID, "demo", "Demo Project", seedOrigins,
	)
	if err != nil {
		return err
	}
	projectID, _ := res.LastInsertId()
	_, err = tx.Exec(
		`INSERT INTO project_keys (project_id, public_key, secret_key) VALUES (?, ?, ?)`,
		projectID, SeedPublicKey, SeedSecretKey,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) LookupProjectKey(ctx context.Context, publicKey string, projectID int64) (*ProjectKey, error) {
	var pk ProjectKey
	err := s.DB.QueryRowContext(ctx,
		`SELECT project_id, public_key, secret_key FROM project_keys WHERE public_key = ? AND project_id = ?`,
		publicKey, projectID,
	).Scan(&pk.ProjectID, &pk.PublicKey, &pk.SecretKey)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &pk, nil
}

func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT p.id, p.slug, p.name, p.allowed_origins, p.created_at,
		       COALESCE((SELECT COUNT(*) FROM issues i WHERE i.project_id = p.id), 0) AS issue_count,
		       (SELECT MAX(i.last_seen) FROM issues i WHERE i.project_id = p.id) AS latest_activity
		FROM projects p
		ORDER BY p.id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		var originsJSON string
		var latest sql.NullString
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &originsJSON, &p.CreatedAt, &p.IssueCount, &latest); err != nil {
			return nil, err
		}
		p.AllowedOrigins = decodeOriginsJSON(originsJSON)
		if latest.Valid {
			p.LatestActivityAt = &latest.String
		}
		out = append(out, p)
	}
	return out, rows.Err()
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

	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Issue
	for rows.Next() {
		iss, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *iss)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.attachIssueTagFacets(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// attachIssueTagFacets fills Environments and Tags (key:value, excluding release) on each issue.
func (s *Store) attachIssueTagFacets(ctx context.Context, issues []Issue) error {
	if len(issues) == 0 {
		return nil
	}
	byID := make(map[int64]int, len(issues))
	placeholders := make([]string, len(issues))
	args := make([]any, len(issues))
	for i := range issues {
		byID[issues[i].ID] = i
		placeholders[i] = "?"
		args[i] = issues[i].ID
	}
	q := `SELECT DISTINCT issue_id, key, value FROM event_tags WHERE issue_id IN (` +
		strings.Join(placeholders, ",") + `) ORDER BY key, value`
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var issueID int64
		var key, value string
		if err := rows.Scan(&issueID, &key, &value); err != nil {
			return err
		}
		idx, ok := byID[issueID]
		if !ok || value == "" {
			continue
		}
		switch key {
		case "environment":
			issues[idx].Environments = append(issues[idx].Environments, value)
		case "release":
			// Releases are already on first_release / last_release.
		default:
			issues[idx].Tags = append(issues[idx].Tags, key+":"+value)
		}
	}
	return rows.Err()
}

type scannable interface {
	Scan(dest ...any) error
}

func scanIssue(row scannable) (*Issue, error) {
	var iss Issue
	var firstRel, lastRel, assignee sql.NullString
	var regressed int
	if err := row.Scan(
		&iss.ID, &iss.ProjectID, &iss.Fingerprint, &iss.Title, &iss.Culprit, &iss.Status, &iss.Level,
		&iss.Count, &iss.FirstSeen, &iss.LastSeen, &firstRel, &lastRel, &regressed, &assignee,
	); err != nil {
		return nil, err
	}
	if firstRel.Valid {
		iss.FirstRelease = &firstRel.String
	}
	if lastRel.Valid {
		iss.LastRelease = &lastRel.String
	}
	if assignee.Valid {
		iss.Assignee = &assignee.String
	}
	iss.Regressed = regressed == 1
	return &iss, nil
}

func (s *Store) GetIssue(ctx context.Context, id int64) (*Issue, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, project_id, fingerprint, title, culprit, status, level, count,
		       first_seen, last_seen, first_release, last_release, regressed, assignee
		FROM issues WHERE id = ?
	`, id)
	iss, err := scanIssue(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	list := []Issue{*iss}
	if err := s.attachIssueTagFacets(ctx, list); err != nil {
		return iss, nil
	}
	return &list[0], nil
}

func (s *Store) UpdateIssueStatus(ctx context.Context, id int64, status string) error {
	if status == "resolved" {
		_, err := s.DB.ExecContext(ctx,
			`UPDATE issues SET status = ?, regressed = 0, resolved_at = ? WHERE id = ?`,
			status, time.Now().UTC().Format(time.RFC3339Nano), id)
		return err
	}
	_, err := s.DB.ExecContext(ctx,
		`UPDATE issues SET status = ?, regressed = 0, resolved_at = NULL WHERE id = ?`,
		status, id)
	return err
}

func (s *Store) UpdateIssueAssignee(ctx context.Context, id int64, assignee string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE issues SET assignee = ? WHERE id = ?`, nullOrNil(assignee), id)
	return err
}

func (s *Store) ListEventsForIssue(ctx context.Context, issueID int64, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, event_id, issue_id, project_id, timestamp, environment, release, platform,
		       message, exception_type, culprit, user_id, user_email, raw_path, payload_json, trace_id
		FROM events WHERE issue_id = ? ORDER BY timestamp DESC LIMIT ?
	`, issueID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
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

func (s *Store) GetEventTags(ctx context.Context, eventID string) (map[string]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT key, value FROM event_tags WHERE event_id = ?`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

func scanEvents(rows *sql.Rows) ([]Event, error) {
	var out []Event
	for rows.Next() {
		var e Event
		var env, rel, plat, msg, exType, culprit, uid, uemail, traceID sql.NullString
		if err := rows.Scan(
			&e.ID, &e.EventID, &e.IssueID, &e.ProjectID, &e.Timestamp,
			&env, &rel, &plat, &msg, &exType, &culprit, &uid, &uemail, &e.RawPath, &e.PayloadJSON, &traceID,
		); err != nil {
			return nil, err
		}
		e.Environment = nullStr(env)
		e.TraceID = nullStr(traceID)
		e.Release = nullStr(rel)
		e.Platform = nullStr(plat)
		e.Message = nullStr(msg)
		e.ExceptionType = nullStr(exType)
		e.Culprit = nullStr(culprit)
		e.UserID = nullStr(uid)
		e.UserEmail = nullStr(uemail)
		out = append(out, e)
	}
	return out, rows.Err()
}

func nullStr(ns sql.NullString) *string {
	if ns.Valid {
		return &ns.String
	}
	return nil
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
}

func (s *Store) UpsertEvent(ctx context.Context, in UpsertEventInput) (*UpsertResult, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	ts := in.Timestamp.UTC().Format(time.RFC3339Nano)

	var issueID int64
	var status string
	var firstRelease, lastRelease, resolvedAt sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT id, status, first_release, last_release, resolved_at FROM issues WHERE project_id = ? AND fingerprint = ?`,
		in.ProjectID, in.Fingerprint,
	).Scan(&issueID, &status, &firstRelease, &lastRelease, &resolvedAt)

	result := &UpsertResult{}
	if err == sql.ErrNoRows {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO issues (project_id, fingerprint, title, culprit, status, level, count, first_seen, last_seen, first_release, last_release, regressed)
			VALUES (?, ?, ?, ?, 'open', ?, 1, ?, ?, ?, ?, 0)
		`, in.ProjectID, in.Fingerprint, in.Title, in.Culprit, in.Level, ts, ts, nullOrNil(in.Release), nullOrNil(in.Release))
		if err != nil {
			return nil, err
		}
		issueID, _ = res.LastInsertId()
		result.IsNew = true
		result.IssueID = issueID
	} else if err != nil {
		return nil, err
	} else {
		result.IssueID = issueID
		newStatus := status
		regressedSQL := "regressed"
		if status == "resolved" && shouldRegress(in.Release, lastRelease, resolvedAt, in.Timestamp) {
			newStatus = "open"
			regressedSQL = "1"
			result.Regressed = true
		}
		firstRel := firstRelease
		lastRel := lastRelease
		if in.Release != "" {
			if !firstRel.Valid {
				firstRel = sql.NullString{String: in.Release, Valid: true}
			}
			lastRel = sql.NullString{String: in.Release, Valid: true}
		}
		if result.Regressed {
			_, err = tx.ExecContext(ctx, `
				UPDATE issues SET count = count + 1, last_seen = ?, status = ?, regressed = 1,
				       title = ?, culprit = ?, first_release = ?, last_release = ?, resolved_at = NULL
				WHERE id = ?`,
				ts, newStatus, in.Title, in.Culprit, nullStrVal(firstRel), nullStrVal(lastRel), issueID)
		} else {
			_, err = tx.ExecContext(ctx, `
				UPDATE issues SET count = count + 1, last_seen = ?, status = ?, regressed = `+regressedSQL+`,
				       title = ?, culprit = ?, first_release = ?, last_release = ?
				WHERE id = ?`,
				ts, newStatus, in.Title, in.Culprit, nullStrVal(firstRel), nullStrVal(lastRel), issueID)
		}
		if err != nil {
			return nil, err
		}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO events (
			event_id, issue_id, project_id, timestamp, environment, release, platform,
			message, exception_type, culprit, user_id, user_email, raw_path, payload_json, trace_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, in.EventID, issueID, in.ProjectID, ts, nullOrNil(in.Environment), nullOrNil(in.Release), nullOrNil(in.Platform),
		nullOrNil(in.Message), nullOrNil(in.ExceptionType), nullOrNil(in.Culprit),
		nullOrNil(in.UserID), nullOrNil(in.UserEmail), in.RawPath, in.PayloadJSON, nullOrNil(in.TraceID))
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	addTag := func(k, v string) error {
		if k == "" || v == "" || seen[k] {
			return nil
		}
		seen[k] = true
		_, err := tx.ExecContext(ctx,
			`INSERT INTO event_tags (event_id, issue_id, project_id, key, value) VALUES (?, ?, ?, ?, ?)`,
			in.EventID, issueID, in.ProjectID, k, v,
		)
		return err
	}
	for k, v := range in.Tags {
		if err := addTag(k, v); err != nil {
			return nil, err
		}
	}
	if err := addTag("environment", in.Environment); err != nil {
		return nil, err
	}
	if err := addTag("release", in.Release); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func nullOrNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullStrVal(ns sql.NullString) any {
	if ns.Valid {
		return ns.String
	}
	return nil
}

// shouldRegress: newer/different release than last_release, OR quiet window elapsed since resolve.
func shouldRegress(eventRelease string, lastRelease, resolvedAt sql.NullString, eventTS time.Time) bool {
	if eventRelease != "" && lastRelease.Valid && eventRelease != lastRelease.String {
		return true
	}
	if !resolvedAt.Valid || resolvedAt.String == "" {
		return true // no resolved_at → treat as regression (legacy rows)
	}
	rt, err := time.Parse(time.RFC3339Nano, resolvedAt.String)
	if err != nil {
		rt, err = time.Parse(time.RFC3339, resolvedAt.String)
		if err != nil {
			return true
		}
	}
	return eventTS.Sub(rt) >= QuietWindow
}

