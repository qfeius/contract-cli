"use strict";

const assert = require("node:assert/strict");
const test = require("node:test");

const { runSetup } = require("./setup.js");

test("contract-cli-setup delegates to the bundled CLI skill installer", () => {
  const calls = [];

  runSetup(["--target", "/tmp/codex skills"], {
    binaryPath: "/package/bin/contract-cli",
    existsSync: () => true,
    execFileSync: (file, args, options) => calls.push({ file, args, options }),
  });

  assert.deepEqual(calls, [
    {
      file: "/package/bin/contract-cli",
      args: ["skills", "install", "--target", "/tmp/codex skills"],
      options: { stdio: "inherit" },
    },
  ]);
});

test("contract-cli-setup does not force-overwrite existing skills by default", () => {
  const calls = [];

  runSetup([], {
    binaryPath: "/package/bin/contract-cli",
    existsSync: () => true,
    execFileSync: (file, args) => calls.push({ file, args }),
  });

  assert.deepEqual(calls[0].args, ["skills", "install"]);
  assert.equal(calls[0].args.includes("--force"), false);
});

test("contract-cli-setup fails clearly when the bundled CLI is unavailable", () => {
  assert.throws(
    () => runSetup([], { binaryPath: "/missing/contract-cli", existsSync: () => false }),
    /binary not found/
  );
});
