"use strict";

const assert = require("node:assert/strict");
const { spawnSync } = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");
const { releaseMetadata } = require("./release-metadata.js");

test("release tags select stable or prerelease channels without shell input", () => {
  assert.deepEqual(releaseMetadata("v1.10.0"), { version: "1.10.0", channel: "latest", prerelease: false });
  for (const suffix of ["beta.1", "rc.2", "alpha.0", "preview-test.3"]) {
    assert.deepEqual(releaseMetadata("v1.10.0-" + suffix), { version: "1.10.0-" + suffix, channel: "beta", prerelease: true });
  }
});

test("invalid, ambiguous and unversioned tags cannot publish", () => {
  for (const tag of ["main", "test", "1.2.3", "v01.2.3", "v1.2", "v1.2.3-", "v1.2.3-01", "v1.2.3-rc.01", "v1.2.3+build", "v1.2.3\n", "v1.2.3;echo bad"]) {
    assert.throws(() => releaseMetadata(tag), /tag/i, tag);
  }
});

test("Actions metadata output rejects injected lines before writing any output", t => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "release-metadata-"));
  t.after(() => fs.rmSync(dir, { recursive: true, force: true }));
  const output = path.join(dir, "github-output");
  const run = tag => spawnSync(process.execPath, [path.join(__dirname, "release-metadata.js"), tag], {
    encoding: "utf8", env: { ...process.env, GITHUB_OUTPUT: output },
  });
  const invalid = run("v1.10.0\nchannel=latest");
  assert.equal(invalid.status, 1);
  assert.equal(invalid.stdout, "");
  assert.equal(fs.existsSync(output), false);
  for (const [tag, channel, prerelease] of [["v1.10.0", "latest", false], ["v1.10.0-rc.1", "beta", true]]) {
    fs.writeFileSync(output, "");
    const valid = run(tag);
    assert.equal(valid.status, 0, valid.stderr);
    assert.deepEqual(JSON.parse(valid.stdout), { version: tag.slice(1), channel, prerelease });
    assert.equal(fs.readFileSync(output, "utf8"), `version=${tag.slice(1)}\nchannel=${channel}\nprerelease=${prerelease}\n`);
  }
});
