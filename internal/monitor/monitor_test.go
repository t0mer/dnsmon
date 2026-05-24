package monitor

import (
	"context"
	"testing"
	"time"

	"github.com/t0mer/dnsmon/internal/checker"
	"github.com/t0mer/dnsmon/internal/dnsclient"
	"github.com/t0mer/dnsmon/internal/settings"
	sqlitestore "github.com/t0mer/dnsmon/internal/storage/sqlite"
)

// fakeCheck returns a canned Check.
type fakeCheck struct{ results []dnsclient.ResolverResult }

func (f *fakeCheck) Check(_ context.Context, _ checker.CheckRequest) (*dnsclient.Check, error) {
	return &dnsclient.Check{Results: f.results}, nil
}

// fakeNotifier records sends.
type fakeNotifier struct{ sent []string }

func (n *fakeNotifier) Send(_ context.Context, _ settings.NotificationChannel, msg string) error {
	n.sent = append(n.sent, msg)
	return nil
}

func okResult(id string, values ...string) dnsclient.ResolverResult {
	ans := make([]dnsclient.Answer, len(values))
	for i, v := range values {
		ans[i] = dnsclient.Answer{Value: v, Type: "A"}
	}
	return dnsclient.ResolverResult{
		Resolver: dnsclient.Resolver{ID: id},
		Status:   dnsclient.StatusOK,
		Answers:  ans,
	}
}

func newStore(t *testing.T) *sqlitestore.Store {
	t.Helper()
	s, err := sqlitestore.New(context.Background(), "file:"+t.TempDir()+"/m.db?_fk=1")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestPropagation_AllMatch(t *testing.T) {
	store := newStore(t)
	notifier := &fakeNotifier{}
	ev := New(&fakeCheck{results: []dnsclient.ResolverResult{okResult("r1", "1.2.3.4"), okResult("r2", "1.2.3.4")}}, store, notifier)

	m := &settings.Monitor{ID: "m1", Type: settings.MonitorPropagation, FQDN: "x.com", RecordType: "A", Expected: []string{"1.2.3.4"}, ChannelID: "c1"}
	e, _ := ev.Run(context.Background(), m)
	if e.Status != settings.MonitorStatusOK {
		t.Errorf("status = %q, want ok", e.Status)
	}
	if len(notifier.sent) != 0 {
		t.Errorf("should not notify on full propagation, sent %v", notifier.sent)
	}
}

func TestPropagation_PartialAlerts(t *testing.T) {
	store := newStore(t)
	// Bind an enabled channel via settings.
	s := settings.Default()
	s.Notifications = []settings.NotificationChannel{{ID: "c1", Type: settings.ChannelShoutrrr, Enabled: true, Config: map[string]string{"url": "x"}}}
	_ = store.SaveSettings(context.Background(), s)

	notifier := &fakeNotifier{}
	ev := New(&fakeCheck{results: []dnsclient.ResolverResult{okResult("r1", "1.2.3.4"), okResult("r2", "9.9.9.9")}}, store, notifier)

	m := &settings.Monitor{ID: "m1", Type: settings.MonitorPropagation, FQDN: "x.com", RecordType: "A", Expected: []string{"1.2.3.4"}, ChannelID: "c1"}
	e, _ := ev.Run(context.Background(), m)
	if e.Status != settings.MonitorStatusNotPropagated {
		t.Errorf("status = %q, want not_propagated", e.Status)
	}
	if !e.Notified || len(notifier.sent) != 1 {
		t.Errorf("expected one notification, got notified=%v sent=%v", e.Notified, notifier.sent)
	}
}

func TestChange_BaselineThenChange(t *testing.T) {
	store := newStore(t)
	s := settings.Default()
	s.Notifications = []settings.NotificationChannel{{ID: "c1", Type: settings.ChannelShoutrrr, Enabled: true, Config: map[string]string{"url": "x"}}}
	_ = store.SaveSettings(context.Background(), s)
	notifier := &fakeNotifier{}

	m := &settings.Monitor{ID: "m1", Type: settings.MonitorChange, FQDN: "x.com", RecordType: "A", ChannelID: "c1"}
	require := func(cond bool, msg string) {
		if !cond {
			t.Fatal(msg)
		}
	}
	require(store.SaveMonitor(context.Background(), m) == nil, "save monitor")

	// First run with empty baseline -> capture, no alert.
	ev1 := New(&fakeCheck{results: []dnsclient.ResolverResult{okResult("r1", "1.1.1.1"), okResult("r2", "1.1.1.1")}}, store, notifier)
	e, _ := ev1.Run(context.Background(), m)
	if e.Status != settings.MonitorStatusOK || len(notifier.sent) != 0 {
		t.Fatalf("baseline run: status=%q sent=%v", e.Status, notifier.sent)
	}
	if len(m.Expected) != 1 || m.Expected[0] != "1.1.1.1" {
		t.Fatalf("baseline not captured: %v", m.Expected)
	}

	// Second run with a different value -> changed + alert + re-baseline.
	ev2 := New(&fakeCheck{results: []dnsclient.ResolverResult{okResult("r1", "2.2.2.2"), okResult("r2", "2.2.2.2")}}, store, notifier)
	e2, _ := ev2.Run(context.Background(), m)
	if e2.Status != settings.MonitorStatusChanged {
		t.Fatalf("expected changed, got %q", e2.Status)
	}
	if !e2.Notified || len(notifier.sent) != 1 {
		t.Fatalf("expected one alert, got notified=%v sent=%v", e2.Notified, notifier.sent)
	}
	if m.Expected[0] != "2.2.2.2" {
		t.Fatalf("expected re-baseline to 2.2.2.2, got %v", m.Expected)
	}

	// Third run, same as new baseline -> no change.
	e3, _ := ev2.Run(context.Background(), m)
	if e3.Status != settings.MonitorStatusOK || len(notifier.sent) != 1 {
		t.Fatalf("expected no new alert, got status=%q sent=%v", e3.Status, notifier.sent)
	}

	// History should have accumulated events.
	events, _ := store.ListMonitorEvents(context.Background(), "m1", 10)
	if len(events) < 3 {
		t.Errorf("expected >=3 history events, got %d", len(events))
	}
	_ = time.Now
}
