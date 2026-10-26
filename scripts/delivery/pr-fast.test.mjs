import assert from "node:assert/strict";
import test from "node:test";
import {
  createManifest,
  computeManifestDigest,
  evaluateManifestOracle,
  oracleEnabledFromManifest,
  validateManifest,
} from "./lib.mjs";

const SHA_A = "a".repeat(40);
const SHA_B = "b".repeat(40);

function manifest(event, files, forceFull = false) {
  return createManifest({
    requestedBaseSha: SHA_A,
    baseSha: SHA_A,
    headSha: SHA_B,
    event,
    ref: event === "pull_request" ? "refs/pull/464/merge" : "refs/heads/main",
    changedPaths: files,
    forceFull,
    generatedAt: "2026-10-26T00:00:00Z",
  });
}

test("PR Fast excludes full E2E and serial monolith even for global/unknown changes", () => {
  for (const files of [
    ["apps/web/src/features/profile/Profile.tsx"],
    ["apps/api/internal/service/user.go"],
    ["apps/ai-service/src/runtime/agent.py"],
    ["packages/contracts/src/shared.ts"],
    ["apps/api/migrations/000061.sql"],
    [".github/workflows/ci.yml"],
    ["unknown-new-root-file.toml"],
  ]) {
    const selected = manifest("pull_request", files);
    assert.equal(selected.stage, "pr-fast", files.join(","));
    assert.equal(selected.lanes.experience, false, files.join(","));
    assert.equal(selected.lanes.full, false, files.join(","));
    assert.deepEqual(selected.laneDetails.experience.files, []);
    assert.deepEqual(validateManifest(selected), []);
    if (files[0] === ".github/workflows/ci.yml") {
      for (const lane of [
        "web",
        "api",
        "ai",
        "contracts",
        "database",
        "component",
      ]) {
        assert.equal(selected.lanes[lane], true, lane);
      }
    }
  }
});

test("PR fast forceFull broadens lower-layer tests but never promotes to full E2E", () => {
  const selected = manifest("pull_request", ["docs/ops.md"], true);
  for (const lane of [
    "governance",
    "web",
    "api",
    "ai",
    "contracts",
    "database",
    "component",
  ]) {
    assert.equal(selected.lanes[lane], true, lane);
  }
  assert.equal(selected.lanes.full, false);
  assert.equal(selected.lanes.experience, false);
  assert.deepEqual(validateManifest(selected), []);
});

test("Main push and Nightly retain complete E2E, full validation and database recovery", () => {
  for (const event of ["push", "schedule", "workflow_dispatch"]) {
    const selected = manifest(event, ["docs/ops.md"], true);
    assert.deepEqual(validateManifest(selected), []);
    assert.equal(
      selected.stage,
      event === "schedule" ? "nightly-audit" : "main-full",
    );
    assert.equal(selected.lanes.full, true);
    assert.equal(selected.lanes.experience, true);
    assert.deepEqual(oracleEnabledFromManifest(selected, "database"), {
      "database-baseline": true,
      "database-recovery": true,
      "database-integration": true,
    });
    assert.deepEqual(oracleEnabledFromManifest(selected, "experience"), {
      experience: true,
      component: true,
    });
  }
});

test("PR PostgreSQL migrations require DB baseline but not production DR or full E2E", () => {
  const selected = manifest("pull_request", ["apps/api/migrations/000061.sql"]);
  assert.equal(selected.lanes.database, true);
  assert.deepEqual(oracleEnabledFromManifest(selected, "database"), {
    "database-baseline": true,
    "database-recovery": false,
    "database-integration": true,
  });
  assert.deepEqual(oracleEnabledFromManifest(selected, "experience"), {
    experience: false,
    component: false,
  });
  const omittedRecovery = evaluateManifestOracle({
    manifest: selected,
    group: "database",
    children: {
      "database-baseline": "success",
      "database-recovery": "skipped",
      "database-integration": "success",
    },
  });
  assert.equal(omittedRecovery.ok, true);
  const missingDb = evaluateManifestOracle({
    manifest: selected,
    group: "database",
    children: {
      "database-baseline": "missing",
      "database-recovery": "skipped",
      "database-integration": "success",
    },
  });
  assert.equal(missingDb.ok, false);
});

test("v6 execution stage is bound to event and Digest; PR cannot pretend Main", () => {
  const selected = manifest("pull_request", [".github/workflows/ci.yml"]);
  selected.stage = "main-full";
  selected.digest = computeManifestDigest(selected);
  assert.ok(validateManifest(selected).some((s) => s.includes("stage")));
  selected.stage = "pr-fast";
  selected.lanes.experience = true;
  selected.digest = computeManifestDigest(selected);
  assert.ok(validateManifest(selected).some((s) => s.includes("PR Fast")));
});

import { readFileSync } from "node:fs";
import YAML from "yaml";

test("PR requires both the independent Lint and the existing Governance child", () => {
  const selected = manifest("pull_request", ["docs/ops.md"]);
  const missingLint = evaluateManifestOracle({
    manifest: selected,
    group: "governance",
    children: { governance: "success" },
  });
  assert.equal(missingLint.ok, false);
  assert.ok(
    missingLint.failures.some((x) => x.includes("lint: expected success")),
  );
  const failedLint = evaluateManifestOracle({
    manifest: selected,
    group: "governance",
    children: { governance: "success", lint: "failure" },
  });
  assert.equal(failedLint.ok, false);
  const complete = evaluateManifestOracle({
    manifest: selected,
    group: "governance",
    children: { governance: "success", lint: "success" },
  });
  assert.equal(complete.ok, true);
});

test("workflow runs Lint without Changes dependency and never runs PR E2E or monolith", () => {
  const source = readFileSync(".github/workflows/ci.yml", "utf8");
  const workflow = YAML.parse(source);
  const jobs = workflow.jobs;
  assert.ok(jobs.lint);
  assert.equal(jobs.lint.needs, undefined);
  assert.ok(jobs["governance-oracle"].needs.includes("lint"));
  assert.ok(jobs["delivery-observation"].needs.includes("lint"));
  for (const job of [
    "quality-full-child",
    "e2e-child",
    "e2e-report-child",
    "recovery-migrations-child",
  ]) {
    assert.match(jobs[job].if, /github.event_name != 'pull_request'/);
  }
  const formatCmds = (steps) =>
    steps.flatMap((step) => (step.run ? [step.run] : []));
  const lintCommands = formatCmds(jobs.lint.steps).join("\n");
  assert.match(lintCommands, /pnpm format:check/);
  assert.match(lintCommands, /pnpm lint:oxlint:types/);
  const governanceCommands = formatCmds(jobs["governance-child"].steps).join(
    "\n",
  );
  assert.doesNotMatch(governanceCommands, /pnpm format:check|pnpm lint:oxlint/);
  const candidate = readFileSync(
    ".github/workflows/candidate-publish.yml",
    "utf8",
  );
  assert.match(candidate, /name: delivery-manifest-v6/);
  assert.match(candidate, /delivery-manifest-v6.json/);
});
