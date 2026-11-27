"use strict";

const fs = require("node:fs");
const os = require("node:os");
const vm = require("node:vm");
const cp = require("node:child_process");
const assert = require("node:assert/strict");
const { test } = require("node:test");
const config = require("../connectors/workbuddy/auth.fragment.json");

test("WorkBuddy config explicitly retains auth process even without a device-modal callback", () => {
  assert.equal(config.authWaitForExit, true);
  assert.ok(config.authDeviceFlow.uriPattern);
});

const archive = process.env.WORKBUDDY_APP_ASAR || "/Applications/WorkBuddy.app/Contents/Resources/app.asar";
const nativeOptions = { skip: !fs.existsSync(archive), timeout: 5000 };

function readAuthStep() {
  const file = fs.openSync(archive, "r");
  try {
    const prefix = Buffer.alloc(16);
    fs.readSync(file, prefix, 0, prefix.length, 0);
    const header = Buffer.alloc(prefix.readUInt32LE(12));
    fs.readSync(file, header, 0, header.length, 16);
    const entry = JSON.parse(header).files.main.files["module.app-server.js"];
    const bytes = Buffer.alloc(entry.size);
    fs.readSync(file, bytes, 0, bytes.length, 8 + prefix.readUInt32LE(4) + Number(entry.offset));
    const source = bytes.toString();
    const start = source.indexOf("\tasync runAuthStep(cmd, env, effective, cliConfig, options, signal) {");
    const end = source.indexOf("\n\tterminateAuthProcess(child) {", start);
    assert.ok(start >= 0 && end > start, "installed WorkBuddy auth implementation changed; inspect before updating the test");
    return source.slice(start, end);
  } finally {
    fs.closeSync(file);
  }
}

async function exercise(effective, options = {}) {
  let child;
  let killed = 0;
  let spawnTimeout;
  let reportCode;
  const reported = new Promise(resolve => { reportCode = resolve; });
  const controller = new AbortController();
  const childSource = "console.log('https://myaccount.qfei.cn/device?user_code=fixture');setTimeout(()=>{console.log('AUTH_RESULT_READY');process.exit(" + (options.exitCode || 0) + ")},300);";
  const executor = vm.runInNewContext("({" + readAuthStep() + "})", {
    os,
    child_process: {
      spawn: (_command, spawnOptions) => {
        spawnTimeout = spawnOptions.timeout;
        // Real subprocess, harmless fixture instead of production authorization.
        child = cp.spawn(process.execPath, ["-e", childSource], { ...spawnOptions, shell: false });
        return child;
      },
    },
  });
  executor.logger = { info() {}, warn() {} };
  executor.hostCapabilities = { openExternal: () => { reportCode(); } };
  executor.extractAuthUrl = text => text.match(new RegExp(config.authDeviceFlow.uriPattern))?.[1];
  executor.extractDeviceFlowInfo = text => {
    const uri = executor.extractAuthUrl(text);
    return uri ? { verificationUri: uri, codeEmbeddedInUri: true } : undefined;
  };
  executor.terminateAuthProcess = target => { killed++; target.kill(); };
  const hostOptions = options.deviceCallback ? { onDeviceCode: () => reportCode() } : {};
  try {
    const finished = executor.runAuthStep("contract-cli auth device-login", process.env, effective, effective, hostOptions, controller.signal);
    await reported;
    if (options.cancel) controller.abort();
    const result = await finished;
    if (child.exitCode === null && child.signalCode === null) {
      await new Promise(resolve => child.once("close", resolve));
    }
    return { result, killed, spawnTimeout };
  } finally {
    if (child && child.exitCode === null && child.signalCode === null) child.kill();
  }
}

test("installed WorkBuddy: default URL mode terminates auth early (control)", nativeOptions, async () => {
  const { result, killed } = await exercise({ authUrlDomain: config.authUrlDomain });
  assert.equal(killed, 1);
  assert.equal(result.stdout.includes("AUTH_RESULT_READY"), false);
});

for (const deviceCallback of [true, false]) {
  test("installed WorkBuddy: retain process through result, device callback=" + deviceCallback, nativeOptions, async () => {
    const { result, killed, spawnTimeout } = await exercise(config, { deviceCallback });
    assert.equal(killed, 0);
    assert.equal(spawnTimeout, 300000);
    assert.equal(result.success, true);
    assert.match(result.stdout, /AUTH_RESULT_READY/);
  });
}

test("installed WorkBuddy: cancellation still terminates and reports failure", nativeOptions, async () => {
  const { result, killed } = await exercise(config, { deviceCallback: true, cancel: true });
  assert.equal(killed, 1);
  assert.equal(result.success, false);
  assert.equal(result.cancelled, true);
});

test("installed WorkBuddy: failed CLI exit is not authorization success", nativeOptions, async () => {
  const { result, killed } = await exercise(config, { deviceCallback: true, exitCode: 1 });
  assert.equal(killed, 0);
  assert.equal(result.success, false);
  assert.equal(result.exitCode, 1);
});
