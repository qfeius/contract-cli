#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
NPM_CACHE="${NPM_CONFIG_CACHE:-/tmp/contract-cli-npm-cache}"
PACK_JSON="$(mktemp)"
trap 'rm -f "$PACK_JSON"' EXIT

cd "$ROOT_DIR"

node --check scripts/install.js
node --check scripts/run.js

env npm_config_cache="$NPM_CACHE" npm pack --dry-run --json > "$PACK_JSON"

node - "$PACK_JSON" <<'NODE'
const fs = require("fs");

const packPath = process.argv[2];
const files = JSON.parse(fs.readFileSync(packPath, "utf8"))[0].files.map((file) => file.path);

const required = [
  "package.json",
  "scripts/install.js",
  "scripts/run.js",
  "README.md",
  "CHANGELOG.md",
  "LICENSE",
  "skills/auth/SKILL.md",
  "skills/contract-cli-shared/SKILL.md",
  "skills/contract-cli-contract/SKILL.md",
  "skills/contract-cli-contract/references/create-contract-fields.md",
  "skills/contract-cli-mdm-vendor/SKILL.md",
  "skills/contract-cli-mdm-legal/SKILL.md",
  "skills/contract-cli-mdm-fields/SKILL.md",
  "skills/contract-cli-api-call/SKILL.md",
];

const forbidden = [
  "skills/embed.go",
  "mcp.yaml",
  ".DS_Store",
  "contract-cli",
];

for (const file of required) {
  if (!files.includes(file)) {
    throw new Error(`npm pack is missing ${file}`);
  }
}

for (const file of forbidden) {
  if (files.includes(file)) {
    throw new Error(`npm pack should not include ${file}`);
  }
}

console.log(`npm pack dry-run ok: ${files.length} files`);
NODE
