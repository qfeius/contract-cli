"use strict";
// Read-only manifest generator. Input: explicitly installed, verified official
// release Skill directories. Redirecting output is not part of release builds.
const fs = require("node:fs");
const path = require("node:path");
const crypto = require("node:crypto");
const sha = data => crypto.createHash("sha256").update(data).digest("hex");
function digest(root) {
  const entries = [];
  function walk(relative) {
    const absolute = path.join(root, relative);
    const stat = fs.lstatSync(absolute);
    if (stat.isSymbolicLink()) throw new Error("links are not historical provenance");
    if (relative) entries.push(stat.isDirectory() ? `d:${relative}\0` : `f:${relative}\0${sha(fs.readFileSync(absolute))}\0`);
    if (stat.isDirectory()) for (const name of fs.readdirSync(absolute).sort()) walk(relative ? `${relative}/${name}` : name);
  }
  walk("");
  return sha(entries.join(""));
}
const manifests = {};
for (const root of process.argv.slice(2)) {
  for (const name of fs.readdirSync(root).sort()) {
    const hashes = manifests[name] ||= [];
    const hash = digest(path.join(root, name));
    if (!hashes.includes(hash)) hashes.push(hash);
  }
}
process.stdout.write(JSON.stringify(manifests, null, 2) + "\n");
