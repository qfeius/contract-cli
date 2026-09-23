#!/usr/bin/env node
"use strict";

const { execFileSync } = require("child_process");
const fs = require("fs");
const path = require("path");

function bundledBinaryPath() {
  const binaryName = process.platform === "win32" ? "contract-cli.exe" : "contract-cli";
  return path.join(__dirname, "..", "bin", binaryName);
}

function runSetup(args = process.argv.slice(2), options = {}) {
  const binaryPath = options.binaryPath || bundledBinaryPath();
  const existsSync = options.existsSync || fs.existsSync;
  const execute = options.execFileSync || execFileSync;

  if (!existsSync(binaryPath)) {
    throw new Error(`contract-cli binary not found at ${binaryPath}; reinstall the npm package first`);
  }

  execute(binaryPath, ["skills", "install", ...args], { stdio: "inherit" });
}

if (require.main === module) {
  try {
    runSetup();
  } catch (error) {
    console.error(`Failed to install contract-cli skills: ${error.message}`);
    process.exit(error.status || 1);
  }
}

module.exports = { runSetup };
