#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BINARY_NAME="${BINARY_NAME:-contract-cli}"
VERSION="${VERSION:-$(node -p "require('./package.json').version")}"
OUT_DIR="${OUT_DIR:-$ROOT_DIR/dist/release-assets}"
GO_CACHE="${GOCACHE:-/tmp/contract-cli-go-build-cache}"
COMMIT="${COMMIT:-$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo unknown)}"
DATE="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
LDFLAGS="-s -w -X cn.qfei/contract-cli/internal/build.Version=${VERSION} -X cn.qfei/contract-cli/internal/build.Commit=${COMMIT} -X cn.qfei/contract-cli/internal/build.Date=${DATE}"

TARGETS=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
  "windows/arm64"
)

if ! command -v go >/dev/null 2>&1; then
  echo "go toolchain not found" >&2
  exit 1
fi

if ! command -v node >/dev/null 2>&1; then
  echo "node not found" >&2
  exit 1
fi

if ! command -v zip >/dev/null 2>&1; then
  echo "zip not found" >&2
  exit 1
fi

mkdir -p "$OUT_DIR" "$GO_CACHE"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

artifacts=()
for target in "${TARGETS[@]}"; do
  goos="${target%/*}"
  goarch="${target#*/}"
  build_dir="$TMP_DIR/$goos-$goarch"
  mkdir -p "$build_dir"

  binary_file="$BINARY_NAME"
  archive_ext=".tar.gz"
  if [ "$goos" = "windows" ]; then
    binary_file="$BINARY_NAME.exe"
    archive_ext=".zip"
  fi

  archive_name="$BINARY_NAME-$VERSION-$goos-$goarch$archive_ext"
  archive_path="$OUT_DIR/$archive_name"

  echo "building $archive_name"
  env \
    CGO_ENABLED=0 \
    GOOS="$goos" \
    GOARCH="$goarch" \
    GOCACHE="$GO_CACHE" \
    go build -ldflags "$LDFLAGS" -o "$build_dir/$binary_file" ./cmd/contract-cli

  if [ "$goos" = "windows" ]; then
    (cd "$build_dir" && zip -q "$archive_path" "$binary_file")
  else
    LC_ALL=C tar -czf "$archive_path" -C "$build_dir" "$binary_file"
  fi

  artifacts+=("$archive_name")
done

checksums_path="$OUT_DIR/checksums.txt"
: > "$checksums_path"
for artifact in "${artifacts[@]}"; do
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$OUT_DIR" && sha256sum "$artifact") >> "$checksums_path"
  elif command -v openssl >/dev/null 2>&1; then
    (cd "$OUT_DIR" && openssl dgst -sha256 -r "$artifact") >> "$checksums_path"
  else
    (cd "$OUT_DIR" && LC_ALL=C shasum -a 256 "$artifact") >> "$checksums_path"
  fi
done

echo "release assets written to $OUT_DIR"
