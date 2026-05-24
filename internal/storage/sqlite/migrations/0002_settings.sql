CREATE TABLE IF NOT EXISTS settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    data BLOB NOT NULL
);

CREATE TABLE IF NOT EXISTS api_tokens (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    hash TEXT NOT NULL,
    prefix TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    last_used_at DATETIME
);

CREATE TABLE IF NOT EXISTS schedules (
    id TEXT PRIMARY KEY,
    data BLOB NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_schedules_created_at ON schedules(created_at);
