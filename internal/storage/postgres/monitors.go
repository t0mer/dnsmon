package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/t0mer/dnsmon/internal/settings"
	"github.com/t0mer/dnsmon/internal/storage"
)

// ListMonitors returns all monitors ordered by creation time.
func (s *Store) ListMonitors(ctx context.Context) ([]*settings.Monitor, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT data FROM monitors ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("querying monitors: %w", err)
	}
	defer rows.Close()

	var out []*settings.Monitor
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scanning monitor row: %w", err)
		}
		var m settings.Monitor
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("unmarshaling monitor: %w", err)
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

// GetMonitor returns a single monitor by ID.
func (s *Store) GetMonitor(ctx context.Context, id string) (*settings.Monitor, error) {
	row := s.db.QueryRowContext(ctx, `SELECT data FROM monitors WHERE id = $1`, id)
	var data []byte
	if err := row.Scan(&data); err != nil {
		if err == sql.ErrNoRows {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("scanning monitor row: %w", err)
	}
	var m settings.Monitor
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshaling monitor: %w", err)
	}
	return &m, nil
}

// SaveMonitor upserts a monitor.
func (s *Store) SaveMonitor(ctx context.Context, m *settings.Monitor) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshaling monitor: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO monitors (id, data, created_at, updated_at) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data, updated_at = EXCLUDED.updated_at`,
		m.ID, data, m.CreatedAt, m.UpdatedAt)
	if err != nil {
		return fmt.Errorf("saving monitor: %w", err)
	}
	return nil
}

// DeleteMonitor removes a monitor and its events.
func (s *Store) DeleteMonitor(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM monitors WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting monitor: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return storage.ErrNotFound
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM monitor_events WHERE monitor_id = $1`, id)
	return nil
}

// ListMonitorEvents returns a monitor's changelog, most recent first.
func (s *Store) ListMonitorEvents(ctx context.Context, monitorID string, limit int) ([]*settings.MonitorEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT data FROM monitor_events WHERE monitor_id = $1 ORDER BY ts DESC LIMIT $2`, monitorID, limit)
	if err != nil {
		return nil, fmt.Errorf("querying monitor events: %w", err)
	}
	defer rows.Close()

	var out []*settings.MonitorEvent
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scanning monitor event row: %w", err)
		}
		var e settings.MonitorEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("unmarshaling monitor event: %w", err)
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// AppendMonitorEvent records a changelog entry.
func (s *Store) AppendMonitorEvent(ctx context.Context, e *settings.MonitorEvent) error {
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshaling monitor event: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO monitor_events (id, monitor_id, ts, data) VALUES ($1, $2, $3, $4)`,
		e.ID, e.MonitorID, e.Timestamp, data)
	if err != nil {
		return fmt.Errorf("appending monitor event: %w", err)
	}
	return nil
}
