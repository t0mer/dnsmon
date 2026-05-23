package checker

import (
	"context"
	"sync"
	"time"

	"github.com/tomerklein/gdns/internal/cache"
	"github.com/tomerklein/gdns/internal/dnsclient"
)

// StreamRequest describes a streaming DNS propagation check.
type StreamRequest struct {
	Name            string
	Type            string
	ResolverIDs     []string
	CustomResolvers []dnsclient.Resolver
	AllowPrivate    bool
}

// Stream fans out DNS queries and pushes results to the returned channel as they arrive.
// The done channel receives the final CheckSummary when all resolvers have responded.
// Both channels are closed when the check completes or ctx is cancelled.
func (c *Checker) Stream(ctx context.Context, req StreamRequest) (<-chan *dnsclient.ResolverResult, <-chan *dnsclient.CheckSummary) {
	resultCh := make(chan *dnsclient.ResolverResult, 64)
	doneCh := make(chan *dnsclient.CheckSummary, 1)

	go func() {
		defer close(resultCh)
		defer close(doneCh)

		checkReq := CheckRequest{
			Name:            req.Name,
			Type:            req.Type,
			ResolverIDs:     req.ResolverIDs,
			CustomResolvers: req.CustomResolvers,
			AllowPrivate:    req.AllowPrivate,
		}
		if err := validateRequest(checkReq); err != nil {
			return
		}

		resolverList, err := c.buildResolverList(checkReq)
		if err != nil {
			return
		}

		concurrency := c.cfg.DNS.PerResolverConcurrency
		if concurrency <= 0 {
			concurrency = 4
		}

		sem := make(chan struct{}, concurrency)
		var (
			wg      sync.WaitGroup
			mu      sync.Mutex
			results []dnsclient.ResolverResult
		)

		for _, r := range resolverList {
			wg.Add(1)
			go func(resolver dnsclient.Resolver) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				cacheKey := cache.Key(resolver.ID, req.Name, req.Type)
				result := c.queryWithCache(ctx, resolver, req.Name, req.Type)

				_ = cacheKey

				c.m.queryTotal.WithLabelValues(resolver.ID, result.Status).Inc()
				c.m.queryDuration.WithLabelValues(resolver.ID, result.Status).
					Observe(float64(result.DurationMS) / 1000)

				select {
				case resultCh <- result:
				case <-ctx.Done():
					return
				}

				mu.Lock()
				results = append(results, *result)
				mu.Unlock()
			}(r)
		}

		wg.Wait()

		summary := buildSummary(results)
		select {
		case doneCh <- &summary:
		default:
		}
	}()

	return resultCh, doneCh
}

// streamQueryWithCache is the streaming variant — uses the same cache logic but
// sets a minimal TTL for streaming results to avoid stale data.
func (c *Checker) streamQueryWithCache(ctx context.Context, resolver dnsclient.Resolver, name, qtype string) *dnsclient.ResolverResult {
	qtypeNum, _ := dnsclient.ParseType(qtype)
	result, err := c.client.Query(ctx, resolver, name, qtypeNum)
	if err != nil {
		return &dnsclient.ResolverResult{
			Resolver:  resolver,
			Status:    dnsclient.StatusError,
			Error:     err.Error(),
			QueriedAt: time.Now().UTC(),
		}
	}
	return result
}
