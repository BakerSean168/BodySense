import {
  MANIFEST_SCHEMA,
  canonicalize,
  sha256Json,
  validateManifest,
} from "./lib.mjs";

// Only actual executable jobs, no duplicate Oracle/Observation fan-in.
export const CI_JOB_IDS = Object.freeze([
  "scope",
  "lint",
  "quality-web-child",
  "quality-api-child",
  "quality-ai-child",
  "contracts-child",
  "component-child",
  "migrations-child",
  "db-integration-child",
  "recovery-migrations-child",
  "quality-release-child",
  "e2e-child",
  "e2e-report-child",
]);

const OUTCOMES = new Set(["success", "skipped", "failure", "cancelled"]);
function identical(a, b) {
  return JSON.stringify(canonicalize(a)) === JSON.stringify(canonicalize(b));
}

export function expectedCiJobs(manifest) {
  const lane = manifest?.lanes ?? {};
  const full = lane.full === true;
  const main = manifest?.event !== "pull_request";
  const e2e = main && (lane.experience === true || full);
  return {
    scope: true,
    lint: true,
    "quality-web-child": lane.web === true,
    "quality-api-child": lane.api === true,
    "quality-ai-child": lane.ai === true,
    "contracts-child": lane.contracts === true || full,
    "component-child": lane.component === true || full,
    "migrations-child": lane.database === true || full,
    "db-integration-child": lane.api === true || lane.database === true || full,
    "recovery-migrations-child": main && (lane.database === true || full),
    "quality-release-child": main && full,
    "e2e-child": e2e,
    "e2e-report-child": e2e,
  };
}

export function verifyCiCheck({
  manifest,
  results,
  expectedDigest,
  expectedHeadSha,
  expectedRunId,
  expectedRunAttempt,
  expectedEvent,
  expectedRef,
}) {
  const failures = validateManifest(manifest).map(
    (error) => "manifest: " + error,
  );
  if (manifest?.schema !== MANIFEST_SCHEMA) {
    failures.push("current CI manifest schema is required");
  }
  if (manifest?.digest !== expectedDigest) {
    failures.push("Scope manifest digest mismatch");
  }
  if (manifest?.headSha !== expectedHeadSha) {
    failures.push("head SHA does not match GitHub event");
  }
  if (manifest?.event !== expectedEvent || manifest?.ref !== expectedRef) {
    failures.push("event/ref does not match GitHub run");
  }
  if (
    !/^[1-9][0-9]*$/.test(String(expectedRunId)) ||
    !Number.isSafeInteger(Number(expectedRunAttempt)) ||
    Number(expectedRunAttempt) < 1 ||
    manifest?.execution?.runId !== String(expectedRunId) ||
    manifest?.execution?.attempt !== Number(expectedRunAttempt)
  ) {
    failures.push("run ID/attempt does not match GitHub run");
  }

  if (!results || typeof results !== "object" || Array.isArray(results)) {
    failures.push("results must be a named job map");
    results = {};
  }
  if (!identical(Object.keys(results).sort(), [...CI_JOB_IDS].sort())) {
    failures.push("job result keys differ from the exact workflow job graph");
  }
  const required = expectedCiJobs(manifest);
  for (const job of CI_JOB_IDS) {
    const actual = results[job] ?? "missing";
    if (!OUTCOMES.has(actual)) {
      failures.push(job + ": unknown or missing result " + actual);
      continue;
    }
    if (required[job] && actual !== "success") {
      failures.push(job + ": expected success, received " + actual);
    }
    if (!required[job] && actual !== "skipped") {
      failures.push(
        job + ": expected skip for unselected lane, received " + actual,
      );
    }
  }

  const receipt = {
    schema: "bodysense.ci-check-receipt/v2",
    runId: String(expectedRunId),
    runAttempt: Number(expectedRunAttempt),
    event: String(expectedEvent),
    ref: String(expectedRef),
    headSha: String(expectedHeadSha),
    manifestDigest: manifest?.digest ?? null,
    results: canonicalize(results),
    passed: failures.length === 0,
    failures,
    generatedAt: new Date().toISOString(),
  };
  const { generatedAt: _ignored, ...identity } = receipt;
  receipt.digest = sha256Json(identity);
  return { ok: receipt.passed, failures, receipt };
}
