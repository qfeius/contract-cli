#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

grep -q "go build -trimpath" "$ROOT_DIR/build.sh"
grep -q "go build -trimpath" "$ROOT_DIR/scripts/build-release-assets.sh"
grep -q "go build -trimpath" "$ROOT_DIR/tests/cli_e2e/smoke.sh"
grep -q "go build" "$ROOT_DIR/tests/release/local-install.sh"
grep -q -- "-trimpath" "$ROOT_DIR/tests/release/local-install.sh"
grep -q "go install -trimpath" "$ROOT_DIR/Makefile"

# 正式源码仅内置 prod；旧版链接参数和 dev 标签均不得开启其他环境。
LEGACY_HELP_OUTPUT="$(cd "$ROOT_DIR" && go run -ldflags '-X cn.qfei/contract-cli/internal/cli.testBuild=true' ./cmd/contract-cli config add --help)"
if [[ "$LEGACY_HELP_OUTPUT" != *"--env <prod>"* || "$LEGACY_HELP_OUTPUT" == *"test"* || "$LEGACY_HELP_OUTPUT" == *"--env blue"* || "$LEGACY_HELP_OUTPUT" == *"--env dev"* ]]; then
  echo "legacy build flag exposed a non-production environment" >&2
  exit 1
fi

DEV_TAG_HELP_OUTPUT="$(cd "$ROOT_DIR" && go run -tags dev ./cmd/contract-cli config add --help)"
if [[ "$DEV_TAG_HELP_OUTPUT" != *"--env <prod>"* || "$DEV_TAG_HELP_OUTPUT" == *"test"* || "$DEV_TAG_HELP_OUTPUT" == *"--env blue"* || "$DEV_TAG_HELP_OUTPUT" == *"--env dev"* ]]; then
  echo "dev build tag exposed a non-production environment" >&2
  exit 1
fi

echo "build flags ok"
