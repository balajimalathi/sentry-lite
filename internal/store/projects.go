package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"
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

	var org Organization
	err := s.DB.WithContext(ctx).Order("id ASC").First(&org).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		org = Organization{Slug: "default", Name: "Default"}
		if err := s.DB.WithContext(ctx).Create(&org).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	base := slug
	for i := 0; i < 50; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i+1)
		}
		var n int64
		if err := s.DB.WithContext(ctx).Model(&ProjectRow{}).Where("slug = ?", candidate).Count(&n).Error; err != nil {
			return nil, err
		}
		if n == 0 {
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

	var projectID int64
	var createdAt string
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		proj := ProjectRow{
			OrganizationID: org.ID,
			Slug:           slug,
			Name:           name,
			AllowedOrigins: originsJSON,
		}
		if err := tx.Create(&proj).Error; err != nil {
			return err
		}
		projectID = proj.ID
		createdAt = proj.CreatedAt
		key := ProjectKeyRow{
			ProjectID: proj.ID,
			PublicKey: pub,
			SecretKey: sec,
		}
		return tx.Create(&key).Error
	})
	if err != nil {
		return nil, err
	}

	if createdAt == "" {
		_ = s.DB.WithContext(ctx).Model(&ProjectRow{}).Select("created_at").Where("id = ?", projectID).Scan(&createdAt).Error
	}
	if publicHost == "" {
		publicHost = "http://localhost:8080"
	}
	publicHost = strings.TrimRight(publicHost, "/")
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
	var row ProjectRow
	err := s.DB.WithContext(ctx).Select("allowed_origins").First(&row, projectID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return decodeOriginsJSON(row.AllowedOrigins), nil
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

func scanStringCol(ctx context.Context, db *gorm.DB, query string, args ...any) ([]string, error) {
	var out []string
	if err := db.WithContext(ctx).Raw(query, args...).Scan(&out).Error; err != nil {
		return nil, err
	}
	filtered := make([]string, 0, len(out))
	for _, v := range out {
		if v != "" {
			filtered = append(filtered, v)
		}
	}
	if filtered == nil {
		filtered = []string{}
	}
	return filtered, nil
}
