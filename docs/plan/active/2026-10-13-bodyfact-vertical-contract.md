# BodySense V2 · BodyFact vertical handoff

Status: ACTIVE / BOUNDED CONTRACT. Parent: [Domain & Schema](./2026-10-13-v2-domain-schema.md), [Engineering](./2026-10-13-v2-engineering.md), [Integration](./2026-10-13-v2-integration-release.md). Product authority: [BodyState](../../product/domain/bodystate.md), [Person / Chat](../../product/domain/person-chat.md), [interface](../../product/interface.md).

## Scope and limits

This executable handoff covers **new BodyFact statements** in Person Records and Habits. It does not declare the whole BodyState domain ready: Episode/Observation/Hypothesis, corrections/withdrawals, shared UserAsset and Equipment are later tickets. No second source of truth or legacy-record mirroring is introduced. Old deployed data is not silently reclassified.

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
