# BodySense V2 · DS-BS-02 / ENG-BS-02 · Authorized Episode recurrence relations

Status: **LOCALLY VERIFIED / exact-SHA GitHub CI pending** · 2026-10-13.
Program: [V2 parallel master](./2026-10-13-v2-parallel-master.md) · [BodyState](../../product/domain/bodystate.md) · [accepted status evidence v9](./2026-10-13-episode-status-admission.md) · [readiness gates](./v2-handoff-state.json).

## Bounded capability and product semantics

A symptom **Episode** is an organizational identity, not a medical disease or diagnosis. A source-backed old Episode may be `ended`, and a later or separately described symptom occurrence may have its own **different Episode ID**. V9 deliberately rejected re-opening an ended Episode when a new positive symptom was linked. This v10 slice adds only one **direct user-authorized, source-backed** relation kind: `recurrence_reported`.

- **Command:** `POST /api/v1/body-state/episodes/{id}/recurrences`, where `{id}` is the exact _old_ Episode ID and request contains exactly `new_episode_id`, `fact_id`, `expected_revision`, `idempotency_key`. A single authenticated user explicitly states this is a separate reoccurrence. No client `claim`, `state`, `authorization_id`, merged identity, medical cause, or shared-disease inference.
- **Preconditions:** both distinct Episodes are current, owned and source-visible; old Episode has a currently source-backed **explicitly ended** projection (not just no symptoms), the new Episode owns the exact asserted/available/direct-edit _primary_ `symptom` Fact, and symptom type, laterality and reported body regions are compatible. If two sufficiently precise user-authored business dates establish **new symptom before old complete end**, reject rather than rewriting history. Uncertain/overlapping times do **not** manufacture chronology; only the explicit user's organizational statement can authorize the link. Unreviewed Chat interpretation cannot impersonate the direct-edit source.
- **Atomic commit:** one new link in `episode_relations` with existing source FK and two owner-bound Episode FKs; `authorization_id` FK to one committed receipt (`operation=recurrence`) plus one BodyState revision and outbox event. No symptom text, duplicate Fact, diagnosis conclusion, or persisted clinical status in this relation. The old Episode's explicit end evidence is never reopened, replaced or copied.
- **Idempotency and replay:** same owner+key+exact fingerprint yields one original receipt and relation ID, even after source changes. Reusing a key for a different old Episode, new Episode or Fact is denied; a second key cannot create a duplicate from/to relation. Writes are serialized under the established BodyState owner/revision lock. Existing recurrence directed graph must remain acyclic.
- **Source-based read:** `GET /api/v1/body-state/episodes/{id}/relations?before_revision=N` returns only the owner's relevant, currently source-visible relation references from a repeatable-read snapshot. `supported` means the explicit user's organizational claim and the older Episode's end basis are both still supported; `needs_review` after the old end is corrected/withdrawn means **historically asserted relationship remains auditable but NOT clinically proven**. Withdrawal/erasure of the new basis source hides the relation from active reads. Never publish source excerpts or health statements via this relation API.
- **Migration:** `000010_episode_recurrence_links.up/down.sql` extends the existing receipt operation CHECK, adds the one canonical FK-enforced relation table, and refuses down-migration when user-authorized recurrence history exists. Historical v1-v9 migration bytes/hashes remain unchanged. Privacy count includes the new direct ownership table, with user-erasure cascades.

## Implementation map

| Layer            | Owner                                                                                                                                                            |
| ---------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PostgreSQL       | `000010_episode_recurrence_links.*` + immutable `checksums.sha256`                                                                                               |
| Domain model     | `model/episode_relation.go`, `model/episode.go` compare only precisely evidenced chronology                                                                      |
| Transactions     | `repository/episode_recurrence_repository.go` + source-scoped `episode_relation_read_repository.go`                                                              |
| Go API           | `service/episode_command_service.go`, `episode_read_service.go`, `transport/httpapi/episode_recurrence_http.go`, canonical generated strict router               |
| Browser adapter  | `bodyEpisodeService.reportRecurrence/listRelations` from generated OpenAPI/TS/Zod (not a new user-facing final Penpot window)                                    |
| Regression       | `episode_recurrence_repository_test.go`, `episode_recurrence_http_test.go`, typed frontend adapter, native PostgreSQL 18 tests, full browser longitudinal parity |
| Design authority | Independent Penpot PR #250 / Track B. No edits, fabricated review approval, or designer acceptance in this worktree.                                             |

## Acceptance gates (bounded)

1. Full isolated PG18 **up v10 → down v9 → up v10** and guarded down refusal after a saved relation; migration check manifest immutable.
2. Atomic direct user owner/Fact/old-ended/chronology checks; stale revision, unknown source, unsupported symptom, foreign ID, same Episode, key reassignment, duplicate target relation, loop, concurrent race and late receipt/relation failure all **fail closed** or replay original.
3. Exact source-backed read from **both** Episode IDs; historical old end withdrawal downgrades review state, new source blocked/withdrawn hides active relation; no original health statements in generic audit/event or DTO.
4. Go model/service/HTTP unit tests, native PG18 repository/transport race tests and real generated validator round trips, Web typed generated adapter tests plus full lint/typecheck/build.
5. Existing full longitudinal Go/PG18/Redis/Python/React Chromium browser regression, deterministic OpenAPI+Postman route parity, plan validator and exact new PR SHA CI.

## Explicit non-goals

No automatic High AI linking, no indirect Ask User tool-based authorization, no probabilistic cause scoring, no automatic recurrence/merge/split, no implementation of Observation/Hypothesis/ConcernGroup, no complete legacy schema retirement or product-wide release signoff. Existing v9 states remain the sole derivation; related Diagnosis or SafetyState never changes from this relation. No staging or production deployment, and **PR #247 remains Draft**.

This bounded slice cannot be marked accepted based on v9 CI alone. Record native PostgreSQL, HTTP, browser and **new exact SHA** evidence below when genuinely complete.

## Verified local acceptance, 2026-10-13

- **Native PostgreSQL 18:** `scripts/validate-body-fact-ledger.sh` PASS with v10 full-up, v9 down, v10 replay-up and immutable v1-v10 checksums; complete repository/HTTP/consultation race tests PASS. A populated user-authorized relation intentionally refuses destructive v10 downgrade. Evidence: `.runtime/schema-first-v2-evidence/episode-recurrence-v10-pg-final-green.log`.
- **Exact recurrence behavior:** six `TestSymptomEpisodeRecurrence*` real-DB cases PASS: source-bound write/read and exact replay, contradictory business chronology rejection, cross-owner denial, end-source correction `needs_review` versus new-source block hidden, concurrent same-key identity, late rollback, independent user-erasure cascade and graph cycle denial with unknown event time. The generated strict HTTP fake and real PostgreSQL round trip PASS, including user authorization and JSON forgery rejection.
- **Canonical contracts:** `pnpm contracts:verify` PASS (OpenAPI lint, generated drift, breaking, mutation, conformance), deterministic Postman parity **106 public operations**. Evidence: `.runtime/schema-first-v2-evidence/episode-recurrence-v10-contracts.log`.
- **Web:** uncached `@bodysense/web` lint, **73 files / 456 Vitest tests**, TypeScript and production bundle all PASS; typed `reportRecurrence/listRelations` tests prove strict source-only requests and review-state decoding. Evidence: `.runtime/schema-first-v2-evidence/episode-recurrence-v10-web-full.log`.
- **Real cross-process browser:** isolated Go + PostgreSQL 18 + Redis + Python AI stub + React in Chromium: **15/15 E2E PASS without retries**, including user/Facts, Episode association and the independent full longitudinal path; synthetic resources cleaned. Evidence: `.runtime/schema-first-v2-evidence/vertical-e2e-2871484/`. The new recurrence UI itself is not claimed as a browser flow until Track B signs off the editable Penpot interaction.
- **Governance:** `pnpm plan:check` 5/5 and migration sequence/immutability validator PASS. Branch `refactor/schema-first-v2-20261013`, [Draft PR #247](https://github.com/BakerSean168/BodySense/pull/247). **New exact-SHA GitHub CI remains mandatory** for delivery; no staging/production deployment or all-domain gate signoff.
