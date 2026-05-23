package storage

import (
	"context"
	"errors"

	"github.com/t0mer/dnsmon/internal/dnsclient"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// Storage persists DNS checks.
type Storage interface {
	SaveCheck(ctx context.Context, check *dnsclient.Check) error
	GetCheck(ctx context.Context, id string) (*dnsclient.Check, error)
	ListChecks(ctx context.Context, limit, offset int) ([]*dnsclient.Check, error)
	DeleteCheck(ctx context.Context, id string) error
	PurgeOldChecks(ctx context.Context, retentionDays int) error
	Close() error
}

// Noop is a Storage that discards everything (used when driver="none").
type Noop struct{}

func (n *Noop) SaveCheck(_ context.Context, _ *dnsclient.Check) error { return nil }

func (n *Noop) GetCheck(_ context.Context, _ string) (*dnsclient.Check, error) {
	return nil, ErrNotFound
}

func (n *Noop) ListChecks(_ context.Context, _, _ int) ([]*dnsclient.Check, error) {
	return nil, nil
}

func (n *Noop) DeleteCheck(_ context.Context, _ string) error { return nil }

func (n *Noop) PurgeOldChecks(_ context.Context, _ int) error { return nil }

func (n *Noop) Close() error { return nil }
