package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/t0mer/dnsmon/internal/settings"
	"github.com/t0mer/dnsmon/internal/storage"
)

// GetSettings returns the singleton settings document, or defaults if unset.
func (s *Store) GetSettings(ctx context.Context) (*settings.Settings, error) {
	row := s.db.QueryRowContext(ctx, `SELECT data FROM settings WHERE id = 1`)

	var data []byte
	if err := row.Scan(&data); err != nil {
		if err == sql.ErrNoRows {
			return settings.Default(), nil
		}
		return nil, fmt.Errorf("scanning settings row: %w", err)
	}

	var out settings.Settings
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("unmarshaling settings: %w", err)
	}
	return &out, nil
}

// SaveSettings upserts the singleton settings document.
func (s *Store) SaveSettings(ctx context.Context, in *settings.Settings) error {
	data, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO settings (id, data) VALUES (1, $1)
		 ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data`, data)
	if err != nil {
		return fmt.Errorf("saving settings: %w", err)
	}
	return nil
}

// ListAPITokens returns all API tokens ordered by creation time.
func (s *Store) ListAPITokens(ctx context.Context) ([]*settings.APIToken, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, hash, prefix, created_at, last_used_at FROM api_tokens ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("querying api tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*settings.APIToken
	for rows.Next() {
		var (
			t        settings.APIToken
			lastUsed sql.NullTime
		)
		if err := rows.Scan(&t.ID, &t.Name, &t.Hash, &t.Prefix, &t.CreatedAt, &lastUsed); err != nil {
			return nil, fmt.Errorf("scanning api token row: %w", err)
		}
		if lastUsed.Valid {
			tm := lastUsed.Time
			t.LastUsedAt = &tm
		}
		tokens = append(tokens, &t)
	}
	return tokens, rows.Err()
}

// CreateAPIToken inserts a new API token.
func (s *Store) CreateAPIToken(ctx context.Context, t *settings.APIToken) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO api_tokens (id, name, hash, prefix, created_at) VALUES ($1, $2, $3, $4, $5)`,
		t.ID, t.Name, t.Hash, t.Prefix, t.CreatedAt)
	if err != nil {
		return fmt.Errorf("creating api token: %w", err)
	}
	return nil
}

// DeleteAPIToken removes an API token by ID.
func (s *Store) DeleteAPIToken(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM api_tokens WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting api token: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// ListSchedules returns all schedules ordered by creation time.
func (s *Store) ListSchedules(ctx context.Context) ([]*settings.Schedule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT data FROM schedules ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("querying schedules: %w", err)
	}
	defer rows.Close()

	var out []*settings.Schedule
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scanning schedule row: %w", err)
		}
		var sc settings.Schedule
		if err := json.Unmarshal(data, &sc); err != nil {
			return nil, fmt.Errorf("unmarshaling schedule: %w", err)
		}
		out = append(out, &sc)
	}
	return out, rows.Err()
}

// SaveSchedule upserts a schedule.
func (s *Store) SaveSchedule(ctx context.Context, sc *settings.Schedule) error {
	data, err := json.Marshal(sc)
	if err != nil {
		return fmt.Errorf("marshaling schedule: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO schedules (id, data, created_at, updated_at) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (id) DO UPDATE SET data = EXCLUDED.data, updated_at = EXCLUDED.updated_at`,
		sc.ID, data, sc.CreatedAt, sc.UpdatedAt)
	if err != nil {
		return fmt.Errorf("saving schedule: %w", err)
	}
	return nil
}

// DeleteSchedule removes a schedule by ID.
func (s *Store) DeleteSchedule(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM schedules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting schedule: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return storage.ErrNotFound
	}
	return nil
}
