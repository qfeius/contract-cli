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
const { extractArchive } = require("./install.js");
const { execFileSync } = require("node:child_process");

test("Windows archive paths remain child environment data, never PowerShell source", () => {
  const calls = [];
  const run = (...args) => calls.push(args);
  for (const directory of ["C:\\Users\\O'Neil\\合同 tmp", "C:\\tmp\\quote’; & (1+1)"]) {
    const source = directory + "\\archive [1].zip";
    extractArchive(source, directory, "win32", run);
    const [command, args, options] = calls.at(-1);
    assert.equal(command, "powershell");
    assert.ok(args.includes("-NoProfile") && args.includes("-NonInteractive"));
    assert.equal(options.env.CONTRACT_CLI_ARCHIVE_PATH, source);
    assert.equal(options.env.CONTRACT_CLI_EXTRACT_DIR, directory);
    assert.ok(!args.some(arg => arg.includes(directory)));
    assert.match(args.at(-1), /ZipFile\]::ExtractToDirectory\(\$env:CONTRACT_CLI_ARCHIVE_PATH, \$env:CONTRACT_CLI_EXTRACT_DIR\)/);
  }
  assert.deepEqual(calls[0][1], calls[1][1]);
});

test("macOS and Linux keep their existing tar extraction arguments", () => {
  for (const platform of ["darwin", "linux"]) {
    const calls = [];
    extractArchive("/tmp/O'Neil/archive.tar.gz", "/tmp/合同 tmp", platform, (...args) => calls.push(args));
    assert.deepEqual(calls, [["tar", ["-xzf", "/tmp/O'Neil/archive.tar.gz", "-C", "/tmp/合同 tmp"], { stdio: "ignore" }]]);
  }
});

test("Windows PowerShell extracts a real archive under quoted Unicode paths", { skip: process.platform !== "win32" }, () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "contract-cli-O'Neil-合同 "));
  try {
    const source = path.join(root, "source [1].txt");
    const archive = path.join(root, "archive [1].zip");
    const target = path.join(root, "destination '测试’");
    fs.writeFileSync(source, "archive bytes");
    execFileSync("tar.exe", ["-a", "-cf", archive, "-C", root, path.basename(source)]);
    extractArchive(archive, target);
    assert.equal(fs.readFileSync(path.join(target, path.basename(source)), "utf8"), "archive bytes");
  } finally { fs.rmSync(root, { recursive: true, force: true }); }
});

test("Windows npm installs the bundled CLI under quoted Unicode paths", { skip: process.platform !== "win32", timeout: 240000 }, () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "contract-cli-npm-O'Neil-合同 "));
  try {
    const packageDir = path.join(root, "package");
    const assets = path.join(packageDir, "dist", "release-assets");
    const scripts = path.join(packageDir, "scripts");
    const home = path.join(root, "home");
    for (const dir of [assets, scripts, home]) fs.mkdirSync(dir, { recursive: true });
    const pkg = JSON.parse(fs.readFileSync(path.join(__dirname, "..", "package.json"), "utf8"));
    fs.writeFileSync(path.join(packageDir, "package.json"), JSON.stringify(pkg));
    for (const file of ["install.js", "run.js", "setup.js"]) fs.copyFileSync(path.join(__dirname, file), path.join(scripts, file));
    const binary = path.join(root, "contract-cli.exe");
    execFileSync("go", ["build", "-o", binary, "./cmd/contract-cli"], { cwd: path.join(__dirname, ".."), stdio: "pipe", timeout: 180000 });
    const arch = { x64: "amd64", arm64: "arm64" }[process.arch];
    const archiveName = `contract-cli-${pkg.version}-windows-${arch}.zip`;
    const archive = path.join(assets, archiveName);
    execFileSync("tar.exe", ["-a", "-cf", archive, "-C", root, path.basename(binary)]);
    const digest = crypto.createHash("sha256").update(fs.readFileSync(archive)).digest("hex");
    fs.writeFileSync(path.join(assets, "checksums.txt"), `${digest}  ${archiveName}\n`);
    const npm = path.join(path.dirname(process.execPath), "node_modules", "npm", "bin", "npm-cli.js");
    assert.ok(fs.existsSync(npm), "npm CLI must be installed with Node");
    const env = { ...process.env, HOME: home, USERPROFILE: home, TEMP: root, TMP: root, CODEX_HOME: path.join(home, "codex"), CONTRACT_CLI_CONFIG_DIR: path.join(home, "config"), CONTRACT_CLI_NO_UPDATE_CHECK: "1", npm_config_cache: path.join(root, "cache"), npm_config_userconfig: path.join(home, ".npmrc"), npm_config_update_notifier: "false" };
    const packed = JSON.parse(execFileSync(process.execPath, [npm, "pack", "--json", "--pack-destination", root], { cwd: packageDir, env, encoding: "utf8", timeout: 60000 }));
    const prefix = path.join(root, "installed '测试’");
    execFileSync(process.execPath, [npm, "install", path.join(root, packed[0].filename), "--prefix", prefix, "--global=false", "--no-audit", "--no-fund", "--foreground-scripts", "--ignore-scripts=false"], { cwd: root, env, stdio: "pipe", timeout: 60000 });
    const installed = path.join(prefix, "node_modules", ...pkg.name.split("/"), "bin", "contract-cli.exe");
    assert.match(execFileSync(installed, ["--version"], { env, encoding: "utf8" }), /contract-cli version/);
    assert.equal(fs.existsSync(path.join(env.CONTRACT_CLI_CONFIG_DIR, "skill-locations.json")), false);
  } finally { fs.rmSync(root, { recursive: true, force: true }); }
});

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
