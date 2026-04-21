#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
GO_CACHE="${GOCACHE:-/tmp/contract-cli-go-build-cache}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

VERSION="${VERSION:-$(git -C "$ROOT_DIR" describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo unknown)}"
DATE="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
LDFLAGS="-s -w -X cn.qfei/contract-cli/internal/build.Version=${VERSION} -X cn.qfei/contract-cli/internal/build.Commit=${COMMIT} -X cn.qfei/contract-cli/internal/build.Date=${DATE}"

cd "$ROOT_DIR"
mkdir -p "$GO_CACHE"
env GOCACHE="$GO_CACHE" go build -ldflags "$LDFLAGS" -o "$TMP_DIR/contract-cli" ./cmd/contract-cli

version_output="$("$TMP_DIR/contract-cli" --version)"
[[ "$version_output" == *"contract-cli version"* ]]

usage_output="$("$TMP_DIR/contract-cli")"
[[ "$usage_output" == *"contract-cli config add"* ]]
[[ "$usage_output" == *"contract-cli skills install"* ]]
[[ "$usage_output" == *"contract-cli update check"* ]]

skills_output="$("$TMP_DIR/contract-cli" skills list)"
[[ "$skills_output" == *"contract-cli-contract"* ]]

echo "smoke ok: $VERSION $COMMIT"
