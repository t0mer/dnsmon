# Configuration Reference

gdns is configured via a YAML file (default: `config.yaml` in the working directory) and/or environment variables. Environment variables take precedence over the config file.

Pass a config file path with `--config /path/to/config.yaml`.

---

## server

HTTP server settings.

| Option | Type | Default | Env Variable | Description |
|---|---|---|---|---|
| `listen` | string | `:8080` | `GDNS_SERVER_LISTEN` | TCP address to listen on. Examples: `:8080`, `0.0.0.0:9090`, `127.0.0.1:8080` |
| `base_url` | string | `http://localhost:8080` | `GDNS_SERVER_BASE_URL` | Public-facing base URL. Used to construct permalink URLs returned in API responses. Include scheme and host, no trailing slash. |
| `read_timeout` | duration | `10s` | `GDNS_SERVER_READ_TIMEOUT` | Maximum duration to read the entire request, including body. Prevents slowloris attacks. |
| `write_timeout` | duration | `30s` | `GDNS_SERVER_WRITE_TIMEOUT` | Maximum duration to write the entire response. Set this longer than the maximum expected DNS fan-out time. |

---

## dns

Controls outbound DNS query behavior.

| Option | Type | Default | Env Variable | Description |
|---|---|---|---|---|
| `query_timeout` | duration | `3s` | `GDNS_DNS_QUERY_TIMEOUT` | Timeout per individual DNS query to a single resolver. Resolvers that don't respond within this window are marked as `timeout`. |
| `per_resolver_concurrency` | int | `4` | `GDNS_DNS_PER_RESOLVER_CONCURRENCY` | Maximum number of simultaneous outbound DNS queries per propagation check. Higher values reduce total check time at the cost of network bandwidth. |
| `default_protocol` | string | `udp` | `GDNS_DNS_DEFAULT_PROTOCOL` | Default transport protocol. Options: `udp`, `tcp`, `dot` (DNS-over-TLS), `doh` (DNS-over-HTTPS). Overridable per-request in the API. |
| `retry` | int | `1` | `GDNS_DNS_RETRY` | Number of times to retry a failed query before marking as error. `0` means no retries. |

---

## resolvers

Resolver registry settings.

| Option | Type | Default | Env Variable | Description |
|---|---|---|---|---|
| `builtin` | bool | `true` | `GDNS_RESOLVERS_BUILTIN` | Whether to include the built-in curated resolver list (~100 global resolvers shipped with the binary). Set to `false` to use only resolvers from `file`. |
| `file` | string | `` | `GDNS_RESOLVERS_FILE` | Optional path to a YAML or JSON file containing additional or replacement resolvers. Each entry must match the `Resolver` type (id, name, ip, port, protocol, country, city, lat, lng). |
| `countries` | []string | `[]` | `GDNS_RESOLVERS_COUNTRIES` | Optional ISO-3166 alpha-2 country code whitelist. If set, only resolvers located in the listed countries are used. Useful for regional deployments. Example: `[US, DE, JP]`. Env: comma-separated string. |

---

## storage

Persistent storage for check history and permalinks.

| Option | Type | Default | Env Variable | Description |
|---|---|---|---|---|
| `driver` | string | `sqlite` | `GDNS_STORAGE_DRIVER` | Storage backend. Options: `sqlite` (pure-Go, zero external deps), `postgres` (production-grade), `none` (disable persistence — no history, no permalinks). |
| `dsn` | string | `file:./gdns.db?cache=shared&_fk=1` | `GDNS_STORAGE_DSN` | Data source name for the selected driver. SQLite: `file:/path/to/gdns.db?cache=shared&_fk=1`. Postgres: `postgres://user:pass@host:5432/dbname?sslmode=disable`. |
| `retention_days` | int | `30` | `GDNS_STORAGE_RETENTION_DAYS` | Automatically delete saved checks older than this many days. A background job runs at startup and periodically. Set to `0` to disable auto-purge. |

---

## cache

Response caching to reduce redundant DNS queries for popular domains.

| Option | Type | Default | Env Variable | Description |
|---|---|---|---|---|
| `driver` | string | `memory` | `GDNS_CACHE_DRIVER` | Cache backend. Options: `memory` (in-process LRU, default), `redis` (shared cache for multi-replica deployments), `none` (disable caching). |
| `ttl` | duration | `60s` | `GDNS_CACHE_TTL` | How long a cached `(name, type, resolver)` response is considered fresh. After expiry the next request triggers a fresh DNS query. |
| `size` | int | `10000` | `GDNS_CACHE_SIZE` | Maximum number of entries in the in-memory LRU cache. Only applies when `driver=memory`. When capacity is exceeded, least-recently-used entries are evicted. |
| `redis_addr` | string | `` | `GDNS_CACHE_REDIS_ADDR` | Redis server address. Only used when `driver=redis`. Format: `host:port`, e.g. `localhost:6379`. |

---

## ratelimit

Per-IP and per-API-key request throttling.

| Option | Type | Default | Env Variable | Description |
|---|---|---|---|---|
| `enabled` | bool | `true` | `GDNS_RATELIMIT_ENABLED` | Whether rate limiting is active. Set to `false` in private/trusted environments. |
| `anon_per_min` | int | `30` | `GDNS_RATELIMIT_ANON_PER_MIN` | Maximum requests per minute for anonymous (unauthenticated) callers, bucketed per IP address. Burst allowance is `ceil(anon_per_min / 3)`. |
| `key_per_min` | int | `600` | `GDNS_RATELIMIT_KEY_PER_MIN` | Maximum requests per minute for API key holders. Admin keys are unlimited. |

---

## geoip

Optional MaxMind GeoLite2 integration for geolocating custom resolvers. The built-in resolver list already contains embedded lat/lng/country data and does not need GeoIP.

| Option | Type | Default | Env Variable | Description |
|---|---|---|---|---|
| `enabled` | bool | `false` | `GDNS_GEOIP_ENABLED` | Whether to enable GeoIP lookups. Requires `mmdb_path` to be set. |
| `mmdb_path` | string | `` | `GDNS_GEOIP_MMDB_PATH` | Absolute path to a MaxMind GeoLite2-City `.mmdb` database file. Obtain a free copy from [maxmind.com](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data). |

---

## log

Logging configuration.

| Option | Type | Default | Env Variable | Description |
|---|---|---|---|---|
| `level` | string | `info` | `GDNS_LOG_LEVEL` | Minimum log level. Options: `debug`, `info`, `warn`, `error`. Use `debug` for verbose output during development. |
| `format` | string | `json` | `GDNS_LOG_FORMAT` | Log output format. `json` produces structured JSON (recommended for log aggregation). `text` produces human-readable output for local development. |

---

## metrics

Prometheus metrics exposure.

| Option | Type | Default | Env Variable | Description |
|---|---|---|---|---|
| `enabled` | bool | `true` | `GDNS_METRICS_ENABLED` | Whether to expose the Prometheus metrics endpoint. |
| `path` | string | `/metrics` | `GDNS_METRICS_PATH` | HTTP path at which Prometheus metrics are served. Note: this path is outside `/api/v1`. |

---

## Full Example

See `config/config.example.yaml` for a fully commented configuration file.

---

## Duration Format

Duration values use Go duration syntax: `ns`, `us`, `ms`, `s`, `m`, `h`.

Examples: `500ms`, `3s`, `10s`, `1m`, `1h30m`.
