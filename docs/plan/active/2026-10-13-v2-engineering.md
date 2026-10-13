# BodySense V2 · Engineering Workstream

Status: ACTIVE / TRACK C. [Primary program](./2026-10-13-v2-parallel-master.md) · [Readiness register](./v2-handoff-state.json) · [Detailed SF implementation ledger](./2026-10-13-schema-first-full-refactor.md).

## Outcome and existing baseline

Build one working vertical at a time: **typed user/Agent intent → Go authentication/validation → transaction / durable evidence → authoritative receipt/outbox → React query/invalidation and visible behavior**. Keep Go as persistent domain owner, Python as proposal/extraction runtime, and React as display+intent client. Do not let Python invent trusted message/run identity, let frontend infer medical judgments, or treat RPC success as a durable business save.

Current worktree `/home/dev/projects/bodysense`, branch `refactor/schema-first-v2-20261013`, baseline `ce00bc0c1`. Draft PR [#247](https://github.com/BakerSean168/BodySense/pull/247) passed 12/12 CI on that exact audited SHA. SF-10/11 are **locally accepted foundations**, not production ready. SF-12 is incomplete; remaining domains are unimplemented. No staging/production migration was run.

Existing code to preserve and build on:

- `apps/api/migrations/000002_body_fact_ledger.*` and `000003_message_fact_source.*`; PostgreSQL 18 integration and native migration replay passed on synthetic data.
- `apps/api/internal/model/body_fact.go`, `service/body_fact_service.go`, `repository/body_fact_repository.go`, `transport/httpapi/body_fact_ledger.go`; Go enforces owner/source, revision, atomic receipt and idempotency.
- `packages/contracts/openapi/bodysense.v1.openapi.yaml` + generated API/TS/Zod and policy; public authenticated `GET/POST /api/v1/body-state/records`.
- `apps/web/src/features/profile/services/bodyFactRecordsService.ts` is an isolated typed adapter with tests, **not yet called by the real Person panel**.
- Existing old `body_state_facts`, diagnosis, treatment, conversation and training consumers are still active. No blanket legacy deletion until the replacement has proven equivalent target behavior.

## Project boundaries and dependency rules

| Owner        | Implementation boundary                                                                                           | Verification                                                                |
| ------------ | ----------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| PostgreSQL   | Versioned forward migrations with checksums, single tenant source of truth, durable transactional receipts/outbox | Native PostgreSQL 18 migration/up-down-up and race-enabled repository tests |
| Go API       | Auth, preconditions, domains, user/message provenance, revision and idempotency, repeatable event projection      | Go model/service/HTTP and authorization/negative-path tests                 |
| Python AI    | Extraction, typed proposals, tool-call envelopes; Go-owned runtime identity                                       | Python unit/integration tests plus real cross-process end-to-end            |
| Contracts    | Canonical OpenAPI/internal Proto/stream schema and pinned Go/TS/Zod generation                                    | `pnpm contracts:verify`, generated diff exactness and lint                  |
| Web          | Query layer, receipt-bound invalidation, local/loading/error/denied states, first functional floating components  | Vitest, TypeScript, Playwright with real server, keyboard/mobile assertions |
| CI / release | Exact SHA and delivery manifest, authoritative PostgreSQL integration and E2E                                     | Github CI/oracles and Track D staging qualification                         |

Business `ready` alone permits detailed implementation plans. A stable Track A `view_model` gate permits parallel TS/Go API and test fixtures even if schema work continues. Mark `engineering=ready` **only** with complete real persistence, typed API and user-visible vertical acceptance; a working mock is not a release candidate.

## Executable tickets and order

### ENG-BASE-01 — Exact generated contract and mocked frontend smoke harness

**Scope:** Inventory transport source, generated Go/TS/Zod, query/cache invalidation, current Person/Chat/Diagnosis component owners and test fixtures. **Acceptance:** one generated-contract regression test and one isolated empty/error/receipt fixture for the first BodyState view, with no fake success; all source paths grounded. Avoid inventing a second parallel client generator.

### ENG-BS-01 — SF-12 real chat-to-Person BodyFact vertical (CURRENT)

**Dependencies:** existing SF-10/11 ledger and DS-BS-01 projection handoff. **Steps:** 1) trace Go-authored user Message/Run/Turn ID and Python consultation tool boundary; 2) validate typed clear user self statement, exclude third-party/AI speculative and ambiguous claims; 3) invoke server-bound `RecordMessageFacts` with stable retry and expected revision; 4) render GET projection in Person Records/Habits and invalidate after true receipt; 5) add negative, retry, conflict and refresh tests. **Acceptance:** true Python→Go→PostgreSQL→React path, rejected writes never 'saved', exact replay one revision, cross-user/AI source rejected. Do not claim completion from adapter tests.

### ENG-BS-02 — SF-20 Episode and source migration

**Dependencies:** DS-BS-02 database gate and ENG-BS-01. **Scope:** Episode links, revision replay, correction vs temporal updates, valid AI hypothesis separation; replace old BodyStateFact/Observation owners/consumers without silent dual truth. **Acceptance:** BodyState acceptances BS-16..32 and cross-domain source replay.

### ENG-PC-01 — SF-30/SF-70 one Conversation and controlled history

**Dependencies:** DS-PC-01. **Scope:** serial Run/Ask User command identity and UI composer replacement, recovery and minimal source preservation; retire list/new/share UX after parity. **Acceptance:** concurrent devices, pending interaction, invalid tool decisions, pruned raw message with persisted fact source.

### ENG-DX-01 — SF-40 Diagnosis V2

**Dependencies:** DS-DX-01 and stable BodyState evidence binding. **Scope:** ConditionEntry, proposals/decisions/atomic revision, independent AI/clinical/user evidence and adopted display. **Acceptance:** no unauthorized partial commit on stale proposal or safety_blocked; history replay.

### ENG-TP-01 — SF-50 Training / Trainer / Safety

**Dependencies:** DS-TP-01 and verified SafetyState source. **Scope:** exact plan acceptance, per-set logging, timezone snapshot, scope-specific safety gate. **Acceptance:** training plan unchanged by safety, no implicit completion, unmatched restrictions do not silently affect unrelated plans.

### ENG-ASSET-01 — SF-60 plus cross-domain source erasure

**Dependencies:** DS-ASSET-01 and DS-PC-01 source/retention rules. **Scope:** minimal Profile/Equipment/shared UserAsset; source removal vs verified all-controlled-copy erasure; source admission and event replay. **Acceptance:** no deletion receipt until actual controlled-copies verified, independent evidence honored.

## Continuous definition of done

Per ticket: add behavior/negative tests, run focused checks, run relevant isolated PostgreSQL and generated contract checks, rerun affected/full CI as warranted, peer-review entire diff, update the exact ticket/SF checkpoint, record a verifiable SHA and evidence paths in `v2-handoff-state.json`. Keep transitional old routes explicitly enumerated; never remove an active consumer before the new read path is accepted. CI `success` on an unrelated SHA is not evidence.

**Next:** ENG-BS-01; the schema/doc/fixture discovery in ENG-BASE-01 can proceed independently of blocked Python or UI code. The separate hourly automation remains off.


## 2026-10-13 verified checkpoint

C: ordinary user self-report / direct-edit BodyFact vertical is implemented and locally verified; next complete corrections/withdrawals and structured source answer gaps. See [bounded contract, changes and evidence](./2026-10-13-bodyfact-vertical-contract.md). Domain-wide gates and production release remain open.
