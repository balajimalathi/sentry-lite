-- V2 performance: transactions, spans, rollups, and trace_id on error events

ALTER TABLE events ADD COLUMN trace_id TEXT;

CREATE INDEX IF NOT EXISTS idx_events_trace ON events(trace_id);

CREATE TABLE IF NOT EXISTS transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT NOT NULL UNIQUE,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    name TEXT NOT NULL,
    op TEXT NOT NULL DEFAULT '',
    trace_id TEXT NOT NULL DEFAULT '',
    span_id TEXT NOT NULL DEFAULT '',
    duration_ms REAL NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT '',
    environment TEXT,
    release TEXT,
    timestamp TEXT NOT NULL,
    raw_path TEXT NOT NULL,
    payload_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_tx_project_ts ON transactions(project_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_tx_project_name_ts ON transactions(project_id, name, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_tx_trace ON transactions(trace_id);

CREATE TABLE IF NOT EXISTS spans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    transaction_event_id TEXT NOT NULL REFERENCES transactions(event_id),
    span_id TEXT NOT NULL DEFAULT '',
    parent_span_id TEXT NOT NULL DEFAULT '',
    op TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    duration_ms REAL NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_spans_tx ON spans(transaction_event_id);

CREATE TABLE IF NOT EXISTS transaction_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    name TEXT NOT NULL,
    window_start TEXT NOT NULL,
    window_sec INTEGER NOT NULL,
    count INTEGER NOT NULL DEFAULT 0,
    p95_ms REAL NOT NULL DEFAULT 0,
    p99_ms REAL NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(project_id, name, window_start, window_sec)
);

CREATE INDEX IF NOT EXISTS idx_tx_stats_project ON transaction_stats(project_id, window_start DESC);
