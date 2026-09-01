package store

// GORM models mirror the SQLite schema formerly defined in migrations/*.sql.

type Organization struct {
	ID        int64  `gorm:"column:id;primaryKey;autoIncrement"`
	Slug      string `gorm:"column:slug;uniqueIndex;not null"`
	Name      string `gorm:"column:name;not null"`
	CreatedAt string `gorm:"column:created_at;not null;default:(datetime('now'))"`
}

func (Organization) TableName() string { return "organizations" }

type ProjectRow struct {
	ID             int64  `gorm:"column:id;primaryKey;autoIncrement"`
	OrganizationID int64  `gorm:"column:organization_id;not null;uniqueIndex:ux_projects_org_slug"`
	Slug           string `gorm:"column:slug;not null;uniqueIndex:ux_projects_org_slug"`
	Name           string `gorm:"column:name;not null"`
	AllowedOrigins string `gorm:"column:allowed_origins;not null;default:'[]'"`
	CreatedAt      string `gorm:"column:created_at;not null;default:(datetime('now'))"`
}

func (ProjectRow) TableName() string { return "projects" }

type ProjectKeyRow struct {
	ID        int64  `gorm:"column:id;primaryKey;autoIncrement"`
	ProjectID int64  `gorm:"column:project_id;not null;index"`
	PublicKey string `gorm:"column:public_key;uniqueIndex;not null"`
	SecretKey string `gorm:"column:secret_key;not null"`
	CreatedAt string `gorm:"column:created_at;not null;default:(datetime('now'))"`
}

func (ProjectKeyRow) TableName() string { return "project_keys" }

type IssueRow struct {
	ID           int64   `gorm:"column:id;primaryKey;autoIncrement"`
	ProjectID    int64   `gorm:"column:project_id;not null;uniqueIndex:ux_issues_project_fp;index:idx_issues_project_last_seen"`
	Fingerprint  string  `gorm:"column:fingerprint;not null;uniqueIndex:ux_issues_project_fp"`
	Title        string  `gorm:"column:title;not null"`
	Culprit      string  `gorm:"column:culprit;not null;default:''"`
	Status       string  `gorm:"column:status;not null;default:'open';index:idx_issues_status"`
	Level        string  `gorm:"column:level;not null;default:'error'"`
	Count        int64   `gorm:"column:count;not null;default:0"`
	FirstSeen    string  `gorm:"column:first_seen;not null"`
	LastSeen     string  `gorm:"column:last_seen;not null;index:idx_issues_project_last_seen,sort:desc"`
	FirstRelease *string `gorm:"column:first_release"`
	LastRelease  *string `gorm:"column:last_release"`
	Regressed    int     `gorm:"column:regressed;not null;default:0"`
	ResolvedAt   *string `gorm:"column:resolved_at"`
	Assignee     *string `gorm:"column:assignee"`
}

func (IssueRow) TableName() string { return "issues" }

type EventRow struct {
	ID            int64   `gorm:"column:id;primaryKey;autoIncrement"`
	EventID       string  `gorm:"column:event_id;uniqueIndex;not null"`
	IssueID       int64   `gorm:"column:issue_id;not null;index:idx_events_issue_ts"`
	ProjectID     int64   `gorm:"column:project_id;not null;index:idx_events_project_ts"`
	Timestamp     string  `gorm:"column:timestamp;not null;index:idx_events_issue_ts,sort:desc;index:idx_events_project_ts,sort:desc"`
	Environment   *string `gorm:"column:environment"`
	Release       *string `gorm:"column:release"`
	Platform      *string `gorm:"column:platform"`
	Message       *string `gorm:"column:message"`
	ExceptionType *string `gorm:"column:exception_type"`
	Culprit       *string `gorm:"column:culprit"`
	UserID        *string `gorm:"column:user_id"`
	UserEmail     *string `gorm:"column:user_email"`
	TraceID       *string `gorm:"column:trace_id;index:idx_events_trace"`
	RawPath       string  `gorm:"column:raw_path;not null"`
	PayloadJSON   string  `gorm:"column:payload_json;not null;default:'{}'"`
	CreatedAt     string  `gorm:"column:created_at;not null;default:(datetime('now'))"`
}

func (EventRow) TableName() string { return "events" }

type EventTagRow struct {
	ID        int64  `gorm:"column:id;primaryKey;autoIncrement"`
	EventID   string `gorm:"column:event_id;not null;index"`
	IssueID   int64  `gorm:"column:issue_id;not null;index:idx_event_tags_issue"`
	ProjectID int64  `gorm:"column:project_id;not null;index:idx_event_tags_kv"`
	Key       string `gorm:"column:key;not null;index:idx_event_tags_kv;index:idx_event_tags_issue"`
	Value     string `gorm:"column:value;not null;index:idx_event_tags_kv"`
}

func (EventTagRow) TableName() string { return "event_tags" }

type ReleaseRow struct {
	ID           int64   `gorm:"column:id;primaryKey;autoIncrement"`
	ProjectID    int64   `gorm:"column:project_id;not null;uniqueIndex:ux_releases_project_version;index:idx_releases_project"`
	Version      string  `gorm:"column:version;not null;uniqueIndex:ux_releases_project_version"`
	Ref          *string `gorm:"column:ref"`
	URL          *string `gorm:"column:url"`
	DateReleased *string `gorm:"column:date_released"`
	CreatedAt    string  `gorm:"column:created_at;not null;default:(datetime('now'));index:idx_releases_project,sort:desc"`
}

func (ReleaseRow) TableName() string { return "releases" }

type AlertRuleRow struct {
	ID        int64   `gorm:"column:id;primaryKey;autoIncrement"`
	ProjectID int64   `gorm:"column:project_id;not null;index:idx_alert_rules_project"`
	Name      string  `gorm:"column:name;not null"`
	Trigger   string  `gorm:"column:trigger;not null"`
	Channel   string  `gorm:"column:channel;not null"`
	Target    string  `gorm:"column:target;not null"`
	Threshold int64   `gorm:"column:threshold;not null;default:0"`
	WindowSec int64   `gorm:"column:window_sec;not null;default:300"`
	Enabled   int     `gorm:"column:enabled;not null;default:1;index:idx_alert_rules_project"`
	Secret    *string `gorm:"column:secret"`
	CreatedAt string  `gorm:"column:created_at;not null;default:(datetime('now'))"`
}

func (AlertRuleRow) TableName() string { return "alert_rules" }

type AlertDeliveryRow struct {
	ID        int64   `gorm:"column:id;primaryKey;autoIncrement"`
	RuleID    int64   `gorm:"column:rule_id;not null;index"`
	IssueID   *int64  `gorm:"column:issue_id"`
	Status    string  `gorm:"column:status;not null"`
	Detail    *string `gorm:"column:detail"`
	CreatedAt string  `gorm:"column:created_at;not null;default:(datetime('now'))"`
}

func (AlertDeliveryRow) TableName() string { return "alert_deliveries" }

type TransactionRow struct {
	ID          int64   `gorm:"column:id;primaryKey;autoIncrement"`
	EventID     string  `gorm:"column:event_id;uniqueIndex;not null"`
	ProjectID   int64   `gorm:"column:project_id;not null;index:idx_tx_project_ts;index:idx_tx_project_name_ts"`
	Name        string  `gorm:"column:name;not null;index:idx_tx_project_name_ts"`
	Op          string  `gorm:"column:op;not null;default:''"`
	TraceID     string  `gorm:"column:trace_id;not null;default:'';index:idx_tx_trace"`
	SpanID      string  `gorm:"column:span_id;not null;default:''"`
	DurationMS  float64 `gorm:"column:duration_ms;not null;default:0"`
	Status      string  `gorm:"column:status;not null;default:''"`
	Environment *string `gorm:"column:environment"`
	Release     *string `gorm:"column:release"`
	Timestamp   string  `gorm:"column:timestamp;not null;index:idx_tx_project_ts,sort:desc;index:idx_tx_project_name_ts,sort:desc"`
	RawPath     string  `gorm:"column:raw_path;not null"`
	PayloadJSON string  `gorm:"column:payload_json;not null;default:'{}'"`
	CreatedAt   string  `gorm:"column:created_at;not null;default:(datetime('now'))"`
}

func (TransactionRow) TableName() string { return "transactions" }

type SpanRow struct {
	ID                 int64   `gorm:"column:id;primaryKey;autoIncrement"`
	TransactionEventID string  `gorm:"column:transaction_event_id;not null;index:idx_spans_tx"`
	SpanID             string  `gorm:"column:span_id;not null;default:''"`
	ParentSpanID       string  `gorm:"column:parent_span_id;not null;default:''"`
	Op                 string  `gorm:"column:op;not null;default:''"`
	Description        string  `gorm:"column:description;not null;default:''"`
	DurationMS         float64 `gorm:"column:duration_ms;not null;default:0"`
	StartOffsetMS      float64 `gorm:"column:start_offset_ms;not null;default:0"`
	Status             string  `gorm:"column:status;not null;default:''"`
}

func (SpanRow) TableName() string { return "spans" }

type TransactionStatRow struct {
	ID          int64   `gorm:"column:id;primaryKey;autoIncrement"`
	ProjectID   int64   `gorm:"column:project_id;not null;uniqueIndex:ux_tx_stats;index:idx_tx_stats_project"`
	Name        string  `gorm:"column:name;not null;uniqueIndex:ux_tx_stats"`
	WindowStart string  `gorm:"column:window_start;not null;uniqueIndex:ux_tx_stats;index:idx_tx_stats_project,sort:desc"`
	WindowSec   int     `gorm:"column:window_sec;not null;uniqueIndex:ux_tx_stats"`
	Count       int64   `gorm:"column:count;not null;default:0"`
	P95MS       float64 `gorm:"column:p95_ms;not null;default:0"`
	P99MS       float64 `gorm:"column:p99_ms;not null;default:0"`
	UpdatedAt   string  `gorm:"column:updated_at;not null;default:(datetime('now'))"`
}

func (TransactionStatRow) TableName() string { return "transaction_stats" }

type CronMonitorRow struct {
	ID             int64   `gorm:"column:id;primaryKey;autoIncrement"`
	ProjectID      int64   `gorm:"column:project_id;not null;uniqueIndex:ux_cron_monitors_project_slug;index:idx_cron_monitors_project"`
	Slug           string  `gorm:"column:slug;not null;uniqueIndex:ux_cron_monitors_project_slug"`
	Name           string  `gorm:"column:name;not null"`
	ScheduleSec    int64   `gorm:"column:schedule_sec;not null"`
	GraceSec       int64   `gorm:"column:grace_sec;not null;default:60"`
	Environment    *string `gorm:"column:environment"`
	Status         string  `gorm:"column:status;not null;default:'unknown'"`
	LastCheckinAt  *string `gorm:"column:last_checkin_at"`
	NextExpectedAt *string `gorm:"column:next_expected_at;index:idx_cron_monitors_next"`
	Token          string  `gorm:"column:token;uniqueIndex:idx_cron_monitors_token;not null"`
	CreatedAt      string  `gorm:"column:created_at;not null;default:(datetime('now'))"`
}

func (CronMonitorRow) TableName() string { return "cron_monitors" }

type CronCheckinRow struct {
	ID         int64    `gorm:"column:id;primaryKey;autoIncrement"`
	MonitorID  int64    `gorm:"column:monitor_id;not null;index:idx_cron_checkins_monitor"`
	Status     string   `gorm:"column:status;not null;default:'ok'"`
	DurationMS *float64 `gorm:"column:duration_ms"`
	Timestamp  string   `gorm:"column:timestamp;not null;index:idx_cron_checkins_monitor,sort:desc"`
	CreatedAt  string   `gorm:"column:created_at;not null;default:(datetime('now'))"`
}

func (CronCheckinRow) TableName() string { return "cron_checkins" }

func allModels() []any {
	return []any{
		&Organization{},
		&ProjectRow{},
		&ProjectKeyRow{},
		&IssueRow{},
		&EventRow{},
		&EventTagRow{},
		&ReleaseRow{},
		&AlertRuleRow{},
		&AlertDeliveryRow{},
		&TransactionRow{},
		&SpanRow{},
		&TransactionStatRow{},
		&CronMonitorRow{},
		&CronCheckinRow{},
	}
}
