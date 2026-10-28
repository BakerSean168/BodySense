#!/usr/bin/env node
import fs from "node:fs";
import { fileURLToPath } from "node:url";
import { sha256Json } from "./lib.mjs";
import { CI_JOB_IDS } from "./ci-check-lib.mjs";

export function validateNightlyReceipt(receipt, { sha, runId, runAttempt }) {
  const failures = [];
  if (!receipt || typeof receipt !== "object" || Array.isArray(receipt)) {
    return ["missing nightly CI Check receipt"];
  }
  if (receipt.schema !== "bodysense.ci-check-receipt/v2")
    failures.push("unexpected receipt schema");
  if (
    receipt.passed !== true ||
    !Array.isArray(receipt.failures) ||
    receipt.failures.length
  )
    failures.push("nightly Check did not pass");
  if (receipt.event !== "schedule" || receipt.ref !== "refs/heads/main")
    failures.push("nightly Check must be scheduled Main Full");
  if (receipt.headSha !== sha || !/^[0-9a-f]{40}$/.test(receipt.headSha ?? ""))
    failures.push("receipt source SHA mismatch");
  if (
    receipt.runId !== String(runId) ||
    receipt.runAttempt !== Number(runAttempt)
  )
    failures.push("receipt run ID/attempt mismatch");
  if (!/^sha256:[0-9a-f]{64}$/.test(receipt.manifestDigest ?? ""))
    failures.push("invalid delivery manifest digest");
  const results = receipt.results ?? {};
  if (
    JSON.stringify(Object.keys(results).sort()) !==
    JSON.stringify([...CI_JOB_IDS].sort())
  )
    failures.push("missing or unexpected CI job result");
  for (const id of CI_JOB_IDS)
    if (results[id] !== "success")
      failures.push(id + ": full nightly result was not successful");
  const { digest: _digest, generatedAt: _generatedAt, ...identity } = receipt;
  if (receipt.digest !== sha256Json(identity))
    failures.push("CI receipt digest mismatch");
  return failures;
}

function arg(flag) {
  const index = process.argv.indexOf("--" + flag);
  return index === -1 ? "" : process.argv[index + 1];
}

if (process.argv[1] && fileURLToPath(import.meta.url) === process.argv[1]) {
  try {
    const receipt = JSON.parse(fs.readFileSync(arg("receipt"), "utf8"));
    const failures = validateNightlyReceipt(receipt, {
      sha: arg("sha"),
      runId: arg("run-id"),
      runAttempt: arg("run-attempt"),
    });
    if (failures.length) throw new Error(failures.join("; "));
    console.log("NIGHTLY_QUALIFICATION=PASS " + receipt.headSha);
  } catch (error) {
    console.error("NIGHTLY_QUALIFICATION=FAIL " + error.message);
    process.exitCode = 1;
  }
}
