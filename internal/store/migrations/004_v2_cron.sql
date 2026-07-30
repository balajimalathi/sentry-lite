-- V2 cron / heartbeat monitoring

CREATE TABLE IF NOT EXISTS cron_monitors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    slug TEXT NOT NULL,
    name TEXT NOT NULL,
    schedule_sec INTEGER NOT NULL,
    grace_sec INTEGER NOT NULL DEFAULT 60,
    environment TEXT,
    status TEXT NOT NULL DEFAULT 'unknown', -- ok | late | missed | unknown
    last_checkin_at TEXT,
    next_expected_at TEXT,
    token TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(project_id, slug)
);

CREATE INDEX IF NOT EXISTS idx_cron_monitors_project ON cron_monitors(project_id);
CREATE INDEX IF NOT EXISTS idx_cron_monitors_next ON cron_monitors(next_expected_at);
CREATE INDEX IF NOT EXISTS idx_cron_monitors_token ON cron_monitors(token);

CREATE TABLE IF NOT EXISTS cron_checkins (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    monitor_id INTEGER NOT NULL REFERENCES cron_monitors(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'ok', -- ok | error | in_progress
    duration_ms REAL,
    timestamp TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_cron_checkins_monitor ON cron_checkins(monitor_id, timestamp DESC);
