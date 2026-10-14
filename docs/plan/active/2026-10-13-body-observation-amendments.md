# BodySense V2 · DS-BS-04 / ENG-BS-04 — Direct BodyObservation Correction & Withdrawal

Status: **LOCAL END-TO-END VERIFIED — exact-SHA GitHub CI pending** (2026-10-13).

Authorities: [V2 master](./2026-10-13-v2-parallel-master.md), [BodyState BS-D9](../../product/domain/bodystate.md), [v11 self-report original](./2026-10-13-body-observation-direct-report.md), [single readiness register](./v2-handoff-state.json).

## Bounded behavior

Correct/withdraw only an **explicit original direct-user BodyObservation**. This is not a later measurement, medical symptom resolution, AI diagnosis, laboratory import, device admission, general clinical correction or personal health-data erasure.

- Correction explicitly supplies one _different_ original `RecordBodyObservationInput` and exact existing `observation_id`. It changes the truthfulness of a previous user report, not the user's later body value. The original report and original `statement_evidence` remain immutable and their assertion status becomes `superseded`. A new source-backed successor becomes the only current visible report via `supersedes_observation_id`.
- Withdrawal names one current observation, includes only expected BodyState revision and unique idempotency key; the assertion becomes `withdrawn`. It is absent from current reads, while source evidence/history are preserved under separate retention and erasure policies. A correction chain is not a clinical recurrence or Episode.
- Both commands share the v11 `body_observation_commits` global per-user idempotency namespace. A replay reproduces the original response regardless of later BodyState revision or source visibility; a different target/action/payload with the same key yields 409. A new key with stale `expected_revision` yields 409. An unavailable foreign target produces 404; a no-longer-current target produces 409.
- Atomic PostgreSQL transaction updates one owner's `BodyState` revision, source and replacement (correction only), old assertion status, ID-only audit/outbox and exact durable receipt. A late failure must roll back all changes, including original status. Returned `committed_at` is already truncated to PostgreSQL microseconds, so HTTP replay is byte-identical.
- Migration `000012_body_observation_amendments` is additive. A receipt gains server-owned `operation` and `target_observation_id`; legacy v11 `record` receipts retain their meaning. FK binds the exact owner and target; `correct` requires distinct successor ID, `withdraw` requires same target ID. The down migration refuses to silently discard accepted amendment history. Old v1–v11 SQL/checksums stay immutable.
- Strict generated public commands are `POST /api/v1/body-state/observation-records/{id}/correct` and `POST /api/v1/body-state/observation-records/{id}/withdraw`. OpenAPI, Go handlers, generated TS/Zod and one typed client adapter comprise the engineering surface. Never accept caller-chosen `user_id`, `source_kind`, `source_evidence_id`, `normalized_value`, device/lab proof, automatic AI interpretation or medical disposition.
- The v12 browser E2E exercises authenticated real Go/PG18/Python/React calls without fabricating an Observation editor. Editable Penpot final visual components, correction affordances, command confirmation, empty/error states and mobile UX remain Track B's separately owned design.

## Explicit validation gates

| Check                 | Required evidence                                                                                                                                                                                    |
| --------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Domain & schema       | Native PostgreSQL 18 forward v12 → down v11 → replay v12, immutable old SQL hashes, amendment operation/target constraints, no destructive down after saved correction/withdrawal                    |
| Identity & permission | Other user cannot correct or withdraw report; invalid source, stale revision, target mismatch, different-action key reuse and duplicate correction are rejected without touching history             |
| Historical accuracy   | Old value, old source and reported time unchanged; corrective successor owns new source; active read excludes both superseded and withdrawn originals; no symptom fact or clinical conclusion minted |
| Atomicity             | Concurrent competing user amendments create exactly one revision; final receipt failure leaves original status/revision/source unchanged; exact HTTP retry returns identical ID/time                 |
| Public interfaces     | Generated strict OpenAPI→Go and TS/Zod, Postman route parity, genuine PostgreSQL HTTP 200/400/401/404/409 and correct source availability classification                                             |
| Regression & delivery | Existing BodyFact/Episode native Go/PG18/race suite; uncached Web tests, lint/typecheck/build; new cross-process real Chromium scenario; plan check; exact new-SHA GitHub CI                         |

## Native acceptance evidence · 2026-10-13

- **PostgreSQL 18 migrations**: v12 full-up → down-v11 → replay-up-v12, immutable v1–v12 checksums **PASS**. Evidence: `/tmp/bodysense-observation-v12-pg18-guarded-final.log`, `/tmp/bodysense-observation-v12-governance.log`.
- **Real Go repository and HTTP**: `TestBodyObservationCorrectionAndWithdrawalImmutableSourcesPostgres`, source blocking/foreign/stale/noop/changed-key, five-way competing explicit authorization, late receipt failure rollback, account cascade, populated v12 downgrade refusal, and `TestBodyObservationGeneratedAmendmentHTTPRoundTripPostgres` **PASS**. Correction always creates new ID/source, old original remains immutable; withdrawn successor disappears from current read. Owner scoping is checked before revision to return foreign 404 instead of revealing current revision.
- **Strict negative contracts**: `TestAmendBodyObservationRequiresExactAuthorizationAndReplacementShape` and `TestBodyObservationGeneratedAmendmentHTTPStrictRejectsForgedSources` **PASS**, including forbidden source/device/normalized fields and missing original replacement.
- **OpenAPI / generated Go / TS / Zod / Postman**: canonical generated drift, compatibility, mutation and conformance **PASS**; Postman **110 public operations**. Evidence: `/tmp/bodysense-observation-v12-contracts.log`.
- **Web**: uncached lint, **75 test files / 464 Vitest tests**, typecheck and production build **PASS**. Evidence: `/tmp/bodysense-observation-v12-web.log`.
- **Governance**: `pnpm plan:check` **5/5**, migration sequence and immutability PASS.
- **Real cross-process Chromium**: isolated native Go API + PostgreSQL 18 + Redis + Python stub runtime + React, **17/17 tests PASS without retries**, including v12 exact correction → read → withdrawal → replay → reload/owner denial; synthetic resources cleaned. Evidence: `/tmp/bodysense-observation-v12-browser17-exact.log` and `.runtime/schema-first-v2-evidence/vertical-e2e-3530521/browser.log`. An initial attempt had one time-limit retry in an unrelated full-loop test; the later 17/17 fresh attempt had no retries.
- **Delivery pending**: publish one independent Draft PR from exact committed SHA; require all current-SHA GitHub CI gates. This local verification is not staging or production acceptance.

## Gates still open

v12 does **not** complete BodyObservation device/document/laboratory admission, explicit user-source selective erasure, controlled history retrieval, normalized measurement schemas, BodyHypothesis, DerivedConcernGroup or merge/split. It also does not imply full V2 schema freeze, domain-wide ViewModel signoff, final Penpot Observation UI, production migration, removal of legacy Observation or clinical Diagnosis/Treatment/Training signoff. All remain separately tracked.

**Delivery:** v12 must remain an independent Draft PR stacked on v11 [PR #253](https://github.com/BakerSean168/BodySense/pull/253) and hence v10 [PR #247](https://github.com/BakerSean168/BodySense/pull/247). Because GitHub CI triggers on target `main`, use `main` for exact-SHA CI but do not merge out of order. No independent Penpot Track B writes.
