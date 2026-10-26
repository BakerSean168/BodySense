import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";
import YAML from "yaml";
import {
  CI_JOB_IDS,
  expectedObservation,
  verifyCiCheck,
} from "./ci-check-lib.mjs";
import {
  MANIFEST_SCHEMA,
  MANIFEST_SCHEMA_V7,
  POLICY_VERSION_V6,
  LANE_TARGETS_V7,
  computeManifestDigest,
  createManifest,
  createRunSummary,
  validateManifest,
  oracleEnabledFromManifest,
} from "./lib.mjs";

const A = "a".repeat(40);
const B = "b".repeat(40);
const JOBS = {
  governance: { governance: "governance-child", lint: "lint" },
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
};

function fixture(
  event = "pull_request",
  files = ["apps/web/src/x.ts"],
  forceFull = false,
) {
  const ref =
    event === "pull_request" ? "refs/pull/473/merge" : "refs/heads/main";
  const manifest = createManifest({
    requestedBaseSha: A,
    baseSha: A,
    headSha: B,
    event,
    ref,
    changedPaths: files,
    forceFull,
    runId: "1234567",
    runAttempt: 2,
    generatedAt: "2026-10-26T01:00:00Z",
  });
  const results = Object.fromEntries(
    CI_JOB_IDS.map((name) => [name, "skipped"]),
  );
  results.scope = "success";
  results["commit-lint"] = event === "pull_request" ? "success" : "skipped";
  results["delivery-observation"] = "success";
  for (const oracle of [
    "governance-oracle",
    "quality-oracle",
    "database-oracle",
    "experience-oracle",
  ]) {
    results[oracle] = "success";
  }
  for (const [group, childJobs] of Object.entries(JOBS)) {
    const expected = oracleEnabledFromManifest(manifest, group);
    for (const [name, enabled] of Object.entries(expected)) {
      if (enabled) results[childJobs[name]] = "success";
    }
  }
  if (manifest.lanes.experience || manifest.lanes.full) {
    results["e2e-child"] = "success";
  }
  const observation = expectedObservation(results);
  const summary = createRunSummary({
    manifest,
    laneResults: observation.laneResults,
    oracleResults: observation.oracleResults,
    generatedAt: "2026-10-26T02:00:00Z",
  });
  return {
    manifest,
    summary,
    results,
    expectedDigest: manifest.digest,
    expectedHeadSha: B,
    expectedRunId: "1234567",
    expectedRunAttempt: "2",
    expectedEvent: event,
    expectedRef: ref,
  };
}

function fails(base, edit, label) {
  const input = structuredClone(base);
  edit(input);
  const verified = verifyCiCheck(input);
  assert.equal(verified.ok, false, label);
  assert.equal(verified.receipt.passed, false);
  assert.ok(verified.failures.length > 0, label);
  return verified;
}

test("v8 manifest binds exact GitHub Run ID and Attempt into the Digest", () => {
  const input = fixture();
  assert.equal(MANIFEST_SCHEMA, "bodysense.delivery-manifest/v8");
  assert.deepEqual(input.manifest.execution, { runId: "1234567", attempt: 2 });
  assert.deepEqual(validateManifest(input.manifest), []);
  assert.equal(verifyCiCheck(input).ok, true);
  fails(
    input,
    (i) => {
      i.expectedRunId = "1234568";
    },
    "cross-run replay",
  );
  fails(
    input,
    (i) => {
      i.expectedRunAttempt = "3";
    },
    "cross-attempt replay",
  );
  fails(
    input,
    (i) => {
      i.manifest.execution.runId = "1234568";
      i.manifest.digest = computeManifestDigest(i.manifest);
    },
    "new digest from different run",
  );
  fails(
    input,
    (i) => {
      i.manifest.execution.attempt = 0;
      i.manifest.digest = computeManifestDigest(i.manifest);
    },
    "invalid attempt zero",
  );
  fails(
    input,
    (i) => {
      i.expectedHeadSha = A;
    },
    "incorrect source SHA",
  );
  fails(
    input,
    (i) => {
      i.expectedEvent = "push";
    },
    "wrong event",
  );
});

test("PR Fast CI Check only passes an exact-SHA, exact-run complete status set", () => {
  for (const files of [
    ["docs/intro.md"],
    ["apps/web/src/x.ts"],
    ["apps/api/internal/model/ledger.go"],
    [".github/workflows/ci.yml"],
    ["packages/contracts/src/foo.ts"],
  ]) {
    const input = fixture("pull_request", files);
    const check = verifyCiCheck(input);
    assert.equal(
      check.ok,
      true,
      files.join(", ") + ": " + check.failures.join("; "),
    );
    assert.equal(input.results["e2e-child"], "skipped");
    assert.equal(input.results["quality-release-child"], "skipped");
    assert.equal(check.receipt.runAttempt, 2);
    assert.equal(check.receipt.headSha, B);
    assert.equal(check.receipt.manifestDigest, input.manifest.digest);
    assert.equal(check.receipt.summaryDigest, input.summary.digest);
    assert.match(check.receipt.digest, /^sha256:[a-f0-9]{64}$/);
  }
});

test("Main and Nightly require full E2E shards, Release Safety and real PG checks", () => {
  for (const event of ["push", "schedule", "workflow_dispatch"]) {
    const input = fixture(event, ["docs/a.md"], true);
    assert.equal(verifyCiCheck(input).ok, true);
    assert.equal(input.results["e2e-child"], "success");
    assert.equal(input.results["quality-release-child"], "success");
    assert.equal(input.results["db-integration-child"], "success");
    assert.equal(input.results["recovery-migrations-child"], "success");
    assert.equal(input.results["commit-lint"], "skipped");
    for (const [job, action] of [
      ["e2e-child", "skipped"],
      ["quality-release-child", "skipped"],
      ["db-integration-child", "failure"],
      ["e2e-report-child", "cancelled"],
      ["recovery-migrations-child", "skipped"],
    ]) {
      fails(
        input,
        (i) => {
          i.results[job] = action;
        },
        event + " " + job + " " + action,
      );
    }
  }
});

test("fail-closed against absent/failed/skipped required children and falsely green Oracles", () => {
  const input = fixture();
  for (const [job, action] of [
    ["scope", "failure"],
    ["lint", "skipped"],
    ["governance-child", "failure"],
    ["quality-web-child", "skipped"],
    ["quality-arch-child", "cancelled"],
    ["component-child", "missing"],
    ["governance-oracle", "skipped"],
    ["quality-oracle", "failure"],
    ["delivery-observation", "failure"],
    ["commit-lint", "skipped"],
  ]) {
    fails(
      input,
      (i) => {
        i.results[job] = action;
      },
      job,
    );
  }
  fails(
    input,
    (i) => {
      delete i.results["quality-web-child"];
    },
    "missing job",
  );
  fails(
    input,
    (i) => {
      i.results["another-green-job"] = "success";
    },
    "unknown job",
  );
  fails(
    input,
    (i) => {
      i.results["e2e-child"] = "success";
    },
    "unexpected PR E2E",
  );
});

test("digest-bound observation artifact is checked against actual individual results", () => {
  const input = fixture();
  fails(
    input,
    (i) => {
      i.summary.digest = "sha256:" + "0".repeat(64);
    },
    "corrupted observation digest",
  );
  fails(
    input,
    (i) => {
      i.summary.headSha = A;
      // Even recomputing the summary digest does not change event identity.
    },
    "wrong summary source",
  );
  fails(
    input,
    (i) => {
      i.summary.laneResults.quality = "skipped";
    },
    "misreported quality status",
  );
  fails(
    input,
    (i) => {
      i.summary.oracleResults.quality = "skipped";
    },
    "misreported Oracle",
  );
  fails(
    input,
    (i) => {
      i.summary = null;
    },
    "missing summary artifact",
  );
  fails(
    input,
    (i) => {
      i.expectedDigest = "sha256:" + "a".repeat(64);
    },
    "wrong Scope digest",
  );
});

test("v7 archived manifest retains old task policy and receives no v8 CI Check owner", () => {
  const input = fixture();
  const old = structuredClone(input.manifest);
  old.schema = MANIFEST_SCHEMA_V7;
  old.policyVersion = POLICY_VERSION_V6;
  delete old.execution;
  for (const lane of Object.keys(old.laneDetails)) {
    old.laneDetails[lane].targets = [...LANE_TARGETS_V7[lane]];
  }
  old.digest = computeManifestDigest(old);
  assert.deepEqual(validateManifest(old), []);
  assert.equal(
    old.laneDetails.governance.targets.includes("job:ci-check"),
    false,
  );
  assert.equal(verifyCiCheck({ ...input, manifest: old }).ok, false);
});

test("GitHub CI Check consumes every required job and exact-attempt artifacts", () => {
  const source = fs.readFileSync(".github/workflows/ci.yml", "utf8");
  const wf = YAML.parseDocument(source, { uniqueKeys: true });
  assert.deepEqual(wf.errors, []);
  const jobs = wf.toJS().jobs;
  assert.ok(jobs["ci-check"]);
  assert.equal(jobs["ci-check"].name, "CI Check");
  assert.equal(jobs["ci-check"].if, "always()");
  assert.deepEqual([...jobs["ci-check"].needs].sort(), [...CI_JOB_IDS].sort());
  const checkSteps = jobs["ci-check"].steps;
  const checkEnv = checkSteps.find((step) => step.env?.CI_RESULTS_JSON).env;
  const bindings = JSON.parse(checkEnv.CI_RESULTS_JSON);
  assert.deepEqual(Object.keys(bindings).sort(), [...CI_JOB_IDS].sort());
  const checkCommands = checkSteps.map((step) => step.run ?? "").join("\n");
  assert.match(checkCommands, /ci-check.mjs/);
  assert.match(checkCommands, /ci-check-receipt-v1.json/);
  assert.ok(
    checkSteps.some((s) => s.with?.name?.includes("delivery-manifest-v8")),
  );
  assert.ok(
    checkSteps.some((s) => s.with?.name?.includes("delivery-run-summary-")),
  );
  assert.ok(
    checkSteps.some((s) => s.with?.name?.includes("ci-check-receipt-")),
  );
  const delivery = jobs["delivery-observation"];
  assert.ok(
    delivery.steps.some((s) => s.with?.name?.includes("github.run_attempt")),
  );
  const candidate = fs.readFileSync(
    ".github/workflows/candidate-publish.yml",
    "utf8",
  );
  assert.match(candidate, /name: delivery-manifest-v8/);
  assert.match(
    candidate,
    /source_manifest="reports\/candidate\/source-ci\/delivery-manifest-v8.json"/,
  );
});

test("CI Check CLI writes an exact-run receipt and exits red on failure", () => {
  const input = fixture();
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "bodysense-check-"));
  const mf = path.join(dir, "manifest.json");
  const sf = path.join(dir, "summary.json");
  const of = path.join(dir, "ci-check-receipt.json");
  const env = {
    ...process.env,
    CI_RESULTS_JSON: JSON.stringify(input.results),
    CI_MANIFEST_DIGEST: input.expectedDigest,
    CI_HEAD_SHA: input.expectedHeadSha,
    CI_RUN_ID: input.expectedRunId,
    CI_RUN_ATTEMPT: input.expectedRunAttempt,
    CI_EVENT: input.expectedEvent,
    CI_REF: input.expectedRef,
  };
  try {
    fs.writeFileSync(mf, JSON.stringify(input.manifest));
    fs.writeFileSync(sf, JSON.stringify(input.summary));
    const run = () =>
      spawnSync(
        "node",
        [
          "scripts/delivery/ci-check.mjs",
          "--manifest",
          mf,
          "--summary",
          sf,
          "--output",
          of,
        ],
        { cwd: process.cwd(), env, encoding: "utf8" },
      );
    const success = run();
    assert.equal(success.status, 0, success.stderr);
    assert.match(success.stdout, /CI_CHECK=PASS/);
    assert.equal(JSON.parse(fs.readFileSync(of, "utf8")).passed, true);
    env.CI_RESULTS_JSON = JSON.stringify({ ...input.results, lint: "skipped" });
    const failure = run();
    assert.equal(failure.status, 1, failure.stdout);
    assert.match(failure.stderr, /CI_CHECK=FAIL/);
    assert.equal(JSON.parse(fs.readFileSync(of, "utf8")).passed, false);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});

test("v7 archived parallel Main keeps per-lane dispatch rather than serial monolith", () => {
  const source = fs.readFileSync("scripts/delivery/run-quality.mjs", "utf8");
  assert.match(source, /MANIFEST_SCHEMA_V7/);
  assert.match(
    source,
    /!\[MANIFEST_SCHEMA, MANIFEST_SCHEMA_V7\]\.includes\(manifest\.schema\)/,
  );
});
