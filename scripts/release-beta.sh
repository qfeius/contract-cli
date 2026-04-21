#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
VERSION=""
PUBLISH=0
DRY_RUN=0
SKIP_TESTS=0
YES=0

REMOTE="${REMOTE:-origin}"
DEFAULT_BRANCH="$(git -C "$ROOT_DIR" branch --show-current 2>/dev/null || true)"
BRANCH="${BRANCH:-${DEFAULT_BRANCH:-main}}"
NPM_TAG="${NPM_TAG:-beta}"
NPM_REGISTRY="${NPM_REGISTRY:-https://registry.npmjs.org}"
GITHUB_REPOSITORY="${GITHUB_REPOSITORY:-qfeius/contract-cli}"
ASSET_DIR="${ASSET_DIR:-$ROOT_DIR/dist/release-assets}"

usage() {
  cat <<'EOF'
Usage:
  scripts/release-beta.sh --version <x.y.z-beta.n> [flags]

Flags:
  --version <version>   Required beta version, for example 0.1.1-beta.1.
  --publish             Push commit/tag, create GitHub Release, and publish npm.
  --yes                 Skip interactive confirmation when --publish is used.
  --dry-run             Print the release plan without changing files or publishing.
  --skip-tests          Skip make release-check. Useful only after a separate verified run.
  --help                Show this help.

Environment:
  GITHUB_TOKEN          Optional GitHub token. gh auth login also works.
  NPM_TOKEN             Optional npm automation/granular token. npm login also works.
  GITHUB_REPOSITORY     GitHub repo, default qfeius/contract-cli.
  NPM_REGISTRY          npm registry, default https://registry.npmjs.org.
  NPM_TAG               npm dist-tag, default beta.
  REMOTE                Git remote, default origin.
  BRANCH                Branch to push, default current branch.

Default mode updates package.json, runs release checks, builds release assets,
and stops before any remote publish. Use --publish --yes for one-command release.
EOF
}

die() {
  echo "release-beta: $*" >&2
  exit 1
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      [ "$#" -ge 2 ] || die "--version requires a value"
      VERSION="$2"
      shift 2
      ;;
    --publish)
      PUBLISH=1
      shift
      ;;
    --yes|-y)
      YES=1
      shift
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --skip-tests)
      SKIP_TESTS=1
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

[ -n "$VERSION" ] || die "--version is required"
if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+-beta\.[0-9]+$ ]]; then
  die "--version must look like 0.1.1-beta.1"
fi

TAG="v$VERSION"
EXPECTED_ARTIFACTS=(
  "checksums.txt"
  "contract-cli-$VERSION-darwin-amd64.tar.gz"
  "contract-cli-$VERSION-darwin-arm64.tar.gz"
  "contract-cli-$VERSION-linux-amd64.tar.gz"
  "contract-cli-$VERSION-linux-arm64.tar.gz"
  "contract-cli-$VERSION-windows-amd64.zip"
  "contract-cli-$VERSION-windows-arm64.zip"
)

quote_command() {
  printf '%q ' "$@"
  printf '\n'
}

run() {
  if [ "$DRY_RUN" -eq 1 ]; then
    printf '+ '
    quote_command "$@"
    return 0
  fi
  "$@"
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "$1 is required"
}

ensure_releasable_worktree() {
  local status
  status="$(git -C "$ROOT_DIR" status --porcelain)"
  if [ -z "$status" ]; then
    return
  fi

  if [ "$(current_package_version)" != "$VERSION" ]; then
    echo "release-beta: dirty files:" >&2
    echo "$status" >&2
    die "working tree is not clean; commit or stash local changes before release"
  fi

  while IFS= read -r line; do
    [ -n "$line" ] || continue
    local path
    path="${line:3}"
    case "$path" in
      package.json|package-lock.json) ;;
      *) die "working tree has unrelated change $path; commit or stash it before release" ;;
    esac
  done <<< "$status"
}

current_package_version() {
  node -p "require('$ROOT_DIR/package.json').version"
}

print_expected_artifacts() {
  echo "Expected GitHub Release assets:"
  for artifact in "${EXPECTED_ARTIFACTS[@]}"; do
    echo "  $artifact"
  done
}

asset_paths() {
  local paths=()
  for artifact in "${EXPECTED_ARTIFACTS[@]}"; do
    paths+=("$ASSET_DIR/$artifact")
  done
  printf '%s\n' "${paths[@]}"
}

check_expected_assets() {
  local missing=0
  for artifact in "${EXPECTED_ARTIFACTS[@]}"; do
    if [ ! -f "$ASSET_DIR/$artifact" ]; then
      echo "missing release asset: $ASSET_DIR/$artifact" >&2
      missing=1
    fi
  done
  [ "$missing" -eq 0 ] || exit 1
}

setup_npm_token() {
  if [ -z "${NPM_TOKEN:-}" ]; then
    return
  fi
  local npmrc_dir
  npmrc_dir="$(mktemp -d)"
  trap 'rm -rf "$npmrc_dir"' EXIT
  local registry_host
  registry_host="${NPM_REGISTRY#https://}"
  registry_host="${registry_host#http://}"
  registry_host="${registry_host%/}"
  {
    echo "registry=$NPM_REGISTRY"
    echo "//$registry_host/:_authToken=\${NPM_TOKEN}"
  } > "$npmrc_dir/.npmrc"
  export NPM_CONFIG_USERCONFIG="$npmrc_dir/.npmrc"
}

confirm_publish() {
  if [ "$PUBLISH" -ne 1 ] || [ "$YES" -eq 1 ]; then
    return
  fi
  printf 'About to publish %s to GitHub %s and npm tag %s. Continue? [y/N] ' "$TAG" "$GITHUB_REPOSITORY" "$NPM_TAG"
  read -r answer
  case "$answer" in
    y|Y|yes|YES) ;;
    *) die "release cancelled" ;;
  esac
}

print_plan() {
  echo "Release plan:"
  echo "  version: $VERSION"
  echo "  tag: $TAG"
  echo "  github: $GITHUB_REPOSITORY"
  echo "  npm: @qfeius/contract-cli@$NPM_TAG via $NPM_REGISTRY"
  echo
  print_expected_artifacts
  echo
  echo "Commands:"
  run npm version "$VERSION" --no-git-tag-version
  if [ "$SKIP_TESTS" -eq 0 ]; then
    run make release-check
  fi
  run make release-assets
  run git add package.json
  run git commit -m "release: prepare $VERSION"
  run git tag "$TAG"
  run git push "$REMOTE" "HEAD:$BRANCH"
  run git push "$REMOTE" "$TAG"
  run gh release create "$TAG" "dist/release-assets/*" --repo "$GITHUB_REPOSITORY" --title "$TAG" --notes "beta release $VERSION" --prerelease --latest=false
  run npm publish --tag "$NPM_TAG" --registry "$NPM_REGISTRY"
}

if [ "$DRY_RUN" -eq 1 ]; then
  print_plan
  exit 0
fi

cd "$ROOT_DIR"

require_command git
require_command node
require_command npm
require_command go
require_command zip
require_command make
if [ "$PUBLISH" -eq 1 ]; then
  require_command gh
fi

ensure_releasable_worktree
confirm_publish

CURRENT_VERSION="$(current_package_version)"
if [ "$CURRENT_VERSION" != "$VERSION" ]; then
  run npm version "$VERSION" --no-git-tag-version
else
  echo "package.json is already at version $VERSION"
fi

if [ "$SKIP_TESTS" -eq 0 ]; then
  run make release-check
else
  echo "skipping release checks"
fi

run make release-assets
check_expected_assets
print_expected_artifacts

if [ "$PUBLISH" -ne 1 ]; then
  cat <<EOF

Prepared $VERSION locally.
Next:
  git diff -- package.json
  scripts/release-beta.sh --version $VERSION --publish --yes
EOF
  exit 0
fi

if ! git diff --quiet -- package.json package-lock.json 2>/dev/null; then
  run git add package.json
  if [ -f package-lock.json ]; then
    run git add package-lock.json
  fi
  run git commit -m "release: prepare $VERSION"
else
  echo "no package version changes to commit"
fi

if git rev-parse "$TAG" >/dev/null 2>&1; then
  tag_commit="$(git rev-parse "$TAG^{commit}")"
  head_commit="$(git rev-parse HEAD)"
  if [ "$tag_commit" != "$head_commit" ]; then
    die "tag $TAG already exists at $tag_commit, but HEAD is $head_commit"
  fi
  echo "tag $TAG already exists at HEAD"
else
  run git tag "$TAG"
fi
run git push "$REMOTE" "HEAD:$BRANCH"
run git push "$REMOTE" "$TAG"

if [ -n "${GITHUB_TOKEN:-}" ] && [ -z "${GH_TOKEN:-}" ]; then
  export GH_TOKEN="$GITHUB_TOKEN"
fi

release_assets=()
while IFS= read -r path; do
  release_assets+=("$path")
done < <(asset_paths)

if gh release view "$TAG" --repo "$GITHUB_REPOSITORY" >/dev/null 2>&1; then
  run gh release upload "$TAG" "${release_assets[@]}" --repo "$GITHUB_REPOSITORY" --clobber
else
  run gh release create "$TAG" "${release_assets[@]}" \
    --repo "$GITHUB_REPOSITORY" \
    --title "$TAG" \
    --notes "beta release $VERSION" \
    --prerelease \
    --latest=false
fi

setup_npm_token
run npm whoami --registry "$NPM_REGISTRY"
run npm publish --tag "$NPM_TAG" --registry "$NPM_REGISTRY"

published_version="$(npm view @qfeius/contract-cli@"$NPM_TAG" version --registry "$NPM_REGISTRY")"
if [ "$published_version" != "$VERSION" ]; then
  die "npm @$NPM_TAG points to $published_version, want $VERSION"
fi

echo "Release completed: $TAG"
