// Package monitor evaluates monitors: it runs a propagation check for a
// monitor's record, decides whether the record has propagated (propagation
// monitors) or drifted from its baseline (change monitors), records a changelog
// entry, and notifies the bound channel on an alert. The same Evaluator is used
// by the manual "Run" action and (later) the scheduled engine.
package monitor

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/t0mer/dnsmon/internal/checker"
	"github.com/t0mer/dnsmon/internal/dnsclient"
	"github.com/t0mer/dnsmon/internal/settings"
	"github.com/t0mer/dnsmon/internal/storage"
)

// CheckRunner performs a propagation check. *checker.Checker satisfies it.
type CheckRunner interface {
	Check(ctx context.Context, req checker.CheckRequest) (*dnsclient.Check, error)
}

// Notifier delivers a message to a channel. *notify.Sender satisfies it.
type Notifier interface {
	Send(ctx context.Context, ch settings.NotificationChannel, message string) error
}

// Evaluator runs monitors.
type Evaluator struct {
	check    CheckRunner
	store    storage.Storage
	notifier Notifier
}

// New returns an Evaluator.
func New(check CheckRunner, store storage.Storage, notifier Notifier) *Evaluator {
	return &Evaluator{check: check, store: store, notifier: notifier}
}

// RunByID loads a monitor and evaluates it once.
func (e *Evaluator) RunByID(ctx context.Context, id string) (*settings.MonitorEvent, error) {
	m, err := e.store.GetMonitor(ctx, id)
	if err != nil {
		return nil, err
	}
	return e.Run(ctx, m)
}

// Run evaluates a monitor once, records a changelog entry, re-baselines change
// monitors after a change, and notifies the bound channel on an alert.
func (e *Evaluator) Run(ctx context.Context, m *settings.Monitor) (*settings.MonitorEvent, error) {
	ev := &settings.MonitorEvent{
		ID:        settings.NewID(),
		MonitorID: m.ID,
		Timestamp: time.Now().UTC(),
	}

	check, err := e.check.Check(ctx, checker.CheckRequest{Name: m.FQDN, Type: m.RecordType, NoCache: true})
	if err != nil {
		ev.Status = settings.MonitorStatusError
		ev.Message = "Check failed: " + err.Error()
		_ = e.store.AppendMonitorEvent(ctx, ev)
		return ev, nil
	}

	switch m.Type {
	case settings.MonitorChange:
		e.evaluateChange(ctx, m, check, ev)
	default:
		e.evaluatePropagation(ctx, m, check, ev)
	}

	_ = e.store.AppendMonitorEvent(ctx, ev)
	return ev, nil
}

func (e *Evaluator) evaluateChange(ctx context.Context, m *settings.Monitor, check *dnsclient.Check, ev *settings.MonitorEvent) {
	observed := consensus(check)
	ev.Observed = observed

	switch {
	case len(m.Expected) == 0:
		m.Expected = observed
		m.UpdatedAt = time.Now().UTC()
		_ = e.store.SaveMonitor(ctx, m)
		ev.Status = settings.MonitorStatusOK
		ev.Message = "Baseline captured: " + join(observed)
	case !setEqual(observed, m.Expected):
		old := m.Expected
		ev.Status = settings.MonitorStatusChanged
		ev.Message = "Changed: " + join(old) + " → " + join(observed)
		// Re-baseline so each distinct change alerts once.
		m.Expected = observed
		m.UpdatedAt = time.Now().UTC()
		_ = e.store.SaveMonitor(ctx, m)
		ev.Notified = e.maybeNotify(ctx, m,
			fmt.Sprintf("[dnsmon] %s: value changed from %s to %s", label(m), join(old), join(observed)))
	default:
		ev.Status = settings.MonitorStatusOK
		ev.Message = "No change."
	}
}

func (e *Evaluator) evaluatePropagation(ctx context.Context, m *settings.Monitor, check *dnsclient.Check, ev *settings.MonitorEvent) {
	responded, matched := 0, 0
	for _, r := range check.Results {
		switch r.Status {
		case dnsclient.StatusOK:
			responded++
			if setEqual(answerValues(r), m.Expected) {
				matched++
			}
		case dnsclient.StatusNXDomain:
			responded++
		}
	}
	ev.Observed = consensus(check)

	if responded > 0 && matched == responded {
		ev.Status = settings.MonitorStatusOK
		ev.Message = fmt.Sprintf("Propagated: all %d resolvers return the expected value.", responded)
		return
	}
	ev.Status = settings.MonitorStatusNotPropagated
	ev.Message = fmt.Sprintf("Not propagated: %d of %d resolvers match the expected value.", matched, responded)
	ev.Notified = e.maybeNotify(ctx, m,
		fmt.Sprintf("[dnsmon] %s: propagation incomplete — %d/%d resolvers match expected %s",
			label(m), matched, responded, join(m.Expected)))
}

// maybeNotify sends to the monitor's bound channel if it exists and is enabled.
func (e *Evaluator) maybeNotify(ctx context.Context, m *settings.Monitor, message string) bool {
	if m.ChannelID == "" {
		return false
	}
	s, err := e.store.GetSettings(ctx)
	if err != nil {
		return false
	}
	for _, c := range s.Notifications {
		if c.ID == m.ChannelID && c.Enabled {
			return e.notifier.Send(ctx, c, message) == nil
		}
	}
	return false
}

func label(m *settings.Monitor) string {
	name := m.Name
	if name == "" {
		name = m.FQDN
	}
	return fmt.Sprintf("%s (%s %s)", name, m.FQDN, m.RecordType)
}

func answerValues(r dnsclient.ResolverResult) []string {
	vals := make([]string, 0, len(r.Answers))
	for _, a := range r.Answers {
		vals = append(vals, a.Value)
	}
	return vals
}

func normalize(vs []string) []string {
	c := append([]string(nil), vs...)
	sort.Strings(c)
	return c
}

func setEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	na, nb := normalize(a), normalize(b)
	for i := range na {
		if na[i] != nb[i] {
			return false
		}
	}
	return true
}

// consensus returns the most common answer-value set among OK results.
func consensus(check *dnsclient.Check) []string {
	counts := map[string]int{}
	repr := map[string][]string{}
	for _, r := range check.Results {
		if r.Status != dnsclient.StatusOK {
			continue
		}
		vs := normalize(answerValues(r))
		key := strings.Join(vs, ",")
		counts[key]++
		repr[key] = vs
	}
	best, bestN := "", 0
	for k, n := range counts {
		if n > bestN {
			best, bestN = k, n
		}
	}
	return repr[best]
}

func join(vs []string) string {
	if len(vs) == 0 {
		return "(empty)"
	}
	return strings.Join(vs, ", ")
}
