package resolvers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/t0mer/dnsmon/internal/config"
	"github.com/t0mer/dnsmon/internal/dnsclient"
)

// Registry holds the set of known resolvers.
type Registry struct {
	mu        sync.RWMutex
	resolvers []dnsclient.Resolver
	byID      map[string]dnsclient.Resolver
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		byID: make(map[string]dnsclient.Resolver),
	}
}

// Load replaces all resolvers in the registry with the provided slice.
func (r *Registry) Load(resolvers []dnsclient.Resolver) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.resolvers = make([]dnsclient.Resolver, 0, len(resolvers))
	r.byID = make(map[string]dnsclient.Resolver, len(resolvers))

	for _, res := range resolvers {
		if IsValid(res) {
			r.resolvers = append(r.resolvers, res)
			r.byID[res.ID] = res
		}
	}
}

// Get returns the resolver with the given ID.
func (r *Registry) Get(id string) (dnsclient.Resolver, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res, ok := r.byID[id]
	return res, ok
}

// All returns a copy of all resolvers in the registry.
func (r *Registry) All() []dnsclient.Resolver {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]dnsclient.Resolver, len(r.resolvers))
	copy(out, r.resolvers)
	return out
}

// Filter returns resolvers matching the given country codes and/or free-text query.
// An empty countries slice and empty q string returns all resolvers.
func (r *Registry) Filter(countries []string, q string) []dnsclient.Resolver {
	r.mu.RLock()
	defer r.mu.RUnlock()

	q = strings.ToLower(q)
	countrySet := make(map[string]struct{}, len(countries))
	for _, c := range countries {
		countrySet[strings.ToUpper(c)] = struct{}{}
	}

	var out []dnsclient.Resolver
	for _, res := range r.resolvers {
		if len(countrySet) > 0 {
			if _, ok := countrySet[res.Country]; !ok {
				continue
			}
		}
		if q != "" {
			haystack := strings.ToLower(res.Name + " " + res.City + " " + res.Country + " " + res.IP + " " + res.ISP)
			if !strings.Contains(haystack, q) {
				continue
			}
		}
		out = append(out, res)
	}
	return out
}

// LoadFile reads a JSON file containing a resolver array and merges them into the registry.
func (r *Registry) LoadFile(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading resolvers file %s: %w", path, err)
	}

	var loaded []dnsclient.Resolver
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("parsing resolvers file %s: %w", path, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, res := range loaded {
		if IsValid(res) {
			r.resolvers = append(r.resolvers, res)
			r.byID[res.ID] = res
		}
	}
	return nil
}

// Reload rebuilds the registry from the builtin list and optional config file.
func (r *Registry) Reload(ctx context.Context, cfg *config.Config, builtin []dnsclient.Resolver) error {
	var all []dnsclient.Resolver

	if cfg.Resolvers.Builtin {
		all = append(all, builtin...)
	}

	r.Load(all)

	if cfg.Resolvers.File != "" {
		if err := r.LoadFile(ctx, cfg.Resolvers.File); err != nil {
			return fmt.Errorf("loading resolver file: %w", err)
		}
	}

	return nil
}
