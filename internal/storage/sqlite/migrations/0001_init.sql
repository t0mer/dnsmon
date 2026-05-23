CREATE TABLE IF NOT EXISTS checks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    data BLOB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_checks_created_at ON checks(created_at);
CREATE INDEX IF NOT EXISTS idx_checks_name_type ON checks(name, type);
