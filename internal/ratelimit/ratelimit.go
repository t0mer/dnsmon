package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Limiter is a token-bucket rate limiter keyed by a string (IP or API key).
type Limiter struct {
	mu         sync.Mutex
	buckets    map[string]*bucket
	ratePerMin int
	burst      int
}

type bucket struct {
	tokens     float64
	lastRefill time.Time
}

// New returns a new Limiter with the given rate (per minute) and burst size.
// It starts a background goroutine that evicts stale buckets; the goroutine
// stops when ctx is cancelled.
func New(ctx context.Context, ratePerMin, burst int) *Limiter {
	l := &Limiter{
		buckets:    make(map[string]*bucket),
		ratePerMin: ratePerMin,
		burst:      burst,
	}
	go l.cleanup(ctx)
	return l
}

// Allow reports whether a request from key is permitted.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{
			tokens:     float64(l.burst),
			lastRefill: now,
		}
		l.buckets[key] = b
	}

	elapsed := now.Sub(b.lastRefill).Minutes()
	b.tokens += elapsed * float64(l.ratePerMin)
	if b.tokens > float64(l.burst) {
		b.tokens = float64(l.burst)
	}
	b.lastRefill = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Reset removes the rate-limit bucket for the given key.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	delete(l.buckets, key)
	l.mu.Unlock()
}

func (l *Limiter) cleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.evictStale()
		}
	}
}

func (l *Limiter) evictStale() {
	cutoff := time.Now().Add(-5 * time.Minute)
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, b := range l.buckets {
		if b.lastRefill.Before(cutoff) {
			delete(l.buckets, key)
		}
	}
}
