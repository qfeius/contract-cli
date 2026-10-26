"use strict";
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { resolvePackageTargets } = require("../../scripts/build-local-test-package.js");
const { runPackageVerification, assertIncreasingTestVersions, assertConfirmationGuidance } = require("../../scripts/verify-skill-guidance-packages.js");
const { assertInstallResultSummary } = require("../../scripts/verify-skill-guidance-packages.js");

test("merged customer guide preserves upstream structure and safe Skill upgrade instructions", () => {
  const guide = fs.readFileSync(path.join(__dirname, "../../README.md"), "utf8");
  assert.doesNotMatch(guide, /^(<<<<<<<|=======|>>>>>>>)/m);
  assert.match(guide, /### 1\. 安装 CLI 与 Agent skills/);
  assert.doesNotMatch(guide, /## Quick Start \(AI Agent\)/);
  const prompt = guide.slice(guide.indexOf("本次任务包含 CLI 升级"), guide.indexOf("也可以在终端中"));
  assert.ok(prompt.indexOf("先备份") >= 0 && prompt.indexOf("先备份") < prompt.indexOf("npm install -g"));
  assert.match(prompt, /全部有效的已记录/);
  assert.match(prompt, /--foreground-scripts/);
  assert.doesNotMatch(prompt, /--target "\$HOME\/\.workbuddy\/skills"/);
  assert.match(prompt, /手工补装成功说成自动同步成功/);
});

test("result summary acceptance requires identity, exact target and registration state", () => {
  const base = "CLI 实际可执行文件：/test/bin/contract-cli\n已登记的 Skill 目录：1\n文件更新不代表客户端已加载\n";
  const output = base + "Skill 安装：成功\nSkill 安装位置：/test/skills\n本次目录登记：成功";
  assert.doesNotThrow(() => assertInstallResultSummary(output, { operation: "install", target: "/test/skills", locations: 1 }));
  for (const value of ["added 1 package", output.replace("本次目录登记：成功", ""), output.replace("/test/skills", "/wrong"), output.replace("已登记的 Skill 目录：1", "已登记的 Skill 目录：0")]) {
    assert.throws(() => assertInstallResultSummary(value, { operation: "install", target: "/test/skills", locations: 1 }));
  }
});

test("new package acceptance rejects missing or late confirmation guidance", () => {
  const output = "CLI 已安装；配套 Skill 尚未安装或更新\n下一步：等待用户确认\n核实后向用户推荐，说明推荐理由；无法核实则停止。文字提示不是交互授权锁\nAfter confirmation only: command";
  assert.doesNotThrow(() => assertConfirmationGuidance(output));
  for (const value of ["", "added 1 package", output.replace("推荐理由", ""), output.replace("After confirmation only: command", ""), "After confirmation only: command\n" + output]) {
    assert.throws(() => assertConfirmationGuidance(value));
  }
});

test("package verification supports increasing local test revisions without pinning to .5 and .6", () => {
  for (const pair of [["1.9.4-test.clm111796.5", "1.9.4-test.clm111796.6"], ["1.9.4-test.clm111796.7", "1.9.4-test.clm111796.8"]]) {
    assert.doesNotThrow(() => assertIncreasingTestVersions(pair));
  }
  for (const pair of [[], ["1.9.4"], ["1.9.4", "1.9.5"], ["1.9.4-test.clm111796.8", "1.9.4-test.clm111796.7"], ["1.9.4-test.clm111796.7", "1.9.4-test.clm111796.7"], ["1.9.4-test.clm111796.7", "2.0.0-test.clm111796.8"]]) {
    assert.throws(() => assertIncreasingTestVersions(pair));
  }
});

test("Linux package verification refuses Windows before executing an installer", { skip: process.platform !== "win32" }, () => {
  let executed = false;
  assert.throws(() => runPackageVerification({ packages: [], execute: () => { executed = true; } }), /do not run Windows postinstall/);
  assert.equal(executed, false);
});

test("local test packaging defaults to the host and can explicitly bundle all six platforms", () => {
  const host = { win32: "windows", darwin: "darwin", linux: "linux" }[process.platform] + "/" + { x64: "amd64", arm64: "arm64" }[process.arch];
  assert.deepEqual(resolvePackageTargets(), [host]);
  assert.deepEqual(resolvePackageTargets("all"), ["darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64"]);
  const input = ["linux/amd64", "windows/amd64"];
  assert.deepEqual(resolvePackageTargets(input), input);
  assert.notEqual(resolvePackageTargets(input), input);
});

test("local test packaging rejects invalid, empty and duplicate platform selections", () => {
  for (const value of [[], ["windows/386"], ["linux/amd64", "linux/amd64"], "unknown", ["../../arbitrary"]]) {
    assert.throws(() => resolvePackageTargets(value), /platform/);
  }
});

test("existing test-package staging includes the setup executable declared by npm", () => {
  const script = fs.readFileSync(path.join(__dirname, "../../scripts/build-test-package.sh"), "utf8");
  const copy = script.split(/\r?\n/).find(line => line.startsWith("cp ") && line.includes("install.js"));
  assert.ok(copy.includes("setup.js"), "test package omits contract-cli-setup");
});
