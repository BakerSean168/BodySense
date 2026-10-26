#!/usr/bin/env node
import fs from "node:fs";
import { validateManifest } from "./lib.mjs";

const argv = process.argv.slice(2);
const digestIndex = argv.indexOf("--expect-digest");
const expectedDigest = digestIndex >= 0 ? argv[digestIndex + 1] : undefined;
if (digestIndex >= 0 && !expectedDigest) {
  console.error("--expect-digest requires a digest value");
  process.exit(2);
}
const positional =
  digestIndex >= 0
    ? argv.filter(
        (_token, index) => index !== digestIndex && index !== digestIndex + 1,
      )
    : argv;
const file = positional[0] ?? process.env.DELIVERY_MANIFEST_PATH;
if (!file) {
  console.error(
    "usage: validate-manifest.mjs <manifest.json> [--expect-digest <digest>]",
  );
  process.exit(2);
}

try {
  const manifest = JSON.parse(fs.readFileSync(file, "utf8"));
  const errors = validateManifest(manifest);
  if (expectedDigest && manifest.digest !== expectedDigest) {
    errors.push(
      `manifest digest does not match expected digest: ${manifest.digest} !== ${expectedDigest}`,
    );
  }
  if (errors.length > 0) {
    console.error(errors.join("\n"));
    process.exit(1);
  }
  console.log(
    `DELIVERY_MANIFEST=PASS digest=${manifest.digest} risk=${manifest.risk}`,
  );
} catch (error) {
  console.error(`delivery manifest validation failed: ${error.message}`);
  process.exit(1);
}
