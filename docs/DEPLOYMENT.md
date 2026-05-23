# Deployment Guide

This guide covers deploying gdns in various environments.

---

## Quick Start — Docker Compose (SQLite)

The simplest way to run gdns. Data persists in a named Docker volume.

```bash
git clone https://github.com/tomerklein/gdns.git
cd gdns
docker compose -f deploy/docker-compose.sqlite.yml up -d
```

Open http://localhost:8080 in your browser.

---

## Docker Compose — Postgres + Redis (Production)

For multi-replica or higher-load deployments:

```bash
# Edit deploy/docker-compose.yml to change passwords before first run
docker compose -f deploy/docker-compose.yml up -d
```

This starts three containers:
- `gdns` — the Go binary
- `postgres:16-alpine` — persistent check history
- `redis:7-alpine` — distributed cache and rate-limit state

Check logs:
```bash
docker compose -f deploy/docker-compose.yml logs -f gdns
```

---

## Kubernetes

Apply the manifests in order:

```bash
kubectl apply -f deploy/k8s/deployment.yaml
kubectl apply -f deploy/k8s/service.yaml
kubectl apply -f deploy/k8s/ingress.yaml
```

The Ingress assumes:
- `ingress-nginx` is installed in the cluster.
- `cert-manager` is installed and a `ClusterIssuer` named `letsencrypt-prod` exists.
- You update `gdns.example.com` to your real hostname.

For custom configuration, mount a ConfigMap as `/etc/gdns/config.yaml` and set the
`--config` flag:

```yaml
# In deployment.yaml, add to spec.containers[0]:
args: ["--config", "/etc/gdns/config.yaml"]
volumeMounts:
  - name: config
    mountPath: /etc/gdns
    readOnly: true
volumes:
  - name: config
    configMap:
      name: gdns-config
```

### Persistent storage on Kubernetes

For SQLite, mount a `PersistentVolumeClaim`:

```yaml
volumeMounts:
  - name: data
    mountPath: /data
volumes:
  - name: data
    persistentVolumeClaim:
      claimName: gdns-data
```

For Postgres, deploy a separate Postgres instance (or use a managed database) and set:
```
GDNS_STORAGE_DRIVER=postgres
GDNS_STORAGE_DSN=postgres://user:pass@host:5432/gdns?sslmode=require
```

---

## systemd (bare metal / VM)

1. **Install the binary:**
   ```bash
   sudo cp bin/gdns /usr/local/bin/gdns
   sudo chmod +x /usr/local/bin/gdns
   ```

2. **Create a dedicated user:**
   ```bash
   sudo useradd --system --no-create-home --shell /usr/sbin/nologin gdns
   ```

3. **Create config directory and file:**
   ```bash
   sudo mkdir -p /etc/gdns
   sudo cp config/config.example.yaml /etc/gdns/config.yaml
   sudo chown -R gdns:gdns /etc/gdns
   ```

4. **Install the systemd unit:**
   ```bash
   sudo cp deploy/systemd/gdns.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable --now gdns
   ```

5. **Check status:**
   ```bash
   sudo systemctl status gdns
   sudo journalctl -u gdns -f
   ```

---

## Behind Nginx (Reverse Proxy)

gdns expects to run behind a reverse proxy that handles TLS termination.

Minimal Nginx config:

```nginx
server {
    listen 80;
    server_name gdns.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name gdns.example.com;

    ssl_certificate     /etc/letsencrypt/live/gdns.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/gdns.example.com/privkey.pem;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    location / {
        proxy_pass         http://127.0.0.1:8080;
        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;

        # WebSocket support for /api/v1/check/stream
        proxy_http_version 1.1;
        proxy_set_header   Upgrade    $http_upgrade;
        proxy_set_header   Connection "upgrade";

        proxy_read_timeout  300s;
        proxy_send_timeout  300s;
    }
}
```

Reload Nginx:
```bash
sudo nginx -t && sudo systemctl reload nginx
```

---

## Environment Variables Reference

All configuration options from `config.yaml` can be overridden with `GDNS_*` environment variables. The mapping is: dots become underscores, all uppercase.

| Variable | Default | Description |
|---|---|---|
| `GDNS_SERVER_LISTEN` | `:8080` | Listen address |
| `GDNS_SERVER_BASE_URL` | `http://localhost:8080` | Public URL for permalinks |
| `GDNS_SERVER_READ_TIMEOUT` | `10s` | HTTP read timeout |
| `GDNS_SERVER_WRITE_TIMEOUT` | `30s` | HTTP write timeout |
| `GDNS_DNS_QUERY_TIMEOUT` | `3s` | Per-resolver DNS query timeout |
| `GDNS_DNS_PER_RESOLVER_CONCURRENCY` | `4` | Max concurrent outbound queries |
| `GDNS_DNS_DEFAULT_PROTOCOL` | `udp` | Default DNS transport |
| `GDNS_DNS_RETRY` | `1` | Query retries on failure |
| `GDNS_RESOLVERS_BUILTIN` | `true` | Use built-in resolver list |
| `GDNS_RESOLVERS_FILE` | `` | Path to extra resolvers file |
| `GDNS_RESOLVERS_COUNTRIES` | `` | Country whitelist (comma-separated) |
| `GDNS_STORAGE_DRIVER` | `sqlite` | `sqlite`, `postgres`, or `none` |
| `GDNS_STORAGE_DSN` | (SQLite file) | Data source name |
| `GDNS_STORAGE_RETENTION_DAYS` | `30` | Auto-delete checks older than N days |
| `GDNS_CACHE_DRIVER` | `memory` | `memory`, `redis`, or `none` |
| `GDNS_CACHE_TTL` | `60s` | Cache entry TTL |
| `GDNS_CACHE_SIZE` | `10000` | LRU cache max entries (memory only) |
| `GDNS_CACHE_REDIS_ADDR` | `` | Redis address (e.g. `localhost:6379`) |
| `GDNS_RATELIMIT_ENABLED` | `true` | Enable rate limiting |
| `GDNS_RATELIMIT_ANON_PER_MIN` | `30` | Anonymous req/min per IP |
| `GDNS_RATELIMIT_KEY_PER_MIN` | `600` | API key req/min |
| `GDNS_GEOIP_ENABLED` | `false` | Enable MaxMind GeoIP |
| `GDNS_GEOIP_MMDB_PATH` | `` | Path to GeoLite2-City.mmdb |
| `GDNS_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `GDNS_LOG_FORMAT` | `json` | `json` or `text` |
| `GDNS_METRICS_ENABLED` | `true` | Enable Prometheus metrics |
| `GDNS_METRICS_PATH` | `/metrics` | Metrics endpoint path |

---

## Building from Source

Requirements: Go 1.23+, Node.js 20+ (build-time only).

```bash
# Clone
git clone https://github.com/tomerklein/gdns.git
cd gdns

# Build UI (Tailwind CSS)
make ui

# Build binary
make build

# Run
./bin/gdns --config config/config.example.yaml
```

---

## Upgrading

1. Pull the new image or binary.
2. Database migrations run automatically on startup.
3. For zero-downtime on Kubernetes, the Deployment uses `RollingUpdate` strategy by default.

---

## Health Checks

| Endpoint | Purpose | Returns 200 when |
|---|---|---|
| `GET /api/v1/health` | Liveness | Always |
| `GET /api/v1/readyz` | Readiness | Storage + cache are reachable |

Use `readyz` for Kubernetes `readinessProbe` and `health` for `livenessProbe`.
