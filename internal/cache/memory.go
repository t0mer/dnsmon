package cache

import (
	"context"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

type entry struct {
	value     []byte
	expiresAt time.Time
}

// Memory is an in-memory LRU cache implementation.
type Memory struct {
	lru *lru.Cache[string, entry]
}

// NewMemory returns a new in-memory LRU cache with the given capacity.
func NewMemory(size int) (*Memory, error) {
	c, err := lru.New[string, entry](size)
	if err != nil {
		return nil, err
	}
	return &Memory{lru: c}, nil
}

// Get retrieves a value from the cache. Returns false if missing or expired.
func (m *Memory) Get(_ context.Context, key string) ([]byte, bool) {
	e, ok := m.lru.Get(key)
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiresAt) {
		m.lru.Remove(key)
		return nil, false
	}
	return e.value, true
}

// Set stores a value in the cache with the given TTL.
func (m *Memory) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	m.lru.Add(key, entry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	})
	return nil
}

// Delete removes a value from the cache.
func (m *Memory) Delete(_ context.Context, key string) error {
	m.lru.Remove(key)
	return nil
}

// Close is a no-op for the in-memory cache.
func (m *Memory) Close() error { return nil }
