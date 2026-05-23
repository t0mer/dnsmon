package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/t0mer/dnsmon/internal/dnsclient"
	"github.com/t0mer/dnsmon/internal/storage"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Store is a SQLite-backed storage implementation.
type Store struct {
	db *sql.DB
}

// New opens a SQLite database at the given DSN and runs migrations.
func New(ctx context.Context, dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite db: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging sqlite db: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return s, nil
}

func (s *Store) migrate(ctx context.Context) error {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("reading migrations dir: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		data, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", name, err)
		}
		if _, err := s.db.ExecContext(ctx, string(data)); err != nil {
			return fmt.Errorf("executing migration %s: %w", name, err)
		}
	}
	return nil
}

// SaveCheck persists a check to the database.
func (s *Store) SaveCheck(ctx context.Context, check *dnsclient.Check) error {
	data, err := json.Marshal(check)
	if err != nil {
		return fmt.Errorf("marshaling check: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO checks (id, name, type, created_at, data) VALUES (?, ?, ?, ?, ?)`,
		check.ID, check.Name, check.Type, check.CreatedAt.UTC().Format(time.RFC3339), data,
	)
	if err != nil {
		return fmt.Errorf("inserting check: %w", err)
	}
	return nil
}

// GetCheck retrieves a check by ID.
func (s *Store) GetCheck(ctx context.Context, id string) (*dnsclient.Check, error) {
	row := s.db.QueryRowContext(ctx, `SELECT data FROM checks WHERE id = ?`, id)

	var data []byte
	if err := row.Scan(&data); err != nil {
		if err == sql.ErrNoRows {
			return nil, storage.ErrNotFound
		}
		return nil, fmt.Errorf("scanning check row: %w", err)
	}

	var check dnsclient.Check
	if err := json.Unmarshal(data, &check); err != nil {
		return nil, fmt.Errorf("unmarshaling check: %w", err)
	}
	return &check, nil
}

// ListChecks returns a paginated list of checks ordered by most recent first.
func (s *Store) ListChecks(ctx context.Context, limit, offset int) ([]*dnsclient.Check, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT data FROM checks ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("querying checks: %w", err)
	}
	defer rows.Close()

	var checks []*dnsclient.Check
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scanning check row: %w", err)
		}
		var check dnsclient.Check
		if err := json.Unmarshal(data, &check); err != nil {
			return nil, fmt.Errorf("unmarshaling check: %w", err)
		}
		checks = append(checks, &check)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating check rows: %w", err)
	}
	return checks, nil
}

// DeleteCheck removes a check by ID.
func (s *Store) DeleteCheck(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM checks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting check: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if n == 0 {
		return storage.ErrNotFound
	}
	return nil
}

// PurgeOldChecks deletes checks older than retentionDays days.
func (s *Store) PurgeOldChecks(ctx context.Context, retentionDays int) error {
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays).Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM checks WHERE created_at < ?`, cutoff,
	)
	if err != nil {
		return fmt.Errorf("purging old checks: %w", err)
	}
	return nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
