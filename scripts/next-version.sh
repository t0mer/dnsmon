#!/usr/bin/env bash
# Prints the next semver tag by bumping the patch of the latest vMAJOR.MINOR.PATCH
# git tag. Defaults to v0.1.0 when no tags exist.
set -euo pipefail

latest="$(git describe --tags --abbrev=0 2>/dev/null || true)"

if [ -z "$latest" ]; then
  echo "v0.1.0"
  exit 0
fi

ver="${latest#v}"
IFS='.' read -r major minor patch <<<"$ver"
major="${major:-0}"
minor="${minor:-0}"
patch="${patch:-0}"

echo "v${major}.${minor}.$((patch + 1))"
