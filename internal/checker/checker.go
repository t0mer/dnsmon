package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/t0mer/dnsmon/internal/cache"
	"github.com/t0mer/dnsmon/internal/config"
	"github.com/t0mer/dnsmon/internal/dnsclient"
	"github.com/t0mer/dnsmon/internal/resolvers"
	"github.com/t0mer/dnsmon/internal/storage"
)

const maxResolvers = 200

var domainRE = regexp.MustCompile(`^[a-zA-Z0-9._\-]+$`)

// CheckRequest describes a propagation check.
type CheckRequest struct {
	Name            string
	Type            string
	ResolverIDs     []string
	CustomResolvers []dnsclient.Resolver
	Save            bool
	AllowPrivate    bool
	// NoCache forces a fresh query to every resolver, bypassing the response
	// cache (used by monitors so each run is a full live test).
	NoCache bool
}

// LookupRequest describes a single-resolver detailed lookup.
type LookupRequest struct {
	Name     string
	Type     string
	Resolver string
	Protocol string
}

type metrics struct {
	checkTotal      *prometheus.CounterVec
	queryDuration   *prometheus.HistogramVec
	queryTotal      *prometheus.CounterVec
	cacheHits       prometheus.Counter
	cacheMisses     prometheus.Counter
}

func registerOrGet[C prometheus.Collector](c C) C {
	if err := prometheus.DefaultRegisterer.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(C)
		}
		panic(err)
	}
	return c
}

func newMetrics() *metrics {
	return &metrics{
		checkTotal: registerOrGet(prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dnsmon_check_total",
			Help: "Total DNS propagation checks.",
		}, []string{"type", "status"})),
		queryDuration: registerOrGet(prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "dnsmon_resolver_query_duration_seconds",
			Help:    "DNS query duration in seconds per resolver.",
			Buckets: prometheus.DefBuckets,
		}, []string{"resolver_id", "status"})),
		queryTotal: registerOrGet(prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dnsmon_resolver_query_total",
			Help: "Total DNS queries per resolver.",
		}, []string{"resolver_id", "status"})),
		cacheHits: registerOrGet(prometheus.NewCounter(prometheus.CounterOpts{
			Name: "dnsmon_cache_hits_total",
			Help: "Total cache hits.",
		})),
		cacheMisses: registerOrGet(prometheus.NewCounter(prometheus.CounterOpts{
			Name: "dnsmon_cache_misses_total",
			Help: "Total cache misses.",
		})),
	}
}

// Checker orchestrates DNS propagation checks.
type Checker struct {
	client   dnsclient.Client
	registry *resolvers.Registry
	cache    cache.Cache
	storage  storage.Storage
	cfg      *config.Config
	m        *metrics
}

// New returns a new Checker.
func New(
	client dnsclient.Client,
	registry *resolvers.Registry,
	c cache.Cache,
	store storage.Storage,
	cfg *config.Config,
) *Checker {
	return &Checker{
		client:   client,
		registry: registry,
		cache:    c,
		storage:  store,
		cfg:      cfg,
		m:        newMetrics(),
	}
}

// Check runs a DNS propagation check across resolvers.
func (c *Checker) Check(ctx context.Context, req CheckRequest) (*dnsclient.Check, error) {
	if err := validateRequest(req); err != nil {
		return nil, err
	}

	resolverList, err := c.buildResolverList(ctx, req)
	if err != nil {
		return nil, err
	}

	results := c.fanOut(ctx, req.Name, req.Type, resolverList, req.NoCache)

	check := &dnsclient.Check{
		ID:        NewID(req.Name, req.Type),
		Name:      req.Name,
		Type:      req.Type,
		CreatedAt: time.Now().UTC(),
		Results:   results,
		Summary:   buildSummary(results),
	}

	status := "ok"
	if check.Summary.Errors > 0 || check.Summary.Timeouts > 0 {
		status = "partial"
	}
	c.m.checkTotal.WithLabelValues(req.Type, status).Inc()

	if req.Save {
		if err := c.storage.SaveCheck(ctx, check); err != nil {
			return check, fmt.Errorf("saving check: %w", err)
		}
	}

	return check, nil
}

// Lookup performs a detailed single-resolver DNS lookup.
func (c *Checker) Lookup(ctx context.Context, req LookupRequest) (*dnsclient.ResolverResult, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if _, ok := dnsclient.ParseType(req.Type); !ok {
		return nil, fmt.Errorf("unsupported record type: %s", req.Type)
	}

	var resolver dnsclient.Resolver

	if req.Resolver == "" {
		resolver = dnsclient.Resolver{ID: "custom", Name: "Custom", IP: "8.8.8.8", Port: 53, Protocol: "udp"}
	} else if r, ok := c.registry.Get(req.Resolver); ok {
		if _, off := c.disabledResolverSet(ctx)[req.Resolver]; off {
			return nil, fmt.Errorf("resolver is disabled: %s", req.Resolver)
		}
		resolver = r
	} else {
		resolver = dnsclient.Resolver{
			ID:       "custom",
			Name:     req.Resolver,
			IP:       req.Resolver,
			Port:     53,
			Protocol: req.Protocol,
		}
		if resolver.Protocol == "" {
			resolver.Protocol = c.cfg.DNS.DefaultProtocol
		}
	}

	qtype, _ := dnsclient.ParseType(req.Type)
	result, err := c.client.Query(ctx, resolver, req.Name, qtype)
	if err != nil {
		return nil, fmt.Errorf("querying resolver: %w", err)
	}
	return result, nil
}

func (c *Checker) buildResolverList(ctx context.Context, req CheckRequest) ([]dnsclient.Resolver, error) {
	disabled := c.disabledResolverSet(ctx)

	var list []dnsclient.Resolver

	if len(req.ResolverIDs) > 0 {
		for _, id := range req.ResolverIDs {
			if _, off := disabled[id]; off {
				continue
			}
			r, ok := c.registry.Get(id)
			if !ok {
				return nil, fmt.Errorf("resolver not found: %s", id)
			}
			list = append(list, r)
		}
	} else {
		for _, r := range c.registry.All() {
			if _, off := disabled[r.ID]; off {
				continue
			}
			list = append(list, r)
		}
	}

	// Custom (ad-hoc) resolvers are not subject to the disabled list.
	list = append(list, req.CustomResolvers...)

	if len(list) > maxResolvers {
		list = list[:maxResolvers]
	}

	return list, nil
}

// disabledResolverSet returns the set of resolver IDs the operator has disabled
// in settings. It fails open (empty set) if settings cannot be read.
func (c *Checker) disabledResolverSet(ctx context.Context) map[string]struct{} {
	set := make(map[string]struct{})
	if c.storage == nil {
		return set
	}
	s, err := c.storage.GetSettings(ctx)
	if err != nil || s == nil {
		return set
	}
	for _, id := range s.DisabledResolvers {
		set[id] = struct{}{}
	}
	return set
}

func (c *Checker) fanOut(ctx context.Context, name, qtype string, resolverList []dnsclient.Resolver, noCache bool) []dnsclient.ResolverResult {
	concurrency := c.cfg.DNS.PerResolverConcurrency
	if concurrency <= 0 {
		concurrency = 4
	}

	sem := make(chan struct{}, concurrency)
	results := make([]dnsclient.ResolverResult, len(resolverList))
	var wg sync.WaitGroup

	for i, r := range resolverList {
		wg.Add(1)
		go func(idx int, resolver dnsclient.Resolver) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := c.queryWithCache(ctx, resolver, name, qtype, noCache)
			results[idx] = *result

			c.m.queryTotal.WithLabelValues(resolver.ID, result.Status).Inc()
			c.m.queryDuration.WithLabelValues(resolver.ID, result.Status).
				Observe(float64(result.DurationMS) / 1000)
		}(i, r)
	}

	wg.Wait()
	return results
}

func (c *Checker) queryWithCache(ctx context.Context, resolver dnsclient.Resolver, name, qtype string, noCache bool) *dnsclient.ResolverResult {
	cacheKey := cache.Key(resolver.ID, name, qtype)

	// noCache bypasses the cache entirely so each call is a fresh live query.
	if !noCache {
		if data, ok := c.cache.Get(ctx, cacheKey); ok {
			c.m.cacheHits.Inc()
			var result dnsclient.ResolverResult
			if err := json.Unmarshal(data, &result); err == nil {
				return &result
			}
		}
		c.m.cacheMisses.Inc()
	}

	qtypeNum, _ := dnsclient.ParseType(qtype)
	result, err := c.client.Query(ctx, resolver, name, qtypeNum)
	if err != nil {
		return &dnsclient.ResolverResult{
			Resolver:   resolver,
			Status:     dnsclient.StatusError,
			Error:      err.Error(),
			QueriedAt:  time.Now().UTC(),
		}
	}

	if !noCache {
		if data, err := json.Marshal(result); err == nil {
			_ = c.cache.Set(ctx, cacheKey, data, c.cfg.Cache.TTL)
		}
	}

	return result
}

func buildSummary(results []dnsclient.ResolverResult) dnsclient.CheckSummary {
	s := dnsclient.CheckSummary{
		TotalResolvers: len(results),
		Consensus:      make(map[string]int),
	}

	for _, r := range results {
		switch r.Status {
		case dnsclient.StatusOK:
			s.Responded++
			key := answersKey(r.Answers)
			s.Consensus[key]++
		case dnsclient.StatusNXDomain:
			s.NXDomain++
			s.Responded++
		case dnsclient.StatusTimeout:
			s.Timeouts++
		default:
			s.Errors++
		}
	}

	seen := make(map[string]struct{})
	for k := range s.Consensus {
		seen[k] = struct{}{}
	}
	s.UniqueAnswerSets = len(seen)

	return s
}

func answersKey(answers []dnsclient.Answer) string {
	if len(answers) == 0 {
		return "<empty>"
	}
	parts := make([]string, len(answers))
	for i, a := range answers {
		parts[i] = a.Value
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func validateRequest(req CheckRequest) error {
	if req.Name == "" {
		return fmt.Errorf("name is required")
	}
	if !domainRE.MatchString(req.Name) {
		return fmt.Errorf("invalid domain name: %s", req.Name)
	}
	if _, ok := dnsclient.ParseType(req.Type); !ok {
		return fmt.Errorf("unsupported record type: %s", req.Type)
	}
	if len(req.ResolverIDs)+len(req.CustomResolvers) > maxResolvers {
		return fmt.Errorf("too many resolvers: max %d", maxResolvers)
	}
	if !req.AllowPrivate {
		if isPrivateOrLocal(req.Name) {
			return fmt.Errorf("private or local domain names are not allowed")
		}
	}
	return nil
}

func isPrivateOrLocal(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".local") || lower == "local" {
		return true
	}
	ip := net.ParseIP(name)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}
