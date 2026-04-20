#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

VERSION="${VERSION:-$(git -C "$ROOT_DIR" describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo unknown)}"
DATE="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
LDFLAGS="-s -w -X cn.qfei/contract-cli/internal/build.Version=${VERSION} -X cn.qfei/contract-cli/internal/build.Commit=${COMMIT} -X cn.qfei/contract-cli/internal/build.Date=${DATE}"

cd "$ROOT_DIR"
go build -ldflags "$LDFLAGS" -o "$TMP_DIR/contract-cli" ./cmd/contract-cli

"$TMP_DIR/contract-cli" --version | grep -q "contract-cli version"
"$TMP_DIR/contract-cli" | grep -q "contract-cli config add"

echo "smoke ok: $VERSION $COMMIT"
