#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="${1:?root directory is required}"
VERSION="${2:?version is required}"

# 1.8.4 起必须携带审批矩阵扩展，阻止发布分支用更高版本号覆盖旧功能基线。
# version_requires_matrix_extensions 判断版本是否需要审批矩阵扩展。
# 入参 $1（字符串）为待检查的版本号；返回码 0 表示需要检查，1 表示无需检查。
version_requires_matrix_extensions() {
  local base_version="${1#v}"
  base_version="${base_version%%-*}"
  if [[ ! "$base_version" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    return 1
  fi
  local major=$((10#${BASH_REMATCH[1]}))
  local minor=$((10#${BASH_REMATCH[2]}))
  local patch=$((10#${BASH_REMATCH[3]}))
  ((major > 1 || (major == 1 && (minor > 8 || (minor == 8 && patch >= 4)))))
}

if version_requires_matrix_extensions "$VERSION"; then
  feature_source="$ROOT_DIR/internal/cli/approval_matrix_extensions.go"
  if [[ ! -f "$feature_source" ]]; then
    echo "VERSION $VERSION requires approval-matrix extensions, but the source is missing" >&2
    exit 1
  fi

  for command in \
    'rule employee batch-get' \
    'rule department search' \
    'rule department batch-get' \
    'rule role search' \
    'rule role batch-get' \
    'rule symbol query' \
    'rule loop-function query' \
    'rule table column patch' \
    'rule table column preview'; do
    if ! grep -Fq "\"$command\"" "$feature_source"; then
      echo "VERSION $VERSION requires feature command: $command" >&2
      exit 1
    fi
  done
fi
