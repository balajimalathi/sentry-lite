package store

import (
	"context"
	"fmt"
	"sort"
	"time"
)

type StatsBucket struct {
	T      string `json:"t"`
	Events int64  `json:"events"`
}

type DashboardStats struct {
	Unresolved     int64            `json:"unresolved"`
	Events         int64            `json:"events"`
	Regressions    int64            `json:"regressions"`
	CronsUnhealthy int64            `json:"crons_unhealthy"`
	ByStatus       map[string]int64 `json:"by_status"`
	Series         []StatsBucket    `json:"series"`
	TopIssues      []Issue          `json:"top_issues"`
}

type DashboardStatsFilter struct {
	ProjectID int64
	From      time.Time
	To        time.Time
	Interval  time.Duration
}

// DashboardStats returns overview KPIs, event volume series, and top open issues.
func (s *Store) DashboardStats(ctx context.Context, f DashboardStatsFilter) (*DashboardStats, error) {
	if f.To.IsZero() {
		f.To = time.Now().UTC()
	}
	if f.From.IsZero() {
		f.From = f.To.Add(-24 * time.Hour)
	}
	if f.Interval <= 0 {
		f.Interval = defaultInterval(f.To.Sub(f.From))
	}

	out := &DashboardStats{
		ByStatus:  map[string]int64{},
		Series:    []StatsBucket{},
		TopIssues: []Issue{},
	}

	fromStr := f.From.UTC().Format(time.RFC3339Nano)
	toStr := f.To.UTC().Format(time.RFC3339Nano)

	if err := s.countIssueKPIs(ctx, f.ProjectID, out); err != nil {
		return nil, err
	}
	if err := s.countEventsInRange(ctx, f.ProjectID, fromStr, toStr, out); err != nil {
		return nil, err
	}
	if err := s.countCronsUnhealthy(ctx, f.ProjectID, out); err != nil {
		return nil, err
	}
	if err := s.buildEventSeries(ctx, f, fromStr, toStr, out); err != nil {
		return nil, err
	}
	if err := s.topOpenIssues(ctx, f.ProjectID, out); err != nil {
		return nil, err
	}
	return out, nil
}

func defaultInterval(window time.Duration) time.Duration {
	switch {
	case window <= time.Hour:
		return 5 * time.Minute
	case window <= 24*time.Hour:
		return time.Hour
	case window <= 7*24*time.Hour:
		return 6 * time.Hour
	default:
		return 24 * time.Hour
	}
}

func (s *Store) countIssueKPIs(ctx context.Context, projectID int64, out *DashboardStats) error {
	type statusCount struct {
		Status string
		N      int64
	}
	q := s.DB.WithContext(ctx).Model(&IssueRow{}).
		Select("status, COUNT(*) as n").
		Group("status")
	if projectID > 0 {
		q = q.Where("project_id = ?", projectID)
	}
	var rows []statusCount
	if err := q.Scan(&rows).Error; err != nil {
		return fmt.Errorf("by_status: %w", err)
	}
	for _, r := range rows {
		out.ByStatus[r.Status] = r.N
		if r.Status == "open" {
			out.Unresolved = r.N
		}
	}

	regQ := s.DB.WithContext(ctx).Model(&IssueRow{}).Where("regressed = 1")
	if projectID > 0 {
		regQ = regQ.Where("project_id = ?", projectID)
	}
	if err := regQ.Count(&out.Regressions).Error; err != nil {
		return fmt.Errorf("regressions: %w", err)
	}
	return nil
}

func (s *Store) countEventsInRange(ctx context.Context, projectID int64, fromStr, toStr string, out *DashboardStats) error {
	q := s.DB.WithContext(ctx).Model(&EventRow{}).
		Where("timestamp >= ? AND timestamp <= ?", fromStr, toStr)
	if projectID > 0 {
		q = q.Where("project_id = ?", projectID)
	}
	if err := q.Count(&out.Events).Error; err != nil {
		return fmt.Errorf("events: %w", err)
	}
	return nil
}

func (s *Store) countCronsUnhealthy(ctx context.Context, projectID int64, out *DashboardStats) error {
	q := s.DB.WithContext(ctx).Model(&CronMonitorRow{}).
		Where("status IN ?", []string{"late", "missed"})
	if projectID > 0 {
		q = q.Where("project_id = ?", projectID)
	}
	if err := q.Count(&out.CronsUnhealthy).Error; err != nil {
		return fmt.Errorf("crons_unhealthy: %w", err)
	}
	return nil
}

func (s *Store) buildEventSeries(ctx context.Context, f DashboardStatsFilter, fromStr, toStr string, out *DashboardStats) error {
	prefixLen, parseLayout := seriesSQLTrunc(f.Interval)

	q := `
SELECT substr(timestamp, 1, ?) AS bucket, COUNT(*) AS n
FROM events
WHERE timestamp >= ? AND timestamp <= ?`
	args := []any{prefixLen, fromStr, toStr}
	if f.ProjectID > 0 {
		q += ` AND project_id = ?`
		args = append(args, f.ProjectID)
	}
	q += ` GROUP BY bucket`

	type row struct {
		Bucket string
		N      int64
	}
	var rows []row
	if err := s.DB.WithContext(ctx).Raw(q, args...).Scan(&rows).Error; err != nil {
		return fmt.Errorf("series: %w", err)
	}

	counts := map[int64]int64{}
	for _, r := range rows {
		t, err := time.ParseInLocation(parseLayout, r.Bucket, time.UTC)
		if err != nil {
			continue
		}
		bucket := t.UTC().Truncate(f.Interval).Unix()
		counts[bucket] += r.N
	}

	start := f.From.UTC().Truncate(f.Interval)
	end := f.To.UTC()
	series := make([]StatsBucket, 0)
	for t := start; !t.After(end); t = t.Add(f.Interval) {
		series = append(series, StatsBucket{
			T:      t.Format(time.RFC3339),
			Events: counts[t.Unix()],
		})
	}
	out.Series = series
	return nil
}

// seriesSQLTrunc picks an ISO prefix length coarse enough to GROUP BY in SQLite,
// then Go re-aggregates into the requested interval.
func seriesSQLTrunc(interval time.Duration) (prefixLen int, layout string) {
	if interval < time.Hour {
		// minute: 2006-01-02T15:04
		return 16, "2006-01-02T15:04"
	}
	if interval < 24*time.Hour {
		// hour: 2006-01-02T15
		return 13, "2006-01-02T15"
	}
	// day: 2006-01-02
	return 10, "2006-01-02"
}

func (s *Store) topOpenIssues(ctx context.Context, projectID int64, out *DashboardStats) error {
	q := s.DB.WithContext(ctx).Model(&IssueRow{}).
		Where("status = ?", "open").
		Order("count DESC").
		Limit(8)
	if projectID > 0 {
		q = q.Where("project_id = ?", projectID)
	}
	var rows []IssueRow
	if err := q.Find(&rows).Error; err != nil {
		return fmt.Errorf("top_issues: %w", err)
	}
	out.TopIssues = make([]Issue, 0, len(rows))
	for i := range rows {
		out.TopIssues = append(out.TopIssues, issueFromRow(&rows[i]))
	}
	// Stable secondary sort by last_seen for equal counts (Find already orders by count).
	sort.SliceStable(out.TopIssues, func(i, j int) bool {
		if out.TopIssues[i].Count != out.TopIssues[j].Count {
			return out.TopIssues[i].Count > out.TopIssues[j].Count
		}
		return out.TopIssues[i].LastSeen > out.TopIssues[j].LastSeen
	})
	return nil
}

// ParseStatsInterval maps query interval strings to durations.
func ParseStatsInterval(s string) (time.Duration, error) {
	switch s {
	case "", "auto":
		return 0, nil
	case "5m":
		return 5 * time.Minute, nil
	case "1h":
		return time.Hour, nil
	case "6h":
		return 6 * time.Hour, nil
	case "1d":
		return 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid interval %q", s)
	}
}
