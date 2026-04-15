#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")"

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
DATE="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

LDFLAGS="-s -w \
  -X cn.qfei/contract-cli/internal/build.Version=${VERSION} \
  -X cn.qfei/contract-cli/internal/build.Commit=${COMMIT} \
  -X cn.qfei/contract-cli/internal/build.Date=${DATE}"

go build -ldflags "${LDFLAGS}" -o contract-cli ./cmd/contract-cli

echo "OK: ./contract-cli (${VERSION}, ${COMMIT}, ${DATE})"
