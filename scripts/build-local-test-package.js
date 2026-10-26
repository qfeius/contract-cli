"use strict";

// Local test packaging defaults to the host; explicit all bundles the release matrix.
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const crypto = require("node:crypto");
const { execFileSync } = require("node:child_process");

function runNpm(args, options = {}) {
  const roots = [path.dirname(process.execPath), path.resolve(path.dirname(process.execPath), "..", "lib"), ...(process.env.PATH || "").split(path.delimiter)];
  const candidates = [process.env.npm_execpath, process.env.NPM_CLI, ...roots.map(root => path.join(root, "node_modules", "npm", "bin", "npm-cli.js")), "/usr/share/nodejs/npm/bin/npm-cli.js"];
  const cli = candidates.find(file => file && fs.existsSync(file));
  if (!cli) throw new Error("npm CLI not found; set NPM_CLI to npm/bin/npm-cli.js");
  return execFileSync(process.execPath, [cli, ...args], { ...options, shell: false });
}

const RELEASE_TARGETS = Object.freeze(["darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64"]);

function resolvePackageTargets(selection) {
  const host = { win32: "windows", darwin: "darwin", linux: "linux" }[process.platform] + "/" + { x64: "amd64", arm64: "arm64" }[process.arch];
  const targets = selection === undefined ? [host] : selection === "all" ? [...RELEASE_TARGETS] : selection;
  if (!Array.isArray(targets) || targets.length === 0 || targets.some(target => !RELEASE_TARGETS.includes(target)) || new Set(targets).size !== targets.length) {
    throw new Error("unsupported, empty or duplicate package platform selection");
  }
  return [...targets];
}

function buildLocalTestPackage({ root = path.resolve(__dirname, ".."), output, version, targets }) {
  if (!/^\d+\.\d+\.\d+-test\.[a-zA-Z0-9.-]+$/.test(version)) throw new Error("local package version must use -test.");
  if (!output) throw new Error("output directory required");
  const selectedTargets = resolvePackageTargets(targets);
  output = path.resolve(output);
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), "contract-cli-local-package-"));
  console.log(`Building local test package ${version}`);
  try {
    const stage = path.join(temp, "package");
    const scripts = path.join(stage, "scripts");
    const assets = path.join(stage, "dist", "release-assets");
    for (const dir of [scripts, assets, output]) fs.mkdirSync(dir, { recursive: true });
    const pkg = JSON.parse(fs.readFileSync(path.join(root, "package.json"), "utf8"));
    pkg.version = version;
    fs.writeFileSync(path.join(stage, "package.json"), JSON.stringify(pkg, null, 2));
    for (const name of ["install.js", "run.js", "setup.js"]) fs.copyFileSync(path.join(root, "scripts", name), path.join(scripts, name));
    for (const name of ["README.md", "CHANGELOG.md", "LICENSE"]) fs.copyFileSync(path.join(root, name), path.join(stage, name));
    fs.cpSync(path.join(root, "skills"), path.join(stage, "skills"), { recursive: true });
    const binaryName = (pkg.config && pkg.config.binaryName) || "contract-cli";
    let commit = "unknown";
    try {
      commit = execFileSync("git", ["rev-parse", "--short", "HEAD"], { cwd: root, encoding: "utf8" }).trim();
      if (execFileSync("git", ["status", "--porcelain"], { cwd: root, encoding: "utf8" }).trim()) commit += "-dirty";
    } catch {}
    const builtAt = new Date().toISOString();
    const flags = `-s -w -X cn.qfei/contract-cli/internal/build.Version=${version} -X cn.qfei/contract-cli/internal/build.Commit=${commit} -X cn.qfei/contract-cli/internal/build.Date=${builtAt}`;
    const checksums = [];
    for (const target of selectedTargets) {
      const [goos, goarch] = target.split("/");
      const windows = goos === "windows";
      const executable = binaryName + (windows ? ".exe" : "");
      const binaryDir = path.join(temp, "binary", `${goos}-${goarch}`);
      fs.mkdirSync(binaryDir, { recursive: true });
      console.log(`Building ${target}`);
      execFileSync(process.env.GO_BINARY || "go", ["build", "-trimpath", "-ldflags", flags, "-o", path.join(binaryDir, executable), "./cmd/contract-cli"], { cwd: root, stdio: "inherit", env: { ...process.env, CGO_ENABLED: "0", GOOS: goos, GOARCH: goarch } });
      const archiveName = `${binaryName}-${version}-${goos}-${goarch}${windows ? ".zip" : ".tar.gz"}`;
      const archive = path.join(assets, archiveName);
      if (windows && process.platform === "win32") {
        execFileSync("powershell.exe", ["-NoProfile", "-Command", "& { param($source,$dest) $ErrorActionPreference='Stop'; Add-Type -AssemblyName System.IO.Compression.FileSystem; [System.IO.Compression.ZipFile]::CreateFromDirectory($source,$dest) }", binaryDir, archive], { stdio: "inherit" });
      } else if (windows) {
        execFileSync("zip", ["-q", archive, executable], { cwd: binaryDir, stdio: "inherit" });
      } else {
        execFileSync("tar", ["-czf", archive, "-C", binaryDir, executable], { stdio: "inherit" });
      }
      const hash = crypto.createHash("sha256").update(fs.readFileSync(archive)).digest("hex");
      checksums.push(`${hash}  ${archiveName}`);
    }
    fs.writeFileSync(path.join(assets, "checksums.txt"), checksums.join("\n") + "\n");
    const packed = runNpm(["pack", "--json"], { cwd: stage, encoding: "utf8" });
    const parsed = JSON.parse(packed);
    const filename = (Array.isArray(parsed) ? parsed[0] : Object.values(parsed)[0]).filename;
    const result = path.join(output, filename);
    fs.copyFileSync(path.join(stage, filename), result);
    const packageHash = crypto.createHash("sha256").update(fs.readFileSync(result)).digest("hex");
    fs.writeFileSync(result + ".sha256", `${packageHash}  ${filename}\n`);
    fs.writeFileSync(result + ".manifest.json", JSON.stringify({ version, commit, builtAt, targets: selectedTargets, packageSha256: packageHash, published: false, platformBuildsAreNotRuntimeAcceptance: true }, null, 2) + "\n");
    console.log(`Local test package ready: ${result}`);
    return result;
  } catch (error) {
    console.error(`Local test packaging failed: ${error.message}`);
    throw error;
  } finally {
    fs.rmSync(temp, { recursive: true, force: true });
  }
}

if (require.main === module) {
  buildLocalTestPackage({ version: process.argv[2], output: process.argv[3], targets: process.argv[4] });
}
module.exports = { buildLocalTestPackage, runNpm, resolvePackageTargets };
