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

const DemoPublicKey = "a1b2c3d4e5f6789012345678abcdef01"

func FormatDSN(publicHost, publicKey string, projectID int64) string {
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
	return fmt.Sprintf("%s://%s@%s/%d", scheme, publicKey, host, projectID)
}

func defaultDemoOrigins() []string {
	return []string{
		"http://localhost:5173",
		"http://localhost:3000",
		"http://localhost:8080",
	}
}

func (s *Store) ProjectExists(ctx context.Context, projectID int64) (bool, error) {
	if projectID <= 0 {
		return false, nil
	}
	var n int64
	err := s.DB.WithContext(ctx).Model(&ProjectRow{}).Where("id = ?", projectID).Count(&n).Error
	return n > 0, err
}

func (s *Store) CreateProject(ctx context.Context, name, slug, publicHost string, allowedOrigins []string) (*CreatedProject, error) {
	return s.createProjectWithKey(ctx, name, slug, publicHost, allowedOrigins, "", "")
}

func (s *Store) insertProject(ctx context.Context, name, slug, publicHost string, allowedOrigins []string, pub, sec string) (*CreatedProject, error) {
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

	var projectID int64
	var createdAt string
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		proj := ProjectRow{
			OrganizationID:   org.ID,
			Slug:             slug,
			Name:             name,
			AllowedOrigins:   originsJSON,
			GroupingConfig:   "sentry-lite:2026-09-01",
			FingerprintRules: "",
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

	return &CreatedProject{
		Project: Project{
			ID:               projectID,
			Slug:             slug,
			Name:             name,
			AllowedOrigins:   allowedOrigins,
			GroupingConfig:   "sentry-lite:2026-09-01",
			FingerprintRules: "",
			CreatedAt:        createdAt,
		},
		PublicKey: pub,
		SecretKey: sec,
		DSN:       FormatDSN(publicHost, pub, projectID),
	}, nil
}

// SeedDemoProject creates the documented first-boot demo project when the DB
// has no projects. The public key matches the README seed DSN.
func (s *Store) SeedDemoProject(ctx context.Context, publicHost string) (*CreatedProject, error) {
	var n int64
	if err := s.DB.WithContext(ctx).Model(&ProjectRow{}).Count(&n).Error; err != nil {
		return nil, err
	}
	if n > 0 {
		return nil, nil
	}
	created, err := s.createProjectWithKey(ctx, "Demo", "demo", publicHost, defaultDemoOrigins(), DemoPublicKey, "")
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *Store) createProjectWithKey(ctx context.Context, name, slug, publicHost string, allowedOrigins []string, pub, sec string) (*CreatedProject, error) {
	if pub == "" {
		var err error
		pub, err = randomKey()
		if err != nil {
			return nil, err
		}
	}
	if sec == "" {
		var err error
		sec, err = randomKey()
		if err != nil {
			return nil, err
		}
	}
	return s.insertProject(ctx, name, slug, publicHost, allowedOrigins, pub, sec)
}

func (s *Store) GetProject(ctx context.Context, id int64) (*Project, error) {
	var row ProjectRow
	err := s.DB.WithContext(ctx).First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &Project{
		ID:               row.ID,
		Slug:             row.Slug,
		Name:             row.Name,
		AllowedOrigins:   decodeOriginsJSON(row.AllowedOrigins),
		GroupingConfig:   row.GroupingConfig,
		FingerprintRules: row.FingerprintRules,
		CreatedAt:        row.CreatedAt,
	}, nil
}

func (s *Store) GetProjectDSN(ctx context.Context, id int64, publicHost string) (*CreatedProject, error) {
	proj, err := s.GetProject(ctx, id)
	if err != nil || proj == nil {
		return nil, err
	}
	key, err := s.latestProjectKey(ctx, id)
	if err != nil || key == nil {
		return nil, err
	}
	return &CreatedProject{
		Project:   *proj,
		PublicKey: key.PublicKey,
		SecretKey: key.SecretKey,
		DSN:       FormatDSN(publicHost, key.PublicKey, id),
	}, nil
}

func (s *Store) latestProjectKey(ctx context.Context, projectID int64) (*ProjectKey, error) {
	var row ProjectKeyRow
	err := s.DB.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("id DESC").
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

func (s *Store) UpdateProject(ctx context.Context, id int64, in ProjectUpdate) (*Project, error) {
	updates := map[string]any{}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return nil, fmt.Errorf("name required")
		}
		updates["name"] = name
	}
	if in.AllowedOrigins != nil {
		updates["allowed_origins"] = encodeOriginsJSON(*in.AllowedOrigins)
	}
	if in.GroupingConfig != nil {
		updates["grouping_config"] = strings.TrimSpace(*in.GroupingConfig)
	}
	if in.FingerprintRules != nil {
		updates["fingerprint_rules"] = *in.FingerprintRules
	}
	if len(updates) == 0 {
		return s.GetProject(ctx, id)
	}
	exists, err := s.ProjectExists(ctx, id)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	res := s.DB.WithContext(ctx).Model(&ProjectRow{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	return s.GetProject(ctx, id)
}

func (s *Store) RotateProjectKey(ctx context.Context, id int64, publicHost string) (*CreatedProject, error) {
	exists, err := s.ProjectExists(ctx, id)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	pub, err := randomKey()
	if err != nil {
		return nil, err
	}
	sec, err := randomKey()
	if err != nil {
		return nil, err
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("project_id = ?", id).Delete(&ProjectKeyRow{}).Error; err != nil {
			return err
		}
		return tx.Create(&ProjectKeyRow{
			ProjectID: id,
			PublicKey: pub,
			SecretKey: sec,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return s.GetProjectDSN(ctx, id, publicHost)
}

func (s *Store) DeleteProject(ctx context.Context, id int64) (rawPaths []string, err error) {
	exists, err := s.ProjectExists(ctx, id)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	var eventPaths []string
	if err := s.DB.WithContext(ctx).Model(&EventRow{}).Where("project_id = ?", id).Pluck("raw_path", &eventPaths).Error; err != nil {
		return nil, err
	}
	var txPaths []string
	if err := s.DB.WithContext(ctx).Model(&TransactionRow{}).Where("project_id = ?", id).Pluck("raw_path", &txPaths).Error; err != nil {
		return nil, err
	}
	rawPaths = append(eventPaths, txPaths...)

	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ruleIDs []int64
		if err := tx.Model(&AlertRuleRow{}).Where("project_id = ?", id).Pluck("id", &ruleIDs).Error; err != nil {
			return err
		}
		if len(ruleIDs) > 0 {
			if err := tx.Where("rule_id IN ?", ruleIDs).Delete(&AlertDeliveryRow{}).Error; err != nil {
				return err
			}
			if err := tx.Where("project_id = ?", id).Delete(&AlertRuleRow{}).Error; err != nil {
				return err
			}
		}
		var monitorIDs []int64
		if err := tx.Model(&CronMonitorRow{}).Where("project_id = ?", id).Pluck("id", &monitorIDs).Error; err != nil {
			return err
		}
		if len(monitorIDs) > 0 {
			if err := tx.Where("monitor_id IN ?", monitorIDs).Delete(&CronCheckinRow{}).Error; err != nil {
				return err
			}
			if err := tx.Where("project_id = ?", id).Delete(&CronMonitorRow{}).Error; err != nil {
				return err
			}
		}
		var txEventIDs []string
		if err := tx.Model(&TransactionRow{}).Where("project_id = ?", id).Pluck("event_id", &txEventIDs).Error; err != nil {
			return err
		}
		if len(txEventIDs) > 0 {
			if err := tx.Where("transaction_event_id IN ?", txEventIDs).Delete(&SpanRow{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("project_id = ?", id).Delete(&TransactionStatRow{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", id).Delete(&TransactionRow{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", id).Delete(&EventTagRow{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", id).Delete(&EventRow{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", id).Delete(&IssueHashRow{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", id).Delete(&IssueRow{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", id).Delete(&ReleaseRow{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", id).Delete(&ProjectKeyRow{}).Error; err != nil {
			return err
		}
		return tx.Delete(&ProjectRow{}, id).Error
	})
	if err != nil {
		return nil, err
	}
	if rawPaths == nil {
		rawPaths = []string{}
	}
	return rawPaths, nil
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
