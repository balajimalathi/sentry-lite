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

type ProjectGrouping struct {
	GroupingConfig   string
	FingerprintRules string
}

type IssueHash struct {
	Hash       string `json:"hash"`
	Variant    string `json:"variant"`
	EventCount int64  `json:"event_count"`
}

type ProjectUpdate struct {
	Name             *string
	AllowedOrigins   *[]string
	GroupingConfig   *string
	FingerprintRules *string
}

func (s *Store) ProjectGrouping(ctx context.Context, projectID int64) (*ProjectGrouping, error) {
	var row ProjectRow
	err := s.DB.WithContext(ctx).Select("grouping_config", "fingerprint_rules").First(&row, projectID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.GroupingConfig == "" {
		row.GroupingConfig = "sentry-lite:v1"
	}
	return &ProjectGrouping{
		GroupingConfig:   row.GroupingConfig,
		FingerprintRules: row.FingerprintRules,
	}, nil
}

func (s *Store) ListIssueHashes(ctx context.Context, issueID int64) ([]IssueHash, error) {
	type scan struct {
		Hash    string
		Variant string
	}
	var rows []scan
	err := s.DB.WithContext(ctx).Raw(
		`SELECT hash, variant FROM issue_hashes WHERE issue_id = ? ORDER BY id ASC`,
		issueID,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]IssueHash, 0, len(rows))
	for _, r := range rows {
		var n int64
		q := s.DB.WithContext(ctx).Model(&EventRow{}).Where("issue_id = ?", issueID)
		if r.Hash != "" {
			q = q.Where("grouping_hash = ?", r.Hash)
		}
		if err := q.Count(&n).Error; err != nil {
			return nil, err
		}
		out = append(out, IssueHash{Hash: r.Hash, Variant: r.Variant, EventCount: n})
	}
	if len(out) == 0 {
		var issue IssueRow
		if err := s.DB.WithContext(ctx).Select("fingerprint", "count").First(&issue, issueID).Error; err == nil && issue.Fingerprint != "" {
			out = append(out, IssueHash{Hash: issue.Fingerprint, Variant: "v1", EventCount: issue.Count})
		}
	}
	return out, nil
}

func findIssueForEvent(tx *gorm.DB, in UpsertEventInput) (*IssueRow, error) {
	hashVals := hashValues(in)
	if len(hashVals) > 0 {
		var rows []IssueHashRow
		if err := tx.Where("project_id = ? AND hash IN ?", in.ProjectID, hashVals).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			var issue IssueRow
			if err := tx.First(&issue, r.IssueID).Error; err != nil {
				continue
			}
			resolved, err := resolveMergedIssue(tx, &issue)
			if err != nil || resolved == nil {
				continue
			}
			return resolved, nil
		}
	}
	if in.Fingerprint == "" {
		return nil, nil
	}
	var issue IssueRow
	q := tx.Where("project_id = ? AND fingerprint = ?", in.ProjectID, in.Fingerprint).Limit(1).Find(&issue)
	if q.Error != nil {
		return nil, q.Error
	}
	if q.RowsAffected == 0 {
		return nil, nil
	}
	return resolveMergedIssue(tx, &issue)
}

func hashValues(in UpsertEventInput) []string {
	seen := map[string]bool{}
	var out []string
	add := func(h string) {
		if h == "" || seen[h] {
			return
		}
		seen[h] = true
		out = append(out, h)
	}
	for _, h := range in.Hashes {
		add(h.Hash)
	}
	add(in.Fingerprint)
	add(in.GroupingHash)
	return out
}

func insertMissingHashes(tx *gorm.DB, projectID, issueID int64, hashes []IssueHashInput, primary, variant string) error {
	if len(hashes) == 0 && primary != "" {
		hashes = []IssueHashInput{{Hash: primary, Variant: variant}}
	}
	for _, h := range hashes {
		if h.Hash == "" {
			continue
		}
		row := IssueHashRow{
			ProjectID: projectID,
			Hash:      h.Hash,
			IssueID:   issueID,
			Variant:   h.Variant,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "project_id"}, {Name: "hash"}},
			DoNothing: true,
		}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func resolveMergedIssue(tx *gorm.DB, issue *IssueRow) (*IssueRow, error) {
	seen := map[int64]bool{}
	cur := issue
	for i := 0; i < 10 && cur != nil && cur.Status == "merged" && cur.MergedInto != nil; i++ {
		if seen[cur.ID] {
			break
		}
		seen[cur.ID] = true
		var next IssueRow
		if err := tx.First(&next, *cur.MergedInto).Error; err != nil {
			return cur, nil
		}
		cur = &next
	}
	return cur, nil
}

func (s *Store) MergeIssues(ctx context.Context, destID int64, sourceIDs []int64) (*Issue, error) {
	if destID <= 0 {
		return nil, fmt.Errorf("destination required")
	}
	seen := map[int64]bool{destID: true}
	var ids []int64
	for _, id := range sourceIDs {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("at least one source issue required")
	}

	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var dest IssueRow
		if err := tx.First(&dest, destID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("destination not found")
			}
			return err
		}
		if dest.Status == "merged" {
			return fmt.Errorf("cannot merge into a merged issue")
		}
		var sources []IssueRow
		if err := tx.Where("id IN ?", ids).Find(&sources).Error; err != nil {
			return err
		}
		if len(sources) != len(ids) {
			return fmt.Errorf("source issue not found")
		}
		for _, src := range sources {
			if src.ProjectID != dest.ProjectID {
				return fmt.Errorf("issues must belong to the same project")
			}
			if src.Status == "merged" {
				return fmt.Errorf("cannot merge a merged issue")
			}
		}

		if err := tx.Model(&EventRow{}).Where("issue_id IN ?", ids).Updates(map[string]any{
			"issue_id": destID,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&EventTagRow{}).Where("issue_id IN ?", ids).Updates(map[string]any{
			"issue_id": destID,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&IssueHashRow{}).Where("issue_id IN ?", ids).Updates(map[string]any{
			"issue_id": destID,
		}).Error; err != nil {
			return err
		}

		for _, src := range sources {
			fp := fmt.Sprintf("merged:%d:%s", src.ID, src.Fingerprint)
			if err := tx.Model(&IssueRow{}).Where("id = ?", src.ID).Updates(map[string]any{
				"status":      "merged",
				"merged_into": destID,
				"fingerprint": fp,
				"count":       0,
			}).Error; err != nil {
				return err
			}
		}
		return recomputeIssueStatsTx(tx, destID)
	})
	if err != nil {
		return nil, err
	}
	return s.GetIssue(ctx, destID)
}

func (s *Store) UnmergeIssueHashes(ctx context.Context, issueID int64, hashes []string) ([]Issue, error) {
	want := map[string]bool{}
	var uniq []string
	for _, h := range hashes {
		h = strings.TrimSpace(h)
		if h == "" || want[h] {
			continue
		}
		want[h] = true
		uniq = append(uniq, h)
	}
	if len(uniq) == 0 {
		return nil, fmt.Errorf("hashes required")
	}

	var createdIDs []int64
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var dest IssueRow
		if err := tx.First(&dest, issueID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("issue not found")
			}
			return err
		}
		if dest.Status == "merged" {
			return fmt.Errorf("cannot unmerge a merged issue")
		}

		var all []IssueHashRow
		if err := tx.Where("issue_id = ?", issueID).Find(&all).Error; err != nil {
			return err
		}
		if len(all) <= 1 {
			return fmt.Errorf("cannot split events that share one grouping hash")
		}
		remain := 0
		byHash := map[string]IssueHashRow{}
		for _, row := range all {
			byHash[row.Hash] = row
			if !want[row.Hash] {
				remain++
			}
		}
		if remain == 0 {
			return fmt.Errorf("cannot unmerge every hash from an issue")
		}
		for _, h := range uniq {
			if _, ok := byHash[h]; !ok {
				return fmt.Errorf("hash %s is not on this issue", h)
			}
		}

		now := time.Now().UTC().Format(time.RFC3339Nano)
		for _, h := range uniq {
			row := byHash[h]
			created := IssueRow{
				ProjectID:   dest.ProjectID,
				Fingerprint: h,
				Title:       dest.Title,
				Culprit:     dest.Culprit,
				Status:      "open",
				Level:       dest.Level,
				Count:       0,
				FirstSeen:   now,
				LastSeen:    now,
			}
			if err := tx.Create(&created).Error; err != nil {
				return err
			}
			if err := tx.Model(&IssueHashRow{}).Where("id = ?", row.ID).Updates(map[string]any{
				"issue_id": created.ID,
			}).Error; err != nil {
				return err
			}
			if err := tx.Model(&EventRow{}).Where("issue_id = ? AND grouping_hash = ?", issueID, h).Updates(map[string]any{
				"issue_id": created.ID,
			}).Error; err != nil {
				return err
			}
			if err := tx.Exec(
				`UPDATE event_tags SET issue_id = ? WHERE event_id IN (SELECT event_id FROM events WHERE issue_id = ?)`,
				created.ID, created.ID,
			).Error; err != nil {
				return err
			}
			if err := recomputeIssueStatsTx(tx, created.ID); err != nil {
				return err
			}
			createdIDs = append(createdIDs, created.ID)
		}
		return recomputeIssueStatsTx(tx, issueID)
	})
	if err != nil {
		return nil, err
	}

	out := make([]Issue, 0, len(createdIDs))
	for _, id := range createdIDs {
		iss, err := s.GetIssue(ctx, id)
		if err != nil {
			return nil, err
		}
		if iss != nil {
			out = append(out, *iss)
		}
	}
	return out, nil
}

func recomputeIssueStatsTx(tx *gorm.DB, issueID int64) error {
	type agg struct {
		Count     int64
		FirstSeen string
		LastSeen  string
	}
	var a agg
	err := tx.Raw(`
		SELECT COUNT(*) AS count,
		       COALESCE(MIN(timestamp), '') AS first_seen,
		       COALESCE(MAX(timestamp), '') AS last_seen
		FROM events WHERE issue_id = ?
	`, issueID).Scan(&a).Error
	if err != nil {
		return err
	}
	var firstRel, lastRel *string
	if a.Count > 0 {
		_ = tx.Raw(`SELECT release FROM events WHERE issue_id = ? AND release IS NOT NULL AND release != '' ORDER BY timestamp ASC LIMIT 1`, issueID).Scan(&firstRel).Error
		_ = tx.Raw(`SELECT release FROM events WHERE issue_id = ? AND release IS NOT NULL AND release != '' ORDER BY timestamp DESC LIMIT 1`, issueID).Scan(&lastRel).Error
	}
	updates := map[string]any{
		"count":         a.Count,
		"first_release": firstRel,
		"last_release":  lastRel,
	}
	if a.FirstSeen != "" {
		updates["first_seen"] = a.FirstSeen
		updates["last_seen"] = a.LastSeen
	}
	return tx.Model(&IssueRow{}).Where("id = ?", issueID).Updates(updates).Error
}
