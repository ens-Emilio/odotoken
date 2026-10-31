#!/usr/bin/env bash
# Syncs the version from package.json into frontend/src/version.ts.
# Run after bumping package.json (manually or in CI) before building/releasing.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PKG="$ROOT/frontend/package.json"
SRC="$ROOT/frontend/src/version.ts"

if [ ! -f "$PKG" ]; then echo "error: $PKG not found" >&2; exit 1; fi
if [ ! -f "$SRC" ]; then echo "error: $SRC not found" >&2; exit 1; fi

VERSION=$(grep -o '"version": *"[^"]*"' "$PKG" | head -1 | sed 's/.*"version": *"\([^"]*\)".*/\1/')
if [ -z "$VERSION" ]; then echo "error: could not parse version from $PKG" >&2; exit 1; fi

sed -i "s/^export const APP_VERSION = .*/export const APP_VERSION = '$VERSION'/" "$SRC"
echo "synced APP_VERSION → $VERSION"