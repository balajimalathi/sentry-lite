package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type CreatedProject struct {
	Project   Project `json:"project"`
	PublicKey string  `json:"public_key"`
	SecretKey string  `json:"secret_key"`
	DSN       string  `json:"dsn"`
}

var slugRe = regexp.MustCompile(`[^a-z0-9-]+`)

func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	s = slugRe.ReplaceAllString(s, "")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "project"
	}
	if len(s) > 48 {
		s = s[:48]
	}
	return s
}

func normalizeOrigins(origins []string) []string {
	out := make([]string, 0, len(origins))
	seen := map[string]bool{}
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o == "" || seen[o] {
			continue
		}
		seen[o] = true
		out = append(out, o)
	}
	return out
}

func encodeOriginsJSON(origins []string) string {
	origins = normalizeOrigins(origins)
	b, err := json.Marshal(origins)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func decodeOriginsJSON(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []string{}
	}
	return normalizeOrigins(out)
}

func randomKey() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *Store) CreateProject(ctx context.Context, name, slug, publicHost string, allowedOrigins []string) (*CreatedProject, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name required")
	}
	if slug == "" {
		slug = name
	}
	slug = Slugify(slug)
	allowedOrigins = normalizeOrigins(allowedOrigins)
	originsJSON := encodeOriginsJSON(allowedOrigins)

	var orgID int64
	err := s.DB.QueryRowContext(ctx, `SELECT id FROM organizations ORDER BY id ASC LIMIT 1`).Scan(&orgID)
	if err == sql.ErrNoRows {
		res, err := s.DB.ExecContext(ctx, `INSERT INTO organizations (slug, name) VALUES (?, ?)`, "default", "Default")
		if err != nil {
			return nil, err
		}
		orgID, _ = res.LastInsertId()
	} else if err != nil {
		return nil, err
	}

	// Ensure unique slug
	base := slug
	for i := 0; i < 50; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i+1)
		}
		var exists int
		if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE slug = ?`, candidate).Scan(&exists); err != nil {
			return nil, err
		}
		if exists == 0 {
			slug = candidate
			break
		}
	}

	pub, err := randomKey()
	if err != nil {
		return nil, err
	}
	sec, err := randomKey()
	if err != nil {
		return nil, err
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO projects (organization_id, slug, name, allowed_origins) VALUES (?, ?, ?, ?)`,
		orgID, slug, name, originsJSON,
	)
	if err != nil {
		return nil, err
	}
	projectID, _ := res.LastInsertId()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO project_keys (project_id, public_key, secret_key) VALUES (?, ?, ?)`,
		projectID, pub, sec,
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	var createdAt string
	_ = s.DB.QueryRowContext(ctx, `SELECT created_at FROM projects WHERE id = ?`, projectID).Scan(&createdAt)
	if publicHost == "" {
		publicHost = "http://localhost:8080"
	}
	publicHost = strings.TrimRight(publicHost, "/")
	// DSN host without scheme for classic form, but Sentry accepts full URL style we already use
	host := strings.TrimPrefix(publicHost, "http://")
	host = strings.TrimPrefix(host, "https://")
	scheme := "http"
	if strings.HasPrefix(publicHost, "https://") {
		scheme = "https"
	}

	return &CreatedProject{
		Project: Project{
			ID:             projectID,
			Slug:           slug,
			Name:           name,
			AllowedOrigins: allowedOrigins,
			CreatedAt:      createdAt,
		},
		PublicKey: pub,
		SecretKey: sec,
		DSN:       fmt.Sprintf("%s://%s@%s/%d", scheme, pub, host, projectID),
	}, nil
}

// ProjectAllowedOrigins returns the CORS allowlist for a project.
// An empty slice means allow any Origin (Sentry-like).
// A nil slice with nil error means the project does not exist.
func (s *Store) ProjectAllowedOrigins(ctx context.Context, projectID int64) ([]string, error) {
	var raw string
	err := s.DB.QueryRowContext(ctx,
		`SELECT allowed_origins FROM projects WHERE id = ?`, projectID,
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return decodeOriginsJSON(raw), nil
}

type Facets struct {
	Environments []string `json:"environments"`
	Releases     []string `json:"releases"`
	Tags         []string `json:"tags"`
}

func (s *Store) ListFacets(ctx context.Context, projectID int64) (*Facets, error) {
	out := &Facets{
		Environments: []string{},
		Releases:     []string{},
		Tags:         []string{},
	}

	envQ := `SELECT DISTINCT value FROM event_tags WHERE key = 'environment'`
	relQ := `SELECT DISTINCT version FROM releases`
	tagQ := `SELECT DISTINCT key || ':' || value FROM event_tags WHERE key NOT IN ('environment', 'release')`
	argsEnv := []any{}
	argsRel := []any{}
	argsTag := []any{}
	if projectID > 0 {
		envQ += ` AND project_id = ?`
		relQ += ` WHERE project_id = ?`
		tagQ += ` AND project_id = ?`
		argsEnv = append(argsEnv, projectID)
		argsRel = append(argsRel, projectID)
		argsTag = append(argsTag, projectID)
	}
	envQ += ` ORDER BY value`
	relQ += ` ORDER BY version DESC`
	tagQ += ` ORDER BY 1 LIMIT 200`

	envs, err := scanStringCol(ctx, s.DB, envQ, argsEnv...)
	if err != nil {
		return nil, err
	}
	out.Environments = envs

	rels, err := scanStringCol(ctx, s.DB, relQ, argsRel...)
	if err != nil {
		return nil, err
	}
	// Also include release values from event_tags if not in releases table
	extraRelQ := `SELECT DISTINCT value FROM event_tags WHERE key = 'release'`
	extraArgs := []any{}
	if projectID > 0 {
		extraRelQ += ` AND project_id = ?`
		extraArgs = append(extraArgs, projectID)
	}
	extraRelQ += ` ORDER BY value DESC`
	extra, err := scanStringCol(ctx, s.DB, extraRelQ, extraArgs...)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	merged := make([]string, 0, len(rels)+len(extra))
	for _, v := range append(rels, extra...) {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		merged = append(merged, v)
	}
	out.Releases = merged

	tags, err := scanStringCol(ctx, s.DB, tagQ, argsTag...)
	if err != nil {
		return nil, err
	}
	out.Tags = tags
	return out, nil
}

func scanStringCol(ctx context.Context, db *sql.DB, query string, args ...any) ([]string, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		if v != "" {
			out = append(out, v)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out, rows.Err()
}
