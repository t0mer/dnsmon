package checker_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/t0mer/dnsmon/internal/cache"
	"github.com/t0mer/dnsmon/internal/checker"
	"github.com/t0mer/dnsmon/internal/config"
	"github.com/t0mer/dnsmon/internal/dnsclient"
	"github.com/t0mer/dnsmon/internal/resolvers"
	"github.com/t0mer/dnsmon/internal/settings"
	"github.com/t0mer/dnsmon/internal/storage"
)

// fakeSettingsStore is a Noop store that reports a fixed disabled-resolver set.
type fakeSettingsStore struct {
	storage.Noop
	disabled []string
}

func (f *fakeSettingsStore) GetSettings(_ context.Context) (*settings.Settings, error) {
	s := settings.Default()
	s.DisabledResolvers = f.disabled
	return s, nil
}

// mockClient is a fake DNS client for testing.
type mockClient struct {
	results map[string]*dnsclient.ResolverResult
}

func (m *mockClient) Query(_ context.Context, resolver dnsclient.Resolver, name string, _ uint16) (*dnsclient.ResolverResult, error) {
	if r, ok := m.results[resolver.ID]; ok {
		return r, nil
	}
	return &dnsclient.ResolverResult{
		Resolver:  resolver,
		Status:    dnsclient.StatusOK,
		Answers:   []dnsclient.Answer{{Name: name, Type: "A", TTL: 300, Value: "1.2.3.4"}},
		QueriedAt: time.Now(),
	}, nil
}

// mockCache is a no-op cache for testing.
type mockCache struct{}

func (m *mockCache) Get(_ context.Context, _ string) ([]byte, bool)                    { return nil, false }
func (m *mockCache) Set(_ context.Context, _ string, _ []byte, _ time.Duration) error  { return nil }
func (m *mockCache) Delete(_ context.Context, _ string) error                          { return nil }
func (m *mockCache) Close() error                                                       { return nil }

func testConfig() *config.Config {
	return &config.Config{
		DNS: config.DNSConfig{
			QueryTimeout:           3 * time.Second,
			PerResolverConcurrency: 4,
			DefaultProtocol:        "udp",
		},
		Cache: config.CacheConfig{
			TTL: 60 * time.Second,
		},
	}
}

func testRegistry(resolverList []dnsclient.Resolver) *resolvers.Registry {
	reg := resolvers.NewRegistry()
	reg.Load(resolverList)
	return reg
}

func TestCheck_Success(t *testing.T) {
	res1 := dnsclient.Resolver{ID: "r1", Name: "R1", IP: "1.1.1.1", Port: 53}
	res2 := dnsclient.Resolver{ID: "r2", Name: "R2", IP: "8.8.8.8", Port: 53}

	client := &mockClient{}
	reg := testRegistry([]dnsclient.Resolver{res1, res2})
	store := &storage.Noop{}
	c := checker.New(client, reg, &mockCache{}, store, testConfig())

	check, err := c.Check(context.Background(), checker.CheckRequest{
		Name: "example.com",
		Type: "A",
	})

	require.NoError(t, err)
	assert.Equal(t, "example.com", check.Name)
	assert.Equal(t, "A", check.Type)
	assert.Len(t, check.Results, 2)
	assert.Equal(t, 2, check.Summary.TotalResolvers)
	assert.Equal(t, 2, check.Summary.Responded)
	assert.NotEmpty(t, check.ID)
}

func TestCheck_ExcludesDisabledResolvers(t *testing.T) {
	res1 := dnsclient.Resolver{ID: "r1", Name: "R1", IP: "1.1.1.1", Port: 53}
	res2 := dnsclient.Resolver{ID: "r2", Name: "R2", IP: "8.8.8.8", Port: 53}

	client := &mockClient{}
	reg := testRegistry([]dnsclient.Resolver{res1, res2})
	store := &fakeSettingsStore{disabled: []string{"r2"}}
	c := checker.New(client, reg, &mockCache{}, store, testConfig())

	check, err := c.Check(context.Background(), checker.CheckRequest{Name: "example.com", Type: "A"})
	require.NoError(t, err)

	assert.Len(t, check.Results, 1)
	assert.Equal(t, 1, check.Summary.TotalResolvers)
	assert.Equal(t, "r1", check.Results[0].Resolver.ID)
}

func TestLookup_DisabledResolverRejected(t *testing.T) {
	res1 := dnsclient.Resolver{ID: "r1", Name: "R1", IP: "1.1.1.1", Port: 53}
	client := &mockClient{}
	reg := testRegistry([]dnsclient.Resolver{res1})
	store := &fakeSettingsStore{disabled: []string{"r1"}}
	c := checker.New(client, reg, &mockCache{}, store, testConfig())

	_, err := c.Lookup(context.Background(), checker.LookupRequest{Name: "example.com", Type: "A", Resolver: "r1"})
	assert.Error(t, err)
}

func TestCheck_OneTimeout(t *testing.T) {
	res1 := dnsclient.Resolver{ID: "r1", Name: "R1", IP: "1.1.1.1", Port: 53}
	res2 := dnsclient.Resolver{ID: "r2", Name: "R2", IP: "8.8.8.8", Port: 53}

	client := &mockClient{
		results: map[string]*dnsclient.ResolverResult{
			"r2": {
				Resolver:  res2,
				Status:    dnsclient.StatusTimeout,
				Error:     "timeout",
				QueriedAt: time.Now(),
			},
		},
	}

	reg := testRegistry([]dnsclient.Resolver{res1, res2})
	store := &storage.Noop{}
	c := checker.New(client, reg, &mockCache{}, store, testConfig())

	check, err := c.Check(context.Background(), checker.CheckRequest{
		Name: "example.com",
		Type: "A",
	})

	require.NoError(t, err)
	assert.Equal(t, 2, check.Summary.TotalResolvers)
	assert.Equal(t, 1, check.Summary.Timeouts)
}

func TestCheck_InvalidDomain(t *testing.T) {
	client := &mockClient{}
	reg := testRegistry(nil)
	store := &storage.Noop{}
	c := checker.New(client, reg, &mockCache{}, store, testConfig())

	_, err := c.Check(context.Background(), checker.CheckRequest{
		Name: "invalid domain!",
		Type: "A",
	})
	assert.Error(t, err)
}

func TestCheck_UnsupportedType(t *testing.T) {
	client := &mockClient{}
	reg := testRegistry(nil)
	store := &storage.Noop{}
	c := checker.New(client, reg, &mockCache{}, store, testConfig())

	_, err := c.Check(context.Background(), checker.CheckRequest{
		Name: "example.com",
		Type: "FOOBAR",
	})
	assert.Error(t, err)
}

func TestCheck_PrivateDomainBlocked(t *testing.T) {
	client := &mockClient{}
	reg := testRegistry(nil)
	store := &storage.Noop{}
	c := checker.New(client, reg, &mockCache{}, store, testConfig())

	_, err := c.Check(context.Background(), checker.CheckRequest{
		Name: "myhost.local",
		Type: "A",
	})
	assert.Error(t, err)
}

func TestStream_DeliversResults(t *testing.T) {
	res1 := dnsclient.Resolver{ID: "r1", Name: "R1", IP: "1.1.1.1", Port: 53}
	res2 := dnsclient.Resolver{ID: "r2", Name: "R2", IP: "8.8.8.8", Port: 53}

	client := &mockClient{}
	reg := testRegistry([]dnsclient.Resolver{res1, res2})
	store := &storage.Noop{}
	c := checker.New(client, reg, &mockCache{}, store, testConfig())

	total, id, resultCh, doneCh, err := c.Stream(context.Background(), checker.StreamRequest{
		Name: "example.com",
		Type: "A",
	}, false)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.NotEmpty(t, id)

	var results []*dnsclient.ResolverResult
	for r := range resultCh {
		results = append(results, r)
	}

	summary := <-doneCh

	assert.Len(t, results, 2)
	require.NotNil(t, summary)
	assert.Equal(t, 2, summary.TotalResolvers)
}

func TestCache_Integration(t *testing.T) {
	res1 := dnsclient.Resolver{ID: "r1", Name: "R1", IP: "1.1.1.1", Port: 53}

	callCount := 0
	client := &countingClient{
		inner:     &mockClient{},
		callCount: &callCount,
	}

	mem, err := cache.NewMemory(100)
	require.NoError(t, err)

	reg := testRegistry([]dnsclient.Resolver{res1})
	store := &storage.Noop{}
	c := checker.New(client, reg, mem, store, testConfig())

	ctx := context.Background()

	_, err = c.Check(ctx, checker.CheckRequest{Name: "example.com", Type: "A"})
	require.NoError(t, err)
	first := callCount

	_, err = c.Check(ctx, checker.CheckRequest{Name: "example.com", Type: "A"})
	require.NoError(t, err)

	// Second check should hit cache, so call count should not increase
	assert.Equal(t, first, callCount)
}

func TestNoCache_AlwaysQueries(t *testing.T) {
	res1 := dnsclient.Resolver{ID: "r1", Name: "R1", IP: "1.1.1.1", Port: 53}

	callCount := 0
	client := &countingClient{inner: &mockClient{}, callCount: &callCount}
	mem, err := cache.NewMemory(100)
	require.NoError(t, err)

	reg := testRegistry([]dnsclient.Resolver{res1})
	c := checker.New(client, reg, mem, &storage.Noop{}, testConfig())
	ctx := context.Background()

	_, err = c.Check(ctx, checker.CheckRequest{Name: "example.com", Type: "A", NoCache: true})
	require.NoError(t, err)
	first := callCount

	// With NoCache the second check must query again, not serve from cache.
	_, err = c.Check(ctx, checker.CheckRequest{Name: "example.com", Type: "A", NoCache: true})
	require.NoError(t, err)
	assert.Greater(t, callCount, first, "NoCache check should re-query, not use the cache")
}

type countingClient struct {
	inner     dnsclient.Client
	callCount *int
}

func (c *countingClient) Query(ctx context.Context, resolver dnsclient.Resolver, name string, qtype uint16) (*dnsclient.ResolverResult, error) {
	*c.callCount++
	return c.inner.Query(ctx, resolver, name, qtype)
}
