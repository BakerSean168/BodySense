#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";
import { verifyCiCheck } from "./ci-check-lib.mjs";

function option(name) {
  const i = process.argv.indexOf("--" + name);
  return i < 0 ? undefined : process.argv[i + 1];
}

try {
  const manifestFile = option("manifest") ?? process.env.DELIVERY_MANIFEST_PATH;
  if (!manifestFile || !option("output") || !process.env.CI_RESULTS_JSON) {
    throw new Error("manifest, --output and CI_RESULTS_JSON are required");
  }
  const manifest = JSON.parse(fs.readFileSync(manifestFile, "utf8"));
  const result = verifyCiCheck({
    manifest,
    results: JSON.parse(process.env.CI_RESULTS_JSON),
    expectedDigest: process.env.CI_MANIFEST_DIGEST,
    expectedHeadSha: process.env.CI_HEAD_SHA,
    expectedRunId: process.env.CI_RUN_ID,
    expectedRunAttempt: process.env.CI_RUN_ATTEMPT,
    expectedEvent: process.env.CI_EVENT,
    expectedRef: process.env.CI_REF,
  });
  const filename = path.resolve(option("output"));
  fs.mkdirSync(path.dirname(filename), { recursive: true });
  fs.writeFileSync(filename, JSON.stringify(result.receipt, null, 2) + "\n");
  if (!result.ok) {
    for (const failure of result.failures)
      console.error("CHECK=FAIL " + failure);
    process.exitCode = 1;
  } else {
    console.log(
      "CHECK=PASS run=" +
        result.receipt.runId +
        " attempt=" +
        result.receipt.runAttempt +
        " head=" +
        result.receipt.headSha +
        " manifest=" +
        result.receipt.manifestDigest +
        " receipt=" +
        result.receipt.digest,
    );
  }
} catch (error) {
  console.error("CHECK=FAIL " + error.message);
  process.exitCode = 1;
}
