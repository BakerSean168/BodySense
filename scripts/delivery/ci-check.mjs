#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";
import { verifyCiCheck } from "./ci-check-lib.mjs";

function arg(name) {
  const index = process.argv.indexOf("--" + name);
  return index < 0 ? undefined : process.argv[index + 1];
}

function readJson(file, title) {
  if (!file) throw new Error(title + " path is required");
  return JSON.parse(fs.readFileSync(file, "utf8"));
}

try {
  const manifest = readJson(
    arg("manifest") ?? process.env.DELIVERY_MANIFEST_PATH,
    "manifest",
  );
  const summary = readJson(arg("summary"), "summary");
  if (!process.env.CI_RESULTS_JSON) {
    throw new Error("CI_RESULTS_JSON is required");
  }
  const result = verifyCiCheck({
    manifest,
    summary,
    results: JSON.parse(process.env.CI_RESULTS_JSON),
    expectedDigest: process.env.CI_MANIFEST_DIGEST,
    expectedHeadSha: process.env.CI_HEAD_SHA,
    expectedRunId: process.env.CI_RUN_ID,
    expectedRunAttempt: process.env.CI_RUN_ATTEMPT,
    expectedEvent: process.env.CI_EVENT,
    expectedRef: process.env.CI_REF,
  });
  const filename = path.resolve(
    arg("output") ?? "reports/delivery/ci-check-receipt-v1.json",
  );
  fs.mkdirSync(path.dirname(filename), { recursive: true });
  fs.writeFileSync(filename, JSON.stringify(result.receipt, null, 2) + "\n");
  if (!result.ok) {
    for (const message of result.failures) {
      console.error("CI_CHECK=FAIL " + message);
    }
    process.exitCode = 1;
  } else {
    console.log(
      "CI_CHECK=PASS run=" +
        result.receipt.runId +
        " attempt=" +
        result.receipt.runAttempt +
        " sha=" +
        result.receipt.headSha +
        " manifest=" +
        result.receipt.manifestDigest +
        " receipt=" +
        result.receipt.digest,
    );
  }
} catch (error) {
  console.error("CI_CHECK=FAIL " + error.message);
  process.exitCode = 1;
}
