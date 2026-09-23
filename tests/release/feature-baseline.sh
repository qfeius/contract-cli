#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

mkdir -p "$TMP_DIR/internal/cli"
if bash "$ROOT_DIR/scripts/verify-feature-baseline.sh" "$TMP_DIR" 1.8.4-test.20260920.1 >/dev/null 2>&1; then
  echo "feature baseline guard accepted a missing matrix source" >&2
  exit 1
fi

cat > "$TMP_DIR/internal/cli/approval_matrix_extensions.go" <<'EOF'
var commands = []string{
  "rule employee batch-get",
  "rule department search",
  "rule department batch-get",
  "rule role search",
  "rule role batch-get",
  "rule symbol query",
  "rule loop-function query",
  "rule table column patch",
  "rule table column preview",
}
EOF

bash "$ROOT_DIR/scripts/verify-feature-baseline.sh" "$TMP_DIR" 1.8.4-test.20260920.1
bash "$ROOT_DIR/scripts/verify-feature-baseline.sh" "$TMP_DIR" 1.8.3-test.1
echo "feature baseline guard ok"
