#!/usr/bin/env bash

# 构建独立预发布 npm 包；版本须带 -test 标记，但环境能力与正式包一样仅 prod。
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${VERSION:-$(node -p "require('$ROOT_DIR/package.json').version")-test.1}"
PACKAGE_OUT_DIR="${PACKAGE_OUT_DIR:-$ROOT_DIR/dist}"
if [[ "$VERSION" != *-test.* ]]; then
  echo "test package VERSION must contain -test. (for example 1.8.3-test.1)" >&2
  exit 1
fi
STAGING_DIR="$(mktemp -d "${TMPDIR:-/tmp}/contract-cli-test-package.XXXXXX")"
trap 'rm -rf "$STAGING_DIR"' EXIT
mkdir -p "$STAGING_DIR/scripts" "$PACKAGE_OUT_DIR"
cp "$ROOT_DIR/package.json" "$ROOT_DIR/README.md" "$ROOT_DIR/CHANGELOG.md" "$ROOT_DIR/LICENSE" "$STAGING_DIR/"
cp "$ROOT_DIR/scripts/install.js" "$ROOT_DIR/scripts/run.js" "$STAGING_DIR/scripts/"
cp -R "$ROOT_DIR/skills" "$STAGING_DIR/skills"
# 仅生成暂存区的包元数据，源码 package.json 与正式版本保持不变。
node - "$STAGING_DIR/package.json" "$VERSION" <<'NODE'
const fs = require("fs");
const [manifestPath, version] = process.argv.slice(2);
const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
manifest.version = version;
fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + "\n");
NODE
cd "$ROOT_DIR"
VERSION="$VERSION" OUT_DIR="$STAGING_DIR/dist/release-assets" bash scripts/build-release-assets.sh
(cd "$STAGING_DIR/dist/release-assets" && LC_ALL=C shasum -a 256 -c checksums.txt)
cd "$STAGING_DIR"
npm pack --pack-destination "$PACKAGE_OUT_DIR"
