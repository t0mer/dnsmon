# CLAUDE.md — Self-Hosted DNS Propagation Checker

> Project codename: **`dnsmon`** (Global DNS Checker)
> A self-hosted, open-source alternative to [whatsmydns.net](https://www.whatsmydns.net/), written in **Go**.

This file is the single source of truth for Claude (and any other contributor / agent) working on this codebase. Read it in full before generating, modifying, or refactoring anything.

---

## 1. Project Overview

`dnsmon` is a web service that performs **DNS propagation checks** by querying a configurable list of public (and optionally private) DNS resolvers around the world and returning the answers each resolver gives for a requested record. It mirrors the feature set of `whatsmydns.net`:

- Global DNS propagation checks across 100+ resolvers
- Single-resolver authoritative DNS lookup ("DNS Lookup" tool)
- World-map visualization of results
- Support for all common record types: **A, AAAA, CNAME, MX, NS, PTR, SOA, TXT, CAA, SRV, DS, DNSKEY, NAPTR, TLSA, SPF (TXT), SVCB, HTTPS**
- Reverse DNS lookups (PTR for IPv4 / IPv6)
- Permalinks for sharing results
- Export results as JSON, CSV, PNG, PDF, SVG
- Public REST API + WebSocket streaming
- Self-hostable, single static binary, optional Postgres / SQLite for history
- Rate limiting, caching, observability built in

The goal: **deploy with one `docker compose up` and have a private whatsmydns inside your own infrastructure.**

---

## 2. Tech Stack (non-negotiable defaults)

| Concern              | Choice                                             | Reason                                                                  |
| -------------------- | -------------------------------------------------- | ----------------------------------------------------------------------- |
| Language             | Go 1.23+                                           | Single static binary, great concurrency for fan-out DNS queries.        |
| HTTP router          | `chi` (`github.com/go-chi/chi/v5`)                 | Idiomatic, fast, middleware-friendly, std-lib `http.Handler` compatible. |
| DNS library          | `github.com/miekg/dns`                             | Industry standard, supports every record type we need.                  |
| Config               | `github.com/spf13/viper` + env vars                | YAML config + `DNSMON_*` env overrides.                                    |
| Logging              | `log/slog` (stdlib, structured)                    | Use JSON handler in prod, text in dev.                                  |
| Validation           | `github.com/go-playground/validator/v10`           | Validate API requests.                                                  |
| Storage (optional)   | SQLite (`modernc.org/sqlite`) or Postgres (`pgx`)  | History, saved checks, rate-limit buckets. Pure-Go SQLite by default.   |
| Cache                | In-memory LRU (`hashicorp/golang-lru/v2`) + opt. Redis | Cache resolver responses by `(name, type, resolver)` for TTL seconds.  |
| WebSockets           | `github.com/coder/websocket`                       | Modern, context-aware, no deps.                                         |
| Templates / Frontend | **Vanilla embedded SPA**: HTML + Tailwind (build-time) + Alpine.js + Leaflet.js for the world map | No node-runtime dep; assets embedded via `embed.FS`.                    |
| Build                | `make` + `goreleaser`                              | Cross-compile linux/darwin/windows × amd64/arm64.                       |
| Container            | Distroless static                                  | <20 MB image.                                                          |
| Testing              | stdlib `testing` + `stretchr/testify`              | Unit + integration; DNS mocked with `miekg/dns` test server.            |
| Linting              | `golangci-lint`                                    | Config in `.golangci.yml`.                                              |
| Service mgmt         | `github.com/kardianos/service`                     | `--service install/uninstall` registers dnsmon as a Windows SCM / systemd / launchd service. Speaks the Windows Service Control Manager protocol so the binary runs correctly under SCM; hand-rolling this per-OS would be far more code. |

**Do not introduce new dependencies without listing them here first and explaining the trade-off.**

---

## 3. Folder Structure

```
dnsmon/
├── CLAUDE.md                       # ← this file
├── README.md                       # User-facing readme (install, run, screenshots)
├── LICENSE                         # MIT
├── Makefile                        # build / test / run / lint / docker targets
├── go.mod
├── go.sum
├── .golangci.yml
├── .goreleaser.yaml
├── .github/
│   └── workflows/
│       ├── ci.yml                  # test + lint on PR
│       └── release.yml             # goreleaser on tag
│
├── cmd/
│   └── dnsmon/
│       └── main.go                 # Entrypoint: parses flags/config, wires DI, starts HTTP server.
│
├── internal/                       # All non-public code lives here.
│   ├── config/
│   │   ├── config.go               # Config struct + loader (viper).
│   │   └── defaults.go             # Embedded default config.yaml.
│   │
│   ├── resolvers/
│   │   ├── resolvers.go            # Resolver struct (Name, IP, Country, City, Lat, Lng, ASN, ISP).
│   │   ├── registry.go             # In-memory registry + reload from file/DB.
│   │   ├── builtin.go              # `//go:embed resolvers.json` (~100 curated public resolvers).
│   │   └── resolvers.json          # Curated list, see §6.
│   │
│   ├── dnsclient/
│   │   ├── client.go               # Thin wrapper over miekg/dns: Query(ctx, server, name, qtype) -> Result.
│   │   ├── result.go               # Normalized Result/Answer types (JSON-safe).
│   │   ├── recordtypes.go          # Supported qtypes + parsers.
│   │   └── client_test.go          # Tests using a local dns.Server.
│   │
│   ├── checker/
│   │   ├── checker.go              # Orchestrates fan-out: given (name, type, []Resolver) -> []ResolverResult.
│   │   ├── stream.go               # Streaming variant: pushes results to a channel as they arrive (for WS/SSE).
│   │   ├── permalink.go            # Encodes a check into a short URL-safe ID (base62 hash + DB row).
│   │   └── checker_test.go
│   │
│   ├── cache/
│   │   ├── cache.go                # Interface: Get/Set/Delete; key = sha1(name|type|resolver).
│   │   ├── memory.go               # LRU implementation (default).
│   │   └── redis.go                # Optional Redis implementation.
│   │
│   ├── storage/
│   │   ├── storage.go              # Interface: SaveCheck, GetCheck, ListChecks, SaveResolverOverride, …
│   │   ├── sqlite/
│   │   │   ├── sqlite.go
│   │   │   └── migrations/         # `0001_init.sql`, …
│   │   └── postgres/
│   │       ├── postgres.go
│   │       └── migrations/
│   │
│   ├── ratelimit/
│   │   ├── ratelimit.go            # Token-bucket per IP + per API key.
│   │   └── middleware.go           # chi middleware.
│   │
│   ├── api/
│   │   ├── server.go               # Builds chi.Router with all routes mounted.
│   │   ├── middleware.go           # logging, recovery, CORS, request-id, ratelimit wiring.
│   │   ├── errors.go               # Standard error envelope.
│   │   ├── v1/
│   │   │   ├── check.go            # POST /api/v1/check, GET /api/v1/check/{id}
│   │   │   ├── lookup.go           # POST /api/v1/lookup (single resolver, detailed)
│   │   │   ├── reverse.go          # POST /api/v1/reverse  (PTR)
│   │   │   ├── resolvers.go        # GET /api/v1/resolvers
│   │   │   ├── recordtypes.go      # GET /api/v1/record-types
│   │   │   ├── export.go           # GET /api/v1/check/{id}/export?format=csv|png|pdf|svg|json
│   │   │   ├── health.go           # GET /api/v1/health, /readyz, /livez
│   │   │   ├── stream.go           # GET /api/v1/check/stream  (WebSocket)
│   │   │   └── handlers_test.go
│   │   └── docs/
│   │       ├── openapi.yaml        # OpenAPI 3.1 spec (source of truth, see §8)
│   │       └── swagger.go          # Serves /api/docs (embedded Swagger UI).
│   │
│   ├── export/
│   │   ├── csv.go
│   │   ├── png.go                  # Renders the world-map + table to PNG (uses fogleman/gg).
│   │   ├── svg.go
│   │   └── pdf.go                  # Uses jung-kurt/gofpdf or wkhtmltopdf (preferred: native gofpdf, no CGo).
│   │
│   ├── geoip/
│   │   ├── geoip.go                # Wraps MaxMind GeoLite2 (optional) for resolver geolocation if user adds custom resolvers.
│   │   └── geoip_test.go
│   │
│   └── version/
│       └── version.go              # Build-time ldflags: Version, Commit, Date.
│
├── web/                            # Frontend (embedded via embed.FS at build).
│   ├── embed.go                    # `//go:embed dist` + http.FileServer wiring.
│   ├── src/
│   │   ├── index.html              # Main page (DNS propagation checker).
│   │   ├── lookup.html             # Detailed DNS Lookup page.
│   │   ├── reverse.html            # Reverse DNS page.
│   │   ├── api-docs.html           # Embeds Swagger UI.
│   │   ├── about.html
│   │   ├── css/
│   │   │   └── tailwind.src.css
│   │   └── js/
│   │       ├── app.js              # Alpine components: form, results table, websocket client.
│   │       ├── map.js              # Leaflet + GeoJSON world map with per-resolver markers.
│   │       └── export.js           # Triggers PNG/PDF/SVG/CSV download via API.
│   ├── dist/                       # Build output (committed for `go build` reproducibility OR built in Dockerfile).
│   ├── package.json                # Only used at build-time for Tailwind compile.
│   ├── tailwind.config.js
│   └── postcss.config.js
│
├── deploy/
│   ├── Dockerfile                  # Multi-stage: node→tailwind, go→binary, distroless final.
│   ├── docker-compose.yml          # dnsmon + (optional) postgres + redis.
│   ├── docker-compose.sqlite.yml   # Minimal SQLite-only stack.
│   ├── systemd/
│   │   └── dnsmon.service
│   └── k8s/
│       ├── deployment.yaml
│       ├── service.yaml
│       └── ingress.yaml
│
├── config/
│   └── config.example.yaml         # Documented example config.
│
├── scripts/
│   ├── update-resolvers.sh         # Refreshes resolvers.json from public lists + verifies each responds.
│   └── test-integration.sh         # Spins up local dns.Server and hits the API.
│
└── docs/
    ├── ARCHITECTURE.md             # High-level diagrams, request flow.
    ├── DEPLOYMENT.md               # Docker / k8s / systemd / behind nginx.
    ├── CONFIGURATION.md            # Every config option.
    ├── API.md                      # Human-readable API reference (mirrors openapi.yaml).
    └── CONTRIBUTING.md
```

**Rules for layout**
- Anything under `internal/` is private to this module. Public reusable bits — if any — go into a new top-level package, never `pkg/`.
- HTTP handlers live under `internal/api/v1/`. Business logic does NOT live in handlers; handlers parse, validate, call into `internal/checker` (or others), serialize.
- Tests live next to the code (`foo.go` → `foo_test.go`). Integration tests live in `internal/api/v1/handlers_test.go`.
- No circular imports. The dependency direction is: `api → checker → dnsclient + resolvers + cache + storage`.

---

## 4. Core Features (parity with whatsmydns.net)

| #  | Feature                                                                                       | Status |
| -- | --------------------------------------------------------------------------------------------- | ------ |
| 1  | DNS propagation check across global resolvers                                                 | MUST   |
| 2  | All record types: A, AAAA, CNAME, MX, NS, PTR, SOA, TXT, CAA, SRV, DS, DNSKEY, NAPTR, TLSA, SVCB, HTTPS | MUST   |
| 3  | Single-resolver detailed DNS Lookup tool (shows TTL, flags, authority section, additional section) | MUST   |
| 4  | Reverse DNS (PTR) lookup for IPv4 and IPv6                                                    | MUST   |
| 5  | Interactive world map showing each resolver's result, colored by consensus / disagreement     | MUST   |
| 6  | Tabular results: resolver name, location, IP, ISP/ASN, response, time-to-resolve              | MUST   |
| 7  | Permalinks (shareable URL like `/check/abc123`)                                               | MUST   |
| 8  | Auto-refresh / live mode via WebSocket streaming                                              | MUST   |
| 9  | Custom resolvers (user-added per check)                                                       | MUST   |
| 10 | Export results: JSON, CSV, PNG (map+table), SVG (map), PDF                                    | MUST   |
| 11 | Public REST API with OpenAPI 3.1 docs at `/api/docs`                                          | MUST   |
| 12 | Rate limiting per-IP and per-API-key                                                          | MUST   |
| 13 | Optional API keys for higher quotas (managed via admin endpoints)                             | SHOULD |
| 14 | Country-flag icons next to resolvers                                                          | MUST   |
| 15 | Dark mode                                                                                     | MUST   |
| 16 | DNSSEC validation indicator (AD flag from response)                                           | SHOULD |
| 17 | Multi-language UI (i18n via JSON catalogs)                                                    | SHOULD |
| 18 | History (last N checks per IP / per user)                                                     | SHOULD |
| 19 | Prometheus metrics at `/metrics`                                                              | MUST   |
| 20 | Healthcheck / readiness endpoints                                                             | MUST   |

---

## 5. Domain Model (key types)

Put these in `internal/dnsclient/result.go` and reuse across the codebase.

```go
type Resolver struct {
    ID       string  `json:"id"`        // stable slug, e.g. "google-us"
    Name     string  `json:"name"`      // "Google"
    IP       string  `json:"ip"`        // "8.8.8.8"
    Port     int     `json:"port"`      // 53
    Protocol string  `json:"protocol"`  // "udp" | "tcp" | "dot" | "doh"
    Country  string  `json:"country"`   // ISO-3166 alpha-2, e.g. "US"
    City     string  `json:"city"`
    Lat      float64 `json:"lat"`
    Lng      float64 `json:"lng"`
    ASN      uint32  `json:"asn,omitempty"`
    ISP      string  `json:"isp,omitempty"`
}

type Answer struct {
    Name  string `json:"name"`
    Type  string `json:"type"`   // "A", "AAAA", ...
    TTL   uint32 `json:"ttl"`
    Value string `json:"value"`  // canonical string form (IP, target, "10 mail.example.com", etc.)
}

type ResolverResult struct {
    Resolver   Resolver  `json:"resolver"`
    Status     string    `json:"status"`     // "ok" | "nxdomain" | "timeout" | "servfail" | "refused" | "error"
    Answers    []Answer  `json:"answers"`
    Authority  []Answer  `json:"authority,omitempty"`
    Additional []Answer  `json:"additional,omitempty"`
    DNSSEC     *bool     `json:"dnssec,omitempty"` // nil if not checked
    DurationMS int64     `json:"duration_ms"`
    Error      string    `json:"error,omitempty"`
    QueriedAt  time.Time `json:"queried_at"`
}

type Check struct {
    ID         string           `json:"id"`         // permalink id
    Name       string           `json:"name"`       // queried name
    Type       string           `json:"type"`       // "A", ...
    CreatedAt  time.Time        `json:"created_at"`
    Results    []ResolverResult `json:"results"`
    Summary    CheckSummary     `json:"summary"`
}

type CheckSummary struct {
    TotalResolvers   int            `json:"total_resolvers"`
    Responded        int            `json:"responded"`
    NXDomain         int            `json:"nxdomain"`
    Timeouts         int            `json:"timeouts"`
    Errors           int            `json:"errors"`
    UniqueAnswerSets int            `json:"unique_answer_sets"` // how many *different* answers were observed
    Consensus        map[string]int `json:"consensus"`          // canonical answer-set string -> count
}
```

---

## 6. Resolver List

- A curated **`resolvers.json`** ships embedded in the binary. Aim for ~100 resolvers across all 6 continents covering major public providers (Google, Cloudflare, Quad9, OpenDNS, AdGuard, Yandex, NextDNS, regional ISPs, etc.).
- Each entry MUST include geolocation (lat/lng) and country so the world map works without a live geoip lookup.
- A `scripts/update-resolvers.sh` script must:
  1. Pull from a maintained public list (e.g. `public-dns.info`).
  2. Probe every resolver with a known control query (e.g. `A google.com`) over UDP with a 2s timeout.
  3. Drop dead resolvers.
  4. Geolocate using MaxMind GeoLite2 (config-driven path) **or** a static IP→geo map shipped in repo.
  5. Write back to `internal/resolvers/resolvers.json`.
- Operators can also supply their own `resolvers.yaml` via `--resolvers-file` to add private / internal resolvers.

---

## 7. Frontend (UI)

The UI lives in `web/`. It must NOT require a runtime Node.js — only build-time. Tailwind compiles `web/src/css/tailwind.src.css` → `web/dist/css/app.css`. JS is plain ES modules (no bundler needed for our scale, or use `esbuild` if necessary).

Pages:

1. **`/`** — Propagation Checker
   - Input: domain name, record type dropdown, "Check" button.
   - Output: world map (Leaflet) with markers per resolver; below the map a sortable table with columns: Flag | Resolver | Location | Result | TTL | Time.
   - "Live" toggle: opens WebSocket to `/api/v1/check/stream` and streams results as they arrive.
   - "Share" button: copies permalink.
   - "Export" dropdown: JSON / CSV / PNG / SVG / PDF.

2. **`/lookup`** — Detailed Single-Resolver Lookup
   - Input: domain, record type, resolver picker (defaults to system / 1.1.1.1).
   - Output: full DNS response sections (Question, Answer, Authority, Additional), flags (QR, AA, TC, RD, RA, AD, CD), RCODE.

3. **`/reverse`** — Reverse DNS
   - Input: IPv4 or IPv6 address.
   - Output: PTR record(s) from each global resolver, same map+table layout as `/`.

4. **`/api-docs`** — embedded Swagger UI rendering `internal/api/docs/openapi.yaml`.

5. **`/about`** — explains what this is, who runs it, link to GitHub.

UI requirements:
- Mobile-responsive (Tailwind breakpoints).
- Dark mode toggle (persists in `localStorage`).
- Country-flag icons via `flag-icons` CSS package (embedded, not CDN).
- Accessibility: semantic HTML, ARIA labels on interactive elements, focus rings.
- No third-party CDNs at runtime — everything must be served from the binary itself for true self-hosting.

---

## 8. REST API

**Base URL**: `/api/v1`
**Content-Type**: `application/json` (UTF-8) unless noted.
**Auth**: optional. Send `Authorization: Bearer <api_key>` for higher rate limits.
**Error envelope**:

```json
{
  "error": {
    "code": "INVALID_RECORD_TYPE",
    "message": "Record type 'FOO' is not supported.",
    "details": { "supported": ["A", "AAAA", ...] }
  },
  "request_id": "01HXYZ..."
}
```

OpenAPI 3.1 spec lives at `internal/api/docs/openapi.yaml` and is served at `GET /api/docs` (Swagger UI) and `GET /api/docs/openapi.yaml` (raw).

### 8.1 Endpoints

#### Health / Meta
| Method | Path                  | Description                                  | Auth |
| ------ | --------------------- | -------------------------------------------- | ---- |
| GET    | `/api/v1/health`      | Liveness, returns `{"status":"ok"}`          | No   |
| GET    | `/api/v1/readyz`      | Readiness (DB + cache reachable)             | No   |
| GET    | `/api/v1/version`     | Build version, commit, date                  | No   |
| GET    | `/metrics`            | Prometheus metrics (not under `/api/v1`)     | No   |

#### Discovery
| Method | Path                          | Description                                                       |
| ------ | ----------------------------- | ----------------------------------------------------------------- |
| GET    | `/api/v1/resolvers`           | List all known resolvers. Query params: `country`, `protocol`, `q` (free text). |
| GET    | `/api/v1/resolvers/{id}`      | Get one resolver.                                                 |
| GET    | `/api/v1/record-types`        | List supported record types with descriptions.                    |

#### Propagation Check (multi-resolver)
| Method | Path                                                   | Description                                                                                                                              |
| ------ | ------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------- |
| POST   | `/api/v1/check`                                        | Run a propagation check. Body: `{ "name": "...", "type": "A", "resolvers": ["google-us", ...] (optional), "custom_resolvers": [Resolver,...] (optional), "save": true }`. Returns full `Check` object. |
| GET    | `/api/v1/check/{id}`                                   | Retrieve a saved (permalinked) check by id.                                                                                              |
| GET    | `/api/v1/check/{id}/export?format=json\|csv\|png\|svg\|pdf` | Export the check.                                                                                                                        |
| GET    | `/api/v1/check/stream` (WebSocket)                     | Streaming variant. Client sends `{"name":"...","type":"A"}` JSON frame, server streams `ResolverResult` frames as each resolver responds. |

#### Single-Resolver Lookup (detailed)
| Method | Path                | Description                                                                                                                                                                     |
| ------ | ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| POST   | `/api/v1/lookup`    | Body: `{ "name": "...", "type": "A", "resolver": "8.8.8.8" or "google-us", "protocol": "udp|tcp|dot|doh" }`. Returns the raw, fully-detailed DNS response from that one server. |

#### Reverse DNS
| Method | Path                 | Description                                                                                                            |
| ------ | -------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| POST   | `/api/v1/reverse`    | Body: `{ "ip": "8.8.8.8", "resolvers": [...] }`. Same shape as propagation check but auto-converts to PTR query.       |

#### History (optional, requires storage enabled)
| Method | Path                          | Description                                  | Auth      |
| ------ | ----------------------------- | -------------------------------------------- | --------- |
| GET    | `/api/v1/history`             | List recent checks (paginated).              | API key   |
| DELETE | `/api/v1/history/{id}`        | Delete a saved check.                        | API key   |

#### Admin (optional, requires `admin` API key)
| Method | Path                          | Description                                  |
| ------ | ----------------------------- | -------------------------------------------- |
| GET    | `/api/v1/admin/api-keys`      | List API keys.                               |
| POST   | `/api/v1/admin/api-keys`      | Create an API key with quota.                |
| DELETE | `/api/v1/admin/api-keys/{id}` | Revoke an API key.                           |
| POST   | `/api/v1/admin/resolvers`     | Add a custom resolver (persisted).           |
| POST   | `/api/v1/admin/resolvers/reload` | Reload the resolver registry from disk.   |

### 8.2 Example: Propagation Check

**Request**
```http
POST /api/v1/check HTTP/1.1
Content-Type: application/json

{
  "name": "example.com",
  "type": "A",
  "save": true
}
```

**Response (200)**
```json
{
  "id": "a8x2k9",
  "name": "example.com",
  "type": "A",
  "created_at": "2026-05-23T12:34:56Z",
  "results": [
    {
      "resolver": {
        "id": "google-us",
        "name": "Google",
        "ip": "8.8.8.8",
        "country": "US",
        "city": "Mountain View",
        "lat": 37.386,
        "lng": -122.084
      },
      "status": "ok",
      "answers": [
        { "name": "example.com.", "type": "A", "ttl": 3600, "value": "93.184.216.34" }
      ],
      "duration_ms": 27,
      "queried_at": "2026-05-23T12:34:56Z"
    }
  ],
  "summary": {
    "total_resolvers": 100,
    "responded": 98,
    "nxdomain": 0,
    "timeouts": 2,
    "errors": 0,
    "unique_answer_sets": 1,
    "consensus": { "93.184.216.34": 98 }
  }
}
```

### 8.3 WebSocket streaming

```
Client → Server  (text frame, JSON)
{ "name": "example.com", "type": "A", "resolvers": [] }

Server → Client  (text frames, one per resolver as it answers)
{ "type": "result", "data": ResolverResult }
{ "type": "result", "data": ResolverResult }
...
{ "type": "done",   "data": { "summary": CheckSummary, "id": "a8x2k9" } }
```

Errors come as `{ "type": "error", "data": { "code": "...", "message": "..." } }`.

### 8.4 Rate Limits

- Anonymous: 30 req/min per IP, burst 10.
- API key (default): 600 req/min, burst 60.
- Admin key: unlimited.
- WebSocket: 1 concurrent connection per IP for anonymous, 10 for API key.

Headers returned: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After` on 429.

---

## 9. Configuration

`config.yaml` (overridable by env vars `DNSMON_*`, e.g. `DNSMON_SERVER_LISTEN`):

```yaml
server:
  listen: ":8080"
  base_url: "https://dnsmon.example.com"
  read_timeout: 10s
  write_timeout: 30s

dns:
  query_timeout: 3s
  per_resolver_concurrency: 4
  default_protocol: "udp"
  retry: 1

resolvers:
  builtin: true
  file: ""                # optional path to extra resolvers
  countries: []           # optional whitelist

storage:
  driver: "sqlite"        # sqlite | postgres | none
  dsn: "file:./dnsmon.db?cache=shared&_fk=1"
  retention_days: 30      # auto-purge old saved checks

cache:
  driver: "memory"        # memory | redis | none
  ttl: 60s
  size: 10000
  redis_addr: ""

ratelimit:
  enabled: true
  anon_per_min: 30
  key_per_min: 600

geoip:
  enabled: false
  mmdb_path: ""

log:
  level: "info"           # debug | info | warn | error
  format: "json"          # json | text

metrics:
  enabled: true
  path: "/metrics"
```

---

## 10. Coding Conventions

- Format with `gofmt -s`; lint with `golangci-lint run` (config in `.golangci.yml` enabling `govet, errcheck, staticcheck, revive, gosec, gocritic, gocyclo (15), gofumpt`).
- Errors: wrap with `fmt.Errorf("context: %w", err)`; never `panic` outside `main` startup.
- Contexts: every exported function that does I/O takes `context.Context` as first arg.
- Logging: structured via `slog`; include `request_id`, `resolver_id`, `qname`, `qtype` as attributes.
- No global state except logger and embedded assets.
- Public types get doc comments starting with the type name.
- Tests: table-driven where it fits. Mock DNS with a local `*dns.Server` rather than hitting the internet.
- Commit messages: Conventional Commits (`feat:`, `fix:`, `chore:`, …).
- Branching: trunk-based. PRs squash-merged.

---

## 11. Security

- Input validation: domains conform to RFC 1035 (allow IDNA; punycode-encode before query). Reject queries to `.local`, RFC1918 IPs, and link-local unless `--allow-private` is set, to prevent SSRF-style DNS abuse.
- Cap query length, record type to the allow-list, resolvers per request (default max 200).
- Strict timeouts on every outbound DNS query.
- `Content-Security-Policy: default-src 'self'`; no external scripts.
- API keys hashed at rest (bcrypt or argon2id).
- TLS terminated by reverse proxy in prod; serve plain HTTP in container.
- DNSSEC AD flag surfaced but not used for trust decisions.

---

## 12. Observability

- `slog` JSON logs to stdout.
- Prometheus metrics:
  - `gdns_check_total{type,status}`
  - `gdns_resolver_query_duration_seconds{resolver_id,status}` (histogram)
  - `gdns_resolver_query_total{resolver_id,status}`
  - `gdns_http_requests_total{path,method,status}`
  - `gdns_http_request_duration_seconds{path,method}` (histogram)
  - `gdns_cache_hits_total`, `gdns_cache_misses_total`
- `/api/v1/health` → 200 always.
- `/api/v1/readyz` → 200 only when storage + cache (if configured) are reachable.

---

## 13. Build & Run

```bash
# Local dev
make dev               # runs go run ./cmd/dnsmon with hot-reload via air

# Build single binary (embedded UI + resolvers)
make build             # → ./bin/dnsmon

# Test
make test              # go test ./...
make test-integration  # spins up local DNS test server

# Lint
make lint              # golangci-lint run

# Frontend
make ui                # tailwind build → web/dist
make ui-watch          # tailwind --watch

# Docker
make docker            # builds deploy/Dockerfile, tags dnsmon:latest

# Release
make release           # goreleaser release --clean
```

Dockerfile uses three stages:
1. `node:20-alpine` — builds Tailwind CSS into `web/dist/`.
2. `golang:1.23-alpine` — `go build -trimpath -ldflags="-s -w -X .../version.Version=..."` with `CGO_ENABLED=0`.
3. `gcr.io/distroless/static:nonroot` — copy binary; `ENTRYPOINT ["/dnsmon"]`.

---

## 14. Roadmap (post-MVP, do NOT build unless asked)

- Scheduled monitoring with email/webhook alerts on DNS change.
- Multi-tenant SaaS mode.
- DNS-over-HTTPS and DNS-over-TLS to ALL resolvers when supported.
- Authoritative-server trace (whatsmydns has a basic one).
- WHOIS lookup integration.
- SSL/TLS certificate inspection.

---

## 15. Instructions for Claude (Agent Behavior)

When working in this repo:

1. **Always read this file first.** If the user's request conflicts with it, surface the conflict and ask before deviating.
2. Before writing code in a new area, check the matching `internal/<pkg>/` directory and follow the patterns already there.
3. **Never** add a dependency without justifying it and updating §2.
4. Keep handlers thin; put logic in `internal/checker`, `internal/dnsclient`, etc.
5. For any new HTTP route, update `internal/api/docs/openapi.yaml` in the same change. The OpenAPI file is the source of truth — generated docs come from it.
6. For any new feature flag or config knob, update `config/config.example.yaml`, `internal/config/defaults.go`, and `docs/CONFIGURATION.md` in the same change.
7. Write at least one unit test and, where applicable, one integration test per new feature.
8. Use `slog` (not `log` or `fmt.Println`) for any operational output.
9. Validate every external input. If it touches DNS, also apply the SSRF guards from §11.
10. When unsure about scope, prefer the smallest change that satisfies the request and leaves a clean extension point.

---

_Last updated: 2026-05-23_
