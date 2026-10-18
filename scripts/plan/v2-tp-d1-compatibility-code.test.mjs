import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

// TP-003 slice 3b deleted the legacy TrainingService outright.
const legacyTrainingService = "apps/api/internal/service/training_service.go";
const repository = fs.readFileSync(
  "apps/api/internal/repository/training_repository.go",
  "utf8",
);
const main = fs.readFileSync("apps/api/cmd/server/main.go", "utf8");
const forward = fs.readFileSync(
  "scripts/plan/sql/tp-d1-multi-active-forward-preview.sql",
  "utf8",
);
const reverse = fs.readFileSync(
  "scripts/plan/sql/tp-d1-multi-active-reverse-preview.sql",
  "utf8",
);
const harness = fs.readFileSync(
  "scripts/plan/test-tp-d1-migration-preview.sh",
  "utf8",
);

test("legacy /training no longer creates, supersedes or activates plans from a Treatment", () => {
  // TP-002 slice 3 removed the Treatment aggregate and with it the TP-D1
  // opt-in compatibility path; plans are created only via PlanProposals.
  // TP-003 slice 3b then deleted the legacy TrainingService itself.
  assert.equal(fs.existsSync(legacyTrainingService), false);
  for (const legacy of [
    /SupersedePlans/,
    /AcceptTreatmentAndEnsurePlan/,
    /EnsurePlanForTreatment/,
    /GetPlanByTreatmentRevision/,
    /independentPlans/,
  ]) {
    assert.doesNotMatch(repository, legacy);
  }
  assert.doesNotMatch(main, /WithIndependentTrainingPlans/);
});

test("TP-D1 legacy single-plan view fails closed on ambiguity", () => {
  assert.match(repository, /ErrMultipleActiveTrainingPlans/);
  assert.match(repository, /Limit\(2\)/);
  assert.match(repository, /if len\(plans\) > 1/);
  assert.match(repository, /return nil, ErrMultipleActiveTrainingPlans/);
  assert.doesNotMatch(
    repository,
    /return &plans\[0\], nil[\s\S]*if len\(plans\) > 1/,
  );
});

test("TP-D1 preview forward removes only the legacy active index", () => {
  assert.match(forward, /PREVIEW ONLY/);
  assert.doesNotMatch(forward, /idx_training_plans_treatment_revision/);
  assert.match(forward, /DROP INDEX public\.idx_training_plans_user_active/);
  assert.doesNotMatch(forward, /\b(?:DELETE|UPDATE|TRUNCATE|DROP TABLE)\b/);
});

test("TP-D1 preview reverse must fail before recreating unique index on multiple active plans", () => {
  assert.match(reverse, /PREVIEW ONLY/);
  assert.match(reverse, /GROUP BY user_id HAVING count\(\*\) > 1/);
  assert.match(reverse, /RAISE EXCEPTION 'TP-D1 reverse blocked:/);
  assert.match(reverse, /CREATE UNIQUE INDEX idx_training_plans_user_active/);
  assert.doesNotMatch(reverse, /\b(?:DELETE|UPDATE|TRUNCATE|DROP TABLE)\b/);
});

test("TP-D1 preview never reaches external databases or live migrations", () => {
  assert.match(harness, /--network none/);
  assert.match(harness, /--pull never --rm -d/);
  assert.match(harness, /--memory 1g --cpus 1 --pids-limit 128/);
  assert.match(harness, /trap cleanup EXIT/);
  assert.match(harness, /ON_ERROR_STOP=1/);
  assert.match(harness, /-name '\*\.up\.sql' \| sort -V/);
  assert.doesNotMatch(harness, /idx_training_plans_treatment_revision/);
  assert.match(harness, /TP_D1_REVERSE_FAIL_CLOSED_WITH_MULTIPLE_ACTIVE=PASS/);
  assert.match(harness, /TP_D1_SYNTHETIC_RECONCILED_REVERSE=PASS/);
  assert.doesNotMatch(harness, /--(?:publish|volume|mount)\b/);
  assert.doesNotMatch(harness, /BODYSENSE_(?:STAGING|PRODUCTION|DATABASE)_URL/);
  const migrations = fs.readdirSync("apps/api/migrations");
  assert.equal(
    migrations.some((name) => /000013.*training.*active/i.test(name)),
    false,
  );
});
