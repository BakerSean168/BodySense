# Phase 06 — Typed and atomic durable state machines

Status: **COMPLETE**

Branch: `refactor/vnext-06-durable-state-machines`

Canonical parent before Phase 06: `refactor/bodysense-vnext` at `eb225db68`

## Goal

Phase 06 moves durable workflow invariants out of string conventions and read-check-write service code into typed lifecycle vocabularies, conditional mutation boundaries, database constraints, and transactions that persist authoritative lifecycle events with their state changes.

The phase does not change clinical Diagnosis/Treatment decision semantics. It hardens the persistence and concurrency semantics underneath existing behavior.

## DURABLE-001 — Canonical typed lifecycle vocabularies

Status: **COMPLETE**

The Go durable model now exposes finite lifecycle types for the main workflow aggregates rather than unconstrained status strings:

- `JobStatus`: `pending | running | waiting_user | completed | failed | cancelled | timed_out`;
- `RunStatus`: `running | waiting_user | completed | failed | cancelled`;
- `AgentInteractionStatus`: `pending | answered | cancelled | expired`;
- `AgentToolCallStatus`: `running | succeeded | failed`;
- `UploadOCRStatus`: `pending | processing | completed | failed`;
- `UploadAnalysisStatus`: `none | pending | processing | completed | failed`;
- `ConsultationPhase`: `collecting | ready_for_analysis`;
- `TreatmentStatus` / `TreatmentAcceptanceState` and `AssessmentReportStatus` are likewise finite typed domain values.

Two migration-era aliases were explicitly retired:

- Job success is only `completed`; `succeeded` is no longer a Job status;
- Consultation no longer carries the `analysis_ready` alias; `ready_for_analysis` is canonical.

`AgentToolCallStatusSucceeded` remains intentional and belongs to a different aggregate; it is not a Job compatibility alias.

Public OpenAPI / StreamEvent projections were regenerated where the canonical Consultation phase vocabulary changed, keeping the public boundary aligned with the durable model.

## DURABLE-002 — Database-enforced finite state

Status: **COMPLETE**

Migration `000063_enforce_durable_state_machines` normalizes the retired aliases and reinforces finite-state invariants at PostgreSQL boundaries for:

- jobs;
- runs;
- agent interactions;
- audited agent tool calls;
- consultation sessions and thread projections;
- upload OCR and posture-analysis lifecycles.

The migration also enforces one active run (`running` / `waiting_user`) per conversation through the active-run uniqueness constraint. Existing Treatment and Assessment migrations already enforce their finite status vocabularies, so Phase 06 does not duplicate those constraints.

The migration checksum manifest was updated during closeout; migration integrity now covers the complete SQL set through version 63.

### Migration evidence

Fresh PostgreSQL validation on the completed branch:

```text
FULL_UP=PASS version=63
LATEST_DOWN=PASS version=62
LATEST_REPLAY_UP=PASS version=63
```

The production-shaped domain validator additionally reports:

```text
BODY_STATE_SEMANTICS=PASS
BODY_REGION_ID_ROUNDTRIP=PASS
TREATMENT_ACTIVATION_ATOMICITY=PASS
OUTCOME_FEEDBACK_ATOMICITY=PASS
DOMAIN_SEMANTICS=PASS
```

## DURABLE-003 — Compare-and-set lifecycle mutation

Status: **COMPLETE**

Lifecycle mutations that can race no longer rely on an application-level read followed by an unconditional write. Repository/service paths now use conditional mutation / compare-and-set semantics so a stale terminal transition cannot overwrite a winner.

Covered paths include Job claiming/transitions, Run completion/cancellation/reconciliation, HITL Interaction transitions, audited ToolCall terminal results, Upload OCR/analysis transitions and Treatment proposal rejection.

Closeout review found one remaining Treatment race: `RejectRevision` previously read the revision state and then wrote `rejected` unconditionally. A concurrent `AcceptRevision` could therefore win first and still be overwritten by the late reject. `RejectRevision` now performs an atomic `proposed -> rejected` conditional update scoped to the owning user. If another terminal transition wins, the failed CAS re-reads the durable state and returns the correct idempotent/conflict result rather than overwriting it.

Regression coverage includes:

- completion vs cancellation single-winner Run behavior;
- PostgreSQL Run lease completion vs reconciliation single-winner behavior;
- ToolCall late terminal result rejection;
- Upload late OCR / posture failure rejection;
- Job transition compare-and-set behavior;
- Treatment late reject cannot overwrite an accepted revision.

## DURABLE-004 — Authoritative state + lifecycle event atomicity

Status: **COMPLETE**

Authoritative lifecycle facts are persisted transactionally with their state transition rather than emitted as a best-effort follow-up. Job and Run lifecycle paths were refactored so event-persistence failure rolls the state mutation back. HITL/runtime milestones use the same authoritative event boundary where the event represents the durable business fact.

Best-effort diagnostics/telemetry remain separate from authoritative lifecycle history and are not allowed to determine whether a business transition committed.

Characterization includes rollback tests proving that an event insertion failure cannot leave a committed terminal state without its corresponding durable lifecycle evidence.

## DURABLE-005 — Runtime behavior preserved

Status: **COMPLETE**

The stricter state machines preserve the existing runtime semantics:

- browser/SSE disconnect is still not business cancellation;
- explicit cancellation remains terminal and cannot be overwritten by completion;
- `waiting_user` remains resumable and is distinct from a finished run;
- execution-lost recovery survives API restart;
- BodyState -> Diagnosis -> Treatment -> Training -> Outcome remains a working longitudinal loop.

Production-shaped E2E on the final branch passed all **10/10** Playwright scenarios, including explicit cancel, API-restart recovery, lifestyle current-state replacement, the full longitudinal loop, reload durability and the real pinned 3D Body Explorer.

## Closeout findings repaired

Two issues were found by the Phase 06 closeout rather than deferred:

1. **Treatment accept/reject terminal race** — repaired with a `proposed -> rejected` compare-and-set and dedicated regression tests.
2. **Migration checksum omission** — `000063` was present but absent from `checksums.sha256`; the manifest now covers both up/down files and passes the migration integrity gate.

## Final Phase 06 acceptance

All Phase 06 acceptance statements are satisfied:

1. concurrent terminal transitions have one durable winner;
2. authoritative state + lifecycle-event mutations are transactional where both represent one business fact;
3. lifecycle vocabularies have one canonical spelling per aggregate;
4. PostgreSQL reinforces finite-state invariants where practical;
5. fresh-DB migration and latest down/up replay pass through migration 63;
6. existing clinical/runtime behavior remains green end-to-end.

Final validation evidence on the completed branch:

- `pnpm lint` — PASS;
- `pnpm typecheck` — PASS; Python Pyright **0 errors / 0 warnings**;
- `pnpm test` — PASS: Python **505/505**, Web **263/263**, Contracts **18/18**, Go repository/full suite PASS;
- `pnpm build` — PASS;
- `pnpm contracts:verify` — PASS; public REST coverage **95/95** and generated artifacts deterministic (the two pre-existing Redocly ambiguous-path warnings remain warnings only);
- `go test ./...` + `go vet ./...` — PASS;
- `git diff --check` — PASS;
- LiteLLM gateway, PydanticAI adapter and AI-service routing smoke — PASS;
- Diagnosis qualification **7/7**, Diagnosis evidence-gap policy **9/9**, Treatment qualification **4/4**, Treatment evidence-gap policy **9/9**;
- off-host DR unit tests **88/88** and DR integration backup/freshness/restore — PASS;
- fresh PostgreSQL: full up **63**, latest down **62**, replay up **63** — PASS;
- production-shaped local deploy: API/AI/Web health, posture mechanism identity, domain validators, knowledge publication/rollback verticals and Playwright **10/10** — PASS;
- final marker: `LOCAL_DEPLOY_VALIDATION=PASS`.

Phase 07 may now remove pre-vNext compatibility and migration-era runtime branches against this typed/atomic durable baseline.
