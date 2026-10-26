import {
  MANIFEST_SCHEMA,
  SUMMARY_SCHEMA,
  LANE_NAMES,
  canonicalize,
  computeSummaryDigest,
  evaluateManifestOracle,
  sha256Json,
  validateManifest,
} from "./lib.mjs";

// The check's inputs must match the exact GitHub job projection. Unknown
// keys and missing jobs are rejected rather than silently trusting a green
// pre-existing Oracle result.
export const CI_JOB_IDS = Object.freeze([
  "scope",
  "commit-lint",
  "lint",
  "governance-child",
  "quality-web-child",
  "quality-api-child",
  "quality-ai-child",
  "quality-arch-child",
  "quality-release-child",
  "contracts-child",
  "migrations-child",
  "recovery-migrations-child",
  "db-integration-child",
  "component-child",
  "e2e-child",
  "e2e-report-child",
  "governance-oracle",
  "quality-oracle",
  "database-oracle",
  "experience-oracle",
  "delivery-observation",
]);

const GROUP_JOBS = Object.freeze({
  governance: {
    governance: "governance-child",
    lint: "lint",
  },
  quality: {
    "quality-web": "quality-web-child",
    "quality-api": "quality-api-child",
    "quality-ai": "quality-ai-child",
    "quality-arch": "quality-arch-child",
    "quality-release": "quality-release-child",
    contracts: "contracts-child",
  },
  database: {
    "database-baseline": "migrations-child",
    "database-recovery": "recovery-migrations-child",
    "database-integration": "db-integration-child",
  },
  experience: {
    experience: "e2e-report-child",
    component: "component-child",
  },
});

const ORACLE_RESULT_JOBS = Object.freeze({
  governance: "governance-oracle",
  quality: "quality-oracle",
  database: "database-oracle",
  experience: "experience-oracle",
});

const RESULT_VALUES = new Set(["success", "failure", "cancelled", "skipped"]);
const QUALITY_JOB_IDS = [
  "quality-web-child",
  "quality-api-child",
  "quality-ai-child",
  "quality-arch-child",
  "quality-release-child",
];

function same(a, b) {
  return JSON.stringify(canonicalize(a)) === JSON.stringify(canonicalize(b));
}

function compactQuality(results) {
  let quality = "skipped";
  for (const name of QUALITY_JOB_IDS) {
    const outcome = results[name];
    if (outcome === "failure" || outcome === "cancelled") {
      quality = "failure";
    } else if (outcome === "success" && quality !== "failure") {
      quality = "success";
    }
  }
  return quality;
}

export function expectedObservation(results) {
  return {
    laneResults: {
      governance: results["governance-child"],
      lint: results.lint,
      quality: compactQuality(results),
      contracts: results["contracts-child"],
      component: results["component-child"],
      "database-baseline": results["migrations-child"],
      "database-recovery": results["recovery-migrations-child"],
      "database-integration": results["db-integration-child"],
      experience: results["e2e-report-child"],
    },
    oracleResults: Object.fromEntries(
      Object.entries(ORACLE_RESULT_JOBS).map(([group, job]) => [
        group,
        results[job],
      ]),
    ),
  };
}

export function verifyCiCheck({
  manifest,
  summary,
  results,
  expectedDigest,
  expectedHeadSha,
  expectedRunId,
  expectedRunAttempt,
  expectedEvent,
  expectedRef,
}) {
  const failures = [];
  const manifestErrors = validateManifest(manifest);
  failures.push(...manifestErrors.map((error) => "manifest: " + error));

  if (manifest?.schema !== MANIFEST_SCHEMA) {
    failures.push("expected the current CI Check manifest schema");
  }
  if (manifest?.digest !== expectedDigest) {
    failures.push("manifest digest mismatch with Scope");
  }
  if (manifest?.headSha !== expectedHeadSha) {
    failures.push("manifest headSha mismatch with the GitHub event");
  }
  if (manifest?.event !== expectedEvent || manifest?.ref !== expectedRef) {
    failures.push("manifest event/ref mismatch with the GitHub execution");
  }
  if (
    manifest?.execution?.runId !== String(expectedRunId) ||
    manifest?.execution?.attempt !== Number(expectedRunAttempt) ||
    !/^[1-9][0-9]*$/.test(String(expectedRunId)) ||
    !Number.isSafeInteger(Number(expectedRunAttempt)) ||
    Number(expectedRunAttempt) < 1
  ) {
    failures.push(
      "manifest runId/runAttempt does not match the exact execution",
    );
  }

  if (!results || typeof results !== "object" || Array.isArray(results)) {
    failures.push("results must be a named job map");
    results = {};
  }
  const actualKeys = Object.keys(results).sort();
  if (!same(actualKeys, [...CI_JOB_IDS].sort())) {
    failures.push("job status keys do not match the exact CI graph");
  }
  for (const job of CI_JOB_IDS) {
    if (!RESULT_VALUES.has(results[job])) {
      failures.push(job + ": missing or invalid job outcome");
    }
  }
  if (results.scope !== "success") {
    failures.push("Delivery Scope did not pass");
  }
  if (results["delivery-observation"] !== "success") {
    failures.push("Delivery Observation did not pass");
  }
  if (expectedEvent === "pull_request") {
    if (results["commit-lint"] !== "success") {
      failures.push("commit-lint is required for every PR");
    }
  } else if (results["commit-lint"] !== "skipped") {
    failures.push("commit-lint must be skipped outside pull_request events");
  }

  for (const [group, jobs] of Object.entries(GROUP_JOBS)) {
    const children = Object.fromEntries(
      Object.entries(jobs).map(([name, job]) => [name, results[job]]),
    );
    const evaluation = evaluateManifestOracle({
      manifest,
      group,
      detector: results.scope,
      children,
    });
    failures.push(
      ...evaluation.failures.map((failure) => group + ": " + failure),
    );
  }
  for (const [group, job] of Object.entries(ORACLE_RESULT_JOBS)) {
    if (results[job] !== "success") {
      failures.push(group + " Oracle did not pass");
    }
  }

  const wantsE2e =
    manifest?.lanes?.experience === true || manifest?.lanes?.full === true;
  if (wantsE2e && results["e2e-child"] !== "success") {
    failures.push("required E2E shards are missing, skipped or failed");
  } else if (!wantsE2e && results["e2e-child"] !== "skipped") {
    failures.push("unselected PR E2E must stay skipped");
  }

  const observation = expectedObservation(results);
  if (!summary || typeof summary !== "object" || Array.isArray(summary)) {
    failures.push("Delivery Observation artifact is missing");
  } else {
    if (summary.schema !== SUMMARY_SCHEMA) {
      failures.push("Delivery Observation schema mismatch");
    }
    if (summary.digest !== computeSummaryDigest(summary)) {
      failures.push("Delivery Observation digest mismatch");
    }
    if (
      summary.manifestDigest !== manifest?.digest ||
      summary.headSha !== manifest?.headSha ||
      summary.baseSha !== manifest?.baseSha ||
      summary.event !== manifest?.event ||
      summary.risk !== manifest?.risk
    ) {
      failures.push("Delivery Observation uses a different manifest identity");
    }
    if (!same(summary.laneResults, observation.laneResults)) {
      failures.push(
        "Delivery Observation lane statuses differ from actual jobs",
      );
    }
    if (!same(summary.oracleResults, observation.oracleResults)) {
      failures.push(
        "Delivery Observation Oracle statuses differ from actual jobs",
      );
    }
    if (
      !same(
        summary.enabledLanes,
        LANE_NAMES.filter((name) => manifest?.lanes?.[name]),
      ) ||
      !same(summary.failedLanes, []) ||
      !same(summary.failedOracles, [])
    ) {
      failures.push("Delivery Observation lane/Oracle coverage is incomplete");
    }
  }

  const receipt = {
    schema: "bodysense.ci-check-receipt/v1",
    runId: String(expectedRunId),
    runAttempt: Number(expectedRunAttempt),
    event: String(expectedEvent),
    ref: String(expectedRef),
    headSha: String(expectedHeadSha),
    manifestDigest: manifest?.digest ?? null,
    summaryDigest: summary?.digest ?? null,
    results: canonicalize(results),
    passed: failures.length === 0,
    failures,
    generatedAt: new Date().toISOString(),
  };
  const { generatedAt: _time, ...identity } = receipt;
  receipt.digest = sha256Json(identity);
  return { ok: receipt.passed, failures, receipt };
}
