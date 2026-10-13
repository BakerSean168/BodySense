# BodySense V2 · BodyFact vertical handoff

Status: ACTIVE / BOUNDED CONTRACT. Parent: [Domain & Schema](./2026-10-13-v2-domain-schema.md), [Engineering](./2026-10-13-v2-engineering.md), [Integration](./2026-10-13-v2-integration-release.md). Product authority: [BodyState](../../product/domain/bodystate.md), [Person / Chat](../../product/domain/person-chat.md), [interface](../../product/interface.md).

## Scope and limits

This executable handoff covers **new BodyFact statements** in Person Records and Habits. It does not declare the whole BodyState domain ready: Episode/Observation/Hypothesis, shared UserAsset and Equipment are later tickets. Explicit direct-user corrections/withdrawals are implemented in the amendment checkpoint below. No second source of truth or legacy-record mirroring is introduced. Old deployed data is not silently reclassified.

## Query and presentation mapping

Canonical API: `GET /api/v1/body-state/records`, optional `before_revision`, generated `BodyFactRecordPage`. The same aggregate query powers both tabs; Habits selects `kind=lifestyle`, Records retains all five categories. Pagination follows server whole-revision cursors and never truncates a semantic batch. All cache keys include the authenticated user ID; disabled/no identity means no data read and no stale prior-user data.

| API field                                        | Presentation                                                                   | Rule                                                                   |
| ------------------------------------------------ | ------------------------------------------------------------------------------ | ---------------------------------------------------------------------- |
| `statement`                                      | Original user assertion, preserved verbatim                                    | Not a diagnosis, not a model-authored paraphrase                       |
| `kind`                                           | Symptom / Health history / Activity / Habit / Training feedback category label | Five approved BodyFact categories only                                 |
| `temporal.original_expression` / `start` / `end` | Occurrence-time description, otherwise unknown                                 | `recorded_at` is not a fabricated occurrence time                      |
| `recorded_at`                                    | Clearly labelled recording timestamp                                           | User-local formatting; do not mutate persisted values                  |
| `body_region_ids`                                | Optional known region labels / focus action                                    | Selection is context, not an inferred symptom                          |
| `subject` / `assertion_status`                   | Self-reported valid assertions only                                            | Never label as clinically confirmed                                    |
| `current_revision`                               | Hidden optimistic concurrency input                                            | Never shown as a normal user-facing version manager                    |
| `has_more` / `next_before_revision`              | Load earlier records button                                                    | Explicit bounded paging; no claim of complete history when more exists |

Unknown fields are not invented. A populated panel may show a refresh error without erasing previously received data. Empty, initial-loading, missing identity, failed initial read and failed next-page read are distinct.

## Direct input command

Canonical API: `POST /api/v1/body-state/records`, generated `RecordBodyFactsRequest` -> `BodyFactCommit`. Explicit user input requires no additional generic confirmation. Request captures a fixed expected revision and idempotency key; retry after transport uncertainty preserves the original complete command. Revision conflict triggers refetch and a visible request to review/retry, not silent revision rewriting.

Successful response is validated against the generated receipt (owner, commit, revision, event, fact IDs). Only then clear the draft and show saved feedback. A subsequent query refresh failure is reported as **saved, refresh failed**, not as a failed write that submits the same statement a second time. No optimistic fact insertion or invented Outbox-delivery event.

## Chat extraction boundary

Python may emit a typed `record_body_state` tool proposal; it may not supply trusted user/message identity or a success receipt. Go binds the proposal to its own Run/Turn and original user message, validates the statement/regions, then uses the existing atomic ledger. A normal `tool.result` reaches the browser only after it contains a real Go receipt or an explicit rejected result. The Python tool acknowledges extraction only, never persistence it has not observed.

Runtime extraction must exclude generic questions, third-party/hypothetical/quoted claims, AI-authored content and unresolved ambiguity. Negative assertions preserve their negative meaning. Corrections/withdrawals do not masquerade as append-new-fact commands. Safety, symptoms and clinical diagnoses remain separate.

## Integration / prior UI audit

The existing floating-window implementation from UI PR #244 is integrated only into the unreleased refactor branch for repair. Its prior Governance failure is a trailing blank line in ConsultationPage.tsx, not a domain violation. Two browser E2E failures still expected the retired `expand training execution` control; repair them to exercise the actual accepted plan's Start Training -> Trainer path and keep their persisted-plan assertions. Do not remove assertions merely to turn tests green.

## Required evidence

Generated boundary tests; panel initial/empty/error/read/pagination and stable-retry tests; actual Person tab usage; Go source/tenant/transaction tests; Python proposed-vs-persisted and negative/third-party tests; real PostgreSQL and authenticated browser create/read/refresh/retry flows. Overall SF-12 stays open until the real cross-process Chat path is verified. Full BodyState readiness remains open until SF-20.

## 2026-10-13 · Source-bound BodyFact vertical implementation

**Bounded slice locally accepted, not full BodyState/SF-20 completion.** The existing #244 floating-window implementation is incorporated in the V2 integration branch, repaired, and exercised against real Go/Python/PostgreSQL. Person Records/Habits now call the authenticated generated record API, use an owner-scoped paged query, preserve an uncertain write's exact idempotency payload, distinguish a successful save from a failed subsequent refresh, and show true commit feedback. The header/3D/floating-window design is retained; final Penpot pixel review is not claimed.

A new immutable Consultation v3 configuration (`consult-config-83b6a1cc70477c82`) is the unreleased code default. Prior v2 and v1 configuration identities remain registered for historical replay. The Python preflight proposes only exact, clear user statement spans; Go binds its own Run/Turn/one user Message, locks provenance rows against cancellation/source edits, performs the atomic ledger write, and substitutes an actual receipt before emitting public `tool.result`. A failed commit emits an explicit rejection and terminates that run; v3 rejects legacy extraction/lifestyle mutation events rather than dual-writing old fact tables. Original-message pruning permits exact committed receipt replay but not a new write for a cancelled/ambiguous run.

Real integration caught and repaired: Protobuf Struct integer revisions arriving as floats (lossless integer normalization only), an inherited `.bc-stage` CSS collision obscuring the training action, outdated training test selectors, trailing EOF whitespace, and a generated DTO crossing directly into a React component. Regression assertions were preserved/replaced with the real user path, not deleted.

**Executed evidence:**

- `scripts/validate-body-fact-ledger.sh`: 15 PostgreSQL cases including run identity/cancellation, ambiguity, cross-user isolation, batch atomicity, replay and native up/down/up, with race detection (`vertical-postgres.log`).
- `scripts/validate-body-fact-vertical.sh`: 6/6 real-server Chromium cases (direct save/read/reload, Python extraction→Go receipt→PostgreSQL→Person Habits and exact replay, negative/third-party behavior, 390px layout without horizontal clipping, and both repaired longitudinal training E2E paths). Log `.runtime/schema-first-v2-evidence/vertical-e2e-final.log`; test model output is synthetic, but production adapters/validators/HTTP/Proto/persistence/browser paths execute. All isolated resources were cleaned.
- Browser full suite: 72 files / 511 tests, lint, TypeScript, production build passed (`vertical-web-quality.log`). Go full package tests and vet passed after updating default-v3 and preserving explicit historical-v2 replay regression (`vertical-go-final.log`). Python full suite/pyright/ruff passed; the exact final count is recorded in CI/logs, not inferred from test file counts.
- Pinned contract regeneration exactness, migration checksums and architecture boundary tests passed (`vertical-generated.log`).
- An additional **live configured-model** probe (no database writes, no actual patient information) passed 7/7 fixed synthetic cases: habit, symptom, negative, third party, general question, hypothesis and correction. `.runtime/schema-first-v2-evidence/live-body-fact-probe.json`. This is a small live-model smoke check, **not broad medical/semantic safety certification**.

**Still open:** source-bound structured Ask User answers, Episode/Observation/user-Hypothesis modeling and corrections/withdrawals, full multi-domain outbox consumption and privacy erasure, Diagnosis/Training/Safety target replacement, editable Penpot final visual acceptance, staging/production qualification. Existing Equipment and legacy Diagnosis/Training remain transitional consumers, not accepted V2 domain implementations. Hourly automation remains disabled.

## 2026-10-13 · Explicit BodyFact correction / withdrawal and projection hardening

The ordinary BodyFact vertical is extended with exact-target user correction and withdrawal. Native migration `000004_body_fact_corrections` adds a tenant-bound successor link and prevents multiple corrective successors. A data-bearing downgrade refuses to discard correction history. `POST /api/v1/body-state/records/{id}/correct` and `/withdraw` are canonical authenticated OpenAPI operations, generated for Go/TypeScript/Zod/Postman. One atomic amendment changes the assertion status, adds a new source/statement only for correction, and records one revision, ID-only outbox event and idempotent receipt. The old text/source and unrelated records remain. A no-op correction is rejected without creating an empty revision. Withdrawal is not privacy erasure or symptom resolution.

Person Records/Habits now provides an explicit inline correction form (including category correction) and a distinct withdrawal confirmation. An uncertain retry preserves the exact target, payload and idempotency key. Conflict/foreign target/malformed receipt does not claim saved. Changing a real habit over time still appends a distinct time-qualified assertion: the earlier report is not rewritten or marked false.

Concurrent amendments can change old-row assertion state. Ledger pagination now reads its aggregate revision and rows in a **repeatable-read transaction**, with a barrier-based PostgreSQL regression proving a concurrent correction cannot make a prior-version page lose its original facts.

Expanded verification reproduced two missed CI/UI paths and one actual runtime integrity defect from the previous `5796f307d` batch. The old BodyState and lifestyle E2E tests were migrated to the actual new UI/API while retaining their persistence/history assertions. The producer's tool ID is now stable per Go-owned Run and distinct across users/runs. A new real-server regression reads the durable thread after two same-user runs and another user's run, so projection errors cannot hide behind successful fact reads. It also exposed a required `ProjectedToolCall.error/result` nullability mismatch: the canonical OpenAPI now explicitly permits null for those absent values, a Go response-schema regression proves this, and the production adapter passes standard request context instead of pooled Gin context.

Evidence (local, not automatic production qualification):

- Native PostgreSQL suite: 20 passing BodyFact cases, including correction history, exact idempotent replay, tenant denial, no-op/stale checks, final-write rollback, concurrent snapshot consistency and refused destructive downgrade. Log `amend-postgres.log` and final rerun `amend-postgres-final.log` in `.runtime/schema-first-v2-evidence/`.
- The complete four affected browser spec files pass **10/10 real-server Chromium tests**, including direct creation, correction, withdrawal, exact replay, mobile 390px layout, source/negative/third-party semantics, cross-run/user thread projection, true temporal habit history and the two full legacy longitudinal regression paths. Log `amend-e2e-repaired.log`; all isolated test resources cleaned. These tests use synthetic model output through the actual Python/Go/Proto/SQL/browser boundary; the separate live-model smoke remains 7 synthetic cases, not clinical validation.
- Focused amendment/record/browser-client tests: **21/21**; full browser tests, lint, TypeScript and build passed earlier in the batch and are rechecked in exact-head CI. Python full suite **695 tests**, Pyright 0 errors/warnings, Ruff passed. Go full packages/vet, generated contract equality and architecture/migration checks passed. Latest logs are `amend-focused-final.log`, `amend-web-quality.log`, `amend-python-all.log`, `amend-pyright.log`, `amend-ruff.log`, `amend-go-final.log`, `amend-generated-check.log`.

Still open: Episode/Observation/user-Hypothesis, exact structured Ask User answer capture and Agent-authorized correction workflow, cross-domain event consumers and verified health-data erasure, remaining Diagnosis/Training/Safety schema/runtime migration, final Penpot visual sign-off and staging/production release. This closes the **direct user Fact correction/withdrawal sub-slice**, not all of SF-20 or BodyState.

## 2026-10-13 · Run-scoped tool projection root repair

A regression first demonstrated that two different Runs sharing a provider tool ID collapsed into one timeline entry. The root fix is now independent of the new BodyFact producer's namespaced ID: migration `000005_run_scoped_tool_projection` changes the projection primary key to `(run_id, tool_call_id)`, and the event-to-read-model builder uses the server-owned Run plus original provider ID. The raw provider ID remains unchanged for tool response/Ask User identity. Distinct runs and users may legitimately use the same ID without collision or overwritten success/failure state.

Thread metadata, messages and projected tools are also read from one repeatable-read transaction. A concurrent complete snapshot replacement no longer combines old metadata with new message/tool rows. Native PostgreSQL tests cover repeated provider IDs across runs/users, failed final tool-write atomic rollback, an actual concurrent read/write barrier, and refused downgrade that would destroy distinct-run history. The browser's message-local tool result matching already has the correct scope; its regression verifies same IDs in separate assistant messages retain distinct results without introducing another ID translation layer.

Executed: full Go tests and vet pass (`projection-go-quality.log`); native PostgreSQL migration v5 full-up/down/up plus **20 BodyFact + 4 ThreadProjection cases** with race detection pass (`projection-postgres.log`); frontend thread mapping tests 8/8 pass; TypeScript and architecture boundaries pass. The four affected real-server browser spec files passed 10/10 again in `projection-e2e.log`; all isolated resources were cleaned. The exact final CI result is recorded separately against its final pushed SHA; earlier CI statuses are not recycled.

[The integration evidence matrix](./2026-10-13-bodyfact-integration-matrix.md) now maps user scenarios, source owners, regression tests and explicit limits, including remaining structured answers, Episode/Observation/Hypothesis, cross-domain erasure and final release gates. This is infrastructure correctness for the ongoing Chat migration, not completion of single-Conversation/retention or the full V2 project.


The prior amendment milestone `7abe9daa60141ac02f7c148bf91e850b16ca55ef` passed all 12 required GitHub CI checks before the projection-root-fix commit. This is evidence for that exact prior SHA, not a substitute for the next commit's CI.
