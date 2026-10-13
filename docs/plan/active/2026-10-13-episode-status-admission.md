# BodySense V2 · DS-BS-02 / ENG-BS-02 · Explicit Episode course-evidence admission

Status: **LOCALLY VERIFIED — exact-SHA GitHub CI and whole-domain acceptance pending** (2026-10-13).
Authority: [V2 four-track master](./2026-10-13-v2-parallel-master.md) · [BodyState product decisions](../../product/domain/bodystate.md) · [Episode v7/v8 source contract](./2026-10-13-bodystate-viewmodel-episode-handoff.md) · [handoff register](./v2-handoff-state.json).

## Bounded capability

This engineering slice introduces a **user-authorized source-evidence command**, not a new user-editable disease or recovery state. The canonical ReadModel `GET /api/v1/body-state/episodes/{id}` remains the only status-projection authority.

1. A user can record an explicit, typed course report on a **self-reported `symptom` Fact**, via the already authenticated `recordBodyFacts` protocol. `details.episode_course_report` is optional and supports only `explicit_current_report` or `explicit_resolution`. With `explicit_current_report` the same Fact must report `presence=present`; with `explicit_resolution` it must report `presence=absent`. Bare `presence=absent`, absence of further reports, improvement, elapsed days, AI speculation or an unlabeled source never yields an Episode status claim.
2. The Fact and its **original source** must be accepted, current, available, owned by the authenticated user and already linked **as a primary symptom source** to the exact current Episode ID. This admission path deliberately accepts only `source_kind=direct_edit` to avoid granting AI-extracted natural-language data an implicit High-confidence status decision. Verified high-confidence message admission is a separate future controlled workflow.
3. `POST /api/v1/body-state/episodes/{id}/status` uses the existing strict `BodyEpisodeAssociationRequest` (`idempotency_key`, `expected_revision`, `fact_id`). No `state`, `claim`, `as_of`, guessed clinical condition or client-supplied authorization ID is accepted. Go reads the already stored Fact's typed report and temporal expression and verifies the Episode's exact source relation.
4. A **single PostgreSQL revision transaction** admits `episode_state_evidence` linked to the same `source_evidence_id` already used by `episode_fact_links`; it appends one `BodyStateRevision`, one outbox event and the durable **ID-only** `EpisodeCommandReceipt` operation `status`. The receipt is checked _before_ revision/source checks so a retry after source correction, erasure or later edits cannot duplicate the semantic write.
5. Existing `ProjectExplicitEpisodeState` rebuilds current/ended/unknown from **available, asserted sources** and explicit relative report time. It never substitutes system recording time for unknown business time; mixed incomparable times remain `unknown/conflicting_evidence`. Source withdrawal or erasure drops an invalid claim from active projection, but retains required audit history pending controlled erasure.
6. A status command is forbidden on a non-current, other-user or completely unsupported Episode. A new current report on a source-verified already-ended Episode cannot silently reopen it; a separately user-authorized recurrence identity is required (still future work). Duplicate evidence for the same exact Fact/Episode is rejected, and the original idempotency key can only replay its original fingerprint.

## Physical change and generated contract

- Additive migration: `000009_episode_status_receipts.up/down.sql` adjusts the **existing** receipt operation constraint from `create/link` to `create/link/status`. No new authority table, clinical lifecycle column or source text copy. Downgrade refuses if an accepted status receipt exists; previously published migrations remain unchanged.
- Go: `model/body_fact.go` details validation, `repository/episode_status_repository.go` (Go transaction + locks), `service/episode_command_service.go`, `transport/httpapi/episode_status_http.go`, `routes.go`.
- Generated public OpenAPI/Go/TS/Zod/Postman are derived from the one `packages/contracts/openapi/bodysense.v1.openapi.yaml`; no hand-edited generated code.
- Web: `bodyEpisodeService.admitStatus` is a strictly typed adapter for a future user-approved UI, **not an unreviewed new Penpot window or an automatic agent tool**.
- Penpot Product/Components/Flows and the official interactive design sign-off stay with **independent draft PR #250**; this engineering worktree does not edit design source or its pending changes.

## Acceptance and negative guarantees

| Gate                          | Required evidence                                                                                                                                                                                                 |
| ----------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Strict Fact admission         | present/current and absent/complete report accepted **only if explicit**; invalid or mismatched course/presence, extra fields, fabricated clinical state rejected before data persistence                         |
| Episode identity & source     | Exact authenticated user, direct-edit original, valid primary Fact link, available/asserted source, current target, no cross-user disclosure                                                                      |
| Replay / consistency          | Same key+fingerprint → original receipt; reused key with different operation/target/fact → 409; stale revision → 409; no second status for same Fact; one outbox and revision                                     |
| Clinical semantics            | Bare negative remains `unknown`; linked explicit current report can produce `ongoing` as of its business date; explicitly later resolution produces `ended`, but unknown/conflicting time never invents certainty |
| Withdrawal, rollback, privacy | Blocked/withdrawn source removes active claim; all writes rollback atomically if receipt creation fails; no original statement in generic revision/outbox                                                         |
| Schema & transport            | PostgreSQL 18 full up/down/up on isolated database; v9 downgrade with active accepted status receipt refuses data loss; real HTTP with generated validator; generated contract remains deterministic              |
| Project release               | Full SF-20, ViewModel, design/pixel, integration and production gates remain **OPEN** until separately signed off                                                                                                 |

## What remains

This is _not_ automatic High-confidence Episode linking, user-authorized merge/split/unlink/recurrence, complete temporal relations, safety incident handling, Observation/Hypothesis/ConcernGroup, singleton Chat or final Person/Episode UI. It does not decide medical etiology or make inferred clinical resolution claims. No staging or production migration is performed. Local evidence: `episode-status-pg18-v9-validated.log` demonstrates PostgreSQL 18 full-up/v8-down/v9-up, race-enabled Go repository/HTTP/consultation integration and the new Status source, replay, denied message provenance, recurrence and rollback cases. `episode-status-web-full.log` confirms 73 files / 453 Web tests plus lint/typecheck/build. Contract verification and cross-process browser evidence have separate logs in `.runtime/schema-first-v2-evidence/`; exact GitHub CI is required on the subsequently pushed PR #247 SHA.

## Local acceptance evidence (verified; exact-SHA CI pending)

- **Database/revision:** `scripts/validate-body-fact-ledger.sh` PASSED in isolated native PostgreSQL 18 with `FULL_UP=9`, `LATEST_DOWN=8`, `LATEST_REPLAY_UP=9` before and after race tests. A downgrade against a populated v9 `status` receipt intentionally fails closed.
- **Go/status HTTP:** 6/6 v9 `SymptomEpisodeStatus*` PostgreSQL test cases and `TestBodyEpisodeHTTPExplicitStatusAndRevocationPostgres` passed inside full race-enabled repository/transport/consultation test run. The tests verify direct-edit-only, real user_message rejection, implicit-negative rejection, identity and source ownership, conflict, rollback, concurrent exact replay, distinct recurrence and source withdrawal projection.
- **Canonical OpenAPI:** `pnpm contracts:verify` PASS (lint, check-generated, breaking, mutation, conformance), plus deterministic `pnpm postman:generate && pnpm postman:verify` with **104 generated routes**.
- **Web:** full uncached `@bodysense/web` lint, Vitest **73 files / 453 tests**, TypeScript and build PASS, including typed `admitStatus` adapter validation that disallows externally supplied medical claims.
- **Cross-process end-to-end:** `scripts/validate-body-fact-vertical.sh` PASS, **15/15 Chromium tests** against real Go, PostgreSQL 18, Redis, stubbed Python AI and live React. Test environment was isolated and disposable, `BODY_FACT_VERTICAL_RESOURCES=CLEANED`, evidence `.runtime/schema-first-v2-evidence/vertical-e2e-2646867/`. This existing browser suite tests regression parity for Person Records, Episode create/link, Ask User, extraction, mobile sheets and complete longitudinal health loop; v9 status admission is independently proven at the real HTTP/DB layer because final UI is still owned by Track B.
- **Governance:** `pnpm plan:check` 5/5 and `scripts/validate-migration-history.sh` immutable manifest PASS. No staging/production code or migration applied.

**External gate:** exact new PR #247 commit-specific GitHub CI (all relevant jobs/oracles) must still complete before this slice may be labelled PR-accepted. No whole-BodyState or V2 product gate changes.
