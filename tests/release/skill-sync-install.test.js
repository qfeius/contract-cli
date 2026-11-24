"use strict";

const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { execFileSync } = require("node:child_process");

// Native account lookup ignores CODEX_HOME/USERPROFILE. A temporary npm prefix
// is not isolation from real WorkBuddy Skills; never touch these in a test.
const historicalClientSafetySkip = process.platform === "win32" && fs.existsSync(path.join(os.userInfo().homedir, ".workbuddy", "skills"))
  ? "Real Windows WorkBuddy Skills exist; use an isolated OS account for native tgz installation tests"
  : false;

// Supply official npm pack tarballs; the upgrade itself needs no registry or real client.
test("official 1.9.5 in place and 1.9.6/7 unregistered migrations via npm", {
  timeout: 240000, skip: historicalClientSafetySkip || !process.env.CONTRACT_CLI_OFFICIAL_TGZ_DIR,
}, () => {
  const { buildLocalTestPackage, runNpm } = require("../../scripts/build-local-test-package.js");
  const temp = fs.realpathSync(fs.mkdtempSync(path.join(os.tmpdir(), "cli-official-upgrade-")));
  const version = "2.0.0-test.skill-upgrade";
  try {
    const current = buildLocalTestPackage({ output: temp, version });
    const major = Number(runNpm(["--version"], { encoding: "utf8" }).split(".")[0]);
    for (const scenario of ["1.9.5", "1.9.6", "1.9.7", "edited-1.9.7"]) {
      const old = scenario.replace("edited-", "");
      const base = path.join(temp, scenario);
      const prefix = path.join(base, "npm");
      const env = { ...process.env, HOME: base, CODEX_HOME: path.join(base, "codex"),
        CONTRACT_CLI_CONFIG_DIR: path.join(base, "config"), CONTRACT_CLI_NO_UPDATE_CHECK: "1" };
      const opts = { env, cwd: temp, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] };
      const install = pkg => runNpm(["install", "-g", pkg, "--prefix", prefix, "--offline",
        "--foreground-scripts", "--no-audit", "--no-fund", ...(major >= 12 ? ["--dangerously-allow-all-scripts"] : [])], opts);
      const entry = path.join(prefix, ...(process.platform === "win32" ? [] : ["lib"]), "node_modules/@qfeius/contract-cli/scripts/run.js");
      const cli = args => execFileSync(process.execPath, [entry, ...args], opts);
      install(path.resolve(process.env.CONTRACT_CLI_OFFICIAL_TGZ_DIR, `qfeius-contract-cli-${old}.tgz`));
      const original = path.join(env.CODEX_HOME, "skills");
      cli(["skills", "install", "--target", original, "--force"]);
      let target = original;
      if (old !== "1.9.5") {
        env.CONTRACT_CLI_CONFIG_DIR = path.join(base, "new-config");
        env.CODEX_HOME = path.join(base, "new-client");
        target = path.join(env.CODEX_HOME, "skills");
        fs.cpSync(original, target, { recursive: true, filter: f => path.basename(f) !== ".contract-cli-managed.json" });
      }
      const review = path.join(target, "contract-cli-review/SKILL.md");
      if (scenario.startsWith("edited")) fs.appendFileSync(review, "\nuser edit\n");
      const before = fs.readFileSync(review, "utf8");
      const output = install(current);
      if (scenario.startsWith("edited")) {
        assert.match(output, /unverified/);
        assert.equal(fs.readFileSync(review, "utf8"), before);
        assert.deepEqual(JSON.parse(cli(["skills", "status", "--output=json"])).data.registration.locations, []);
      } else {
        assert.match(output, /Skill 自动同步：成功/, scenario + ": " + output);
        const {cli: identity, registration} = JSON.parse(cli(["skills", "status", "--output=json"])).data;
        assert.equal(identity.version, version);
        assert.deepEqual(registration.locations.map(l => [l.target, l.version, l.status]), [[target, version, "success"]]);
        const expected = JSON.parse(cli(["skills", "list", "--output=json"])).data.skills;
        for (const skill of expected) assert.ok(fs.readFileSync(path.join(target, skill.directory, "SKILL.md"), "utf8").includes("version: " + version));
      }
    }
  } finally { fs.rmSync(temp, { recursive: true, force: true }); }
});

test("real tgz upgrade runs postinstall, updates registered locations and respects install scope", { timeout: 240000, skip: historicalClientSafetySkip }, () => {
  const { buildLocalTestPackage, runNpm } = require("../../scripts/build-local-test-package.js");
  const root = path.resolve(__dirname, "../..");
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), "contract-cli-sync-e2e-"));
  const env = { ...process.env, CONTRACT_CLI_CONFIG_DIR: path.join(temp, "config"), CODEX_HOME: path.join(temp, "codex"), CONTRACT_CLI_NO_UPDATE_CHECK: "1" };
  if (process.platform === "win32") env.PSModulePath = path.join(process.env.SystemRoot, "System32", "WindowsPowerShell", "v1.0", "Modules");
  const prefix = path.join(temp, "prefix");
  const a = path.join(temp, "agent-a");
  const b = path.join(temp, "agent-b");
  const npmMajor = Number(runNpm(["--version"], { encoding: "utf8" }).trim().split(".")[0]);
  function npm(args, extra = {}) {
    // Isolated, locally built fixture without dependencies; never changes user npm policy.
    const approved = npmMajor >= 12 ? [...args, "--dangerously-allow-all-scripts"] : args;
    return runNpm(approved, { env: { ...env, ...extra }, cwd: temp, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] });
  }
  function actualCLI(args) {
    const packageRoot = process.platform === "win32" ? path.join(prefix, "node_modules", "@qfeius", "contract-cli") : path.join(prefix, "lib", "node_modules", "@qfeius", "contract-cli");
    return execFileSync(process.execPath, [path.join(packageRoot, "scripts", "run.js"), ...args], { env, encoding: "utf8", stdio: ["ignore", "pipe", "pipe"] });
  }
  const skill = target => path.join(target, "auth", "SKILL.md");
  try {
    const p1 = buildLocalTestPackage({ root, output: temp, version: "1.9.4-test.clm111796.1" });
    const p2 = buildLocalTestPackage({ root, output: temp, version: "1.9.4-test.clm111796.2" });
    const first = npm(["install", "-g", p1, "--prefix", prefix, "--foreground-scripts", "--no-audit", "--no-fund"]);
    assert.match(first, /no confirmed Skill locations/);
    actualCLI(["skills", "install", "--target", a, "--force"]);
    actualCLI(["skills", "install", "--target", b, "--force"]);
    fs.writeFileSync(skill(a), "local user changes");
    fs.mkdirSync(path.join(a, "third-party"));
    fs.writeFileSync(path.join(a, "third-party", "SKILL.md"), "keep");
    const upgraded = npm(["install", "-g", p2, "--prefix", prefix, "--foreground-scripts", "--no-audit", "--no-fund"]);
    assert.match(upgraded, /official Skill version: 1.9.4-test.clm111796.2/);
    assert.match(upgraded, /当前 Agent.*skills status --output json/);
    const status = JSON.parse(actualCLI(["skills", "status", "--output", "json"]));
    assert.equal(status.ok, true);
    assert.deepEqual(status.data.registration.locations.map(l => [l.target, l.version, l.status]),
      [a, b].map(t => [fs.realpathSync(t), "1.9.4-test.clm111796.2", "success"]));
    for (const target of [a, b]) assert.match(fs.readFileSync(skill(target), "utf8"), /version: 1.9.4-test.clm111796.2/);
    assert.equal(fs.readFileSync(path.join(a, "third-party", "SKILL.md"), "utf8"), "keep");
    fs.writeFileSync(skill(a), "local unchanged");
    const project = path.join(temp, "project");
    fs.mkdirSync(project);
    fs.writeFileSync(path.join(project, "package.json"), JSON.stringify({ name: "isolated-project", version: "1.0.0", allowScripts: { [p1]: true } }));
    npm(["install", p1, "--prefix", project, "--foreground-scripts", "--no-audit", "--no-fund"]);
    assert.equal(fs.readFileSync(skill(a), "utf8"), "local unchanged");
    npm(["install", "-g", p1, "--prefix", prefix, "--ignore-scripts", "--no-audit", "--no-fund"]);
    assert.equal(fs.readFileSync(skill(a), "utf8"), "local unchanged");
    fs.rmSync(b, { recursive: true });
    npm(["install", "-g", p2, "--prefix", prefix, "--foreground-scripts", "--no-audit", "--no-fund"]);
    assert.equal(fs.existsSync(b), false);
    assert.match(fs.readFileSync(skill(a), "utf8"), /version: 1.9.4-test.clm111796.2/);
  } finally {
    fs.rmSync(temp, { recursive: true, force: true });
  }
});

test("modified baseline history requires explicit installation before future sync", { timeout: 240000, skip: historicalClientSafetySkip || !process.env.CONTRACT_CLI_BASELINE_ROOT }, () => {
  const { buildLocalTestPackage, runNpm } = require("../../scripts/build-local-test-package.js");
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), "contract-cli-history-e2e-"));
  const prefix = path.join(temp, "prefix");
  const env = { ...process.env, CONTRACT_CLI_CONFIG_DIR: path.join(temp, "config"), CODEX_HOME: path.join(temp, "codex"), CONTRACT_CLI_NO_UPDATE_CHECK: "1" };
  if (process.platform === "win32") env.PSModulePath = path.join(process.env.SystemRoot, "System32", "WindowsPowerShell", "v1.0", "Modules");
  const npmMajor = Number(runNpm(["--version"], { encoding: "utf8" }).trim().split(".")[0]);
  const install = pkg => runNpm(["install", "-g", pkg, "--prefix", prefix, "--foreground-scripts", "--no-audit", "--no-fund", ...(npmMajor >= 12 ? ["--dangerously-allow-all-scripts"] : [])], { env, cwd: temp, encoding: "utf8" });
  try {
    const old = buildLocalTestPackage({ root: process.env.CONTRACT_CLI_BASELINE_ROOT, output: temp, version: "1.9.4-test.baseline" });
    const current = buildLocalTestPackage({ output: temp, version: "1.9.4-test.clm111796.history" });
    install(old);
    const pkgRoot = process.platform === "win32" ? path.join(prefix, "node_modules", "@qfeius", "contract-cli") : path.join(prefix, "lib", "node_modules", "@qfeius", "contract-cli");
    execFileSync(process.execPath, [path.join(pkgRoot, "scripts", "run.js"), "skills", "install", "--force"], { env, stdio: "pipe" });
    assert.equal(fs.existsSync(path.join(env.CONTRACT_CLI_CONFIG_DIR, "skill-locations.json")), false);
    const skill = path.join(env.CODEX_HOME, "skills", "auth", "SKILL.md");
    fs.writeFileSync(skill, "historical user modified file");
    const output = install(current);
    assert.match(output, /历史 Skill 未自动接管/);
    assert.equal(fs.readFileSync(skill, "utf8"), "historical user modified file");
    assert.equal(fs.existsSync(path.join(env.CONTRACT_CLI_CONFIG_DIR, "skill-locations.json")), false);
    execFileSync(process.execPath, [path.join(pkgRoot, "scripts", "run.js"), "skills", "install", "--target", path.join(env.CODEX_HOME, "skills"), "--force"], { env, stdio: "pipe" });
    assert.match(fs.readFileSync(skill, "utf8"), /version: 1.9.4-test.clm111796.history/);
    const registry = JSON.parse(fs.readFileSync(path.join(env.CONTRACT_CLI_CONFIG_DIR, "skill-locations.json"), "utf8"));
    assert.equal(registry.locations.length, 1);
    assert.equal(registry.locations[0].status, "success");
  } finally {
    fs.rmSync(temp, { recursive: true, force: true });
  }
});
