#!/usr/bin/env bash
# test-integration.sh — Smoke tests the gdns HTTP API against a locally running server.
# Starts the binary with SQLite in a temp dir, runs a basic propagation check,
# and verifies the response shape.
#
# Usage: ./scripts/test-integration.sh
# Exit code: 0 on success, 1 on failure.

set -euo pipefail

###############################################################################
# Configuration
###############################################################################
BINARY="${BINARY:-./bin/gdns}"
PORT="${PORT:-18080}"
BASE_URL="http://localhost:${PORT}"
TMPDIR=$(mktemp -d)
SERVER_PID=""

###############################################################################
# Dependency checks
###############################################################################
for cmd in curl jq; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "ERROR: '$cmd' is required but not found in PATH." >&2
    exit 1
  fi
done

if [[ ! -f "$BINARY" ]]; then
  echo "ERROR: Binary not found at $BINARY. Run 'make build' first." >&2
  exit 1
fi

###############################################################################
# Cleanup trap
###############################################################################
cleanup() {
  if [[ -n "$SERVER_PID" ]]; then
    echo "Stopping server (PID $SERVER_PID)..."
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$TMPDIR"
}
trap cleanup EXIT

###############################################################################
# Start server
###############################################################################
echo "Starting gdns on port $PORT with SQLite in $TMPDIR ..."
GDNS_SERVER_LISTEN=":${PORT}" \
GDNS_STORAGE_DRIVER="sqlite" \
GDNS_STORAGE_DSN="file:${TMPDIR}/gdns.db?cache=shared&_fk=1" \
GDNS_CACHE_DRIVER="memory" \
GDNS_LOG_FORMAT="text" \
GDNS_LOG_LEVEL="warn" \
GDNS_METRICS_ENABLED="false" \
  "$BINARY" &
SERVER_PID=$!

###############################################################################
# Wait for health endpoint
###############################################################################
echo "Waiting for server to become ready..."
RETRIES=30
until curl -sf "${BASE_URL}/api/v1/health" >/dev/null 2>&1; do
  RETRIES=$((RETRIES - 1))
  if [[ $RETRIES -le 0 ]]; then
    echo "FAIL: Server did not become healthy in time." >&2
    exit 1
  fi
  sleep 0.5
done
echo "Server is healthy."

###############################################################################
# Test 1: Health endpoint
###############################################################################
echo ""
echo "Test 1: GET /api/v1/health"
HEALTH=$(curl -sf "${BASE_URL}/api/v1/health")
STATUS=$(echo "$HEALTH" | jq -r '.status')
if [[ "$STATUS" != "ok" ]]; then
  echo "FAIL: Expected status=ok, got: $STATUS" >&2
  exit 1
fi
echo "PASS: health status=$STATUS"

###############################################################################
# Test 2: Version endpoint
###############################################################################
echo ""
echo "Test 2: GET /api/v1/version"
VERSION_RESP=$(curl -sf "${BASE_URL}/api/v1/version")
VERSION_VAL=$(echo "$VERSION_RESP" | jq -r '.version')
if [[ -z "$VERSION_VAL" || "$VERSION_VAL" == "null" ]]; then
  echo "FAIL: Version response missing 'version' field." >&2
  exit 1
fi
echo "PASS: version=$VERSION_VAL"

###############################################################################
# Test 3: List resolvers
###############################################################################
echo ""
echo "Test 3: GET /api/v1/resolvers"
RESOLVERS=$(curl -sf "${BASE_URL}/api/v1/resolvers")
RESOLVER_COUNT=$(echo "$RESOLVERS" | jq 'length')
if [[ "$RESOLVER_COUNT" -lt 1 ]]; then
  echo "FAIL: Expected at least 1 resolver, got $RESOLVER_COUNT." >&2
  exit 1
fi
echo "PASS: $RESOLVER_COUNT resolvers returned"

###############################################################################
# Test 4: Propagation check
###############################################################################
echo ""
echo "Test 4: POST /api/v1/check (example.com A)"
CHECK_RESP=$(curl -sf -X POST "${BASE_URL}/api/v1/check" \
  -H "Content-Type: application/json" \
  -d '{"name":"example.com","type":"A","save":false}')

# Verify top-level fields
for field in name type results; do
  VAL=$(echo "$CHECK_RESP" | jq -r --arg f "$field" '.[$f]')
  if [[ "$VAL" == "null" || -z "$VAL" ]]; then
    echo "FAIL: Response missing field '$field'." >&2
    echo "Response: $CHECK_RESP" >&2
    exit 1
  fi
done

RESULT_COUNT=$(echo "$CHECK_RESP" | jq '.results | length')
CHECK_NAME=$(echo "$CHECK_RESP" | jq -r '.name')
CHECK_TYPE=$(echo "$CHECK_RESP" | jq -r '.type')

if [[ "$CHECK_NAME" != "example.com" ]]; then
  echo "FAIL: Expected name=example.com, got $CHECK_NAME" >&2
  exit 1
fi
if [[ "$CHECK_TYPE" != "A" ]]; then
  echo "FAIL: Expected type=A, got $CHECK_TYPE" >&2
  exit 1
fi

echo "PASS: check returned $RESULT_COUNT resolver results"

###############################################################################
# Test 5: Record types endpoint
###############################################################################
echo ""
echo "Test 5: GET /api/v1/record-types"
TYPES=$(curl -sf "${BASE_URL}/api/v1/record-types")
TYPE_COUNT=$(echo "$TYPES" | jq 'length')
if [[ "$TYPE_COUNT" -lt 10 ]]; then
  echo "FAIL: Expected at least 10 record types, got $TYPE_COUNT." >&2
  exit 1
fi
echo "PASS: $TYPE_COUNT record types returned"

###############################################################################
# Summary
###############################################################################
echo ""
echo "All integration tests passed."
