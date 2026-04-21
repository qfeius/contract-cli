#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
NPM_CACHE="${NPM_CONFIG_CACHE:-/tmp/contract-cli-npm-cache}"
GO_CACHE="${GOCACHE:-/tmp/contract-cli-go-build-cache}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

PREFIX_DIR="$TMP_DIR/prefix"
CODEX_SKILLS_DIR="$TMP_DIR/codex-skills"
PACK_JSON="$TMP_DIR/pack.json"
ASSET_DIR="$TMP_DIR/assets"
ASSET_BUILD_DIR="$TMP_DIR/asset-build"

cd "$ROOT_DIR"

env npm_config_cache="$NPM_CACHE" npm pack --json > "$PACK_JSON"

read -r PACKAGE_FILE VERSION BINARY_NAME PLATFORM ARCHIVE_ARCH ARCHIVE_EXT < <(node - "$PACK_JSON" "$ROOT_DIR" <<'NODE'
const fs = require("fs");
const path = require("path");

const packPath = process.argv[2];
const rootDir = process.argv[3];
const pkg = require(path.join(rootDir, "package.json"));
const filename = JSON.parse(fs.readFileSync(packPath, "utf8"))[0].filename;
const platformMap = { darwin: "darwin", linux: "linux", win32: "windows" };
const archMap = { x64: "amd64", arm64: "arm64" };
const platform = platformMap[process.platform];
const arch = archMap[process.arch];
if (!platform || !arch) {
  throw new Error(`unsupported platform for local install check: ${process.platform}-${process.arch}`);
}
const binaryName = (pkg.config && pkg.config.binaryName) || "contract-cli";
const archiveExt = process.platform === "win32" ? ".zip" : ".tar.gz";
console.log([
  path.join(rootDir, filename),
  pkg.version,
  binaryName,
  platform,
  arch,
  archiveExt,
].join(" "));
NODE
)

cleanup_package() {
  rm -f "$PACKAGE_FILE"
}
trap 'cleanup_package; rm -rf "$TMP_DIR"' EXIT

mkdir -p "$ASSET_DIR" "$ASSET_BUILD_DIR" "$GO_CACHE"
env GOCACHE="$GO_CACHE" go build \
  -ldflags "-s -w -X cn.qfei/contract-cli/internal/build.Version=$VERSION" \
  -o "$ASSET_BUILD_DIR/$BINARY_NAME" \
  ./cmd/contract-cli

ARCHIVE_NAME="$BINARY_NAME-$VERSION-$PLATFORM-$ARCHIVE_ARCH$ARCHIVE_EXT"
if [ "$ARCHIVE_EXT" = ".zip" ]; then
  (cd "$ASSET_BUILD_DIR" && zip -q "$ASSET_DIR/$ARCHIVE_NAME" "$BINARY_NAME")
else
  LC_ALL=C tar -czf "$ASSET_DIR/$ARCHIVE_NAME" -C "$ASSET_BUILD_DIR" "$BINARY_NAME"
fi

env \
  CONTRACT_CLI_DOWNLOAD_BASE_URL_TEMPLATE="file://$ASSET_DIR" \
  npm_config_cache="$NPM_CACHE" \
  npm install -g "$PACKAGE_FILE" --prefix "$PREFIX_DIR"

"$PREFIX_DIR/bin/contract-cli" --version | grep -q "contract-cli version"
"$PREFIX_DIR/bin/contract-cli" skills list | grep -q "contract-cli-contract"
"$PREFIX_DIR/bin/contract-cli" skills install --target "$CODEX_SKILLS_DIR"

test -f "$CODEX_SKILLS_DIR/auth/SKILL.md"
test -f "$CODEX_SKILLS_DIR/contract-cli-contract/SKILL.md"
test -f "$CODEX_SKILLS_DIR/contract-cli-contract/references/create-contract-fields.md"

echo "local npm install ok: $PACKAGE_FILE"
