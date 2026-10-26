"use strict";

// Linux-only package acceptance in a temporary prefix/config/CODEX_HOME.
// Never execute native Windows postinstall against the primary account.
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const crypto = require("node:crypto");
const assert = require("node:assert/strict");
const { spawnSync } = require("node:child_process");

function assertIncreasingTestVersions(versions) {
  assert.ok(Array.isArray(versions) && versions.length === 2, "two increasing local test versions required");
  const parts = versions.map(version => {
    assert.equal(typeof version, "string");
    const match = /^(\d+\.\d+\.\d+)-test\.clm111796\.(\d+)$/.exec(version);
    assert.ok(match, "only reviewed CLM-111796 test versions are supported");
    return match;
  });
  assert.equal(parts[0][1], parts[1][1], "same test baseline required");
  assert.ok(BigInt(parts[1][2]) > BigInt(parts[0][2]), "test revision must increase");
}

function assertConfirmationGuidance(output) {
  for (const message of ["CLI 已安装；配套 Skill 尚未安装或更新", "下一步：等待用户确认", "核实后向用户推荐", "推荐理由", "无法核实则停止", "文字提示不是交互授权锁"]) {
    assert.ok(output.includes(message), "missing confirmation guidance: " + message);
  }
  const confirmation = output.indexOf("下一步：等待用户确认");
  const command = output.indexOf("After confirmation only:");
  assert.ok(command > confirmation, "confirmation must precede candidate commands");
}

function assertInstallResultSummary(output, { operation, target, locations }) {
  for (const text of ["CLI 实际可执行文件：", `已登记的 Skill 目录：${locations}`, "文件更新不代表客户端已加载"]) {
    assert.ok(output.includes(text), "missing result summary: " + text);
  }
  if (operation === "first") {
    assert.ok(output.includes("Skill 自动同步：未执行（没有可同步位置）"));
  } else if (operation === "install") {
    for (const text of ["Skill 安装：成功", "Skill 安装位置：" + target, "本次目录登记：成功"]) assert.ok(output.includes(text), "missing install result: " + text);
  } else if (operation === "upgrade") {
    assert.ok(output.includes("Skill 自动同步：成功"));
    assert.ok(output.includes("同步成功：" + target));
  } else {
    throw new Error("unknown summary operation");
  }
}

function runPackageVerification({ packages, evidenceRoot, execute = spawnSync }) {

  console.log("开始隔离 Linux 包验证：尚未执行安装。");
  if (process.platform !== "linux") throw new Error("Use an isolated Linux environment; do not run Windows postinstall in the primary account");
  if (!Array.isArray(packages) || packages.length !== 2) throw new Error("two increasing local test packages required");
  const verified = packages.map(file => {
    const manifest = JSON.parse(fs.readFileSync(file + ".manifest.json", "utf8"));
    assert.equal(manifest.published, false);
    assert.deepEqual(manifest.targets, ["darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64"]);
    assert.equal(crypto.createHash("sha256").update(fs.readFileSync(file)).digest("hex"), manifest.packageSha256);
    return { file: path.resolve(file), manifest };
  });
  assertIncreasingTestVersions(verified.map(item => item.manifest.version));
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "contract-cli-guidance-verification-"));
  const evidence = evidenceRoot ? path.resolve(evidenceRoot) : path.join(root, "evidence");
  fs.mkdirSync(evidence, { recursive: true });
  const env = { ...process.env, PATH: [path.dirname(process.execPath), process.env.PATH || ""].join(path.delimiter), npm_config_cache: path.join(root, "npm-cache"), npm_config_update_notifier: "false", CONTRACT_CLI_CONFIG_DIR: path.join(root, "config"), CODEX_HOME: path.join(root, "configured-codex"), CONTRACT_CLI_NO_UPDATE_CHECK: "1" };
  const npmCli = path.resolve(path.dirname(process.execPath), "../lib/node_modules/npm/bin/npm-cli.js");
  assert.ok(fs.existsSync(npmCli), "Linux npm CLI unavailable");
  const prefix = path.join(root, "npm");
  const entry = path.join(prefix, "lib/node_modules/@qfeius/contract-cli/scripts/run.js");
  const registryPath = path.join(env.CONTRACT_CLI_CONFIG_DIR, "skill-locations.json");
  const target = path.join(env.CODEX_HOME, "skills");
  const report = { startedAt: new Date().toISOString(), root, evidence, platform: process.platform, node: process.version, packages: verified.map(x => x.manifest), commands: [], cases: [], notIncluded: ["Windows primary-account npm postinstall", "macOS actual execution", "ARM64 actual execution", "AI client loading", "client private configuration adapters"] };
  function command(name, executable, args, combineOutput = false) {
    const result = execute(executable, args, { cwd: root, env, encoding: "utf8", timeout: 120000, maxBuffer: 16 * 1024 * 1024 });
    fs.writeFileSync(path.join(evidence, name + ".stdout.log"), result.stdout || "");
    fs.writeFileSync(path.join(evidence, name + ".stderr.log"), result.stderr || "");
    report.commands.push({ name, executable, args, status: result.status, signal: result.signal, error: result.error ? result.error.message : null });
    if (result.error || result.status !== 0) throw new Error(`${name} failed: ${result.error ? result.error.message : result.stderr}`);
    return combineOutput ? (result.stdout || "") + "\n" + (result.stderr || "") : result.stdout;
  }
  function verifySkills(version, expectedLocations = 1) {
    const registry = JSON.parse(fs.readFileSync(registryPath, "utf8"));
    assert.equal(registry.locations.length, expectedLocations);
    const location = registry.locations.find(item => item.target === target);
    assert.equal(location.target, target);
    assert.equal(location.status, "success");
    assert.equal(location.version, version);
    let files = 0;
    const source = path.join(prefix, "lib/node_modules/@qfeius/contract-cli/skills");
    const bundledNames = fs.readdirSync(source, { withFileTypes: true })
      .filter(item => item.isDirectory() && fs.existsSync(path.join(source, item.name, "SKILL.md")))
      .map(item => item.name)
      .sort();
    assert.deepEqual([...location.managed_skills].sort(), bundledNames, "registered bundled Skill set differs from package source");
    function walk(from, to) {
      for (const entry of fs.readdirSync(from, { withFileTypes: true })) {
        const a = path.join(from, entry.name), b = path.join(to, entry.name);
        if (entry.isDirectory()) walk(a, b);
        else {
          let expected = fs.readFileSync(a);
          if (entry.name === "SKILL.md") {
            const lines = expected.toString("utf8").split("\n");
            for (let i = 1; i < lines.length; i++) {
              if (lines[i].trim() === "---") { lines.splice(i, 0, "version: " + version); break; }
              if (lines[i].split(":")[0].trim() === "version") { lines[i] = "version: " + version; break; }
            }
            expected = Buffer.from(lines.join("\n"));
          }
          assert.ok(expected.equals(fs.readFileSync(b)), "different Skill file: " + b);
          files++;
        }
      }
      for (const item of fs.readdirSync(to)) assert.ok(fs.existsSync(path.join(from, item)), "unexpected official file: " + item);
    }
    for (const name of location.managed_skills) walk(path.join(source, name), path.join(target, name));
    return { files, skills: location.managed_skills.length };
  }
  try {
    const npmArgs = file => [npmCli, "install", "-g", file, "--prefix", prefix, "--foreground-scripts", "--no-audit", "--no-fund"];
    const first = command("01-install-dot5", process.execPath, npmArgs(verified[0].file));
    assert.match(first, /installed from bundled npm assets/);
    assert.match(first, /not necessarily a new user/);
    assert.match(first, /CODEX_HOME/);
    assert.match(first, /confirmation required/);
    // Earlier .5/.6 evidence remains valid; .7 introduced the stronger guidance.
    if (BigInt(verified[0].manifest.version.split(".").at(-1)) >= 7n) assertConfirmationGuidance(first);
    const summaryEnabled = BigInt(verified[0].manifest.version.split(".").at(-1)) >= 9n;
    if (summaryEnabled) assertInstallResultSummary(first, { operation: "first", locations: 0 });
    assert.equal(fs.existsSync(target), false);
    assert.equal(fs.existsSync(registryPath), false);
    report.cases.push({ name: "Linux npm 首次安装只提示候选，不创建 Skill 或登记", status: "PASS" });
    const installed = command("02-confirmed-full-install", process.execPath, [entry, "skills", "install", "--target", target, "--force"]);
    if (summaryEnabled) assertInstallResultSummary(installed, { operation: "install", target, locations: 1 });
    report.cases.push({ name: `确认后完整安装 ${verified[0].manifest.version}，14 个 Skill 及附属文件与内置来源一致`, status: "PASS", ...verifySkills(verified[0].manifest.version) });
    fs.writeFileSync(path.join(target, "auth/SKILL.md"), "local modified content");
    fs.writeFileSync(path.join(target, "auth/local-extra.txt"), "remove");
    fs.mkdirSync(path.join(target, "independent-third-party"));
    fs.writeFileSync(path.join(target, "independent-third-party/SKILL.md"), "keep");
    const upgrade = command("03-upgrade-dot6", process.execPath, npmArgs(verified[1].file));
    assert.match(upgrade, /Skill location .*: success/);
    if (summaryEnabled) assertInstallResultSummary(upgrade, { operation: "upgrade", target, locations: 1 });
    assert.equal(fs.existsSync(path.join(target, "auth/local-extra.txt")), false);
    assert.equal(fs.readFileSync(path.join(target, "independent-third-party/SKILL.md"), "utf8"), "keep");
    report.cases.push({ name: `Linux npm ${verified[0].manifest.version} 到 ${verified[1].manifest.version} 自动同步、覆盖修改、清除旧文件、保留第三方`, status: "PASS", ...verifySkills(verified[1].manifest.version) });
    const version = command("04-version", process.execPath, [entry, "--version"]);
    assert.ok(version.includes("contract-cli version " + verified[1].manifest.version));
    const help = command("05-contract-help", process.execPath, [entry, "contract", "--help"]);
    assert.match(help, /search/);
    report.cases.push({ name: "Linux " + verified[1].manifest.version + " 版本与合同帮助只读验证", status: "PASS" });
    if (summaryEnabled) {
      const missing = path.join(root, "second-client", "skills");
      const second = command("06-register-second-target", process.execPath, [entry, "skills", "install", "--target", missing, "--force"]);
      assertInstallResultSummary(second, { operation: "install", target: missing, locations: 2 });
      fs.renameSync(missing, missing + "-manual-backup");
      fs.writeFileSync(path.join(target, "auth/SKILL.md"), "partial-sync-modified");
      const partial = command("07-npm-partial-failure", process.execPath, npmArgs(verified[1].file), true);
      for (const text of ["CLI 安装：成功", "Skill 自动同步：部分失败", "同步成功：" + target, "同步失败：" + missing, "已登记的 Skill 目录：2", "记录状态：invalid", "失败原因：", "恢复结果：无法确认", "处理建议：", "Skill 同步：失败或未完成"]) {
        assert.ok(partial.includes(text), "partial failure result missing: " + text);
      }
      assert.equal(fs.existsSync(missing), false);
      const bytes = verifySkills(verified[1].manifest.version, 2);
      report.cases.push({ name: "Linux npm 部分失败：CLI 成功、两个登记位置逐项结果、原因/恢复/处理提示，缺失目录不重建", status: "PASS", ...bytes });
      const registryBefore = fs.readFileSync(registryPath);
      fs.writeFileSync(path.join(evidence, "registry-before-corrupt-test.json"), registryBefore);
      const skillHashBefore = crypto.createHash("sha256").update(fs.readFileSync(path.join(target, "auth/SKILL.md"))).digest("hex");
      fs.writeFileSync(registryPath, "intentional corrupt registry in isolated fixture");
      const corrupt = command("08-npm-corrupt-registry", process.execPath, npmArgs(verified[1].file), true);
      for (const text of ["CLI 安装：成功", "Skill 自动同步：失败", "已登记的 Skill 目录：读取失败", "失败原因：", "Skill 同步：失败或未完成"]) assert.ok(corrupt.includes(text), "corrupt registry result missing: " + text);
      assert.ok(!corrupt.includes("已登记的 Skill 目录：0"));
      assert.equal(crypto.createHash("sha256").update(fs.readFileSync(path.join(target, "auth/SKILL.md"))).digest("hex"), skillHashBefore);
      fs.writeFileSync(registryPath, registryBefore);
      report.cases.push({ name: "Linux npm 登记损坏：明确读取失败，不伪报空登记或 Skill 成功，不修改 Skill；测试记录已恢复", status: "PASS" });
    }
    report.status = "PASS";
  } catch (error) {
    report.status = "FAIL";
    report.error = error.message;
    console.error("Linux package verification failed: " + error.message);
    throw error;
  } finally {
    report.finishedAt = new Date().toISOString();
    fs.writeFileSync(path.join(evidence, "report.json"), JSON.stringify(report, null, 2) + "\n");
    console.log(`Package verification evidence: ${evidence}`);
    // Preserve temporary installation and raw logs for review; no recursive deletion.
  }
  return report;
}

if (require.main === module) {
  try {
    runPackageVerification({ packages: process.argv.slice(2, 4), evidenceRoot: process.argv[4] });
  } catch (error) {
    console.error("Package verification stopped: " + error.message);
    process.exitCode = 1;
  }
}
module.exports = { runPackageVerification, assertIncreasingTestVersions, assertConfirmationGuidance, assertInstallResultSummary };
