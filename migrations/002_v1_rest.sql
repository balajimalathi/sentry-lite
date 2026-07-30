-- V1 remaining: releases, alerts, resolved_at for quiet-window regression

ALTER TABLE issues ADD COLUMN resolved_at TEXT;
ALTER TABLE issues ADD COLUMN assignee TEXT;

CREATE TABLE IF NOT EXISTS releases (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    version TEXT NOT NULL,
    ref TEXT,
    url TEXT,
    date_released TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(project_id, version)
);

CREATE INDEX IF NOT EXISTS idx_releases_project ON releases(project_id, created_at DESC);

CREATE TABLE IF NOT EXISTS alert_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    name TEXT NOT NULL,
    trigger TEXT NOT NULL, -- new_issue | regressed_issue | error_volume
    channel TEXT NOT NULL, -- slack | email | webhook
    target TEXT NOT NULL,  -- webhook URL, email address
    threshold INTEGER NOT NULL DEFAULT 0, -- for error_volume: events in window
    window_sec INTEGER NOT NULL DEFAULT 300,
    enabled INTEGER NOT NULL DEFAULT 1,
    secret TEXT, -- HMAC secret for webhooks
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_alert_rules_project ON alert_rules(project_id, enabled);

CREATE TABLE IF NOT EXISTS alert_deliveries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER NOT NULL REFERENCES alert_rules(id),
    issue_id INTEGER,
    status TEXT NOT NULL,
    detail TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
