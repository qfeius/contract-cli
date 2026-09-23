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

assert_contains() {
  if [[ "$1" != *"$2"* ]]; then
    echo "smoke assertion failed: expected $2" >&2
    exit 1
  fi
}

cd "$ROOT_DIR"
bash "$ROOT_DIR/scripts/verify-feature-baseline.sh" "$ROOT_DIR" "$VERSION"
mkdir -p "$GO_CACHE"
env GOCACHE="$GO_CACHE" go build -trimpath -ldflags "$LDFLAGS" -o "$TMP_DIR/contract-cli" ./cmd/contract-cli

version_output="$("$TMP_DIR/contract-cli" --version)"
assert_contains "$version_output" "contract-cli version"

usage_output="$("$TMP_DIR/contract-cli")"
assert_contains "$usage_output" "contract-cli config add"
assert_contains "$usage_output" "contract-cli skills install"
assert_contains "$usage_output" "contract-cli update check [flags]"
assert_contains "$usage_output" "contract-cli environment inspect"

environment_output="$("$TMP_DIR/contract-cli" environment inspect --output json)"
assert_contains "$environment_output" '"channel_type": "cli"'
assert_contains "$environment_output" '"agent_source_type":'
assert_contains "$environment_output" '"detector_version": "process-ancestry-v6"'

skills_output="$("$TMP_DIR/contract-cli" skills list)"
assert_contains "$skills_output" "contract-cli-contract"
assert_contains "$skills_output" "contract-cli-employee"
assert_contains "$skills_output" "contract-cli-department"

echo "smoke ok: $VERSION $COMMIT"
