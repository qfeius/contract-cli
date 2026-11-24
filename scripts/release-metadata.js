"use strict";

const fs = require("node:fs");

function releaseMetadata(tag) {
  const number = "(?:0|[1-9][0-9]*)";
  const identifier = "(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)";
  const pattern = new RegExp(`^v(${number}\\.${number}\\.${number}(?:-${identifier}(?:\\.${identifier})*)?)$`);
  const match = typeof tag === "string" && pattern.exec(tag);
  if (!match || match[0] !== tag) throw new Error("release tag must be v<major>.<minor>.<patch> with an optional valid prerelease");
  const version = match[1];
  const prerelease = version.includes("-");
  return { version, channel: prerelease ? "beta" : "latest", prerelease };
}

if (require.main === module) {
  try {
    const metadata = releaseMetadata(process.argv[2]);
    if (process.env.GITHUB_OUTPUT) {
      fs.appendFileSync(process.env.GITHUB_OUTPUT, Object.entries(metadata).map(([key, value]) => `${key}=${value}\n`).join(""));
    }
    console.log(JSON.stringify(metadata));
  } catch (error) {
    console.error(`Release metadata rejected: ${error.message}`);
    process.exitCode = 1;
  }
}

module.exports = { releaseMetadata };
