#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

grep -q "go build -trimpath" "$ROOT_DIR/build.sh"
grep -q "go build -trimpath" "$ROOT_DIR/scripts/build-release-assets.sh"
grep -q "go build -trimpath" "$ROOT_DIR/tests/cli_e2e/smoke.sh"
grep -q "go build" "$ROOT_DIR/tests/release/local-install.sh"
grep -q -- "-trimpath" "$ROOT_DIR/tests/release/local-install.sh"
grep -q "go install -trimpath" "$ROOT_DIR/Makefile"

# 旧版联调链接参数即使被外部构建误传，也不得重新暴露非生产环境。
LEGACY_HELP_OUTPUT="$(cd "$ROOT_DIR" && go run -ldflags '-X cn.qfei/contract-cli/internal/cli.testBuild=true' ./cmd/contract-cli config add --help)"
if [[ "$LEGACY_HELP_OUTPUT" != *"--env <prod>"* || "$LEGACY_HELP_OUTPUT" == *"--env <prod|"* ]]; then
  echo "legacy build flag exposed a non-production environment" >&2
  exit 1
fi

echo "build flags ok"
