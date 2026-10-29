#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

grep -q "go build -trimpath" "$ROOT_DIR/build.sh"
grep -q "go build -trimpath" "$ROOT_DIR/scripts/build-release-assets.sh"
grep -q "go build -trimpath" "$ROOT_DIR/tests/cli_e2e/smoke.sh"
grep -q "go build" "$ROOT_DIR/tests/release/local-install.sh"
grep -q -- "-trimpath" "$ROOT_DIR/tests/release/local-install.sh"
grep -q "go install -trimpath" "$ROOT_DIR/Makefile"

# prod/test 共用一套构建；旧版联调链接参数不得额外开启 dev/blue。
LEGACY_HELP_OUTPUT="$(cd "$ROOT_DIR" && go run -ldflags '-X cn.qfei/contract-cli/internal/cli.testBuild=true' ./cmd/contract-cli config add --help)"
if [[ "$LEGACY_HELP_OUTPUT" != *"--env <prod|test>"* || "$LEGACY_HELP_OUTPUT" == *"--env blue"* || "$LEGACY_HELP_OUTPUT" == *"--env dev"* ]]; then
  echo "legacy build flag changed the supported prod/test environments" >&2
  exit 1
fi

DEV_TAG_HELP_OUTPUT="$(cd "$ROOT_DIR" && go run -tags dev ./cmd/contract-cli config add --help)"
if [[ "$DEV_TAG_HELP_OUTPUT" != *"--env <prod|test>"* || "$DEV_TAG_HELP_OUTPUT" == *"--env blue"* || "$DEV_TAG_HELP_OUTPUT" == *"--env dev"* ]]; then
  echo "dev build tag changed the supported prod/test presets" >&2
  exit 1
fi

echo "build flags ok"
