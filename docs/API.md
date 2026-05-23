# API Reference

**Base URL**: `/api/v1`
**Content-Type**: `application/json` (UTF-8)
**Auth**: Optional. Send `Authorization: Bearer <api_key>` for higher rate limits.

The full machine-readable spec is available at:
- `GET /api/docs/openapi.yaml` — raw OpenAPI 3.1 YAML
- `GET /api/docs` — Swagger UI

---

## Error Envelope

All error responses use the following shape:

```json
{
  "error": {
    "code": "INVALID_RECORD_TYPE",
    "message": "Record type 'FOO' is not supported.",
    "details": { "supported": ["A", "AAAA", "..."] }
  },
  "request_id": "01HXYZ..."
}
```

Common error codes:

| Code | HTTP Status | Description |
|---|---|---|
| `INVALID_NAME` | 400 | Domain name failed RFC 1035 validation |
| `INVALID_RECORD_TYPE` | 400 | Record type not in the supported list |
| `INVALID_IP` | 400 | IP address failed validation (for reverse endpoint) |
| `PRIVATE_TARGET` | 400 | Target is a private/loopback address (SSRF guard) |
| `CHECK_NOT_FOUND` | 404 | Permalink ID does not exist |
| `RATE_LIMITED` | 429 | Rate limit exceeded |
| `INTERNAL_ERROR` | 500 | Unexpected server error |

---

## Health / Meta

### GET /api/v1/health

Liveness check. Always returns 200 while the process is running.

**Response 200:**
```json
{ "status": "ok" }
```

---

### GET /api/v1/readyz

Readiness check. Returns 200 only when the storage backend and cache are reachable.

**Response 200:**
```json
{ "status": "ok", "checks": { "storage": "ok", "cache": "ok" } }
```

**Response 503 (when a dependency is unavailable):**
```json
{ "status": "degraded", "checks": { "storage": "error: connection refused", "cache": "ok" } }
```

---

### GET /api/v1/version

Returns build version information.

**Response 200:**
```json
{
  "version": "v1.2.3",
  "commit": "abc1234",
  "date": "2026-05-23T00:00:00Z"
}
```

---

## Discovery

### GET /api/v1/resolvers

Returns the list of available DNS resolvers.

**Query parameters:**

| Parameter | Type | Description |
|---|---|---|
| `country` | string | Filter by ISO-3166 alpha-2 country code, e.g. `US` |
| `protocol` | string | Filter by protocol: `udp`, `tcp`, `dot`, `doh` |
| `q` | string | Free-text search across name, city, ISP fields |

**Response 200:**
```json
[
  {
    "id": "google-us",
    "name": "Google",
    "ip": "8.8.8.8",
    "port": 53,
    "protocol": "udp",
    "country": "US",
    "city": "Mountain View",
    "lat": 37.386,
    "lng": -122.084,
    "asn": 15169,
    "isp": "Google LLC"
  }
]
```

---

### GET /api/v1/resolvers/{id}

Returns a single resolver by its stable slug ID.

**Path parameters:**

| Parameter | Description |
|---|---|
| `id` | Resolver ID, e.g. `google-us` |

**Response 200:** Single Resolver object (same shape as above).

**Response 404:** Error envelope with `CHECK_NOT_FOUND`.

---

### GET /api/v1/record-types

Returns the list of supported DNS record types with descriptions.

**Response 200:**
```json
[
  { "type": "A",     "description": "IPv4 address record" },
  { "type": "AAAA",  "description": "IPv6 address record" },
  { "type": "CNAME", "description": "Canonical name alias" },
  { "type": "MX",    "description": "Mail exchange record" },
  { "type": "NS",    "description": "Name server record" },
  { "type": "PTR",   "description": "Reverse DNS pointer record" },
  { "type": "SOA",   "description": "Start of authority record" },
  { "type": "TXT",   "description": "Text record" },
  { "type": "CAA",   "description": "Certification Authority Authorization" },
  { "type": "SRV",   "description": "Service locator record" },
  { "type": "DS",    "description": "Delegation signer (DNSSEC)" },
  { "type": "DNSKEY","description": "DNS public key (DNSSEC)" },
  { "type": "NAPTR", "description": "Naming authority pointer" },
  { "type": "TLSA",  "description": "TLS authentication record (DANE)" },
  { "type": "SVCB",  "description": "Service binding record" },
  { "type": "HTTPS", "description": "HTTPS service binding" }
]
```

---

## Propagation Check (multi-resolver)

### POST /api/v1/check

Run a DNS propagation check across all (or a subset of) resolvers.

**Request body:**

```json
{
  "name": "example.com",
  "type": "A",
  "resolvers": ["google-us", "cloudflare-us"],
  "custom_resolvers": [
    {
      "id": "my-resolver",
      "name": "My Internal Resolver",
      "ip": "10.0.0.1",
      "port": 53,
      "protocol": "udp",
      "country": "US"
    }
  ],
  "save": true
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | Yes | Domain name to query. IDNA/punycode accepted. |
| `type` | string | Yes | Record type. One of the 16 supported types. |
| `resolvers` | []string | No | Subset of resolver IDs to use. If omitted, all built-in resolvers are used. |
| `custom_resolvers` | []Resolver | No | Additional resolver objects not in the registry. |
| `save` | bool | No | Whether to persist the check for permalink access. Default: `false`. |

**Response 200:**

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
        "port": 53,
        "protocol": "udp",
        "country": "US",
        "city": "Mountain View",
        "lat": 37.386,
        "lng": -122.084
      },
      "status": "ok",
      "answers": [
        { "name": "example.com.", "type": "A", "ttl": 3600, "value": "93.184.216.34" }
      ],
      "authority": [],
      "additional": [],
      "dnssec": false,
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

---

### GET /api/v1/check/{id}

Retrieve a previously saved (permalinked) check.

**Path parameters:**

| Parameter | Description |
|---|---|
| `id` | Permalink ID returned by `POST /api/v1/check` (only available when `save: true`) |

**Response 200:** Full Check object (same shape as POST response).

**Response 404:** Error envelope — check not found or not saved.

---

### GET /api/v1/check/{id}/export

Export a saved check in the requested format.

**Query parameters:**

| Parameter | Values | Description |
|---|---|---|
| `format` | `json`, `csv`, `png`, `svg`, `pdf` | Output format |

**Response 200:** Binary file with appropriate Content-Type and Content-Disposition headers for download.

---

### GET /api/v1/check/stream (WebSocket)

Real-time streaming variant. The server pushes results as each resolver responds, rather than waiting for all resolvers to complete.

**Upgrade:** `Connection: Upgrade`, `Upgrade: websocket`

**Client sends** (once, after connection opens):
```json
{ "name": "example.com", "type": "A", "resolvers": [] }
```

**Server sends** (one frame per event):
```json
{ "type": "total",  "data": { "total": 100 } }
{ "type": "result", "data": { ...ResolverResult... } }
{ "type": "result", "data": { ...ResolverResult... } }
{ "type": "done",   "data": { "summary": { ...CheckSummary... }, "id": "a8x2k9" } }
```

On error:
```json
{ "type": "error", "data": { "code": "INVALID_NAME", "message": "..." } }
```

---

## Single-Resolver Lookup

### POST /api/v1/lookup

Query a single resolver and return the full DNS response (all sections, flags, RCODE).

**Request body:**

```json
{
  "name": "example.com",
  "type": "A",
  "resolver": "8.8.8.8",
  "protocol": "udp"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | Yes | Domain name to query |
| `type` | string | Yes | Record type |
| `resolver` | string | No | Resolver IP or registry ID. Default: `1.1.1.1` |
| `protocol` | string | No | Transport protocol. Default: `udp` |

**Response 200:** `ResolverResult` object with all sections populated:

```json
{
  "resolver": { "id": "cloudflare", "name": "Cloudflare", "ip": "1.1.1.1", ... },
  "status": "ok",
  "answers": [
    { "name": "example.com.", "type": "A", "ttl": 3600, "value": "93.184.216.34" }
  ],
  "authority": [],
  "additional": [],
  "flags": { "QR": true, "AA": false, "TC": false, "RD": true, "RA": true, "AD": false, "CD": false },
  "dnssec": false,
  "duration_ms": 18,
  "queried_at": "2026-05-23T12:34:56Z"
}
```

---

## Reverse DNS

### POST /api/v1/reverse

Perform a reverse DNS (PTR) lookup for an IPv4 or IPv6 address across global resolvers.

The server automatically converts the IP address to the appropriate `in-addr.arpa` (IPv4) or `ip6.arpa` (IPv6) form before querying.

**Request body:**

```json
{
  "ip": "8.8.8.8",
  "resolvers": []
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `ip` | string | Yes | IPv4 or IPv6 address to look up |
| `resolvers` | []string | No | Subset of resolver IDs. If omitted, all resolvers are used. |

**Response 200:** Same shape as `POST /api/v1/check`, but `name` is the PTR query form (e.g. `8.8.8.8.in-addr.arpa.`) and `type` is `PTR`.

---

## History

Requires storage to be enabled (`storage.driver != none`).

### GET /api/v1/history

List recent saved checks.

**Auth:** API key required.

**Query parameters:**

| Parameter | Type | Description |
|---|---|---|
| `page` | int | Page number, 1-indexed. Default: `1` |
| `per_page` | int | Results per page. Default: `20`, max: `100` |

**Response 200:**
```json
{
  "checks": [ { ...Check... } ],
  "total": 42,
  "page": 1,
  "per_page": 20
}
```

---

### DELETE /api/v1/history/{id}

Delete a saved check.

**Auth:** API key required.

**Response 204:** No content on success.

---

## Rate Limits

Rate limit headers are included on every response:

```
X-RateLimit-Limit: 30
X-RateLimit-Remaining: 27
X-RateLimit-Reset: 1716465600
```

HTTP 429 responses include a `Retry-After` header (seconds).

| Caller type | Limit |
|---|---|
| Anonymous (per IP) | 30 req/min, burst 10 |
| API key (default) | 600 req/min, burst 60 |
| Admin key | Unlimited |
| WebSocket (anonymous) | 1 concurrent connection per IP |
| WebSocket (API key) | 10 concurrent connections |
