package v1_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/tomerklein/gdns/internal/api/v1"
	"github.com/tomerklein/gdns/internal/checker"
	"github.com/tomerklein/gdns/internal/config"
	"github.com/tomerklein/gdns/internal/dnsclient"
	"github.com/tomerklein/gdns/internal/resolvers"
	"github.com/tomerklein/gdns/internal/storage"
)

// mockDNSClient is a stub DNS client.
type mockDNSClient struct{}

func (m *mockDNSClient) Query(_ context.Context, resolver dnsclient.Resolver, name string, _ uint16) (*dnsclient.ResolverResult, error) {
	return &dnsclient.ResolverResult{
		Resolver:  resolver,
		Status:    dnsclient.StatusOK,
		Answers:   []dnsclient.Answer{{Name: name, Type: "A", TTL: 300, Value: "93.184.216.34"}},
		QueriedAt: time.Now(),
	}, nil
}

// mockCache discards all cache operations.
type mockCache struct{}

func (m *mockCache) Get(_ context.Context, _ string) ([]byte, bool)                    { return nil, false }
func (m *mockCache) Set(_ context.Context, _ string, _ []byte, _ time.Duration) error  { return nil }
func (m *mockCache) Delete(_ context.Context, _ string) error                          { return nil }
func (m *mockCache) Close() error                                                       { return nil }

func newTestChecker() *checker.Checker {
	res := dnsclient.Resolver{ID: "test-r1", Name: "Test R1", IP: "1.1.1.1", Port: 53}
	reg := resolvers.NewRegistry()
	reg.Load([]dnsclient.Resolver{res})

	cfg := &config.Config{
		DNS: config.DNSConfig{
			QueryTimeout:           3 * time.Second,
			PerResolverConcurrency: 4,
			DefaultProtocol:        "udp",
		},
		Cache: config.CacheConfig{TTL: 60 * time.Second},
	}

	return checker.New(&mockDNSClient{}, reg, &mockCache{}, &storage.Noop{}, cfg)
}

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rr := httptest.NewRecorder()

	http.HandlerFunc(v1.Health).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}

func TestVersion(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	rr := httptest.NewRecorder()

	http.HandlerFunc(v1.Version).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Contains(t, body, "version")
}

func TestPostCheck_Success(t *testing.T) {
	chkr := newTestChecker()
	handler := v1.PostCheck(chkr)

	body := `{"name":"example.com","type":"A","save":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/check", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var check dnsclient.Check
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &check))
	assert.Equal(t, "example.com", check.Name)
	assert.Equal(t, "A", check.Type)
	assert.NotEmpty(t, check.ID)
}

func TestPostCheck_InvalidDomain(t *testing.T) {
	chkr := newTestChecker()
	handler := v1.PostCheck(chkr)

	body := `{"name":"bad domain!","type":"A"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/check", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetCheck_NotFound(t *testing.T) {
	store := &storage.Noop{}
	handler := v1.GetCheck(store)

	r := chi.NewRouter()
	r.Get("/api/v1/check/{id}", handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/check/nonexistent", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestRecordTypes(t *testing.T) {
	handler := v1.RecordTypes()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/record-types", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var types []map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &types))
	assert.NotEmpty(t, types)

	// Verify A record is present
	found := false
	for _, t2 := range types {
		if t2["name"] == "A" {
			found = true
			break
		}
	}
	assert.True(t, found, "A record type should be present")
}

func TestListResolvers(t *testing.T) {
	res := dnsclient.Resolver{ID: "google-us", Name: "Google", IP: "8.8.8.8", Port: 53, Country: "US"}
	reg := resolvers.NewRegistry()
	reg.Load([]dnsclient.Resolver{res})

	handler := v1.ListResolvers(reg)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/resolvers", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var list []dnsclient.Resolver
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &list))
	assert.Len(t, list, 1)
}

func TestGetResolver_NotFound(t *testing.T) {
	reg := resolvers.NewRegistry()
	handler := v1.GetResolver(reg)

	r := chi.NewRouter()
	r.Get("/api/v1/resolvers/{id}", handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resolvers/nonexistent", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestReadyz_WithNoop(t *testing.T) {
	store := &storage.Noop{}
	handler := v1.Readyz(store)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/readyz", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}
