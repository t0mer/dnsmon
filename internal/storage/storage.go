package storage

import (
	"context"
	"errors"

	"github.com/t0mer/dnsmon/internal/dnsclient"
	"github.com/t0mer/dnsmon/internal/settings"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")

// Storage persists DNS checks and user-editable application settings.
type Storage interface {
	SaveCheck(ctx context.Context, check *dnsclient.Check) error
	GetCheck(ctx context.Context, id string) (*dnsclient.Check, error)
	ListChecks(ctx context.Context, limit, offset int) ([]*dnsclient.Check, error)
	DeleteCheck(ctx context.Context, id string) error
	PurgeOldChecks(ctx context.Context, retentionDays int) error

	// Settings (singleton document).
	GetSettings(ctx context.Context) (*settings.Settings, error)
	SaveSettings(ctx context.Context, s *settings.Settings) error

	// External API tokens.
	ListAPITokens(ctx context.Context) ([]*settings.APIToken, error)
	CreateAPIToken(ctx context.Context, t *settings.APIToken) error
	DeleteAPIToken(ctx context.Context, id string) error

	// Monitoring schedules.
	ListSchedules(ctx context.Context) ([]*settings.Schedule, error)
	SaveSchedule(ctx context.Context, s *settings.Schedule) error
	DeleteSchedule(ctx context.Context, id string) error

	// Monitors and their changelog.
	ListMonitors(ctx context.Context) ([]*settings.Monitor, error)
	GetMonitor(ctx context.Context, id string) (*settings.Monitor, error)
	SaveMonitor(ctx context.Context, m *settings.Monitor) error
	DeleteMonitor(ctx context.Context, id string) error
	ListMonitorEvents(ctx context.Context, monitorID string, limit int) ([]*settings.MonitorEvent, error)
	AppendMonitorEvent(ctx context.Context, e *settings.MonitorEvent) error

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

func (n *Noop) GetSettings(_ context.Context) (*settings.Settings, error) {
	return settings.Default(), nil
}

func (n *Noop) SaveSettings(_ context.Context, _ *settings.Settings) error { return nil }

func (n *Noop) ListAPITokens(_ context.Context) ([]*settings.APIToken, error) { return nil, nil }

func (n *Noop) CreateAPIToken(_ context.Context, _ *settings.APIToken) error { return nil }

func (n *Noop) DeleteAPIToken(_ context.Context, _ string) error { return nil }

func (n *Noop) ListSchedules(_ context.Context) ([]*settings.Schedule, error) { return nil, nil }

func (n *Noop) SaveSchedule(_ context.Context, _ *settings.Schedule) error { return nil }

func (n *Noop) DeleteSchedule(_ context.Context, _ string) error { return nil }

func (n *Noop) ListMonitors(_ context.Context) ([]*settings.Monitor, error) { return nil, nil }

func (n *Noop) GetMonitor(_ context.Context, _ string) (*settings.Monitor, error) {
	return nil, ErrNotFound
}

func (n *Noop) SaveMonitor(_ context.Context, _ *settings.Monitor) error { return nil }

func (n *Noop) DeleteMonitor(_ context.Context, _ string) error { return nil }

func (n *Noop) ListMonitorEvents(_ context.Context, _ string, _ int) ([]*settings.MonitorEvent, error) {
	return nil, nil
}

func (n *Noop) AppendMonitorEvent(_ context.Context, _ *settings.MonitorEvent) error { return nil }

func (n *Noop) Close() error { return nil }
