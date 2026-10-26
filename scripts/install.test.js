"use strict";

const assert = require("node:assert/strict");
const crypto = require("node:crypto");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

const { verifyArchiveChecksum } = require("./install.js");
const { synchronizeSkills } = require("./install.js");
const { cliInstallSummary } = require("./install.js");

test("CLI summary reports actual binary path and keeps local installation scope explicit", () => {
  const binary = path.resolve("test prefix", "bin", "contract-cli");
  const global = cliInstallSummary(binary, "1.9.4-test.clm111796.9", { npm_config_global: "true" });
  assert.match(global, /CLI 安装：成功/);
  assert.ok(global.includes(binary));
  assert.ok(global.includes("1.9.4-test.clm111796.9"));
  assert.match(global, /不代表其他 CLI 副本已更新/);
  const local = cliInstallSummary(binary, "test", {});
  assert.match(local, /Skill 自动同步：未执行（项目内安装/);
  assert.doesNotMatch(global, /Skill 安装：成功/);
});

test("synchronization process failures explain why without claiming Skill success", () => {
  for (const result of [{ status: 1 }, { status: null, signal: "SIGTERM" }, { status: null, error: new Error("spawn denied") }]) {
    const warnings = [];
    synchronizeSkills("/package/bin/contract-cli", { npm_config_global: "true" }, () => result, value => warnings.push(value));
    const output = warnings.join("\n");
    assert.match(output, /CLI 已安装/);
    assert.match(output, /Skill 同步：失败或未完成/);
    assert.match(output, /失败原因：/);
    assert.match(output, /恢复结果：无法确认/);
    assert.match(output, /处理建议：/);
    assert.match(output, /不代表所有位置失败/);
  }
});

test("global installation synchronizes using the newly installed binary", () => {
  const calls = [];
  synchronizeSkills("/package/bin/contract-cli", { npm_config_global: "true" }, (...args) => { calls.push(args); return { status: 0 }; }, () => {});
  assert.equal(calls.length, 1);
  assert.equal(calls[0][0], "/package/bin/contract-cli");
  assert.deepEqual(calls[0][1], ["__internal", "skill-sync-on-install"]);
  assert.equal(calls[0][2].shell, false);
});

test("project installation and npx do not synchronize user skills", () => {
  for (const env of [{}, { npm_config_global: "false" }]) {
    synchronizeSkills("/package/bin/contract-cli", env, () => { throw new Error("must not execute"); }, () => {});
  }
});

test("skill failure is reported without failing the successful CLI installation", () => {
  const warnings = [];
  assert.doesNotThrow(() => synchronizeSkills("/package/bin/contract-cli", { npm_config_global: "true" }, () => ({ status: 1 }), value => warnings.push(value)));
  assert.match(warnings.join("\n"), /Skill/);
});

test("verifyArchiveChecksum accepts the published SHA-256", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "contract-cli-install-test-"));
  try {
    const archive = path.join(dir, "contract-cli-1.2.0-linux-amd64.tar.gz");
    fs.writeFileSync(archive, "trusted archive");
    const digest = crypto.createHash("sha256").update("trusted archive").digest("hex");
    const checksums = path.join(dir, "checksums.txt");
    fs.writeFileSync(checksums, `${digest}  ${path.basename(archive)}\n`);

    verifyArchiveChecksum(archive, checksums);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test("verifyArchiveChecksum rejects a mismatched archive", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "contract-cli-install-test-"));
  try {
    const archive = path.join(dir, "contract-cli-1.2.0-linux-amd64.tar.gz");
    fs.writeFileSync(archive, "tampered archive");
    const checksums = path.join(dir, "checksums.txt");
    fs.writeFileSync(checksums, `${"0".repeat(64)}  ${path.basename(archive)}\n`);

    assert.throws(() => verifyArchiveChecksum(archive, checksums), /checksum mismatch/);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});
