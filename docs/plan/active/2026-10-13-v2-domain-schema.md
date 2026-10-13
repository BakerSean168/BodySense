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
