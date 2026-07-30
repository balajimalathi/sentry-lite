-- Per-project CORS allowed origins (JSON string array)

ALTER TABLE projects ADD COLUMN allowed_origins TEXT NOT NULL DEFAULT '[]';
