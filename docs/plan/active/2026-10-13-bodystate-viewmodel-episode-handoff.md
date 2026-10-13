# BodySense V2 · BodyState ViewModel and Episode evidence foundation

Status: **ACTIVE / BOUNDED IMPLEMENTATION CHECKPOINT** (2026-10-13). Owners: DS-BS-01 / DS-BS-02 / ENG-BS-02 / PD-BS-01 / IR-BS-01. [Master](./2026-10-13-v2-parallel-master.md) · [Domain & Schema](./2026-10-13-v2-domain-schema.md) · [Product Design](./2026-10-13-v2-product-design.md) · [Engineering](./2026-10-13-v2-engineering.md) · [readiness](./v2-handoff-state.json). Product authority: [BodyState BS-D1..D10](../../product/domain/bodystate.md), [approved acceptance](../../product/acceptance.md).

## 1. Implementation boundary: what is actually runnable

- **Person Records/Habits ViewModel is implemented and used.** `apps/web/src/features/body-canvas/model/bodyFactViewModel.ts` projects the already generated `BodyFactRecordPage` API. `BodyFactRecords.tsx` now consumes it. This introduces **no parallel health-data store** and no new user-visible medical interpretation.
- **Episode physical identity + evidence foundation is implemented; see §7 for the new bounded read-only public API. No Episode write API is enabled.** Migration `000007_symptom_episode_evidence`, Go structures and pure state projector `apps/api/internal/model/episode.go` are installed and tested. The migration is additive and preserves the existing `000001` baseline and `000002..000006` checksums.
- The Go read projector can interpret **only already-admitted explicit status claims**. The new owner-scoped list/detail source joins (§7) are implemented, but they **do not** grant an AI permission to mark an Episode ongoing/ended from a plain symptom Fact or time elapsed. User-authorized **write** association, merge/split/recurrence transitions, complete history/relationships, UI rendering and legacy retirement remain unimplemented.
- No live Penpot design file is modified in this batch. The following screen/component contract is the **ready-to-use Penpot design brief**, not editable Penpot acceptance.

## 2. Person Records/Habits read-model contract (actually implemented)

Authority: `GET /api/v1/body-state/records?before_revision=...`, canonical `BodyFactRecordPage` with `user_id`, `current_revision`, `facts[]`, `has_more`, `next_before_revision`. The authenticated Go service owns every fact, original statement, temporal precision and correction status; React only derives display labels and screen states.

| ViewModel property       | Derived from                                                  | UX rule                                                                                                   |
| ------------------------ | ------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| `records[].id/record`    | `fact.id` and generated discriminated `BodyFactRecord`        | Stable target for explicit correction/withdrawal; don't manufacture IDs                                   |
| `records[].label`        | Approved five `fact.kind` discriminators                      | Translation only; not a diagnosis class                                                                   |
| `records[].statement`    | `fact.statement`                                              | User statement verbatim; may be negative                                                                  |
| `records[].occurrence`   | `temporal.original_expression`, else explicit `start/end`     | Show “未说明” when absent; NEVER fall back to `recorded_at`                                               |
| `records[].recordedAt`   | `recorded_at`                                                 | Separate system-recorded timestamp                                                                        |
| `records[].isCorrection` | `supersedes_fact_id`                                          | “更正后的陈述”; different from symptom recovery                                                           |
| `currentRevision`        | `current_revision`                                            | Internal write precondition, not a default UI badge                                                       |
| `hasMore`                | Server pagination metadata                                    | Habits filters `kind=lifestyle` over the same fact ledger; count is loaded count only                     |
| `pagesDisagree`          | Different `user_id` or `current_revision` across loaded pages | Hide inconsistent older pages, block writes, request a fresh read; don't silently present mixed revisions |

### Explicit display states

| State             | Display                                                              | Actions                                                       |
| ----------------- | -------------------------------------------------------------------- | ------------------------------------------------------------- |
| `unauthenticated` | Login-needed; no old owner's data or requests                        | Log in                                                        |
| `loading`         | Non-empty loading indicator                                          | No saved inference                                            |
| `unavailable`     | Read failed, not “no records”                                        | Retry                                                         |
| `empty`           | Definitely no records in loaded, fully scanned scope                 | Start direct record or Chat                                   |
| `partial_empty`   | No matching Habits in currently loaded pages, but older pages exist  | “读取更早记录”; not false “尚未记录”                          |
| `ready`           | Asserted facts with original occurrence/record timestamps            | Correct/withdraw only with expected revision and real receipt |
| `stale`           | Cross-page revision/owner mismatch; show only first snapshot's facts | Refresh before edit, don't blend or accept stale writes       |

Separate write feedback remains `pending`, `commit_confirmed`, `commit_uncertain`, `conflict`, `committed_refresh_failed`. UI does not clear a pending command's idempotency key on uncertain transport failure. A legitimate correction changes the assertion chain; a later real symptom change creates a new fact, not a correction.

## 3. Episode storage shape (additive SQL 000007)

| Physical object          | Canonical role                                                                               | Constraints                                                                                                                                     |
| ------------------------ | -------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| `symptom_episodes`       | Stable organization ID, type, region/side, identity status and creation revision             | Unique user/ID; **no stored symptom-lifecycle state**; the identity is not a diagnosis                                                          |
| `episode_fact_links`     | Link exactly one existing **symptom** Fact and its genuine source evidence                   | Composite owner+fact+kind+source FK, at most one primary Episode for a fact; secondary links explicit; corrected links require authorization ID |
| `episode_state_evidence` | Claim `explicit_current_report` or `explicit_resolution` with source-backed temporal `as_of` | One status claim per episode/fact and owner; cannot link unrelated/foreign, non-symptom or unlinked evidence                                    |

All records bind to the existing user's BodyState revision and account-cascade relationships. The three new ownership tables also participate in the privacy dry-run inventory, with PostgreSQL tests for exact per-table counts and account-deletion cascades. This is **not** complete cross-domain erasure propagation or backup/cache destruction. Downgrade refuses to discard any real Episode/link/evidence data; the synthetic empty-database full-up / down / up validator is separate. Do not interpret these DB CHECKs as sufficient to authorize semantic links: the future Go write path must check exact user/agent source and consent; no generic endpoint exists yet.

## 4. Pure Episode status projection (implemented, not yet routed)

`ProjectExplicitEpisodeState(identityStatus, basedOnRevision, evidence)` yields:

```ts
type EpisodeStateView = {
  state: "unknown" | "ongoing" | "ended";
  state_as_of?: string; // explicit supported event time; never persistence time
  reason:
    | "insufficient_evidence"
    | "explicit_current_report"
    | "explicit_resolution"
    | "conflicting_evidence";
  basis_fact_ids: string[]; // accepted source-backed IDs at this snapshot
  based_on_revision: number; // internal consistency, not routine UI text
};
```

Evidence with superseded/withdrawn assertion, blocked/erased source or future revision is excluded. A symptom report alone, a negative symptom statement, inactivity, the clock, or absence of updates **cannot** end an Episode. An explicit later resolution can end a course if all contrary evidence is strictly older by supported business time. An explicit current report after an ended course is **not** automatically accepted as reopening the same Episode: it requires a new Episode identity / recurrence-link decision. Ambiguous temporal order or materially conflicting evidence projects `unknown` plus a reason and underlying basis; no invented threshold.

This is deliberately a strict minimal **state** projection, not the complete target `SymptomEpisode` ViewModel. The bounded public read path in §7 now includes EpisodeFactLink identifiers and some evidence visibility; **full** onset/end temporal precision, relation history, merge/split replacement, authored source-removal commands, linked Observation, user Hypothesis and ConcernGroup query composition remain to be completed.

## 5. Penpot Track B · immediate executable design brief

**Use the existing editable Product page** cited in [Track B](./2026-10-13-v2-product-design.md); keep the living 3D body scene underneath floating windows. Do not replace the Product board with static screen captures. Design components in the shared Penpot component hierarchy.

- **Person / Records:** populated, truly empty, loading, initial error/retry, recording, confirmed receipt, uncertain save/retry, conflict, committed-but-refresh-failed, correction, withdrawal, stale cross-page refresh; occurrence time and recorded time separated.
- **Person / Habits:** same underlying list component with `lifestyle` filter and loaded count; include `partial_empty` and “读取更早记录”, not a false global zero count.
- **BodyState / Episode detail:** unknown, explicitly ongoing (time-qualified “截至这次报告仍持续”), explicitly ended (with supporting source), conflicting/Ask User, invalidated identity, corrected/withdrawn basis and source unavailable; distinguish organizational identity from clinical diagnosis, and disable unimplemented merge/split controls.
- **Chat / Ask User:** current shared replacement composer, persisted answer vs fact commit, cross-device resume, failure preserves draft, narrow-screen keyboard and both themes; no second Conversation creation.
- For each UI state, annotate `query/read-model field → component property → supported user command → resulting state`, product acceptance ID, desktop and 390px Viewer evidence. No “saved/erased/diagnosed” copy without a corresponding authoritative receipt.

## 6. Evidence and incomplete gates

- UI typecheck; focused ViewModel + Person component Vitest including historical paging and stale multi-revision guard.
- Go Episode projector model tests cover insufficient evidence, exact explicit course, recurrence conflict, ambiguous/same-day time, withdrawn/erased basis and invalid labels.
- Native PostgreSQL 18 full-up/down/up at v7 plus race-enabled repository constraint tests: foreign owner, non-symptom, wrong source, second primary link, arbitrary status, id-bound source and account-erasure cascade.
- Exact SHA CI must be read after the commit/push; local green does not imply PR acceptance.

**Gate decision:** bounded Records/Habits and read-only Episode projections can serve as a design handoff. The **whole BodyState `view_model` and `database` gates remain IN_PROGRESS**: no public Episode create/link/merge/split commands, no Observation or user Hypothesis, no verified derived ConcernGroup, and legacy consumers remain. Track B may design layouts and supported states immediately, but editable Penpot sign-off remains open.

## 7. 2026-10-13 · Public Episode read vertical (DS-BS-01 / ENG-BS-02, bounded)

**Actual code:** `GET /api/v1/body-state/episodes` and `GET /api/v1/body-state/episodes/{id}` are owner-authenticated **read-only** endpoints defined in `packages/contracts/openapi/bodysense.v1.openapi.yaml` and registered via generated strict Go transport; generated TypeScript/Zod and Postman consumers are provided. Canonical internal read models live in `apps/api/internal/model/episode_read.go`, the repeatable-read repository in `apps/api/internal/repository/episode_read_repository.go` and the thin owner-validating service in `apps/api/internal/service/episode_read_service.go`. Browser calls use `bodyEpisodeService.ts`. No write route for creating, linking, concluding, merging, splitting or correcting Episode is available.

| Read field or status                                                                    | Backing authority and source rule                                                                                                            | Product presentation                                                                                                |
| --------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| `id`, `symptom_type`, `body_region_ids`, `laterality`, `created_revision`, `created_at` | Owner-bound `symptom_episodes`; listed only if a linked symptom Fact retains **available original evidence**                                 | Stable organizational identity, not clinical diagnosis; no “current” claim inferred from `created_at`               |
| `identity_status`                                                                       | Stored `current/superseded/invalidated` plus read-only effective invalidation when no currently asserted source remains                      | `invalidated` means identity lost support; it **never** means recovered                                             |
| `links[].fact_id/role/association_method/is_primary/linked_revision`                    | `episode_fact_links` JOIN matching owner, symptom-only Fact and same genuine evidence ID; Fact must remain asserted and its source available | Link to the separately owned statement; omit source text/excerpt from Episode API                                   |
| `projection.state/reason/state_as_of/basis_fact_ids/based_on_revision`                  | Recomputed deterministically from `episode_state_evidence` joined to still-asserted source-backed links and the same aggregate snapshot      | `unknown` on absent/conflicting claim; `ongoing` and `ended` only for explicit source-backed claims, time-qualified |
| `current_revision/has_more/next_before_revision`                                        | Whole-creation-revision pagination (max 20 revisions), owner snapshot pinned via PostgreSQL repeatable-read                                  | Refresh if separate pages disagree about revision; don't claim all episodes were loaded                             |

**Read-state/error matrix:** 200 + `episodes=[]` means no _source-visible_ Episode; no finding must be inferred for hidden/pruned/deleted data. 400 invalid UUID/query, 401 no authenticated user, 404 missing/other-owner/fully blocked detail (indistinguishable), 500 malformed nested/foreign projection or true DB failure without exposing health text, 503 service unavailable. Browser uses strict generated validators and rejects malformed status/false success. 200 details never embed `BodyFact.statement` or minimum source excerpt. A fully unavailable Episode identity is suppressed from both list and detail, including its `symptom_type` title; a withdrawn/superseded assertion with still-available source may appear as `invalidated/unknown` but cannot drive `ongoing/ended`.

**Executed acceptance evidence:** native PostgreSQL 18 schema v7 full-up/down/up and `-race` repository+HTTP tests in `.runtime/schema-first-v2-evidence/episode-public-final-pg18.log`. Covers owner mismatch, explicit end vs absent symptom, withdrawal and source blocking, page boundaries, full blocked identity, 404 and no raw text over actual generated HTTP transport, and a channel-barrier test where the source is blocked during an existing repeatable-read transaction. Full Go/Web/contract and exact-head CI outcomes are recorded separately and must be checked before sign-off.

**NOT accepted as complete:** user/Agent source/authorization validation on Episode mutation, automatic High links, recurrence vs corrected boundary, merge/split, link removal audit, status/fact revision replay semantics, bounded presentation of onset/end history and uncertainty, Observation/Hypothesis/ConcernGroup and Safety consumer, Penpot editable final sign-off, staging/production rollout. The broader BodyState domain `view_model/database/engineering/integration` gates stay `in_progress`; the new source-bound Episode read API is not a final Episode UI or full SF-20 replacement.
