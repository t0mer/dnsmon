#!/usr/bin/env bash
# update-resolvers.sh — Refreshes internal/resolvers/resolvers.json from public-dns.info
# Usage: ./scripts/update-resolvers.sh [--dry-run]
#
# Requirements: curl, dig, jq
# The script probes each resolver with a known control query and drops non-responding ones.
# It outputs a summary of how many resolvers were kept and dropped.

set -euo pipefail

###############################################################################
# Configuration
###############################################################################
SOURCE_URL="https://public-dns.info/nameservers.json"
OUTPUT_FILE="$(dirname "$0")/../internal/resolvers/resolvers.json"
CONTROL_DOMAIN="google.com"
PROBE_TIMEOUT=2        # seconds per probe
PROBE_RETRIES=1
MAX_RESOLVERS=150      # cap to keep the binary lean
DRY_RUN=false

for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=true ;;
  esac
done

###############################################################################
# Dependency checks
###############################################################################
for cmd in curl dig jq; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "ERROR: '$cmd' is required but not found in PATH." >&2
    exit 1
  fi
done

###############################################################################
# Download source list
###############################################################################
echo "Downloading resolver list from $SOURCE_URL ..."
RAW_JSON=$(curl -fsSL --max-time 30 "$SOURCE_URL")

TOTAL_RAW=$(echo "$RAW_JSON" | jq 'length')
echo "Downloaded $TOTAL_RAW resolver candidates."

###############################################################################
# Filter: keep only IPv4 resolvers with a name and country code
###############################################################################
FILTERED=$(echo "$RAW_JSON" | jq '[
  .[] |
  select(
    (.ip | test("^[0-9]+\\.[0-9]+\\.[0-9]+\\.[0-9]+$")) and
    (.country_id != null and .country_id != "") and
    (.name != null and .name != "")
  )
]')

TOTAL_FILTERED=$(echo "$FILTERED" | jq 'length')
echo "After basic filtering: $TOTAL_FILTERED candidates."

###############################################################################
# Probe each resolver
###############################################################################
KEPT=0
DROPPED=0
RESULTS="[]"

# Limit to a reasonable number to probe
CANDIDATES=$(echo "$FILTERED" | jq --argjson max "$MAX_RESOLVERS" '.[0:($max * 2)]')

echo "Probing resolvers (this may take a few minutes)..."

while IFS= read -r resolver_json; do
  IP=$(echo "$resolver_json" | jq -r '.ip')
  NAME=$(echo "$resolver_json" | jq -r '.name')
  COUNTRY=$(echo "$resolver_json" | jq -r '.country_id' | tr '[:lower:]' '[:upper:]')
  CITY=$(echo "$resolver_json" | jq -r '.city // ""')
  LAT=$(echo "$resolver_json" | jq -r '.latitude // 0')
  LNG=$(echo "$resolver_json" | jq -r '.longitude // 0')

  # Skip RFC1918 / link-local / loopback addresses
  if echo "$IP" | grep -qE '^(10\.|172\.(1[6-9]|2[0-9]|3[01])\.|192\.168\.|127\.|169\.254\.)'; then
    continue
  fi

  # Probe the resolver
  if dig "@${IP}" "${CONTROL_DOMAIN}" A +time=${PROBE_TIMEOUT} +tries=${PROBE_RETRIES} +short &>/dev/null; then
    # Build a resolver entry
    SLUG=$(echo "${NAME}-${COUNTRY}" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9]/-/g' | sed 's/--*/-/g' | sed 's/^-//;s/-$//')
    ENTRY=$(jq -n \
      --arg id    "${SLUG}-${IP//./-}" \
      --arg name  "$NAME" \
      --arg ip    "$IP" \
      --arg country "$COUNTRY" \
      --arg city  "$CITY" \
      --argjson lat "$LAT" \
      --argjson lng "$LNG" \
      '{
        id:       $id,
        name:     $name,
        ip:       $ip,
        port:     53,
        protocol: "udp",
        country:  $country,
        city:     $city,
        lat:      $lat,
        lng:      $lng
      }')

    RESULTS=$(echo "$RESULTS" | jq --argjson e "$ENTRY" '. + [$e]')
    KEPT=$((KEPT + 1))

    if [[ $KEPT -ge $MAX_RESOLVERS ]]; then
      break
    fi
  else
    DROPPED=$((DROPPED + 1))
  fi
done < <(echo "$CANDIDATES" | jq -c '.[]')

###############################################################################
# Output
###############################################################################
echo ""
echo "Results:"
echo "  Kept:    $KEPT resolvers"
echo "  Dropped: $DROPPED resolvers (did not respond)"

if [[ "$DRY_RUN" == "true" ]]; then
  echo ""
  echo "DRY RUN: would write $KEPT resolvers to $OUTPUT_FILE"
  echo "$RESULTS" | jq '.' | head -50
  echo "..."
  exit 0
fi

echo "$RESULTS" | jq '.' > "$OUTPUT_FILE"
echo ""
echo "Written to $OUTPUT_FILE"
