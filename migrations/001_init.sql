CREATE TABLE IF NOT EXISTS organizations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    organization_id INTEGER NOT NULL REFERENCES organizations(id),
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(organization_id, slug)
);

CREATE TABLE IF NOT EXISTS project_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    public_key TEXT NOT NULL UNIQUE,
    secret_key TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS issues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    fingerprint TEXT NOT NULL,
    title TEXT NOT NULL,
    culprit TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open',
    level TEXT NOT NULL DEFAULT 'error',
    count INTEGER NOT NULL DEFAULT 0,
    first_seen TEXT NOT NULL,
    last_seen TEXT NOT NULL,
    first_release TEXT,
    last_release TEXT,
    regressed INTEGER NOT NULL DEFAULT 0,
    UNIQUE(project_id, fingerprint)
);

CREATE INDEX IF NOT EXISTS idx_issues_project_last_seen ON issues(project_id, last_seen DESC);
CREATE INDEX IF NOT EXISTS idx_issues_status ON issues(status);

CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT NOT NULL UNIQUE,
    issue_id INTEGER NOT NULL REFERENCES issues(id),
    project_id INTEGER NOT NULL REFERENCES projects(id),
    timestamp TEXT NOT NULL,
    environment TEXT,
    release TEXT,
    platform TEXT,
    message TEXT,
    exception_type TEXT,
    culprit TEXT,
    user_id TEXT,
    user_email TEXT,
    raw_path TEXT NOT NULL,
    payload_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_events_issue_ts ON events(issue_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_project_ts ON events(project_id, timestamp DESC);

CREATE TABLE IF NOT EXISTS event_tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT NOT NULL REFERENCES events(event_id),
    issue_id INTEGER NOT NULL REFERENCES issues(id),
    project_id INTEGER NOT NULL REFERENCES projects(id),
    key TEXT NOT NULL,
    value TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_event_tags_kv ON event_tags(project_id, key, value);
CREATE INDEX IF NOT EXISTS idx_event_tags_issue ON event_tags(issue_id, key);
