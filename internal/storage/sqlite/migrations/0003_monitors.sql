CREATE TABLE IF NOT EXISTS monitors (
    id TEXT PRIMARY KEY,
    data BLOB NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_monitors_created_at ON monitors(created_at);

CREATE TABLE IF NOT EXISTS monitor_events (
    id TEXT PRIMARY KEY,
    monitor_id TEXT NOT NULL,
    ts DATETIME NOT NULL DEFAULT (datetime('now')),
    data BLOB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_monitor_events_monitor ON monitor_events(monitor_id, ts);
