import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

const service = fs.readFileSync(
  "apps/api/internal/service/training_service.go",
  "utf8",
);
const repository = fs.readFileSync(
  "apps/api/internal/repository/training_repository.go",
  "utf8",
);
const native = fs.readFileSync(
  "scripts/plan/test-tp-d1-owner-lock-pg18.sh",
  "utf8",
);
const integration = fs.readFileSync(
  "apps/api/internal/repository/training_owner_lock_postgres_test.go",
  "utf8",
);
const server = fs.readFileSync("apps/api/cmd/server/main.go", "utf8");

test("TP-D1 owner lock is an explicit opt-in only, not wired to legacy Server", () => {
  assert.match(service, /WithIndependentTrainingPlans\(\)/);
  assert.match(service, /if s\.independentPlans/);
  assert.doesNotMatch(server, /WithIndependentTrainingPlans\(/);
});

test("TP-D1 repository locks a real scoped owner row in the outer transaction", () => {
  assert.match(repository, /LockOwnerForPlanActivation/);
  assert.match(repository, /database\.HasTransaction\(ctx\)/);
  assert.match(repository, /Table\("users"\)/);
  assert.match(repository, /Where\("id = \?", userID\)/);
  assert.match(repository, /clause\.Locking\{Strength: "UPDATE"\}/);
  assert.match(repository, /ErrTrainingPlanOwnerNotFound/);
  assert.match(repository, /ErrTrainingPlanOwnerLockRequiresTransaction/);
  assert.doesNotMatch(repository, /pg_advisory_lock/);
});

test("TP-D1 locks owner before proposal acceptance, not after an idempotency lookup", () => {
  const activation = service.slice(
    service.indexOf("func (s *TrainingService) AcceptTreatmentAndEnsurePlan("),
    service.indexOf("func (s *TrainingService) EnsurePlanForTreatment("),
  );
  assert.ok(
    activation.indexOf("s.lockIndependentTrainingOwner") <
      activation.indexOf("s.treatment.AcceptProposal"),
  );
  assert.ok(
    activation.indexOf("s.treatment.AcceptProposal") <
      activation.indexOf("s.EnsurePlanForTreatment"),
  );
  assert.match(activation, /s\.unitOfWork\.WithinTransaction/);
});

test("TP-D1 cannot write in an independent mode with an unlocked or other-owner context", () => {
  const projection = service.slice(
    service.indexOf("func (s *TrainingService) EnsurePlanForTreatment("),
    service.indexOf("func (s *TrainingService) GetPlanByTreatmentRevision("),
  );
  assert.match(projection, /independentTrainingOwnerLockKey/);
  assert.match(projection, /lockedOwner != userID/);
  assert.match(projection, /ErrIndependentTrainingOwnerLockRequired/);
  assert.ok(
    projection.indexOf("lockedOwner != userID") <
      projection.indexOf("CreatePlan("),
  );
});

test("TP-D1 recovery reacquires owner lock within a unit-of-work transaction", () => {
  const recovery = service.slice(
    service.indexOf("func (s *TrainingService) EnsureCurrentPlan("),
    service.indexOf("func (s *TrainingService) GetPlan("),
  );
  assert.match(recovery, /s\.unitOfWork\.WithinTransaction/);
  assert.ok(
    recovery.indexOf("lockIndependentTrainingOwner") <
      recovery.indexOf("s.treatment.GetCurrent(lockedCtx"),
  );
});

test("TP-D1 native fixture never exposes TCP or uses a shared database", () => {
  assert.match(native, /pgvector\/pgvector:pg18@sha256:[0-9a-f]{64}/);
  assert.match(native, /--pull never/);
  assert.match(native, /--network none/);
  assert.match(native, /-type s/);
  assert.match(native, /TP_D1_EPHEMERAL_SOCKET_CLEANUP=PASS/);
  assert.match(native, /listen_addresses=''/);
  assert.match(native, /socket_dir="[^"]*\/tmp\/bs-tpd1-owner-/);
  assert.match(
    native,
    /--mount "type=bind,src=\$socket_dir,dst=\/tmp\/pgsocket"/,
  );
  assert.match(native, /-name '\*\.up\.sql' \| sort -V/);
  assert.match(native, /tp-d1-multi-active-forward-preview\.sql/);
  assert.doesNotMatch(native, /--(?:publish|volume)\b/);
  assert.doesNotMatch(native, /BODYSENSE_(?:STAGING|PRODUCTION|DATABASE)_URL/);
});

test("TP-D1 native regression verifies serialization, tenant separation, rollback and fail-closed view", () => {
  assert.match(integration, /inet_server_addr\(\) IS NULL/);
  assert.match(integration, /LockOwnerForPlanActivation/);
  assert.match(integration, /firstAcquired/);
  assert.match(integration, /secondStarted/);
  assert.match(integration, /time\.After\(180 \* time\.Millisecond\)/);
  assert.match(integration, /otherResult/);
  assert.match(integration, /ErrMultipleActiveTrainingPlans/);
  assert.match(integration, /rollbackOnly/);
  assert.match(integration, /TP_D1_NATIVE_PG18_ROLLBACK_INTEGRITY=PASS/);
});
