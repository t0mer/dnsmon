# dnsmon

[![Docker Hub](https://img.shields.io/docker/v/techblog/dnsmon?sort=semver&label=docker%20hub)](https://hub.docker.com/r/techblog/dnsmon)
[![Docker pulls](https://img.shields.io/docker/pulls/techblog/dnsmon)](https://hub.docker.com/r/techblog/dnsmon)
[![GitHub release](https://img.shields.io/github/v/release/t0mer/dnsmon)](https://github.com/t0mer/dnsmon/releases)
[![Go version](https://img.shields.io/github/go-mod/go-version/t0mer/dnsmon)](go.mod)
[![License](https://img.shields.io/github/license/t0mer/dnsmon)](LICENSE)

**Self-hosted DNS propagation checker**: an open-source alternative to
[whatsmydns.net](https://www.whatsmydns.net/) that you run on your own
infrastructure. Check how a record resolves across **124 public resolvers**
worldwide, look up records in detail, run reverse lookups, trace the delegation
chain, and set up **monitors** that alert you (Slack, Telegram, email, WhatsApp, …)
when a record fails to propagate or changes unexpectedly.

It ships as a single static Go binary with an embedded web UI. Run
`docker compose up` (after the one-time volume fix in [Docker Compose](#docker-compose))
and you have a private whatsmydns.

![DNS propagation check](https://raw.githubusercontent.com/t0mer/dnsmon/main/docs/screenshots/propagation.png)

---

## Table of contents

- [Features](#features)
- [Screenshots](#screenshots)
- [How it works](#how-it-works)
- [Requirements](#requirements)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
- [REST API](#rest-api)
- [Prometheus metrics](#prometheus-metrics)
- [Security notes](#security-notes)
- [Troubleshooting](#troubleshooting)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

---

## Features

**DNS tools**
- 🌍 **Propagation check** across 124 curated global resolvers, with an
  interactive world map, consensus summary, country flags, and a sortable,
  paginated results table. Results stream in over a WebSocket as each resolver
  answers, so you see progress live.
- ⏱️ **Propagation ETA**: estimates how long until full propagation, based on
  the maximum TTL observed across resolvers that still serve an old answer.
  Shows convergence % and a human-readable countdown (e.g. "~2m 30s").
- 📸 **Snapshot diff**: save a check result as a baseline, then re-run it to
  see a per-resolver Δ column (changed / unchanged). A "Δ Changed" toggle
  narrows the table to the resolvers that flipped, which makes in-progress
  propagation easy to track.
- 🔗 **Authoritative trace**: walks the full DNS delegation chain from the root
  nameservers down to the authoritative server, like `dig +trace`.
  Each hop shows the zone, nameserver IP, referral or authoritative status,
  answer records, and round-trip time.
- 🔎 **DNS Lookup**: the full single-resolver response (Answer / Authority /
  Additional sections, TTLs, flags) against any listed resolver or a custom public IP.
- 🔁 **Reverse DNS**: PTR lookups for IPv4/IPv6 across all resolvers.
- 🔗 **Permalinks** (`/check/{id}`) and **exports**: JSON, CSV and PDF from the
  UI; the API additionally renders PNG and SVG.
- 16 record types: `A AAAA CNAME MX NS PTR SOA TXT CAA SRV DS DNSKEY NAPTR TLSA SVCB HTTPS`.
- Always fresh: response caching is **off by default**, so every check is live.

**Monitoring & alerting** (configured in **Settings**)
- 🔔 **Notification channels**: [Shoutrrr](https://containrrr.dev/shoutrrr/)
  (Slack, Telegram, Discord, email, and many more via one URL), **WhatsApp via
  GreenAPI**, and **WhatsApp via go-whatsapp-web-multidevice**. Each has a
  **Test** button.
- 🛰️ **Monitors**, in two types:
  - **Propagation monitoring**: alert if the expected value hasn't propagated
    to every resolver.
  - **Change detection**: alert when a record drifts from a saved baseline
    (it re-baselines after each change).

  Create one straight from a propagation check, bind it to a schedule and a
  channel, **Run** it on demand, and review its **changelog**.
- ⏱️ **Schedulers**: reusable cadences (`@hourly`, `@daily`, `@weekly`,
  `@monthly`, `@yearly`/`@annually`, `@midnight`, `@every <duration>`, or a 5/6-field cron
  expression) that monitors attach to.
  > **Note:** automatic, schedule-driven execution of monitors is not implemented
  > yet. Today a monitor is evaluated when you press **Run** (or call
  > `POST /api/v1/settings/monitors/{id}/run`).
- 🧩 **Resolver management**: enable or disable resolvers; disabled ones are
  excluded from every check.

**Access & operations**
- 🔐 Optional **UI authentication** (single admin, argon2id-hashed password,
  signed session cookie) that gates the Settings area and history when enabled.
- 🎫 **API tokens** can be created and listed in Settings (only an argon2id hash
  is stored). <!-- TODO: verify — tokens are not yet accepted by the auth middleware -->
- 🧰 **REST API** with an **OpenAPI 3.1 spec** at `/api/docs/openapi.yaml` and a Swagger UI
  page at `/api/docs` (see the note under [REST API](#rest-api)).
- 📈 **Prometheus metrics** at `/metrics`; `/api/v1/health`, `/api/v1/readyz`,
  and `/api/v1/version` for probes.
- 🖥️ CLI: `--port`, the `PORT` env var, and `--service install|uninstall|…` to run
  as a Windows / systemd / launchd service.
- 🗄️ Storage: SQLite (default, pure Go), PostgreSQL, or none. Optional
  in-memory or Redis response cache.
- 🌓 Dark mode, mobile-responsive, embedded UI assets (the main UI makes no CDN calls;
  only the Swagger UI page at `/api/docs` loads from unpkg.com).

---

## Screenshots

| DNS Lookup | Reverse DNS |
|---|---|
| ![DNS Lookup](https://raw.githubusercontent.com/t0mer/dnsmon/main/docs/screenshots/lookup.png) | ![Reverse DNS](https://raw.githubusercontent.com/t0mer/dnsmon/main/docs/screenshots/reverse.png) |

| Monitors | Notification channels |
|---|---|
| ![Monitors](https://raw.githubusercontent.com/t0mer/dnsmon/main/docs/screenshots/settings-monitors.png) | ![Notifications](https://raw.githubusercontent.com/t0mer/dnsmon/main/docs/screenshots/settings-notifications.png) |

**API docs** (`/api/docs`)

![API docs](https://raw.githubusercontent.com/t0mer/dnsmon/main/docs/screenshots/api-docs.png)

<!-- TODO: screenshot — the Trace page, and a propagation check showing the ETA and snapshot diff, have no screenshots yet -->

---

## How it works

```mermaid
flowchart LR
    B[Browser / API client] -->|HTTP + WebSocket| R[chi router<br/>embedded UI · /api/v1 · /metrics]
    R --> C[checker]
    C -->|fan-out, bounded concurrency| D[dnsclient<br/>miekg/dns · UDP / TCP / DoT]
    D --> P[(124 public resolvers)]
    C <--> K[(cache: none · memory · Redis)]
    R <--> S[(storage: SQLite · PostgreSQL · none)]
    C -->|saved checks, disabled resolvers| S
    R --> M[monitor evaluator]
    M --> C
    M --> N[notify: Shoutrrr · GreenAPI · go-whatsapp-web]
```

1. A check request (REST or WebSocket) is validated and expanded into the list
   of enabled resolvers (built-in list, plus an optional extra resolvers file,
   minus the ones disabled in Settings).
2. `checker` queries all resolvers in parallel (bounded by
   `dns.per_resolver_concurrency`), with a per-query timeout and retries.
3. Results are aggregated into a summary (consensus, convergence, ETA). Checks
   saved with `save: true` get a permalink and appear in history.
4. Settings (channels, auth, tokens, schedules, monitors, disabled resolvers) are
   stored in the same database as check history.

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for more detail. Parts of it are
outdated (rate limiting, `gdns_` metric names, an `--allow-private` flag); where it
disagrees with this README, the README is authoritative.

---

## Requirements

- **Docker**, or a supported OS for the prebuilt binary (Linux, macOS, Windows).
- Outbound **DNS (UDP/TCP 53)** to the public resolvers. Checks from a network
  that blocks or intercepts outbound DNS will show timeouts or rewritten answers.
- Optional: **PostgreSQL** (instead of SQLite) and **Redis** (shared cache).
- Build from source: **Go 1.25+** and **Node 20+** (Node only compiles the Tailwind CSS).

---

## Installation

### Docker Compose

The bundled [`docker-compose.yml`](docker-compose.yml) runs `techblog/dnsmon:latest`
with SQLite stored in a named volume:

```bash
git clone https://github.com/t0mer/dnsmon.git && cd dnsmon
docker compose up -d
# → http://localhost:8080
```

> **First run:** the image runs as the distroless `nonroot` user (UID 65532),
> but a fresh named volume is owned by root, so SQLite fails with
> `unable to open database file (14)`. Fix the volume ownership once, then start
> again (the volume name is `<project>_dnsmondata`, where `<project>` is the
> directory name):
>
> ```bash
> docker run --rm -v dnsmon_dnsmondata:/data busybox chown 65532:65532 /data
> docker compose up -d
> ```

### Docker

```bash
docker volume create dnsmon
docker run --rm -v dnsmon:/data busybox chown 65532:65532 /data   # see note above

docker run -d --name dnsmon -p 8080:8080 -v dnsmon:/data \
  -e DNSMON_STORAGE_DSN="file:/data/dnsmon.db?cache=shared&_fk=1" \
  techblog/dnsmon:latest
```

Multi-arch images (`linux/amd64`, `linux/arm64`, `linux/arm/v7`) are published to
[`techblog/dnsmon`](https://hub.docker.com/r/techblog/dnsmon) with `latest` and
version tags (e.g. `v0.1.3`).

### Prebuilt binary

Download an archive for your OS/arch from the
[Releases](https://github.com/t0mer/dnsmon/releases) page, extract, and run:

```bash
./dnsmon --port 8080
```

Release archives are built for `linux_amd64`, `linux_arm64`, `linux_armv7`,
`darwin_amd64`, `darwin_arm64` and `windows_amd64`. By default the SQLite
database is created as `./dnsmon.db` in the working directory.

### Other deployment examples

The [`deploy/`](deploy) directory and [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md)
contain further examples:

| Example | Files |
|---|---|
| Docker Compose with SQLite | [`deploy/docker-compose.sqlite.yml`](deploy/docker-compose.sqlite.yml) |
| Docker Compose with PostgreSQL + Redis | [`deploy/docker-compose.yml`](deploy/docker-compose.yml), [`deploy/.env.example`](deploy/.env.example) |
| Kubernetes (Deployment, Service, Ingress) | [`deploy/k8s/`](deploy/k8s) |
| systemd unit | [`deploy/systemd/gdns.service`](deploy/systemd/gdns.service) (install it as `dnsmon.service`) |
| Nginx reverse proxy | [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md#behind-nginx-reverse-proxy) |

> These examples reference `ghcr.io/t0mer/dnsmon:latest`, which is **not
> published** at the moment. Use `techblog/dnsmon:latest` instead, and don't use
> their `build:` sections (they point to `deploy/Dockerfile`, which is outdated).
> In the PostgreSQL + Redis example, see [Troubleshooting](#troubleshooting)
> for `redis_addr` and `sslmode`.
> `deploy/docker-compose.sqlite.yml` has the same `/data` permission issue as the
> main compose file; fix it with
> `docker run --rm -v deploy_dnsmondata:/data busybox chown 65532:65532 /data`.
> The Kubernetes Deployment runs `replicas: 2` with SQLite on an `emptyDir`, so each
> pod has its own history and settings, and both are lost when a pod restarts. Use
> one replica with a PersistentVolumeClaim, or PostgreSQL.
> [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md) is partly outdated (`ghcr.io` image,
> `dnsmon.service` file name, cache default); the README is authoritative.
<!-- TODO: verify — deploy/ examples need the fixes listed above before they work as-is -->

### Run as a system service

`dnsmon` can register itself with the host service manager (systemd, Windows
Service Control Manager, or launchd). Flags passed together with
`--service install` (`--config`, `--resolvers-file`, `--listen`, `--port`,
`--log-level`) are baked into the service definition; file paths are made absolute.

```bash
# Linux: run as root (writes a systemd unit). Windows: run from an elevated prompt.
sudo dnsmon --port 8080 --config /etc/dnsmon/config.yaml --service install
sudo dnsmon --service start
sudo dnsmon --service stop
sudo dnsmon --service uninstall
```

### Build from source

```bash
make build      # builds the UI, then the binary → bin/dnsmon
./bin/dnsmon
```

Or manually:

```bash
cd web && npm ci && npm run build && cd ..
CGO_ENABLED=0 go build -o bin/dnsmon ./cmd/dnsmon
```

---

## Configuration

dnsmon reads configuration from, in increasing order of precedence:

1. Built-in defaults.
2. A YAML file: `--config <path>`, or `config.yaml` found in `.`, `$HOME/.dnsmon/`
   or `/etc/dnsmon/`.
3. `DNSMON_*` environment variables (`section.key` → `DNSMON_SECTION_KEY`).
4. Command-line flags (only the options listed under [flags](#command-line-flags)).

For the listen address the full order is: `server.listen` (file /
`DNSMON_SERVER_LISTEN`) < `PORT` env < `--listen` < `--port`.

See [`config/config.example.yaml`](config/config.example.yaml) for a commented
example and [`docs/CONFIGURATION.md`](docs/CONFIGURATION.md) for longer descriptions.
Both are partly outdated, and the table below is authoritative:

- `config.example.yaml` sets `base_url: "https://dnsmon.example.com"`. If you copy it
  unchanged, CORS is restricted to that origin; set your own URL or remove the line.
  Its comments also claim every option can be set via `DNSMON_*` variables and that
  `doh` is supported; neither is true (see ¹ below and `dns.default_protocol`).
- `docs/CONFIGURATION.md` lists wrong defaults for several keys, documents
  `DNSMON_CACHE_REDIS_ADDR` and other env vars that are ignored, and describes
  rate limiting, GeoIP and retention as working.

### Options

Defaults below are the ones set in code
([`internal/config/defaults.go`](internal/config/defaults.go)).

| YAML key | Env var | Default | Description |
|---|---|---|---|
| `server.listen` | `DNSMON_SERVER_LISTEN` | `:8080` | Listen address. `PORT` / `--listen` / `--port` override it. |
| `server.base_url` | YAML only ¹ | *(empty)* | Public URL (e.g. `https://dnsmon.example.com`). When set, it is the only allowed CORS origin (otherwise `*`), and an additional allowed WebSocket origin (same-host origins are always accepted). |
| `server.read_timeout` | `DNSMON_SERVER_READ_TIMEOUT` | `10s` | HTTP read timeout. |
| `server.write_timeout` | `DNSMON_SERVER_WRITE_TIMEOUT` | `120s` | HTTP write timeout. Keep it above the longest check. |
| `dns.query_timeout` | `DNSMON_DNS_QUERY_TIMEOUT` | `2s` | Timeout for a single query to a single resolver. |
| `dns.per_resolver_concurrency` | `DNSMON_DNS_PER_RESOLVER_CONCURRENCY` | `100` | Maximum concurrent outbound queries per check. |
| `dns.default_protocol` | `DNSMON_DNS_DEFAULT_PROTOCOL` | `udp` | Transport when a resolver doesn't set one: `udp`, `tcp`, or `dot` (DNS-over-TLS). |
| `dns.retry` | `DNSMON_DNS_RETRY` | `1` | Retries per failed query (`0` = none). |
| `resolvers.builtin` | `DNSMON_RESOLVERS_BUILTIN` | `true` | Include the 124 built-in resolvers. |
| `resolvers.file` | `--resolvers-file` ¹ | *(empty)* | Extra resolvers from a **JSON** file (same shape as [`resolvers.json`](internal/resolvers/resolvers.json)). |
| `storage.driver` | `DNSMON_STORAGE_DRIVER` | `sqlite` | `sqlite`, `postgres`, or `none` (no history, permalinks or persisted settings). |
| `storage.dsn` | `DNSMON_STORAGE_DSN` | `file:./dnsmon.db?cache=shared&_fk=1` | SQLite DSN, or `postgres://user:pass@host:5432/dnsmon?sslmode=disable`. |
| `cache.driver` | `DNSMON_CACHE_DRIVER` | `none` | `none` (always live), `memory` (in-process LRU), or `redis`. Monitors always bypass the cache. |
| `cache.ttl` | `DNSMON_CACHE_TTL` | `60s` | Lifetime of a cached response. |
| `cache.size` | `DNSMON_CACHE_SIZE` | `10000` | LRU capacity (`memory` only). |
| `cache.redis_addr` | YAML only ¹ | *(empty)* | Redis `host:port` (required for `redis`). |
| `log.level` | `DNSMON_LOG_LEVEL` / `--log-level` | `info` | `debug`, `info`, `warn`, `error`. |
| `log.format` | `DNSMON_LOG_FORMAT` | `json` | `json` or `text`. |
| `metrics.enabled` | `DNSMON_METRICS_ENABLED` | `true` | Expose Prometheus metrics. |
| `metrics.path` | `DNSMON_METRICS_PATH` | `/metrics` | Metrics path (outside `/api/v1`). |

¹ Environment variables only work for keys that have a built-in default. Keys
without one (`server.base_url`, `cache.redis_addr`, `resolvers.file`,
`resolvers.countries`, `geoip.*`) are **ignored when set through `DNSMON_*`
variables**; set them in the YAML file (or with `--resolvers-file`).

`config.example.yaml` also lists `storage.retention_days`, `resolvers.countries`,
`ratelimit.*` and `geoip.*`. These keys are parsed but **not used by the current
code** (no automatic purge, country filter, rate limiting or GeoIP lookups yet).

Durations use Go syntax: `500ms`, `3s`, `1m`, `1h30m`.

### Command-line flags

| Flag | Description |
|---|---|
| `--config <path>` | Path to a YAML config file. |
| `--listen <addr>` | Override the listen address (e.g. `:8080`, `127.0.0.1:9090`). |
| `--port <n>` | Override only the port, keeping any host from the listen address. |
| `--log-level <lvl>` | `debug` \| `info` \| `warn` \| `error`. |
| `--resolvers-file <path>` | Extra resolvers (JSON). |
| `--service <action>` | `install` \| `uninstall` \| `start` \| `stop` \| `restart`, then exit. |

### In-app settings

Everything that changes at runtime lives in the database and is edited on the
**Settings** page (or through `/api/v1/settings*`): notification channels, UI
authentication, API tokens, schedulers, monitors, and disabled resolvers.

---

## Usage

### Web UI

| Page | Path | What it does |
|---|---|---|
| Propagation | `/` | Multi-resolver check with map, summary, ETA, snapshot diff, export, "create monitor". |
| Permalink | `/check/{id}` | Re-opens a saved check. |
| DNS Lookup | `/lookup` | Detailed single-resolver response. |
| Reverse DNS | `/reverse` | PTR lookup across resolvers. |
| Trace | `/trace` | Delegation walk from the root servers. |
| API | `/api/docs` | Swagger UI (see the note under [REST API](#rest-api)). |
| About | `/about` | About page. |
| Settings | `/settings` | Channels, auth, tokens, schedulers, monitors, resolvers. |
| Login | `/login` | Shown when UI authentication is enabled. |

### API examples

```bash
# Propagation check (save it to get a permalink)
curl -s -X POST http://localhost:8080/api/v1/check \
  -H 'Content-Type: application/json' \
  -d '{"name":"example.com","type":"A","save":true}'

# Detailed lookup against one resolver (registry ID or public IP)
curl -s -X POST http://localhost:8080/api/v1/lookup \
  -H 'Content-Type: application/json' \
  -d '{"name":"example.com","type":"MX","resolver":"1.1.1.1"}'

# Reverse lookup
curl -s -X POST http://localhost:8080/api/v1/reverse \
  -H 'Content-Type: application/json' -d '{"ip":"8.8.8.8"}'

# Export a saved check as CSV
curl -s -o check.csv "http://localhost:8080/api/v1/check/<id>/export?format=csv"
```

---

## REST API

Base URL `/api/v1`. The app serves:

- OpenAPI 3.1 spec: `GET /api/docs/openapi.yaml`. This is the complete, reliable reference;
  load it into any OpenAPI viewer.
- Swagger UI: `GET /api/docs`. The page loads Swagger UI's CSS and JS from unpkg.com,
  but the app's Content-Security-Policy only allows same-origin scripts and styles, so
  the page may render blank in the browser. <!-- TODO: verify -->

[`docs/API.md`](docs/API.md) has more detail but is partly outdated: it describes
`Authorization: Bearer` auth and rate limits that don't exist, and misses the trace,
auth and settings endpoints. Where it disagrees with this README, the README is
authoritative.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/health`, `/api/v1/readyz`, `/api/v1/version` | Liveness, readiness (storage reachable), build info |
| `GET` | `/api/v1/resolvers`, `/api/v1/resolvers/{id}` | List resolvers (`?country=`, `?q=`) / get one |
| `GET` | `/api/v1/record-types` | Supported record types |
| `POST` | `/api/v1/check` | Run a propagation check |
| `GET` | `/api/v1/check/stream` | Stream a check (WebSocket) |
| `GET` | `/api/v1/check/{id}` | Get a saved check |
| `GET` | `/api/v1/check/{id}/export?format=json\|csv\|png\|svg\|pdf` | Export a saved check |
| `POST` | `/api/v1/lookup` | Single-resolver detailed lookup |
| `POST` | `/api/v1/reverse` | Reverse (PTR) lookup |
| `POST` | `/api/v1/trace` | Authoritative delegation trace |
| `POST` | `/api/v1/auth/login`, `/api/v1/auth/logout` | Start / end a UI session |
| `GET` | `/api/v1/auth/session` | Current session state |
| `GET` | `/api/v1/history` | List saved checks (`?limit=`, `?offset=`) 🔒 |
| `DELETE` | `/api/v1/history/{id}` | Delete a saved check 🔒 |
| `GET`, `PUT` | `/api/v1/settings` | Read / update settings 🔒 |
| `POST` | `/api/v1/settings/notifications/test` | Send a test notification 🔒 |
| `GET`, `POST` | `/api/v1/settings/tokens` | List / create API tokens 🔒 |
| `DELETE` | `/api/v1/settings/tokens/{id}` | Delete an API token 🔒 |
| `GET`, `POST` | `/api/v1/settings/schedules` | List / create schedulers 🔒 |
| `PUT`, `DELETE` | `/api/v1/settings/schedules/{id}` | Update / delete a scheduler 🔒 |
| `GET`, `POST` | `/api/v1/settings/monitors` | List / create monitors 🔒 |
| `PUT`, `DELETE` | `/api/v1/settings/monitors/{id}` | Update / delete a monitor 🔒 |
| `GET` | `/api/v1/settings/monitors/{id}/history` | Monitor changelog 🔒 |
| `POST` | `/api/v1/settings/monitors/{id}/run` | Evaluate a monitor now 🔒 |

🔒 When UI authentication is enabled, these endpoints require the session cookie
set by `POST /api/v1/auth/login` and otherwise return `401`. With authentication
disabled (the default) they are open.
<!-- TODO: verify — API tokens (Authorization: Bearer …) are not checked by the auth middleware yet -->

---

## Prometheus metrics

Served at `/metrics` (configurable with `metrics.path`), together with the
standard Go and process collectors:

| Metric | Type | Description |
|---|---|---|
| `dnsmon_check_total` | counter | Propagation checks run |
| `dnsmon_resolver_query_duration_seconds` | histogram | Per-resolver query latency |
| `dnsmon_resolver_query_total` | counter | Per-resolver queries |
| `dnsmon_cache_hits_total` | counter | Cache hits |
| `dnsmon_cache_misses_total` | counter | Cache misses |

---

## Security notes

- **Enable UI authentication** (Settings → Authentication) on any instance
  reachable by others. It is **off by default**: until you enable it, anyone who
  can reach the port can read and change settings, including notification-channel
  credentials.
- Notification-channel credentials (Shoutrrr URLs, GreenAPI tokens, WhatsApp
  passwords) are stored as plain text in the database and returned by
  `GET /api/v1/settings`. Protect the database and the settings API accordingly.
  The admin password and API tokens are stored as argon2id hashes.
- The session cookie is `HttpOnly` and `SameSite=Lax`, but not `Secure`: serve
  dnsmon behind a TLS-terminating reverse proxy (see
  [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md)).
- Set `server.base_url` to restrict CORS to your hostname; without it, CORS allows `*`.
  WebSocket connections always accept same-host origins, and `base_url` is added
  as one more allowed origin.
- Custom resolver IPs in `/lookup` and `/check` are rejected when they are
  loopback, private (RFC 1918 / ULA), link-local, or `.local`.
- Every response carries a Content-Security-Policy, `X-Frame-Options: DENY`,
  `X-Content-Type-Options: nosniff` and a strict referrer policy.
- Rate limiting is **not active** in the current code, whatever `ratelimit.*` says.
  Put a reverse proxy with rate limits in front of a public instance.
- The container runs as the non-root distroless user (UID 65532).

---

## Troubleshooting

| Symptom | Cause / fix |
|---|---|
| `pinging sqlite db: unable to open database file (14)` in Docker | The volume at `/data` is owned by root. Run `docker run --rm -v <volume>:/data busybox chown 65532:65532 /data` once. |
| `cache driver redis requires cache.redis_addr` although `DNSMON_CACHE_REDIS_ADDR` is set | That key can't be set through the environment (see ¹ above). Set `cache.redis_addr` in a YAML config file. |
| PostgreSQL connection fails with the `deploy/docker-compose.yml` example | Its DSN uses `sslmode=require`, but the bundled `postgres:16-alpine` has no TLS. Use `sslmode=disable` on a private network. |
| `invalid PORT environment value` | `PORT` must be a positive integer. |
| Many resolvers show `timeout` | Outbound DNS (UDP/TCP 53) is blocked or rate-limited on your network. Raise `dns.query_timeout` or lower `dns.per_resolver_concurrency`. |
| Live results never appear behind a proxy | The proxy must pass WebSocket upgrades for `/api/v1/check/stream`. If the proxy rewrites the `Host` header, set `server.base_url` to the public origin so it is accepted. |
| Settings changes don't stick | With `storage.driver: none`, saving is a no-op, so changes are lost immediately. With SQLite, changes are lost on restart if the database file isn't on a persistent volume. |

---

## Development

```bash
make build             # UI + binary → bin/dnsmon
make dev               # hot-reload dev server (air)
make test              # go test -race ./...
make test-integration  # smoke-test the API against bin/dnsmon (needs curl, jq)
make ui-watch          # rebuild Tailwind on change
make lint              # golangci-lint
make docker            # build the image (dnsmon:latest)
make release           # cross-compile release archives into dist/
```

`scripts/update-resolvers.sh` refreshes the built-in resolver list from
public-dns.info (needs `curl`, `dig`, `jq`).

Layout:

```
cmd/dnsmon/          entry point, flags, service mode
internal/api/        chi router, middleware, v1 handlers, OpenAPI spec
internal/checker/    fan-out, summary/ETA, streaming, trace
internal/dnsclient/  miekg/dns client and record types
internal/resolvers/  embedded resolvers.json and registry
internal/storage/    sqlite / postgres backends and migrations
internal/cache/      none / memory (LRU) / redis
internal/monitor/    monitor evaluator
internal/notify/     Shoutrrr, GreenAPI, go-whatsapp-web senders
internal/export/     CSV, PNG, SVG, PDF renderers
internal/settings/   settings model, argon2id hashing, sessions
web/                 embedded UI (Tailwind, Alpine.js, Leaflet)
```

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for design details (partly outdated;
see the note under [How it works](#how-it-works)).

### Tech stack

Go · [chi](https://github.com/go-chi/chi) · [miekg/dns](https://github.com/miekg/dns) ·
[viper](https://github.com/spf13/viper) · SQLite ([modernc](https://gitlab.com/cznic/sqlite)) /
PostgreSQL ([pgx](https://github.com/jackc/pgx)) · Redis ·
[shoutrrr](https://github.com/containrrr/shoutrrr) ·
[kardianos/service](https://github.com/kardianos/service) · Tailwind + Alpine.js +
Leaflet (embedded).

---

## Contributing

Issues and pull requests are welcome. See
[`docs/CONTRIBUTING.md`](docs/CONTRIBUTING.md) for the development setup,
commit format and PR checklist. Run `make test` and `make lint` before opening a PR.

---

## License

Apache License 2.0. See [LICENSE](LICENSE).
