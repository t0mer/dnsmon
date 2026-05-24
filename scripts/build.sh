#!/usr/bin/env bash
# Cross-compiles dnsmon into release archives under dist/.
# Assumes the embedded frontend (web/dist) has already been built.
#
#   VERSION     version string baked into the binary (default: dev)
#   BUILD_MODE  "prod" builds all release targets; anything else builds only
#               the host platform (default: dev)
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${VERSION:-dev}"
BUILD_MODE="${BUILD_MODE:-dev}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
MODULE="github.com/t0mer/dnsmon"
LDFLAGS="-s -w \
  -X ${MODULE}/internal/version.Version=${VERSION} \
  -X ${MODULE}/internal/version.Commit=${COMMIT} \
  -X ${MODULE}/internal/version.Date=${DATE}"

OUT="dist"
rm -rf "$OUT"
mkdir -p "$OUT"

build() {
  local goos="$1" goarch="$2" goarm="${3:-}"
  local suffix="$goarch"
  [ -n "$goarm" ] && suffix="${goarch}v${goarm}"
  local name="dnsmon_${VERSION}_${goos}_${suffix}"
  local bin="dnsmon"
  [ "$goos" = "windows" ] && bin="dnsmon.exe"

  local dir="$OUT/$name"
  mkdir -p "$dir"
  echo "==> building $name"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" GOARM="$goarm" \
    go build -trimpath -ldflags="$LDFLAGS" -o "$dir/$bin" ./cmd/dnsmon

  cp README.md LICENSE "$dir/" 2>/dev/null || true

  if [ "$goos" = "windows" ]; then
    (cd "$OUT" && zip -qr "${name}.zip" "$name")
  else
    tar -C "$OUT" -czf "${OUT}/${name}.tar.gz" "$name"
  fi
  rm -rf "$dir"
}

if [ "$BUILD_MODE" = "prod" ]; then
  build linux amd64
  build linux arm64
  build linux arm 7
  build darwin amd64
  build darwin arm64
  build windows amd64
else
  build "$(go env GOOS)" "$(go env GOARCH)"
fi

echo "==> artifacts:"
ls -1 "$OUT"
