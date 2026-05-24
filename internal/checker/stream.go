package checker

import (
	"context"
	"sync"
	"time"

	"github.com/t0mer/dnsmon/internal/dnsclient"
)

// StreamRequest describes a streaming DNS propagation check.
type StreamRequest struct {
	Name            string
	Type            string
	ResolverIDs     []string
	CustomResolvers []dnsclient.Resolver
	AllowPrivate    bool
}

// Stream fans out DNS queries and pushes each ResolverResult to the results
// channel as it arrives; the done channel receives the final CheckSummary.
// Validation and resolver-list resolution happen synchronously so the caller
// gets the total resolver count (for progress reporting) and a stable check ID
// up front; an error is returned if the request is invalid. When save is true
// the completed check is persisted under that ID. Both channels close when the
// check completes or ctx is cancelled.
func (c *Checker) Stream(ctx context.Context, req StreamRequest, save bool) (int, string, <-chan *dnsclient.ResolverResult, <-chan *dnsclient.CheckSummary, error) {
	checkReq := CheckRequest{
		Name:            req.Name,
		Type:            req.Type,
		ResolverIDs:     req.ResolverIDs,
		CustomResolvers: req.CustomResolvers,
		AllowPrivate:    req.AllowPrivate,
	}
	if err := validateRequest(checkReq); err != nil {
		return 0, "", nil, nil, err
	}

	resolverList, err := c.buildResolverList(ctx, checkReq)
	if err != nil {
		return 0, "", nil, nil, err
	}

	id := NewID(req.Name, req.Type)
	resultCh := make(chan *dnsclient.ResolverResult, 64)
	doneCh := make(chan *dnsclient.CheckSummary, 1)

	go func() {
		defer close(resultCh)
		defer close(doneCh)

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

				result := c.queryWithCache(ctx, resolver, req.Name, req.Type, true)

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

		// Don't persist a partial check if the client disconnected mid-stream.
		if ctx.Err() != nil {
			return
		}

		summary := buildSummary(results)
		status := "ok"
		if summary.Errors > 0 || summary.Timeouts > 0 {
			status = "partial"
		}
		c.m.checkTotal.WithLabelValues(req.Type, status).Inc()

		if save {
			check := &dnsclient.Check{
				ID:        id,
				Name:      req.Name,
				Type:      req.Type,
				CreatedAt: time.Now().UTC(),
				Results:   results,
				Summary:   summary,
			}
			_ = c.storage.SaveCheck(ctx, check)
		}

		doneCh <- &summary
	}()

	return len(resolverList), id, resultCh, doneCh, nil
}
