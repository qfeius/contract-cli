"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const root = path.resolve(__dirname, "../..");
const read = name => fs.readFileSync(path.join(root, name), "utf8");

test("only GitHub Actions publishes; local build and test packaging remain", () => {
  for (const name of ["scripts/release.sh", "scripts/release-beta.sh", "tests/release/release-script.sh", "tests/release/release-beta-script.sh"]) {
    assert.equal(fs.existsSync(path.join(root, name)), false, name);
  }
  for (const name of ["scripts/build-release-assets.sh", "scripts/build-test-package.sh", "scripts/build-local-test-package.js"]) {
    assert.equal(fs.existsSync(path.join(root, name)), true, name);
  }
  for (const name of ["Makefile", "docs/current-production-release-process.md", "docs/npm-trusted-publishing.md"]) {
    assert.doesNotMatch(read(name), /scripts\/release(?:-beta)?\.sh/);
  }
});

test("release workflow gates publication and reuses a single asset build", () => {
  const workflow = read(".github/workflows/release.yml");
  assert.match(workflow, /pull_request:/);
  assert.match(workflow, /macos-latest/);
  assert.match(workflow, /ubuntu-latest/);
  assert.match(workflow, /windows-latest/);
  assert.match(workflow, /make release-check/);
  assert.match(workflow, /golangci-lint run --enable=exhaustive/);
  assert.match(workflow, /needs: \[checks, windows-security\]/);
  assert.match(workflow, /github\.ref_type == 'tag'/);
  assert.match(workflow, /github\.repository == 'qfeius\/contract-cli'/);
  assert.match(workflow, /release-metadata\.js/);
  assert.match(workflow, /steps\.metadata\.outputs\.channel/);
  assert.equal((workflow.match(/make release-assets/g) || []).length, 1);
  assert.equal((workflow.match(/npm publish/g) || []).length, 1);
  assert.doesNotMatch(workflow, /goreleaser|--clobber|contains\(github\.ref_name, 'beta'\)/);
});

test("local installation checks isolate real user credentials and Skill locations", () => {
  const script = read("tests/release/local-install.sh");
  for (const key of ["HOME", "CODEX_HOME", "CONTRACT_CLI_CONFIG_DIR"]) {
    assert.match(script, new RegExp("export " + key + "="), key);
  }
  assert.match(script, /pwd -P/);
});
