#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
VERSION_BEFORE="$(node -p "require('$ROOT_DIR/package.json').version")"
OUTPUT="$("$ROOT_DIR/scripts/release-beta.sh" --version 0.1.1-beta.1 --dry-run --skip-tests)"
VERSION_AFTER="$(node -p "require('$ROOT_DIR/package.json').version")"

if [ "$VERSION_BEFORE" != "$VERSION_AFTER" ]; then
  echo "dry-run changed package version: $VERSION_BEFORE -> $VERSION_AFTER" >&2
  exit 1
fi

printf '%s' "$OUTPUT" | grep -q "contract-cli-0.1.1-beta.1-darwin-amd64.tar.gz"
printf '%s' "$OUTPUT" | grep -q "contract-cli-0.1.1-beta.1-windows-arm64.zip"
printf '%s' "$OUTPUT" | grep -q "gh release create v0.1.1-beta.1"
printf '%s' "$OUTPUT" | grep -q "npm publish --tag beta"

"$ROOT_DIR/scripts/release-beta.sh" --help | grep -q "Usage:"

echo "release beta script dry-run ok"
