package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/t0mer/dnsmon/internal/settings"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(context.Background(), "file:"+t.TempDir()+"/test.db?_fk=1")
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSettingsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	// Defaults when unset.
	got, err := s.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings (empty): %v", err)
	}
	if got.Auth.Enabled {
		t.Error("expected auth disabled by default")
	}

	want := settings.Default()
	want.Auth = settings.AuthSettings{Enabled: true, Username: "admin", PasswordHash: "$argon2id$..."}
	want.Notifications = []settings.NotificationChannel{
		{ID: "c1", Type: settings.ChannelShoutrrr, Name: "ops", Enabled: true, Config: map[string]string{"url": "slack://x"}},
	}
	want.DisabledResolvers = []string{"google-us", "cloudflare"}

	if err := s.SaveSettings(ctx, want); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	got, err = s.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if !got.Auth.Enabled || got.Auth.Username != "admin" || got.Auth.PasswordHash == "" {
		t.Errorf("auth not persisted: %+v", got.Auth)
	}
	if len(got.Notifications) != 1 || got.Notifications[0].Config["url"] != "slack://x" {
		t.Errorf("notifications not persisted: %+v", got.Notifications)
	}
	if len(got.DisabledResolvers) != 2 {
		t.Errorf("disabled resolvers not persisted: %+v", got.DisabledResolvers)
	}
}

func TestAPITokenCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	tok := &settings.APIToken{ID: "t1", Name: "ci", Hash: "$argon2id$...", Prefix: "abcd", CreatedAt: time.Now().UTC()}
	if err := s.CreateAPIToken(ctx, tok); err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}
	list, err := s.ListAPITokens(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "ci" {
		t.Fatalf("ListAPITokens = %+v, err %v", list, err)
	}
	if err := s.DeleteAPIToken(ctx, "t1"); err != nil {
		t.Fatalf("DeleteAPIToken: %v", err)
	}
	if list, _ := s.ListAPITokens(ctx); len(list) != 0 {
		t.Errorf("expected no tokens after delete, got %d", len(list))
	}
}

func TestScheduleCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	now := time.Now().UTC()
	sc := &settings.Schedule{ID: "s1", Name: "watch example", Domain: "example.com", Type: "A",
		IntervalSec: 300, Enabled: true, CreatedAt: now, UpdatedAt: now}
	if err := s.SaveSchedule(ctx, sc); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}
	// Update (upsert).
	sc.Name = "renamed"
	sc.UpdatedAt = now.Add(time.Minute)
	if err := s.SaveSchedule(ctx, sc); err != nil {
		t.Fatalf("SaveSchedule update: %v", err)
	}
	list, err := s.ListSchedules(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "renamed" {
		t.Fatalf("ListSchedules = %+v, err %v", list, err)
	}
	if err := s.DeleteSchedule(ctx, "s1"); err != nil {
		t.Fatalf("DeleteSchedule: %v", err)
	}
	if list, _ := s.ListSchedules(ctx); len(list) != 0 {
		t.Errorf("expected no schedules after delete, got %d", len(list))
	}
}
