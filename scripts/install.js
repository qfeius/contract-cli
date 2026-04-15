"use strict";

const fs = require("fs");
const os = require("os");
const path = require("path");
const { execFileSync, spawnSync } = require("child_process");

const pkg = require("../package.json");
const binaryName = (pkg.config && pkg.config.binaryName) || "contract-cli";
const version = pkg.version;
const platformMap = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows",
};
const archMap = {
  x64: "amd64",
  arm64: "arm64",
};

const platform = platformMap[process.platform];
const arch = archMap[process.arch];
if (!platform || !arch) {
  console.error(`Unsupported platform: ${process.platform}-${process.arch}`);
  process.exit(1);
}

const isWindows = process.platform === "win32";
const archiveExt = isWindows ? ".zip" : ".tar.gz";
const archiveName = `${binaryName}-${version}-${platform}-${arch}${archiveExt}`;
const rootDir = path.join(__dirname, "..");
const binDir = path.join(rootDir, "bin");
const destination = path.join(binDir, binaryName + (isWindows ? ".exe" : ""));

function resolveDownloadBaseURL() {
  const template =
    process.env.CONTRACT_CLI_DOWNLOAD_BASE_URL_TEMPLATE ||
    process.env.npm_package_config_downloadBaseURLTemplate ||
    (pkg.config && pkg.config.downloadBaseURLTemplate) ||
    "";
  if (!template) {
    return "";
  }
  return template.replace(/\{version\}/g, version).replace(/\/$/, "");
}

function commandExists(command) {
  const checker = process.platform === "win32" ? "where" : "which";
  const args = [command];
  const result = spawnSync(checker, args, { stdio: "ignore", shell: false });
  return result.status === 0;
}

function sourceCheckoutExists() {
  return (
    fs.existsSync(path.join(rootDir, "go.mod")) &&
    fs.existsSync(path.join(rootDir, "cmd", "contract-cli", "main.go"))
  );
}

function buildLdflags() {
  const commit = readGitValue(["rev-parse", "--short", "HEAD"], "unknown");
  const date = new Date().toISOString();
  return `-s -w -X cn.qfei/contract-cli/internal/build.Version=${version} -X cn.qfei/contract-cli/internal/build.Commit=${commit} -X cn.qfei/contract-cli/internal/build.Date=${date}`;
}

function readGitValue(args, fallback) {
  try {
    return execFileSync("git", args, {
      cwd: rootDir,
      stdio: ["ignore", "pipe", "ignore"],
      encoding: "utf8",
    }).trim() || fallback;
  } catch (_) {
    return fallback;
  }
}

function buildFromSource() {
  if (!sourceCheckoutExists()) {
    throw new Error("Go source checkout not found");
  }
  if (!commandExists("go")) {
    throw new Error("go toolchain not found and no binary download URL configured");
  }

  fs.mkdirSync(binDir, { recursive: true });
  execFileSync(
    "go",
    ["build", "-ldflags", buildLdflags(), "-o", destination, "./cmd/contract-cli"],
    {
      cwd: rootDir,
      stdio: "inherit",
    }
  );
}

function downloadArchive(downloadURL, archivePath) {
  if (!commandExists("curl")) {
    throw new Error("curl not found");
  }

  const curlArgs = [
    "--fail",
    "--location",
    "--silent",
    "--show-error",
    "--connect-timeout",
    "10",
    "--max-time",
    "120",
    "--output",
    archivePath,
    downloadURL,
  ];
  if (isWindows) {
    curlArgs.unshift("--ssl-revoke-best-effort");
  }
  execFileSync("curl", curlArgs, { stdio: ["ignore", "ignore", "pipe"] });
}

function extractArchive(archivePath, tempDir) {
  if (isWindows) {
    execFileSync(
      "powershell",
      [
        "-Command",
        `Expand-Archive -Path '${archivePath}' -DestinationPath '${tempDir}' -Force`,
      ],
      { stdio: "ignore" }
    );
    return;
  }
  execFileSync("tar", ["-xzf", archivePath, "-C", tempDir], { stdio: "ignore" });
}

function installFromDownload(downloadBaseURL) {
  if (!downloadBaseURL) {
    throw new Error("download base URL template not configured");
  }

  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "contract-cli-"));
  const archivePath = path.join(tempDir, archiveName);
  const downloadURL = `${downloadBaseURL}/${archiveName}`;

  try {
    downloadArchive(downloadURL, archivePath);
    extractArchive(archivePath, tempDir);
    const extractedBinary = path.join(tempDir, binaryName + (isWindows ? ".exe" : ""));
    if (!fs.existsSync(extractedBinary)) {
      throw new Error(`binary ${path.basename(extractedBinary)} not found in archive`);
    }
    fs.mkdirSync(binDir, { recursive: true });
    fs.copyFileSync(extractedBinary, destination);
    fs.chmodSync(destination, 0o755);
  } finally {
    fs.rmSync(tempDir, { recursive: true, force: true });
  }
}

function install() {
  const downloadBaseURL = resolveDownloadBaseURL();

  if (downloadBaseURL) {
    try {
      installFromDownload(downloadBaseURL);
      console.log(`${binaryName} ${version} installed from release assets`);
      return;
    } catch (error) {
      if (!sourceCheckoutExists()) {
        throw error;
      }
      console.warn(`release download failed, falling back to local build: ${error.message}`);
    }
  }

  buildFromSource();
  console.log(`${binaryName} ${version} built from local sources`);
}

try {
  install();
} catch (error) {
  console.error(`Failed to install ${binaryName}: ${error.message}`);
  console.error(
    "Set CONTRACT_CLI_DOWNLOAD_BASE_URL_TEMPLATE to a real release address, or run npm install from a checkout that has Go sources."
  );
  process.exit(1);
}
