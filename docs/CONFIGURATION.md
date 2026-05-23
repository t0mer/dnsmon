# Configuration Reference

dnsmon is configured via a YAML file (default: `config.yaml` in the working directory) and/or environment variables. Environment variables take precedence over the config file.

Pass a config file path with `--config /path/to/config.yaml`.

---

## Command-Line Flags

Flags override the config file and environment for the current run.

| Flag | Description |
|---|---|
| `--config` | Path to a YAML config file. |
| `--listen` | Override the full listen address, e.g. `:8080` or `0.0.0.0:9090`. |
| `--port` | Override only the port, preserving any host in `server.listen`. Example: `--port 9090`. Takes effect after `--listen` if both are given. |
| `--log-level` | Override the log level (`debug`, `info`, `warn`, `error`). |
| `--resolvers-file` | Path to an extra resolvers JSON/YAML file. |
| `--service` | Manage dnsmon as a system service: `install`, `uninstall`, `start`, `stop`, or `restart`. |

### Running as a system service

`--service` registers dnsmon with the host service manager (Windows Service
Control Manager, Linux systemd, or macOS launchd):

```bash
# Install (run with the flags the service should use; paths are made absolute)
dnsmon --port 8080 --config /etc/dnsmon/config.yaml --service install

# Then control it through the OS, or via dnsmon:
dnsmon --service start
dnsmon --service stop
dnsmon --service uninstall
```

On Linux this requires root (it writes `/etc/systemd/system/dnsmon.service`);
on Windows run the command from an elevated prompt. Any `--config`,
`--resolvers-file`, `--listen`, `--port`, and `--log-level` flags passed
alongside `--service install` are baked into the service definition so the
service starts with the same settings.

---

## server

HTTP server settings.

| Option | Type | Default | Env Variable | Description |
|---|---|---|---|---|
| `listen` | string | `:8080` | `DNSMON_SERVER_LISTEN` | TCP address to listen on. Examples: `:8080`, `0.0.0.0:9090`, `127.0.0.1:8080` |
| `base_url` | string | `http://localhost:8080` | `DNSMON_SERVER_BASE_URL` | Public-facing base URL. Used to construct permalink URLs returned in API responses. Include scheme and host, no trailing slash. |
| `read_timeout` | duration | `10s` | `DNSMON_SERVER_READ_TIMEOUT` | Maximum duration to read the entire request, including body. Prevents slowloris attacks. |
| `write_timeout` | duration | `30s` | `DNSMON_SERVER_WRITE_TIMEOUT` | Maximum duration to write the entire response. Set this longer than the maximum expected DNS fan-out time. |

---

## dns

Controls outbound DNS query behavior.

| Option | Type | Default | Env Variable | Description |
|---|---|---|---|---|
| `query_timeout` | duration | `3s` | `DNSMON_DNS_QUERY_TIMEOUT` | Timeout per individual DNS query to a single resolver. Resolvers that don't respond within this window are marked as `timeout`. |
| `per_resolver_concurrency` | int | `4` | `DNSMON_DNS_PER_RESOLVER_CONCURRENCY` | Maximum number of simultaneous outbound DNS queries per propagation check. Higher values reduce total check time at the cost of network bandwidth. |
| `default_protocol` | string | `udp` | `DNSMON_DNS_DEFAULT_PROTOCOL` | Default transport protocol. Options: `udp`, `tcp`, `dot` (DNS-over-TLS), `doh` (DNS-over-HTTPS). Overridable per-request in the API. |
| `retry` | int | `1` | `DNSMON_DNS_RETRY` | Number of times to retry a failed query before marking as error. `0` means no retries. |

---

## resolvers

Resolver registry settings.

| Option | Type | Default | Env Variable | Description |
|---|---|---|---|---|
| `builtin` | bool | `true` | `DNSMON_RESOLVERS_BUILTIN` | Whether to include the built-in curated resolver list (~100 global resolvers shipped with the binary). Set to `false` to use only resolvers from `file`. |
| `file` | string | `` | `DNSMON_RESOLVERS_FILE` | Optional path to a YAML or JSON file containing additional or replacement resolvers. Each entry must match the `Resolver` type (id, name, ip, port, protocol, country, city, lat, lng). |
| `countries` | []string | `[]` | `DNSMON_RESOLVERS_COUNTRIES` | Optional ISO-3166 alpha-2 country code whitelist. If set, only resolvers located in the listed countries are used. Useful for regional deployments. Example: `[US, DE, JP]`. Env: comma-separated string. |

---

## storage

Persistent storage for check history and permalinks.

| Option | Type | Default | Env Variable | Description |
|---|---|---|---|---|
| `driver` | string | `sqlite` | `DNSMON_STORAGE_DRIVER` | Storage backend. Options: `sqlite` (pure-Go, zero external deps), `postgres` (production-grade), `none` (disable persistence — no history, no permalinks). |
| `dsn` | string | `file:./dnsmon.db?cache=shared&_fk=1` | `DNSMON_STORAGE_DSN` | Data source name for the selected driver. SQLite: `file:/path/to/dnsmon.db?cache=shared&_fk=1`. Postgres: `postgres://user:pass@host:5432/dbname?sslmode=disable`. |
| `retention_days` | int | `30` | `DNSMON_STORAGE_RETENTION_DAYS` | Automatically delete saved checks older than this many days. A background job runs at startup and periodically. Set to `0` to disable auto-purge. |

---

## cache

Response caching to reduce redundant DNS queries for popular domains.

| Option | Type | Default | Env Variable | Description |
|---|---|---|---|---|
| `driver` | string | `memory` | `DNSMON_CACHE_DRIVER` | Cache backend. Options: `memory` (in-process LRU, default), `redis` (shared cache for multi-replica deployments), `none` (disable caching). |
| `ttl` | duration | `60s` | `DNSMON_CACHE_TTL` | How long a cached `(name, type, resolver)` response is considered fresh. After expiry the next request triggers a fresh DNS query. |
| `size` | int | `10000` | `DNSMON_CACHE_SIZE` | Maximum number of entries in the in-memory LRU cache. Only applies when `driver=memory`. When capacity is exceeded, least-recently-used entries are evicted. |
| `redis_addr` | string | `` | `DNSMON_CACHE_REDIS_ADDR` | Redis server address. Only used when `driver=redis`. Format: `host:port`, e.g. `localhost:6379`. |

---

## ratelimit

Per-IP and per-API-key request throttling.

| Option | Type | Default | Env Variable | Description |
|---|---|---|---|---|
| `enabled` | bool | `true` | `DNSMON_RATELIMIT_ENABLED` | Whether rate limiting is active. Set to `false` in private/trusted environments. |
| `anon_per_min` | int | `30` | `DNSMON_RATELIMIT_ANON_PER_MIN` | Maximum requests per minute for anonymous (unauthenticated) callers, bucketed per IP address. Burst allowance is `ceil(anon_per_min / 3)`. |
| `key_per_min` | int | `600` | `DNSMON_RATELIMIT_KEY_PER_MIN` | Maximum requests per minute for API key holders. Admin keys are unlimited. |

---

## geoip

Optional MaxMind GeoLite2 integration for geolocating custom resolvers. The built-in resolver list already contains embedded lat/lng/country data and does not need GeoIP.

| Option | Type | Default | Env Variable | Description |
|---|---|---|---|---|
| `enabled` | bool | `false` | `DNSMON_GEOIP_ENABLED` | Whether to enable GeoIP lookups. Requires `mmdb_path` to be set. |
| `mmdb_path` | string | `` | `DNSMON_GEOIP_MMDB_PATH` | Absolute path to a MaxMind GeoLite2-City `.mmdb` database file. Obtain a free copy from [maxmind.com](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data). |

---

## log

Logging configuration.

| Option | Type | Default | Env Variable | Description |
|---|---|---|---|---|
| `level` | string | `info` | `DNSMON_LOG_LEVEL` | Minimum log level. Options: `debug`, `info`, `warn`, `error`. Use `debug` for verbose output during development. |
| `format` | string | `json` | `DNSMON_LOG_FORMAT` | Log output format. `json` produces structured JSON (recommended for log aggregation). `text` produces human-readable output for local development. |

---

## metrics

Prometheus metrics exposure.

| Option | Type | Default | Env Variable | Description |
|---|---|---|---|---|
| `enabled` | bool | `true` | `DNSMON_METRICS_ENABLED` | Whether to expose the Prometheus metrics endpoint. |
| `path` | string | `/metrics` | `DNSMON_METRICS_PATH` | HTTP path at which Prometheus metrics are served. Note: this path is outside `/api/v1`. |

---

## Full Example

See `config/config.example.yaml` for a fully commented configuration file.

---

## Duration Format

Duration values use Go duration syntax: `ns`, `us`, `ms`, `s`, `m`, `h`.

Examples: `500ms`, `3s`, `10s`, `1m`, `1h30m`.
