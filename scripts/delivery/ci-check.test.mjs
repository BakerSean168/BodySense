import assert from "node:assert/strict";
import { readFileSync, mkdtempSync, writeFileSync, rmSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import test from "node:test";
import YAML from "yaml";
import {
  createManifest,
  validateManifest,
  computeManifestDigest,
  MANIFEST_SCHEMA,
  MANIFEST_SCHEMA_V8,
  POLICY_VERSION_V7,
  LANE_TARGETS_V8,
} from "./lib.mjs";
import { CI_JOB_IDS, expectedCiJobs, verifyCiCheck } from "./ci-check-lib.mjs";

const SHA_A = "a".repeat(40),
  SHA_B = "b".repeat(40);
function sample(
  event = "pull_request",
  files = ["apps/web/src/x.ts"],
  wide = false,
) {
  const ref =
    event === "pull_request" ? "refs/pull/477/merge" : "refs/heads/main";
  const manifest = createManifest({
    requestedBaseSha: SHA_A,
    baseSha: SHA_A,
    headSha: SHA_B,
    event,
    ref,
    changedPaths: files,
    forceFull: wide,
    runId: "1234567",
    runAttempt: 2,
  });
  const expected = expectedCiJobs(manifest);
  const results = Object.fromEntries(
    CI_JOB_IDS.map((job) => [job, expected[job] ? "success" : "skipped"]),
  );
  return {
    manifest,
    results,
    expectedDigest: manifest.digest,
    expectedHeadSha: SHA_B,
    expectedRunId: "1234567",
    expectedRunAttempt: "2",
    expectedEvent: event,
    expectedRef: ref,
  };
}
function rejected(original, mutate, label) {
  const f = structuredClone(original);
  mutate(f);
  const r = verifyCiCheck(f);
  assert.equal(r.ok, false, label);
  assert.ok(r.failures.length, label);
  return r;
}

test("PR and Main require exact selected outcomes without independent Oracle runners", () => {
  assert.equal(MANIFEST_SCHEMA, "bodysense.delivery-manifest/v9");
  for (const [event, files, wide] of [
    ["pull_request", ["docs/a.md"], false],
    ["pull_request", ["apps/web/src/x.ts"], false],
    ["pull_request", ["apps/api/internal/service/foo.go"], false],
    ["pull_request", ["apps/ai-service/src/foo.py"], false],
    ["pull_request", ["packages/contracts/src/x.ts"], false],
    ["pull_request", [".github/workflows/ci.yml"], false],
    ["push", ["docs/a.md"], true],
    ["schedule", [], true],
    ["workflow_dispatch", [], true],
  ]) {
    const f = sample(event, files, wide);
    assert.deepEqual(validateManifest(f.manifest), []);
    const r = verifyCiCheck(f);
    assert.equal(r.ok, true, event + " " + files.join(",") + ": " + r.failures);
    assert.equal(r.receipt.schema, "bodysense.ci-check-receipt/v2");
    assert.match(r.receipt.digest, /^sha256:[0-9a-f]{64}$/);
    assert.equal(
      f.results["e2e-child"],
      event === "pull_request" ? "skipped" : "success",
    );
    assert.equal(
      f.results["quality-release-child"],
      event === "pull_request" ? "skipped" : "success",
    );
  }
});

test("required failure, illegal skip, missing/extra result fail closed", () => {
  const x = sample("pull_request", [".github/workflows/ci.yml"]);
  for (const job of CI_JOB_IDS.filter((k) => x.results[k] === "success")) {
    for (const value of ["failure", "cancelled", "skipped"])
      rejected(
        x,
        (f) => {
          f.results[job] = value;
        },
        job + value,
      );
  }
  for (const job of CI_JOB_IDS.filter((k) => x.results[k] === "skipped")) {
    rejected(
      x,
      (f) => {
        f.results[job] = "success";
      },
      job + "unexpected",
    );
  }
  rejected(
    x,
    (f) => {
      delete f.results.lint;
    },
    "missing",
  );
  rejected(
    x,
    (f) => {
      f.results["governance-oracle"] = "success";
    },
    "extra fake oracle",
  );
  rejected(
    x,
    (f) => {
      f.results.scope = "unknown";
    },
    "unknown",
  );
});

test("wrong SHA, run, attempt, digest and stage cannot pass", () => {
  const x = sample();
  rejected(
    x,
    (f) => {
      f.expectedDigest = "sha256:" + "f".repeat(64);
    },
    "digest",
  );
  rejected(
    x,
    (f) => {
      f.expectedHeadSha = SHA_A;
    },
    "head",
  );
  rejected(
    x,
    (f) => {
      f.expectedRunId = "9876543";
    },
    "run",
  );
  rejected(
    x,
    (f) => {
      f.expectedRunAttempt = 3;
    },
    "attempt",
  );
  rejected(
    x,
    (f) => {
      f.expectedEvent = "push";
    },
    "event",
  );
  rejected(
    x,
    (f) => {
      f.manifest.execution.attempt = 3;
      f.manifest.digest = computeManifestDigest(f.manifest);
    },
    "altered run",
  );
  rejected(
    x,
    (f) => {
      f.manifest.policyVersion = "delivery-policy-v7";
      f.manifest.digest = computeManifestDigest(f.manifest);
    },
    "policy",
  );
});

test("archived v8 manifests validate but cannot masquerade as current run", () => {
  const x = sample(),
    old = structuredClone(x.manifest);
  old.schema = MANIFEST_SCHEMA_V8;
  old.policyVersion = POLICY_VERSION_V7;
  for (const lane of Object.keys(old.laneDetails))
    old.laneDetails[lane].targets = [...LANE_TARGETS_V8[lane]];
  old.digest = computeManifestDigest(old);
  assert.deepEqual(validateManifest(old), []);
  rejected(
    x,
    (f) => {
      f.manifest = old;
    },
    "v8 replay",
  );
});

test("workflow has exactly one final Check and retains parallel Test Server/E2E", () => {
  const wf = YAML.parseDocument(
    readFileSync(".github/workflows/ci.yml", "utf8"),
    { uniqueKeys: true },
  );
  assert.deepEqual(wf.errors, []);
  const jobs = wf.toJS().jobs;
  assert.equal(Object.keys(jobs).length, 14);
  assert.equal(jobs["ci-check"].name, "Check");
  assert.equal(jobs["ci-check"].if, "always()");
  assert.deepEqual([...jobs["ci-check"].needs].sort(), [...CI_JOB_IDS].sort());
  for (const removed of [
    "governance-oracle",
    "quality-oracle",
    "database-oracle",
    "experience-oracle",
    "delivery-observation",
    "governance-child",
    "quality-arch-child",
    "commit-lint",
  ])
    assert.equal(jobs[removed], undefined, removed);
  assert.equal(jobs.lint.needs, undefined);
  const lint = jobs.lint.steps.map((s) => s.run || "").join("\n");
  for (const cmd of [
    "pnpm format:check",
    "pnpm lint:oxlint:types",
    "pnpm quality:verify",
    "scripts/quality/lint-commit-range.sh",
    "scripts/delivery/*.test.mjs",
    "git diff --check",
  ])
    assert.ok(lint.includes(cmd), cmd);
  for (const job of [
    "quality-web-child",
    "quality-api-child",
    "quality-ai-child",
    "contracts-child",
    "component-child",
    "migrations-child",
    "db-integration-child",
  ])
    assert.equal(jobs[job].needs, "scope", job);
  for (const job of [
    "e2e-child",
    "quality-release-child",
    "recovery-migrations-child",
  ])
    assert.match(jobs[job].if, /github.event_name != 'pull_request'/);
  assert.deepEqual(jobs["e2e-child"].strategy.matrix.shard, [1, 2, 3]);
  assert.deepEqual(jobs["e2e-report-child"].needs, ["scope", "e2e-child"]);
  assert.match(
    readFileSync(".github/workflows/repository-governance.yml", "utf8"),
    /"contexts": \["Check"\]/,
  );
});

test("Browser caches track Playwright version instead of the whole lockfile", () => {
  const wf = YAML.parseDocument(
    readFileSync(".github/workflows/ci.yml", "utf8"),
    { uniqueKeys: true },
  ).toJS();
  for (const name of ["component-child", "e2e-child"]) {
    const steps = wf.jobs[name].steps;
    const cache = steps.find((step) => step.id === "playwright-cache");
    assert.match(cache.with.key, /playwright-version.outputs.version/);
    assert.doesNotMatch(cache.with.key, /pnpm-lock.yaml/);
    assert.ok(
      steps.some((step) => step.run?.includes("--only-shell chromium")),
    );
  }
});

test("CLI emits a versioned receipt and fails red on incomplete Lint", () => {
  const x = sample();
  const dir = mkdtempSync(path.join(os.tmpdir(), "body-check-"));
  const mf = path.join(dir, "manifest.json"),
    receipt = path.join(dir, "receipt.json");
  try {
    writeFileSync(mf, JSON.stringify(x.manifest));
    const env = {
      ...process.env,
      CI_RESULTS_JSON: JSON.stringify(x.results),
      CI_MANIFEST_DIGEST: x.expectedDigest,
      CI_HEAD_SHA: x.expectedHeadSha,
      CI_RUN_ID: x.expectedRunId,
      CI_RUN_ATTEMPT: x.expectedRunAttempt,
      CI_EVENT: x.expectedEvent,
      CI_REF: x.expectedRef,
    };
    const run = () =>
      spawnSync(
        "node",
        [
          "scripts/delivery/ci-check.mjs",
          "--manifest",
          mf,
          "--output",
          receipt,
        ],
        { cwd: process.cwd(), env, encoding: "utf8" },
      );
    const success = run();
    assert.equal(success.status, 0, success.stderr);
    assert.match(success.stdout, /CHECK=PASS/);
    env.CI_RESULTS_JSON = JSON.stringify({ ...x.results, lint: "skipped" });
    const failure = run();
    assert.equal(failure.status, 1);
    assert.match(failure.stderr, /CHECK=FAIL/);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
