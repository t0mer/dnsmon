# gdns Architecture

## Overview

gdns is a self-hosted DNS propagation checker. A single Go binary embeds the frontend (HTML/CSS/JS), the curated resolver list, and serves all traffic. No separate web server, asset CDN, or runtime Node.js required.

---

## Request Flow

### Propagation Check (HTTP)

```
Browser
  │
  │  POST /api/v1/check  { name, type, save }
  ▼
chi Router  (internal/api/server.go)
  │
  ├── Middleware: request-id, logging, recovery, CORS, rate-limit
  │
  ▼
Handler: api/v1/check.go
  │
  ├── Input validation (validator/v10)
  │   └── SSRF guard: reject private IPs / .local domains
  │
  ▼
checker.Checker  (internal/checker/checker.go)
  │
  ├── Loads resolver list from Registry
  │
  ├── Fan-out: goroutine per resolver (bounded by semaphore)
  │   │
  │   ├── cache.Cache.Get(name|type|resolver)  — cache hit? return cached
  │   │
  │   └── dnsclient.Query(ctx, resolver, name, qtype)
  │         └── miekg/dns → UDP/TCP/DoT/DoH query
  │
  ├── Collects ResolverResult per resolver into []ResolverResult
  │
  ├── Computes CheckSummary (consensus, NXDOMAIN count, etc.)
  │
  ├── cache.Cache.Set(...)  — populate cache
  │
  └── storage.SaveCheck(...)  — persist if save=true
  │
  ▼
JSON response  → Browser
```

### Propagation Check (WebSocket / Live mode)

```
Browser
  │  WS upgrade GET /api/v1/check/stream
  ▼
Handler: api/v1/stream.go
  │
  ├── Reads JSON frame: { name, type }
  │
  ▼
checker.StreamCheck(ctx, params, resultsCh)
  │
  ├── Fan-out same as above, but each result is sent to resultsCh immediately
  │
  └── Handler reads from resultsCh and sends JSON frames:
        { "type": "result", "data": ResolverResult }
        { "type": "result", "data": ... }
        ...
        { "type": "done",   "data": { summary, id } }
```

---

## Package Dependency Graph

```
cmd/gdns
    └── internal/api
            ├── internal/checker
            │       ├── internal/dnsclient
            │       ├── internal/resolvers
            │       └── internal/cache
            ├── internal/ratelimit
            ├── internal/storage
            │       ├── internal/storage/sqlite
            │       └── internal/storage/postgres
            └── internal/version

internal/config     ← consumed by main, no upstream deps
internal/geoip      ← consumed by resolvers/registry, no upstream deps
internal/export     ← consumed by api/v1/export.go
```

Strict rule: no circular imports. `api` depends on `checker`; `checker` depends on `dnsclient`, `resolvers`, `cache`; nothing downstream imports `api`.

---

## Storage Options

| Driver   | Use case                                   | Notes                                  |
|----------|--------------------------------------------|----------------------------------------|
| `none`   | Stateless / ephemeral                      | No history, no permalinks              |
| `sqlite` | Single-node, dev, small deployments        | Pure-Go driver, zero external deps     |
| `postgres` | Multi-replica, production deployments    | `pgx` driver, connection pooling       |

The `storage.Storage` interface is implemented by both drivers so the rest of the code is driver-agnostic:

```go
type Storage interface {
    SaveCheck(ctx, *Check) error
    GetCheck(ctx, id string) (*Check, error)
    ListChecks(ctx, opts ListOptions) ([]*Check, int, error)
    DeleteCheck(ctx, id string) error
    // API key management
    CreateAPIKey(ctx, *APIKey) error
    GetAPIKey(ctx, hash string) (*APIKey, error)
    // ...
}
```

---

## Cache Strategy

Cache key: `sha256(name + "|" + type + "|" + resolverIP)`

TTL is configurable (default 60s). The cache prevents hammering the same resolver for the same query when many users check the same domain in quick succession.

Two implementations:
- `memory.go`: thread-safe LRU via `hashicorp/golang-lru/v2`. Evicts least-recently-used entries when capacity (`cache.size`) is reached.
- `redis.go`: Optional Redis backend for multi-replica deployments where a shared cache is desired.

Cache hit path: `checker` calls `cache.Get` before dispatching to `dnsclient`. On a hit the `ResolverResult` is returned immediately with `duration_ms = 0` and `queried_at` = original query time.

---

## Rate Limiting Design

Token bucket per `(IP, endpoint)` pair, implemented in `internal/ratelimit`.

- Anonymous (no API key): 30 req/min, burst 10.
- API key holder: 600 req/min, burst 60.
- Admin key: unlimited.
- WebSocket: 1 concurrent connection per IP (anonymous), 10 per API key.

Rate limit state lives in memory (default) or Redis (for multi-replica deployments). The chi middleware injects headers:

```
X-RateLimit-Limit: 30
X-RateLimit-Remaining: 27
X-RateLimit-Reset: 1716465600
```

HTTP 429 is returned with `Retry-After` on bucket exhaustion.

---

## WebSocket Streaming Protocol

```
Client → Server  (text frame, JSON, once)
{ "name": "example.com", "type": "A", "resolvers": [] }

Server → Client  (text frames, one per resolver as it answers)
{ "type": "total",  "data": { "total": 100 } }
{ "type": "result", "data": { ...ResolverResult... } }
{ "type": "result", "data": { ...ResolverResult... } }
...
{ "type": "done",   "data": { "summary": CheckSummary, "id": "a8x2k9" } }

On error:
{ "type": "error",  "data": { "code": "INVALID_NAME", "message": "..." } }
```

The `total` frame is sent immediately after the server resolves the resolver list, before any DNS queries begin. The frontend uses it to drive the progress bar.

---

## Frontend Architecture

The frontend is vanilla HTML + Alpine.js + Leaflet.js + Tailwind CSS. No bundler or runtime Node.js is needed.

- `web/src/*.html` — page templates
- `web/src/css/tailwind.src.css` — Tailwind input, compiled at build time
- `web/src/js/app.js` — Alpine.js components (`gdnsApp`, `lookupApp`, `reverseApp`)
- `web/src/js/map.js` — Leaflet map helpers
- `web/src/js/export.js` — download helper for export API

All assets are embedded into the Go binary via `//go:embed dist` in `web/embed.go` and served from memory. No external CDN access is required at runtime (Alpine and Leaflet are loaded from CDN in the HTML; for a fully air-gapped deployment, vendor these files into `web/dist/`).

---

## Observability

- **Logs**: structured JSON via `log/slog` to stdout. Fields: `request_id`, `resolver_id`, `qname`, `qtype`, `duration_ms`, `status`.
- **Metrics**: Prometheus at `/metrics`. Key metrics: `gdns_check_total`, `gdns_resolver_query_duration_seconds`, `gdns_cache_hits_total`.
- **Health**: `/api/v1/health` (liveness), `/api/v1/readyz` (readiness — checks DB + cache).

---

## Security Considerations

- All inbound domain names are validated against RFC 1035 and IDNA rules before querying.
- RFC1918, loopback, link-local, and `.local` targets are rejected (SSRF guard) unless `--allow-private` is set.
- API keys are hashed at rest (bcrypt/argon2id); only the hash is stored.
- `Content-Security-Policy: default-src 'self'` header on all HTML responses.
- TLS is expected to be terminated by an upstream reverse proxy (Nginx/Caddy/ingress-nginx).
