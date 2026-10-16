# BodySense V2 · Engineering Workstream

> **2026-10-16 current checkpoint: PR6 A–E COMPLETE / MERGED.** All native BodyState writer/consumer migration, independent Safety, Web cutover, old storage/API retirement and Python/documentation/browser closeout are merged through #298 (`af3e1993b1f62930e3b680b54abab5ade456935b`). Exact-main CI passes, including PostgreSQL baseline/recovery and browser checks; [PR6 E closeout](../archive/2026-10-16-pr6-e-closeout.md) records full Chromium 25/25 without skips/retries. `PR6-001` is done in the queue and `completedPrograms` in the handoff records each A–E merge. Earlier checkpoints are historical: do not restart any PR6 slice. Remaining full-domain design, singleton Chat/retention, Diagnosis V2, TrainingPlan/Trainer, shared assets/erasure and release acceptance are separate work. Staging promotion remains gated.

Status: ACTIVE / TRACK C. [Primary program](./2026-10-13-v2-parallel-master.md) · [Readiness register](./v2-handoff-state.json) · [Detailed SF implementation ledger](./2026-10-13-schema-first-full-refactor.md).

## Outcome and initial 2026-10-13 baseline (historical)

Build one working vertical at a time: **typed user/Agent intent → Go authentication/validation → transaction / durable evidence → authoritative receipt/outbox → React query/invalidation and visible behavior**. Keep Go as persistent domain owner, Python as proposal/extraction runtime, and React as display+intent client. Do not let Python invent trusted message/run identity, let frontend infer medical judgments, or treat RPC success as a durable business save.

At the initial 2026-10-13 checkpoint, worktree `/home/dev/projects/bodysense`, branch `refactor/schema-first-v2-20261013`, baseline `ce00bc0c1`. Draft PR [#247](https://github.com/BakerSean168/BodySense/pull/247) passed 12/12 CI on that exact audited SHA. SF-10/11 are **locally accepted foundations**, not production ready. SF-12 is incomplete; remaining domains are unimplemented. No staging/production migration was run.

Initial foundation inventory (historical migration paths may now be consolidated into the baseline):

- `apps/api/migrations/000002_body_fact_ledger.*` and `000003_message_fact_source.*`; PostgreSQL 18 integration and native migration replay passed on synthetic data.
- `apps/api/internal/model/body_fact.go`, `service/body_fact_service.go`, `repository/body_fact_repository.go`, `transport/httpapi/body_fact_ledger.go`; Go enforces owner/source, revision, atomic receipt and idempotency.
- `packages/contracts/openapi/bodysense.v1.openapi.yaml` + generated API/TS/Zod and policy; public authenticated `GET/POST /api/v1/body-state/records`.
- `apps/web/src/features/profile/services/bodyFactRecordsService.ts` is an isolated typed adapter with tests, **not yet called by the real Person panel**.

Current PR6 state: legacy BodyState fact/observation/evidence storage and routes are retired through PR6-C. Diagnosis, Treatment, conversation and training still have transitional consumers; their retirement requires their own proven target verticals.

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

### Explicit assertion amendments checkpoint

Migration 000004 and authenticated generated correction/withdrawal endpoints, actual Person UI and ten affected browser E2E paths are locally verified. Read [the bounded handoff and exact acceptance evidence](./2026-10-13-bodyfact-vertical-contract.md). Episode/Observation/Hypothesis and whole-domain release gates remain open. The prior failed #247 browser run is not labelled green; the next pushed SHA must pass fresh CI.

### Same-Run Ask User checkpoint

V3 resumes the original Run from its stored native interrupt. Answer/messages/lease/milestones commit atomically; full literal self-statements use the exact accepted-answer source. Retry, source preservation, negative safety behavior and competing updates are covered. Short contextual interpretation and whole-Conversation/retention remain open; next DS-BS-02 / ENG-BS-02 and remaining ENG-PC-01. See [bounded contract, acceptance and test evidence](./2026-10-13-ask-user-source-contract.md). The latest execution registry is [v2-handoff-state.json](./v2-handoff-state.json); all whole-domain/release gates remain open.

### 2026-10-13 · Episode persistent evidence foundation and live Person ViewModel

The existing `GET /body-state/records` DTO now feeds a pure `bodyFactViewModel.ts` consumed by real Person Records/Habits. Stale cross-page versions are detected and inconsistent older pages hidden before correction; filtering out all currently loaded habits no longer produces a false global-empty state. Migration `000007` adds stable symptom_episode identity, evidence-linked primary/secondary Fact associations and explicit-status source constraints. Go `ProjectExplicitEpisodeState` reconstructs a conservative state without timeout, assumed absence or record-time inference. See [implementation handoff](./2026-10-13-bodystate-viewmodel-episode-handoff.md). **Still no Episode admission/command/query API, Observation/Hypothesis, user-confirmed merge/split or retired legacy schema. ENG-BS-02 remains open.**

### 2026-10-13 · ENG-BS-02 Episode source-bound read vertical

Implemented `EpisodeReadService`, `BodyStateRepository.ListEpisodes/GetEpisode` (one repeatable-read source snapshot), `GET /body-state/episodes` and `GET /body-state/episodes/{id}` via owner-protected strict generated routes, and typed browser read adapter. All Episode read data is derived from stable Episode identity plus currently source-available symptom Facts and explicit status evidence; no health statement text is copied into the Episode response. Missing/foreign or fully blocked identities read as unavailable; corrected/withdrawn available-source identities cannot support ongoing/ended. Tests cover negative symptom, proof-bearing end, tenant isolation, pagination, concurrent source-block snapshot, 404 and no accidental POST. See [bounded read handoff](./2026-10-13-bodystate-viewmodel-episode-handoff.md#7-2026-10-13--public-episode-read-vertical-ds-bs-01--eng-bs-02-bounded). **ENG-BS-02 remains OPEN** for Go-validated user-authorized Episode mutations, recurrence/merge/split, additional BodyState domains and legacy retirement. No UI episode detail was claimed.

### 2026-10-13 · ENG-BS-02 direct-user Episode transaction

Implemented `CreateEpisode` and `LinkEpisodeFact` with exact authenticated user + Fact ID + expected revision and retry-safe commit receipt; no self-derived ongoing/ended, no raw source text in audit/outbox. PostgreSQL `FOR SHARE` guards available original evidence during transaction, one BodyState revision+outbox per semantic command, and one primary link per Fact. The generated OpenAPI strict handlers and browser Zod adapter share a single contract. Negative/foreign/blocked/mismatched sources and forgery/stale requests fail closed. [Exact API→Source→Revision mapping](./2026-10-13-bodystate-viewmodel-episode-handoff.md#8-2026-10-13--explicit-user-episode-admission-schema-v8-bounded). **Still pending:** actual Person Episode action UI and AI High admission, complex user authorization, explicit status admission, recurrence/history relations, Observation/Hypothesis and legacy replacement; ENG-BS-02 remains open.

### 2026-10-13 · ENG-BS-02 actual Person Episode functional bridge

An independent, lazy-loaded `BodyEpisodeAssociations` component is mounted exclusively within existing **Person → 身体记录**, without modifying PR #250's editable Penpot Product/Components/Flows or existing BodyFactRecords owner. It consumes **real** generated `bodyFactRecordsService` and `bodyEpisodeService`, joins owner+whole-revision pages conservatively, offers explicit user create/link choice and a durable-receipt-dependent success message. It preserves original command ID and revision across ambiguous transport failures, distinguishes 400/404/409 from unknown commit state, and invalidates/refetches `workspace` after actual commit. The user can manually link a negative symptom as an observation; the UI **never** claims “ended/recovered” without server-backed status evidence. [Bounded UI handoff](./2026-10-13-episode-person-ui-integration.md). Browser E2E with native Go/PG18/AI stub and 390px proof is now an explicit gate; **final Penpot styling/interaction rehosting belongs to Track B**. No AI High, merges, split or diagnosis changes in this slice.

### 2026-10-13 · ENG-BS-02 source-uncertainty hardening / compositional component boundary

On top of the locally accepted real Person Episode vertical, the functional source/read/write orchestration is extracted into a single hook with stateless listing and typed symptom/association forms. This preserves the same owner-scoped Go API, generation and revision contract while letting Track B rehost presentation. An initial read failure and cached refetch failure now forbid empty-health claims and new writes; ambiguous Fact or Episode commits block editing their original exact target/input until the original idempotent command is retried or definitively rejected. Four failing pre-fix regression tests became green. [Details and verification](./2026-10-13-episode-person-ui-integration.md). No new domain Schema, automatic AI association, final Episode navigation or deployment is claimed.

### 2026-10-13 · ENG-BS-02 explicit status-evidence command

Go now has a narrow `AdmitEpisodeStatus` transaction and owner-protected strict `POST /body-state/episodes/{id}/status`, with the generated TS/Zod adapter `bodyEpisodeService.admitStatus`. The request accepts only key + expected revision + existing Fact ID, not a desired state. Admission rejects generic negative/present, absent or blocked source, foreign Episode and non-direct-edit chat source. One approved direct-report claim is linked back to its original statement, BodyState revision and receipt; repeat retries do not mint another revision. An already-ended Episode cannot be reopened by linking a newly explicit current report, requiring a new user-authorized recurrence identity. Domain-wide ENG-BS-02 and Penpot screen acceptance **remain open**. [Bounded v9 handoff](./2026-10-13-episode-status-admission.md).

### 2026-10-13 · ENG-BS-02 explicit recurrence relation command and read (v10)

Implemented strict owner-scoped `POST /body-state/episodes/{id}/recurrences` and source-filtered `GET /body-state/episodes/{id}/relations`. Go validates both real identities and old Episode explicit end, new direct-edit primary symptom Fact and non-contradictory precise event time. One transaction stores only ID-bound relation+authorization receipt, revision and outbox; receipt replay is deterministic, graph cycles are rejected and source withdrawals downgrade or hide relation views. New generated TS/Zod `bodyEpisodeService.reportRecurrence/listRelations` is an adapter, **not** final Penpot UI. [Implementation/negative-gate handoff](./2026-10-13-episode-recurrence-relations.md). Still no AI High relation mutation, formal history merge/split, full BodyState, final design or deployment.

### 2026-10-13 · ENG-BS-03 direct user Observation vertical (v11)

The independently modeled `BodyObservation` direct-user ledger uses the existing one-BodyState transaction/revision authority with the same server-owned `statement_evidence` source chain, an id-only durable receipt and owner-scoped source-available read. Canonical strict OpenAPI → generated Go and TS/Zod give `POST/GET /body-state/observation-records` without changing the old `POST /body-state/observations` consumers; no extra clinical review field, medical diagnosis, device proof or normalization are fabricated. Browser `bodyObservationService` remains a typed adapter, not an arbitrary new UI inside the separate Penpot workstream. [v11 scope and negative acceptance](./2026-10-13-body-observation-direct-report.md). **ENG-BS-03 stays bounded; the full BodyState/AI/erasure/visual acceptance remains open.**

### 2026-10-13 · ENG-BS-04 · explicit BodyObservation correction/withdrawal commands (v12)

New `CorrectDirectObservation` and `WithdrawDirectObservation` Go domain actions reuse the one owner/revision transaction and v11 per-user idempotent commit authority. A correction preserves original source, time and raw value in history and creates a new source-backed successor; withdrawal only removes one current report from the active projection. Real generated `POST /observation-records/{id}/correct|withdraw` OpenAPI/Go/TS/Zod and strict `bodyObservationService` browser adapter reject forged caller provenance, unowned or stale targets and duplicate-key different semantics, without adding final Penpot UI. No medical diagnosis/normality/recovery inference, device admission or legacy dual-write. [Implementation and negative cases](./2026-10-13-body-observation-amendments.md). Full BodyState and V2 release gates remain OPEN.
