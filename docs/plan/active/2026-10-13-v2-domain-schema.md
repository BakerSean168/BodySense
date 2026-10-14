# BodySense V2 · Domain & Schema Workstream

Status: ACTIVE / TRACK A. [Primary program](./2026-10-13-v2-parallel-master.md) · [Readiness register](./v2-handoff-state.json) · [Original SF engineering ledger](./2026-10-13-schema-first-full-refactor.md).

## Goal and contract boundary

Publish a stable **business domain + PostgreSQL persistence + presentation/query/command/event contract** one domain at a time, so Product Design and Engineering can work in parallel without deriving product UI from SQL tables. The approved business decisions remain in [docs/product](../../product/README.md); this track may not silently revise them.

Current evidence: target business domains documented in [domain/README](../../product/domain/README.md); the implemented additive migrations are `apps/api/migrations/000002_body_fact_ledger.*` and `000003_message_fact_source.*` on PR #247. Baseline `000001` still creates old domain objects. Thus the **BodyFact ledger is schema-tested**, but BodyState's complete Episode/query model and the other V2 schemas are not ready. No new migration is claimed merely by creating this plan.

## Definition of handoff (DS-GATE-01)

A domain's `view_model` gate may change to `ready` only after the following have an explicit linked, versioned artifact and targeted test or review evidence:

1. **Aggregate and ownership:** canonical IDs, cardinality, user/tenant boundary, cross-aggregate read-only links, temporal scope, classification/source distinctions, allowed lifecycle states.
2. **Presentation queries:** named list/detail/current projections by window; display fields and units, nullable/absent distinctions, provenance/freshness/visibility, derived counts and sorting/pagination; no raw technical revision exposed to users. Define loading/empty/denied/stale/partial conditions.
3. **Commands:** typed request, precondition, actor/consent, expected revision, idempotency, success receipt, negative/conflict/duplicate behavior, exact refresh target; patient statement is not clinical verification.
4. **Events and Agent boundary:** Go-owned event/commit identity, causality and replay, redacted payload policy, Python tool proposal versus actual persistence, stream/interruption IDs and rollback.
5. **Cross-domain safety and privacy:** provenance, stable evidence version, source deletion and data erasure, no false clinician claims, independent SafetyState; forbid unclear defaults.
6. **Evidence:** product rule IDs + acceptance cases, canonical OpenAPI/internal Proto/stream schema paths, generated type checks, and a specific owner of remaining gaps.

A `database` gate additionally requires executable forward and reverse/controlled-replay migration on **verified synthetic BodySense PostgreSQL**, tenant isolation, referential integrity, revision/idempotency/concurrency tests, and a data transition/rollback plan. A conceptual ERD or generated DTO is insufficient.

Handoff items are not duplicated as another `docs/product` source. Each accepted per-domain read/command shape must be committed in the canonical transport/schema sources, with concise presentation mapping and evidence references in this plan and `v2-handoff-state.json`. SQL remains storage, not component props.

## Per-domain presentation mapping (targets, not completed API contracts)

| Domain                          | Canonical persistent owner                                                                | Required named presentation queries                                                    | Critical operations/negative paths                                                                   |
| ------------------------------- | ----------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| BodyState                       | BodyFact, Episode, Observation, user Hypothesis, SafetyState                              | Person Records/Habits; Episode timeline/detail; selected body-region summary           | Record explicit statement; correct vs temporal update; withdraw; user/AI attribution; stale revision |
| Conversation / Chat             | One Conversation, user/assistant Message, serial Run, pending Ask User, retention cursors | Current timeline, active Run, pending interaction, recovered composer                  | User send; exact Ask User response; resume; conflict; source-copy-before-prune                       |
| Diagnosis V2                    | One Diagnosis, ConditionEntry, AnalysisRun/Proposal/Decision/Revision                     | Adopted condition list/count/details; evidence/admissibility; pending proposal batch   | Exact accept/reject; supersede stale decision; atomic multi-proposal commit; safety_blocked          |
| Training / Safety               | TrainingPlan, PlanProposal, Trainer session logs; SafetyState in BodyState                | Current accepted plan/template; proposal diff; Trainer progress; matching restrictions | Accept change; complete/skip a set; pause/resume; fail-closed gate; timezone snapshot                |
| Profile / Equipment / UserAsset | Minimal Profile, Equipment, shared UserAsset, ExtractionRun and source links              | Person identity popover; Equipment; Media; Reports and source availability             | Edit preferences; asset permission; remove original vs erase health data; OCR admission/review       |

UI status also requires explicit distinction between `persisted`, `pending`, `rejected`, `needs_review` and `unavailable` where relevant; do not put medical classification into presentation-layer inference. Avoid a generic `BodyConcern` table or a second BodyState owner.

## Ordered executable tickets

### DS-BASE-01 — Canonical contract inventory and boundary map

**Scope:** Inventory `apps/api/migrations`, Go model/service/repository/transport, OpenAPI/Zod generation, `contracts/internal/agent-runtime`, stream schema and existing React consumers. Write a per-domain mapping of legacy→replacement aggregates/queries/commands with stable IDs and required old API retirement. **Test:** all referenced owners/paths actually exist; cross-check [acceptance](../../product/acceptance.md). **Done when:** initial field-level contract inventory exists and links to real code, no speculative fields are marked deployed.

### DS-BS-01 — BodyState Records/Habits and Episode view/command contract

**Scope:** Precisely define query projections by Person Records/Habits, statement source/fact categories, Episode temporal links, annotation vs correction, derived concern projection, copy-minimum and expected revision. **Dependencies:** SF-10/11 and [BodyState rules](../../product/domain/bodystate.md). **Acceptance:** positive/negative/ambiguous user statements and unknown event time are representable without inventing clinical semantics; new/old consumers identified; typed endpoints + reproducible tests before `view_model=ready`. **Non-goal:** do not implement Trainer-linked unverifiable facts or choose a medical threshold.

### DS-BS-02 — Complete BodyState storage and migration

**Scope:** Forward migration for Episode identity/link/revisions and source evidence; deterministic projection and corrections/withdrawal; controlled legacy read-path retirement. **Dependencies:** DS-BS-01. **Acceptance:** PostgreSQL native full up/down/up on isolated instance, concurrent idempotency, cross-user and rollback tests. Preserve 000001 immutable and previously published migration checksums. **Reference:** SF-20.

### DS-PC-01 — One Conversation + Ask User + retention boundary

**Scope:** Query/command shapes for a single Conversation, serial Run, stable interaction/run/tool IDs and three retention cursors; protect source excerpt transfer before raw-message prune. **Acceptance:** schema and generator checks plus cross-device/concurrent-user negative cases; no public health-sharing route in target UX. **Reference:** SF-30/SF-70; [Person Chat](../../product/domain/person-chat.md).

### DS-DX-01 — Diagnosis condition/proposal/decision contract

**Scope:** ConditionEntry, AnalysisRun append-only audit, exact version/fingerprint decision and atomic commit; evidence relation and safety interrupt. **Acceptance:** AI/clinical/user evidence provenance distinct; no arbitrary Q-17 TTL; multiple pending decisions cannot partly commit. **Reference:** SF-40; [Diagnosis](../../product/domain/diagnosis.md).

### DS-TP-01 — Training, Trainer, independent Safety

**Scope:** accepted plans, preview proposals, execution per-set actions, independent restrictions and user timezone. **Acceptance:** Trainer cannot mark completed from navigation; Safety restricted/urgent fails closed; no implicit calendar. **Reference:** SF-50; [Training](../../product/domain/training.md), [Safety](../../product/domain/safety.md).

### DS-ASSET-01 — Minimal Person, shared assets and erasure

**Scope:** Profile, Equipment, shared Media/Reports identity, extraction admission and two distinct deletion commands; mutable source availability status. **Acceptance:** retention/provenance and controlled-copy erasure semantics, account isolation, fail-closed evidence after source removal. **Reference:** SF-60, SF-70; [Person Chat](../../product/domain/person-chat.md).

## Checkpoint and next handoff

At foundation creation: business-level decisions are documented; the comprehensive typed V2 read/query surface is **not**. The immediate next owner is **DS-BASE-01 / DS-BS-01**. Handoff to track B requires accepted display/query contract; handoff to track C persistence requires tested migrations. Update only the relevant readiness gates with exact evidence after their acceptance; leave all other domains pending.

## 2026-10-13 verified checkpoint

A: bounded Records/Habits projection and command/receipt handoff established; full Episode/Observation/Hypothesis contract remains open. See [bounded contract, changes and evidence](./2026-10-13-bodyfact-vertical-contract.md). Domain-wide gates and production release remain open.

### Explicit assertion amendments checkpoint

Migration 000004 and authenticated generated correction/withdrawal endpoints, actual Person UI and ten affected browser E2E paths are locally verified. Read [the bounded handoff and exact acceptance evidence](./2026-10-13-bodyfact-vertical-contract.md). Episode/Observation/Hypothesis and whole-domain release gates remain open. The prior failed #247 browser run is not labelled green; the next pushed SHA must pass fresh CI.

### Same-Run Ask User checkpoint

Migration 000006, typed answer validation and Go-owned structured-answer source contract are implemented and native-PostgreSQL tested. No new public transport is needed: existing canonical answer/stream/receipt APIs are reused. Full singleton/retention and Episode storage remain open. See [bounded contract, acceptance and test evidence](./2026-10-13-ask-user-source-contract.md). The latest execution registry is [v2-handoff-state.json](./v2-handoff-state.json); all whole-domain/release gates remain open.

### 2026-10-13 · BodyState bounded ViewModel and Episode v7 schema

Records/Habits presentation projection is now executable and used by the Person window (`bodyFactViewModel.ts`). [The exact Track A/Track B handoff](./2026-10-13-bodystate-viewmodel-episode-handoff.md) maps generated fields to empty/partial/stale/commit views. Additive migration `000007_symptom_episode_evidence` establishes owner-bound Episode identities, symptom Fact links and explicit state evidence, with no persisted guessed course. PostgreSQL 18 native replay and constraint/race checks passed locally. **DS-BS-02 remains IN PROGRESS**: no public Episode write/read endpoint, user-authorized link admission/merge/split, Observation/Hypothesis or full ConcernGroup.

### 2026-10-13 · Source-visible Episode list/detail contract

Canonical public read-only routes are now `GET /api/v1/body-state/episodes` and `GET /api/v1/body-state/episodes/{id}`, generated from OpenAPI into Go/TypeScript/Zod/Postman. `BodyEpisodePage` and `BodyEpisodeRecord` return creation-revision-batched identities, source-backed Fact links, conservative explicit-status projections, and owner-scoped 404 hiding when the source is unavailable. The exact physical field-to-UX mapping and accepted source visibility rules are in [§7 Episode handoff](./2026-10-13-bodystate-viewmodel-episode-handoff.md). Tested synthetic PostgreSQL 18 native v7 full-up/down/up, race-enabled owner/source blocks, cross-user denial, concurrent repeatable-read, and real HTTP generated-contract round trips. **DS-BS-02 is still IN PROGRESS**: association and user-authorized command API, relation/event replay, Observation/Hypothesis, ConcernGroup, legacy retirement and complete privacy erasure are pending.

### 2026-10-13 · v8 user-authorized Episode admission

Additive `000008_episode_command_receipts` preserves exact idempotent user-action history and privacy/cascade semantics without copying source text. The canonical public `BodyEpisodeAssociationRequest` contains exactly `idempotency_key/expected_revision/fact_id`; `BodyEpisodeCommandReceipt` returns durable IDs and one domain revision/outbox event. Go checks original owner-bound asserted `symptom` Fact plus available source; create requires `presence=present` and nonempty supported symptom type; link rejects incompatible side/type/region and second primary association. This deliberately creates only **observation** links and keeps `Episode.projection.state=unknown` absent independently admitted explicit status evidence. Real PostgreSQL 18 v8 recovery, race-enabled concurrent replay/rollback/account cascade and HTTP tests: `.runtime/schema-first-v2-evidence/episode-command-final-pg18.log`. Full [source admission contract](./2026-10-13-bodystate-viewmodel-episode-handoff.md#8-2026-10-13--explicit-user-episode-admission-schema-v8-bounded). **DS-BS-02 stays in_progress:** AI High, user-approved key boundary edits/merge/split, status claims, complete Episode relations, Observation/Hypothesis and legacy retirement remain open.

### 2026-10-13 · DS-BS-02 explicit Episode course evidence schema v9 (bounded)

The existing v7 `episode_state_evidence` remains the _single_ replayable source of Episode course claims. New forward-only v9 migration extends `episode_command_receipts.operation` to `status`, with a **downgrade refusal if status receipts exist**; v7/v8 and older historical SQL and hashes remain immutable. The canonical symptom Fact now supports optional `episode_course_report` (`explicit_current_report` / `explicit_resolution`), validated against positive/negative user presence without treating a bare negative as an explicit end. The new direct-edit-only command consumes a linked, asserted and source-available self Fact and stores one evidence basis + revision + outbox + receipt; status is reprojected, never directly assigned. PostgreSQL 18 full up/down/up and source/recurrence/rollback tests are the acceptance gate. [Schema, limits, transport and negative cases](./2026-10-13-episode-status-admission.md). **DS-BS-02 remains in progress** pending relations, correction admission, Observation/Hypothesis and legacy retirement.

### 2026-10-14 · DS-TP-01 Training/Safety specification-only handoff

A bounded [TrainingPlan/Trainer/SafetyState target contract](./2026-10-14-ds-tp-01-training-safety-handoff.md) now maps approved plan/session/restriction identities, named presentation queries, exact authorization/commit receipts, conservative Trainer gate semantics and reproducible negative-case ownership. The isolated reference tests `scripts/plan/v2-trainer-safety-model.test.mjs` (12 Safety gate/lift cases) and `scripts/plan/v2-trainer-actions-model.test.mjs` (11 Trainer action/session/retention-of-history cases) exercise 23 synthetic scenarios from TP-D6–D10 and SAFE-D1–D8; it is **specification-level evidence only**, not a real Go transaction, medical classification, or clinical policy validator. This is **NOT** a generated DTO, persisted migration, validated clinical safety policy, or `view_model=ready` evidence. Track C must accept typed transport and then deliver isolated native PostgreSQL/Go behavior; Track D owns integrated gate/replay/permission acceptance. The single handoff JSON is unchanged while shared writer coordination and domain acceptance remain open.

### 2026-10-13 · DS-BS-02 bounded recurrence relation (v10)

The persisted recurrence is no longer an implied merge or Episode reopening. Additive `000010_episode_recurrence_links` adds one owner/source/authorization-FK-bound `episode_relations` table and extends only the existing durable receipt operation to `recurrence`; the old and new Episode identities, Fact truth, and independent v9 end/current evidence stay intact. `ReportEpisodeRecurrence` requires a **direct user command**, exact old/current-ended and distinct source-visible new Episode, primary asserted source Fact, optimistic revision and idempotency; contradictory precisely known business chronology is rejected. `ListEpisodeRelations` supplies a separate repeatable-read projection, with `supported` / `needs_review` on source correction, without inventing a common disease. [Bounded v10 contract](./2026-10-13-episode-recurrence-relations.md). **DS-BS-02 remains in_progress:** formal merge/split authorization, complete source correction history, Observation/Hypothesis/ConcernGroup and legacy retirement remain unsolved.
