import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

const harness = fs.readFileSync(
  "scripts/plan/test-tp-d1-multi-active-compat.sh",
  "utf8",
);
const baseline = fs.readFileSync(
  "apps/api/migrations/000001_vnext_baseline.up.sql",
  "utf8",
);
const approved = fs.readFileSync("docs/product/domain/training.md", "utf8");

test("TP-D1 synthetic PG18 harness is pinned, offline, bounded and self-cleaning", () => {
  assert.match(harness, /^#!\/usr\/bin\/env bash/m);
  assert.match(harness, /set -Eeuo pipefail/);
  assert.match(harness, /pgvector\/pgvector:pg18@sha256:[0-9a-f]{64}/);
  assert.match(harness, /docker image inspect/);
  assert.match(harness, /--pull never --rm -d/);
  assert.match(harness, /--network none/);
  assert.match(harness, /--memory 1g --cpus 1 --pids-limit 128/);
  assert.doesNotMatch(harness, /--(?:publish|volume|mount|network host)\b/);
  assert.match(harness, /trap cleanup EXIT/);
  assert.match(harness, /docker rm -f "\$container"/);
  assert.match(harness, /PostgreSQL init process complete/);
});

test("TP-D1 preflight uses canonical schema but never a shared database", () => {
  assert.match(harness, /repo_root\/apps\/api\/migrations/);
  assert.match(harness, /-name '\*\.up\.sql' \| sort -V/);
  assert.match(harness, /ON_ERROR_STOP=1/);
  assert.match(harness, /POSTGRES_DB=synthetic_tpd1/);
  assert.match(harness, /@sha256:/);
  assert.match(harness, /ROLLBACK;/);
  assert.doesNotMatch(harness, /BODYSENSE_(?:DATABASE|PRODUCTION|STAGING)_URL/);
  assert.doesNotMatch(harness, /\b(?:kubectl|ssh|scp|pg_dump)\b/);
});

test("TP-D1 V2 mode requires the baseline to allow independent active plans", () => {
  assert.match(harness, /TP_D1_EXPECTATION:-legacy/);
  assert.match(harness, /legacy\|v2/);
  assert.match(
    harness,
    /if \[\[ "\$mode" == "v2" && "\$legacy_index" != "0" \]\]/,
  );
  assert.match(harness, /TP-D1 V2 BLOCKED/);
  assert.match(harness, /TP_D1_MULTI_ACTIVE_DB_COMPATIBILITY=PASS/);
  assert.doesNotMatch(
    baseline,
    /CREATE UNIQUE INDEX idx_training_plans_user_active\b/,
  );
});

test("TP-D1 fixture distinguishes physical index and service responsibilities", () => {
  // TP-002 slice 3 removed the TreatmentRevision column and its index.
  assert.doesNotMatch(harness, /idx_training_plans_treatment_revision/);
  assert.match(
    harness,
    /GET STACKED DIAGNOSTICS violated_name = CONSTRAINT_NAME/,
  );
  assert.match(harness, /idx_training_plans_user_active/);
  assert.match(harness, /DROP INDEX public\.idx_training_plans_user_active;/);
  assert.match(harness, /TP_D1_ISOLATED_COMPATIBILITY_COUNT/);
  assert.match(harness, /synthetic B/);
  assert.match(harness, /independent owner/);
  assert.match(approved, /TrainingPlan/);
});

test("TP-D1 diagnostic mode is not a release-ready certification", () => {
  assert.match(harness, /TP_D1_LEGACY_BLOCKER_REPRODUCED=PASS/);
  assert.match(harness, /future acceptance test/);
  assert.match(harness, /MUST fail while the legacy unique index survives/);
  assert.doesNotMatch(
    harness,
    /(?:READY_FOR_RELEASE|STAGING_PROMOTION=ENABLED)/,
  );
});
