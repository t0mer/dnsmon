# Deployment Guide

This guide covers deploying dnsmon in various environments.

---

## Quick Start — Docker Compose (SQLite)

The simplest way to run dnsmon. Data persists in a named Docker volume.

```bash
git clone https://github.com/t0mer/dnsmon.git
cd dnsmon
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
- `dnsmon` — the Go binary
- `postgres:16-alpine` — persistent check history
- `redis:7-alpine` — distributed cache and rate-limit state

Check logs:
```bash
docker compose -f deploy/docker-compose.yml logs -f dnsmon
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
- You update `dnsmon.example.com` to your real hostname.

For custom configuration, mount a ConfigMap as `/etc/dnsmon/config.yaml` and set the
`--config` flag:

```yaml
# In deployment.yaml, add to spec.containers[0]:
args: ["--config", "/etc/dnsmon/config.yaml"]
volumeMounts:
  - name: config
    mountPath: /etc/dnsmon
    readOnly: true
volumes:
  - name: config
    configMap:
      name: dnsmon-config
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
      claimName: dnsmon-data
```

For Postgres, deploy a separate Postgres instance (or use a managed database) and set:
```
DNSMON_STORAGE_DRIVER=postgres
DNSMON_STORAGE_DSN=postgres://user:pass@host:5432/dnsmon?sslmode=require
```

---

## systemd (bare metal / VM)

1. **Install the binary:**
   ```bash
   sudo cp bin/dnsmon /usr/local/bin/dnsmon
   sudo chmod +x /usr/local/bin/dnsmon
   ```

2. **Create a dedicated user:**
   ```bash
   sudo useradd --system --no-create-home --shell /usr/sbin/nologin dnsmon
   ```

3. **Create config directory and file:**
   ```bash
   sudo mkdir -p /etc/dnsmon
   sudo cp config/config.example.yaml /etc/dnsmon/config.yaml
   sudo chown -R dnsmon:dnsmon /etc/dnsmon
   ```

4. **Install the systemd unit:**
   ```bash
   sudo cp deploy/systemd/dnsmon.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable --now dnsmon
   ```

5. **Check status:**
   ```bash
   sudo systemctl status dnsmon
   sudo journalctl -u dnsmon -f
   ```

---

## Behind Nginx (Reverse Proxy)

dnsmon expects to run behind a reverse proxy that handles TLS termination.

Minimal Nginx config:

```nginx
server {
    listen 80;
    server_name dnsmon.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name dnsmon.example.com;

    ssl_certificate     /etc/letsencrypt/live/dnsmon.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/dnsmon.example.com/privkey.pem;
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

All configuration options from `config.yaml` can be overridden with `DNSMON_*` environment variables. The mapping is: dots become underscores, all uppercase.

| Variable | Default | Description |
|---|---|---|
| `DNSMON_SERVER_LISTEN` | `:8080` | Listen address |
| `DNSMON_SERVER_BASE_URL` | `http://localhost:8080` | Public URL for permalinks |
| `DNSMON_SERVER_READ_TIMEOUT` | `10s` | HTTP read timeout |
| `DNSMON_SERVER_WRITE_TIMEOUT` | `30s` | HTTP write timeout |
| `DNSMON_DNS_QUERY_TIMEOUT` | `3s` | Per-resolver DNS query timeout |
| `DNSMON_DNS_PER_RESOLVER_CONCURRENCY` | `4` | Max concurrent outbound queries |
| `DNSMON_DNS_DEFAULT_PROTOCOL` | `udp` | Default DNS transport |
| `DNSMON_DNS_RETRY` | `1` | Query retries on failure |
| `DNSMON_RESOLVERS_BUILTIN` | `true` | Use built-in resolver list |
| `DNSMON_RESOLVERS_FILE` | `` | Path to extra resolvers file |
| `DNSMON_RESOLVERS_COUNTRIES` | `` | Country whitelist (comma-separated) |
| `DNSMON_STORAGE_DRIVER` | `sqlite` | `sqlite`, `postgres`, or `none` |
| `DNSMON_STORAGE_DSN` | (SQLite file) | Data source name |
| `DNSMON_STORAGE_RETENTION_DAYS` | `30` | Auto-delete checks older than N days |
| `DNSMON_CACHE_DRIVER` | `memory` | `memory`, `redis`, or `none` |
| `DNSMON_CACHE_TTL` | `60s` | Cache entry TTL |
| `DNSMON_CACHE_SIZE` | `10000` | LRU cache max entries (memory only) |
| `DNSMON_CACHE_REDIS_ADDR` | `` | Redis address (e.g. `localhost:6379`) |
| `DNSMON_RATELIMIT_ENABLED` | `true` | Enable rate limiting |
| `DNSMON_RATELIMIT_ANON_PER_MIN` | `30` | Anonymous req/min per IP |
| `DNSMON_RATELIMIT_KEY_PER_MIN` | `600` | API key req/min |
| `DNSMON_GEOIP_ENABLED` | `false` | Enable MaxMind GeoIP |
| `DNSMON_GEOIP_MMDB_PATH` | `` | Path to GeoLite2-City.mmdb |
| `DNSMON_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `DNSMON_LOG_FORMAT` | `json` | `json` or `text` |
| `DNSMON_METRICS_ENABLED` | `true` | Enable Prometheus metrics |
| `DNSMON_METRICS_PATH` | `/metrics` | Metrics endpoint path |

---

## Building from Source

Requirements: Go 1.23+, Node.js 20+ (build-time only).

```bash
# Clone
git clone https://github.com/t0mer/dnsmon.git
cd dnsmon

# Build UI (Tailwind CSS)
make ui

# Build binary
make build

# Run
./bin/dnsmon --config config/config.example.yaml
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
