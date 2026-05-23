package cache

import (
	"context"
	"crypto/sha1"
	"fmt"
	"time"

	"github.com/t0mer/dnsmon/internal/config"
)

// Cache is a key-value store for DNS query results.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Close() error
}

// Key returns a cache key for the given resolver ID, query name, and query type.
func Key(resolverID, name, qtype string) string {
	h := sha1.New()
	fmt.Fprintf(h, "%s|%s|%s", resolverID, name, qtype)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// New constructs a Cache based on the provided configuration.
func New(cfg *config.Config) (Cache, error) {
	switch cfg.Cache.Driver {
	case "redis":
		if cfg.Cache.RedisAddr == "" {
			return nil, fmt.Errorf("cache driver redis requires cache.redis_addr")
		}
		r, err := NewRedis(cfg.Cache.RedisAddr)
		if err != nil {
			return nil, err
		}
		return r, nil
	case "none":
		return &noopCache{}, nil
	default:
		m, err := NewMemory(cfg.Cache.Size)
		if err != nil {
			return nil, fmt.Errorf("creating memory cache: %w", err)
		}
		return m, nil
	}
}

type noopCache struct{}

func (n *noopCache) Get(_ context.Context, _ string) ([]byte, bool)            { return nil, false }
func (n *noopCache) Set(_ context.Context, _ string, _ []byte, _ time.Duration) error { return nil }
func (n *noopCache) Delete(_ context.Context, _ string) error                  { return nil }
func (n *noopCache) Close() error                                               { return nil }
