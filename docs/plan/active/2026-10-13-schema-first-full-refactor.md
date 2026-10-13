# BodySense V2 Schema-First · Engineering SF Ticket Ledger

Status: ACTIVE / ENGINEERING DETAIL. [Primary V2 program](./2026-10-13-v2-parallel-master.md) · [Engineering workstream](./2026-10-13-v2-engineering.md) · [Single handoff state](./v2-handoff-state.json). This document preserves the SF-00–SF-90 implementation checkpoints and verification evidence, not a competing sequential master. Execution owner: ChatGPT Web; Agent Fabric is the execution transport, not a second implementation agent.

## Outcome and authority

Replace the obsolete business implementation with the approved `docs/product/` contracts: BodyState, Diagnosis V2, Training/Trainer, SafetyState, Person/Chat, and cross-domain provenance/authorization/erasure. The product shell is the existing Penpot body-first 3D desktop with floating business windows, not the retired split-screen layout.

On 2026-10-13 the user authorized implementation, removal/recreation of BodySense dev/staging/production business data, removal of obsolete code, merge, staging and production deployment, and hourly recovery without repeated approval. This does not authorize changes to unrelated projects, shared infrastructure, credentials, or bypassing application-level diagnosis/training/safety consent. Verify host, database, schema and resource ownership before any destructive operation. No destructive action is needed merely to start this work.

Business authority: [product index](../../product/README.md), [BodyState](../../product/domain/bodystate.md), [Diagnosis](../../product/domain/diagnosis.md), [Training](../../product/domain/training.md), [Safety](../../product/domain/safety.md), [Person/Chat](../../product/domain/person-chat.md), [acceptance](../../product/acceptance.md). These own rules; this plan owns implementation sequence and evidence only.

## Verified starting checkpoint

- PR #246, head `402d63159de69619d8c3e730d894c0667206add4`, was ready/merged after checking its successful CI and clean diff. Main merge: `4f50f14a481757b52b903a3e93c37fd2a0315e8b`.
- Main canonical worktree: GCP `gcp-dev-01`, `/home/dev/projects/bodysense`; implementation branch `refactor/schema-first-v2-20261013` starts from that merge.
- No existing full-refactor active plan or running implementation thread was found. The previous Penpot T3 thread `fabric:0b9f7c91d2ab0fb93611fa4fb05110ef320c75bf03ad3759ce457096db1cf32c` reports `failed`; do not restart its obsolete brief concurrently.
- UI PR #244 / worktree `/home/dev/projects/bodysense-penpot-ui` / branch `feat/penpot-v55-business-windows-20261012` is clean at `3597a5f29bc1085f319c91f6b51c1bdaa481f084`. Preserve and integrate its useful work after review. Its latest Governance and browser E2E checks failed; it is not a release-ready baseline.
- Current code uses `BodyStateFact.ReviewState/LifecycleState/Trend`, AI hypotheses in BodyState, old DiagnosisAnalysis/Treatment, multiple Conversations and public shares. Do not interpret those as target semantics.
- Existing technology authorities: PostgreSQL migrations in `apps/api/migrations`, public OpenAPI in `packages/contracts/openapi/bodysense.v1.openapi.yaml`, stream JSON Schema in `packages/contracts/schemas`, internal Go/Python Proto in `contracts/internal/agent-runtime`. Reuse their pinned generators; generated types are transport contracts, not domain models.
- Actual remote branches/PRs and delivery workflows use `main`; there is no current `dev` branch. The old three-branch paragraph in AGENT.md is stale relative to deployed workflow. Do not invent another integration branch.
- Canonical deployment documentation identifies GCP dev/staging and Alibaba ECS production; Oracle2 is detached from BodySense. Verify live state again before rollout instead of reusing historical host assumptions.

## Protected boundaries

1. Go owns persistent domain changes, revision checking, authorization and commit receipts. Python proposes/extracts and invokes typed operations; successful tool transport is not a successful domain commit.
2. One semantic BodyState operation produces one revision. Idempotent retry returns the original receipt; a reused key with different content conflicts. Outbox event and receipt commit with domain rows, never before them.
3. User statements remain user statements, not clinical evidence or AI hypotheses. Unknown occurrence time stays unknown. Assertion validity is distinct from symptom course. No inactivity timeout proves recovery.
4. Stable source evidence retains only necessary excerpts before message pruning. Tenant and version binding are enforced at persistence boundaries. Raw health text must not be duplicated into general audit/outbox payloads.
5. Diagnosis proposals are decided serially and committed once atomically; changed proposals require exact refreshed authorization. Training completion/skip requires actual user action. Safety has an independent conservative gate.
6. Source removal and health-data erasure are different commands. Neither automatically edits adopted diagnoses, clears safety, nor deletes independent sources. No 'erased' success until controlled copies are verified.
7. Reuse auth, 3D anatomy, stream/replay identity and delivery entry points unless a ticket explicitly replaces them. Do not retrofit old domain fields into the new contract as a compatibility shim.
8. Only one writer per mutable workspace. Each run first checks branch/HEAD/status, recent edits, jobs and T3 state. Preserve changes when interrupted. Do not create one branch/worktree per hourly run.

## Roadmap and acceptance gates

| Ticket | Scope and dependency | Observable gate | Status |
| --- | --- | --- | --- |
| SF-00 | Merge approved contracts, inventory existing implementation and establish this plan | Merged #246, clean baseline, exact ownership and resumable checkpoint | DONE |
| SF-10 | BodyFact/source-evidence/commit-receipt/outbox schema and atomic Go persistence, additive staging of replacement tables | Actual PostgreSQL migration; one revision for a batch; concurrent retry, stale revision, cross-user reference and rollback tests | LOCAL ACCEPTED / CI 12/12 PASS on ce00bc0c1, UNMERGED |
| SF-11 | Typed public read/record operations and generated Go/TS validation for explicitly entered user facts | API auth and schema rejection tests; generated artifacts reproduce exactly; returned receipt reflects persisted revision | LOCAL ACCEPTED / CI 12/12 PASS on ce00bc0c1, UNMERGED |
| SF-12 | Python extraction/tool binding for clear self statements and Person Records/Habits read/write UI | User message -> validated extraction -> Go -> DB -> receipt -> refreshed Person; failure never displayed as saved | IN PROGRESS — ordinary self-report Chat/Go/DB/Person vertical locally accepted; structured answers and broader BodyState follow-ups pending |
| SF-20 | Episode identity/links/deterministic projection, corrections/withdrawals and user hypotheses; replace old Fact/Observation owners | BS-16..32, negative/ambiguous/recurrence/correction cases and reference replay | IN PROGRESS — direct Fact correction/withdrawal sub-slice locally verified; Episode/Observation/Hypothesis remain |
| SF-30 | Single Conversation, serial run/Ask User identity, replacement composer and recovery; remove share/list/create UX | PC3-01..05/19, concurrent devices, no second run while pending, old public tokens inaccessible | PENDING |
| SF-40 | Diagnosis V2 schema, analysis/proposals/decisions/atomic revisions and frontend | DXV2-01..28; stale content, partial confirmation, urgent interruption and retries cause no unauthorized partial commit | PENDING |
| SF-50 | TrainingPlan proposals/revisions, Trainer logs and independent SafetyState rules | TP/SAFE matrix; real per-set action, user timezone snapshot, unavailable safety fails closed, evidence-bound clearance | PENDING |
| SF-60 | Minimal Profile/Equipment/UserAsset, report admission/source availability, two deletion commands | PC3-11..18/CD3-06..07, independent sources retained, retryable deletion and verified receipt | PENDING |
| SF-70 | Three chat-retention cursors, minimum source transfer, checkpoints/caches/index cleanup | PC3-06..10; complete tool boundaries preserved; processed/context/pruned ordering and actual controlled-copy cleanup | PENDING |
| SF-80 | Integrate/refine existing Penpot/React windows with real view models; retire old UI/models/routes/tables | No obsolete business routes/schemas/models except explicitly documented transitional consumers; desktop/mobile browser evidence | PENDING |
| SF-90 | Full CI, isolated prod-like migration, staging, fresh review, production promotion | Exact-SHA artifact/manifest and health checks; critical end-to-end and privacy/safety tests; no fabricated completion | PENDING |

Do not call the full refactor complete because SF-10 or CI alone passed. A ticket may close independently only on recorded evidence. SF-10/11 are foundations for SF-12, not a claim that the chat extraction vertical is already migrated.

## Implemented foundation batch: SF-10 / SF-11

Goal: introduce the approved five-category BodyFact record path without storing old review/lifecycle/trend semantics in the replacement model. Preserve a minimal direct-user-statement source, one authoritative BodyState revision, idempotent receipt and metadata-only outbox.

Scope: `apps/api/migrations`, `internal/model`, `internal/repository`, `internal/service`, generated HTTP adapter and canonical OpenAPI/generated browser client. Use the existing `body_states` aggregate and revision sequence; stage replacement fact rows without dual-writing old fact tables. Old consumers remain explicitly unconverted until SF-12/20; do not expose a misleading 'complete BodyState' projection.

Out of scope for this batch: no automatic medical classification, no clinical thresholds, no invented diagnosis expiry, no forced reset of production data, no claim of completed privacy erasure, no silent migration of AI hypotheses into user hypotheses.

Steps:

1. Define a forward migration with tenant-bound source-evidence, replacement BodyFact rows, operation receipts and outbox metadata; keep deployed migration 000001 immutable.
2. Add failing PostgreSQL tests for batch atomicity, unchanged retry, mismatched retry, concurrent revision conflict, isolation and transaction rollback.
3. Implement typed domain validation and transactional persistence using the existing aggregate lock/revision machinery, with idempotency checked before stale expected-revision rejection.
4. Define new explicit record/read routes in the existing OpenAPI authority; register new routes in the public-route policy. Generated DTOs stop at the handwritten adapter.
5. Generate artifacts with `bash scripts/contracts/generate-public-rest.sh`; run focused Go tests, OpenAPI lint, route governance, generated checks and actual PostgreSQL tests on an isolated synthetic database.
6. Review the entire diff, record exact results, commit/push the same branch and update this checkpoint. Next deliver SF-12 rather than restart discovery.

Expected verification commands (execute and record results; not evidence until run):

- `git diff --check`
- `bash scripts/validate-migration-history.sh`
- `bash scripts/contracts/generate-public-rest.sh`
- `pnpm exec redocly lint packages/contracts/openapi/bodysense.v1.openapi.yaml`
- `cd apps/api && go test ./internal/model ./internal/service ./internal/repository ./internal/transport/httpapi`
- `cd apps/api && go vet ./...`
- PostgreSQL tests with `BODYSENSE_INTEGRATION_DATABASE_URL` pointing only at a verified synthetic BodySense test database.
- `pnpm nx run @bodysense/web:typecheck` after generated/browser changes.

Containment: new tables/routes are additive during the vertical transition, not permanent parallel truth or a compatibility adapter. Do not drop old data until all consumers for that slice are replaced and acceptance passes. Down migrations are for isolated test containment; production uses the verified forward reset/migration strategy authorized by the user.

## Next executable batch: SF-12

After checking current PR CI and preserving this branch, implement the first real chat-to-Person vertical. Do not repeat SF-00/10/11 discovery.

1. Inspect existing Go/Python consultation tool and internal runtime Proto boundaries, plus the Person Records/Habits consumers in the canonical tree and preserved UI worktree. List exact consumers before changing source identity or UI contracts.
2. Add a server-verified user-message/structured-answer source binding with owner, stable message/run/tool identity and minimum excerpt. A model must not supply a trusted source identity or relabel its own generated output as a user statement. Extend the source schema through a new forward migration after SF-10 is committed; do not rewrite deployed migration history.
3. Route clear self statements through the new typed BodyFact ledger with one semantic batch/revision and stable retry identity. Ask only for genuine ambiguity/conflict; do not add generic approval for every fact. Distinguish stale-revision repair from exact replay of an already committed command.
4. Connect Person Records/Habits to the generated read/record contract and authoritative returned revision. Refresh from durable state; never show 'saved' from tool transport success or manufacture a received event. Implement bounded outbox dispatch/checkpoints as needed for actual subscribers rather than claiming that a stored outbox row has been delivered.
5. Exercise user message -> Python tool -> Go source/owner validation -> PostgreSQL -> commit receipt -> Person display, including rejected cross-user/AI-authored sources, retries, conflicts, failed writes and real UI refresh. Keep current 3D body-first floating-window design; preserve useful #244 work and reproduce/fix its failed checks before integration.

## 2026-10-13 · SF-12 partial implementation checkpoint

**Completed in this bounded run (not SF-12 closure):**

- Recovered clean branch `refactor/schema-first-v2-20261013` at `68cb0ae33`, identified the stale ChatGPT-Web lock with no live same-scope writer and removed it with an exact-owner/age guard. The incomplete `000003_message_fact_source.up.sql` draft was retained and completed with a symmetric down migration and checksums rather than silently discarded.
- Added `statement_evidence` message provenance (`source_conversation_id`, `source_run_id`, `source_message_id`) with a constraint requiring the complete triple for `user_message` and rejecting those fields for `direct_edit`/`structured_answer`. No FK to prunable messages/runs: the persisted minimal excerpt and stable identities survive controlled chat retention.
- Implemented `BodyStateRepository.RecordMessageFacts` and `BodyFactService.RecordMessageFacts` using the existing atomic revision/receipt/outbox transaction. The repository joins the actual `runs` and user-authored `messages` by the Go-created shared turn and conversation. User messages currently have nullable `Message.RunID`; treating that field as mandatory would be incorrect. It checks conversation ownership, `run.user_id`, completed user role, run/turn agreement and literal excerpt membership in structured message parts. It does not accept an AI-provided source label or permit unrelated text. Direct-edit fingerprints are unchanged; message fingerprints include run/message identity, so cross-source key reuse conflicts. An exact replay returns the original receipt even after lawful raw-message pruning.
- Added synthetic PostgreSQL regression cases for verified origin, assistant-origin forgery, wrong run/turn, empty/nonmatching source, other user, reused key with different source, old message physically removed with minimal evidence retained, and privacy-bounded generic event payloads.
- Added `apps/web/src/features/profile/services/bodyFactRecordsService.ts` and 5 focused API tests. The service uses strict generated request validation, authenticated generated HTTP, and Zod-verified durable commit receipts; conflicts remain real 409 errors. It does not claim a Person window already consumes the new ledger.
- **Executed PASS:** `bash scripts/validate-migration-history.sh` (latest 3, immutable checksums); real `scripts/validate-body-fact-ledger.sh` with native migration full-up/down/up and PostgreSQL/race tests (`BODY_FACT_LEDGER_POSTGRES=PASS`, job `gcp-dev-01::job_req_f8d0f68dcaf695b16f0cb1cd65ac1539`, log `.runtime/schema-first-v2-evidence/message-source-postgres.log`); Go model/service/repository targeted tests and vet; browser Vitest 14/14, targeted ESLint and `pnpm nx run @bodysense/web:typecheck` (after correcting the deliberately forged TypeScript fixture's static excess-property error). Full Go suite `go test -p 2 ./...` and full `go vet -p 2 ./...` completed with exit 0 in job `gcp-dev-01::job_req_fa4eba03fafa9581d57809e4171c4c5b` (logs `.runtime/schema-first-v2-evidence/source-full-go-test.log` and `source-full-go-vet.log`).
- Prior head of PR #247 (`68cb0ae33`) had 12 successful checks when inspected. These do **not** validate this run's new changes until an updated commit receives a fresh CI result. No dev/staging/production business data or deployment was changed.

**Remaining SF-12 work:** Connect the bounded Python consultation extraction/tool result to Go-owned run/message identity instead of exposing a public model-supplied source endpoint; validate semantics including negative self-report, third-party discussion and ambiguity; read/refresh actual Person Records/Habits through the new ledger and receipt with 3D body-first floating windows, taking existing UI PR #244 into account. Verify actual Agent → Go → PostgreSQL → Person E2E and failed-write behavior. The current literal-excerpt check guards fabricated source *text*, not full clinical/semantic truth; do not call the path complete or promote AI claims to clinical evidence.

## Open items, not invented product decisions

- Q-17: formal Diagnosis proposal expiry remains unapproved. Implement evidence/revision invalidation, do not invent a fixed business TTL.
- Q-19: Diagnosis confirmation visual component remains design work; integrate the existing Penpot file rather than creating a second source.
- Clinical safety thresholds/professional review are not established by software testing. Fail closed where the required verified evidence/policy is unavailable.
- Numeric normalization catalogs, concern-group display keys and physical erasure propagation need engineering contracts and tests, not implicit legacy mappings.
- UI PR #244 failed checks need reproduction and focused repair before integration/rollout.

## On-demand continuation (hourly automation disabled)

On 2026-10-13, the user requested disabling the hourly continuation automation `6acd19647d148191acb982240a5e9c80`; `automations.update` confirmed `is_enabled=false`. Do **not** reactivate or create a duplicate. A manual continuation must first read the [V2 primary program](./2026-10-13-v2-parallel-master.md), its four workstream plans, this SF checkpoint, and live branch/worktree/jobs/lock. Follow one writer per checkout and the shared handoff gates. On a transient tool failure, inspect, perform bounded retry, isolate the blocker and proceed with independent safe tasks. Never bypass persistent platform safety/authentication controls. Model substitution does not change implementation ownership.

## Checkpoint ledger

- 2026-10-13: SF-00 complete; #246 merged as `4f50f14a4`; direct implementation branch created.
- 2026-10-13: SF-10 code and migration pair implemented; real isolated PostgreSQL 18 tests with Go race detection passed for one-revision batches, 12 concurrent identical requests, distinct-command conflict, idempotent replay, mismatched payload rejection, final-write rollback, tenant source FKs, blocked-source filtering and whole-revision pagination. Migration down/up also passed. Job `gcp-dev-01::job_req_82fad5b56bfa390f262a914163752007`.
- 2026-10-13: SF-11 authenticated POST/GET `/api/v1/body-state/records` implemented and registered in the actual production router. OpenAPI discriminator schemas, generated Go/TS/Zod and lint passed. Full model/service/repository/HTTP package unit tests passed in job `gcp-dev-01::job_req_623ec3d647c55186b536d6b58c927888`; PostgreSQL HTTP round-trip and complete generated/quality gates are being run next. No application environments have been reset or promoted.

- 2026-10-13: The first real HTTP/PostgreSQL race run exposed a pooled Gin context escaping into database/sql cancellation observers. The new ledger adapters now resolve identity first and pass the standard request context into application work. The final isolated run passed all 10 PostgreSQL cases with race detection, including HTTP record/read/retry, privacy inventory/account cascades and null-precision SQL rejection. Existing legacy HTTP adapters have not all been audited for this issue; apply the same boundary as their slices migrate.
- 2026-10-13: Generated contracts reproduced exactly; Postman generation/verification covered 97 routes; architecture boundaries and complete browser TypeScript checking passed. The new browser contract suite passed 9 tests. New tables were added to the account-erasure inventory; bound SQL parameter interpolation is disabled at every configured log level. This does not constitute selective UserAsset erasure or whole-system privacy acceptance.

- 2026-10-13: Native migration replay found that baseline 000001 leaves the connection search_path empty. New migration 000002 now schema-qualifies all relations; the test harness uses the actual golang-migrate validator rather than separate psql sessions. The corrected native full-up/down/up sequence passed before and after all 10 race-enabled PostgreSQL tests. Independent native schema capture also passed at version `2:false`, 52 tables / 706 columns. Job `gcp-dev-01::job_req_81c9233934519003b95d60f858088763`; local bounded log `.runtime/schema-first-v2-evidence/postgres-native-final.log`. No production data was involved.
- 2026-10-13: Full Go package tests, `go vet -p 2 ./...` and pinned Staticcheck `v0.8.1` with `SA*,S1*,QF*` completed with exit 0 in job `gcp-dev-01::job_req_0c674edc2ed818c9a8c3258b03f445d2`, including the new SQL parameter privacy tests. Browser contract tests (9), full browser TypeScript and architecture checks completed with exit 0 in job `gcp-dev-01::job_req_23c4ae6240bd3f9c2aa271b4c3461537`; targeted ESLint also passed. The ordinary Go run skips unrelated integration suites without their environment; only the explicitly recorded new PostgreSQL suite is claimed here.
- Current handoff: SF-10/11 locally accepted, implementation branch ready for/pushed as a draft refactor PR; read live GitHub checks before any merge. The full project has not passed staging/production or UI acceptance. Next implementation remains SF-12 after addressing any actual PR check failures. Do not restart the failed legacy T3 brief.

Current direct-entry boundary: provenance is assigned `direct_edit` by Go, never accepted from a caller. The idempotency fingerprint covers the canonical whole command, including expected revision; a retry must preserve its original payload. Linked Trainer IDs are rejected until verified binding is implemented, rather than accepted as unchecked JSON. This batch does not yet expose arbitrary clinical/report/message admission. Outbox production is transactional; subscriber delivery/checkpointing remains subsequent work. Do not claim SF-12 or complete health-data erasure from these passing foundation tests.


## Latest parallel execution checkpoint


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

Contract and detailed evidence: [BodyFact vertical handoff](./2026-10-13-bodyfact-vertical-contract.md).


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


### Run-scoped projection checkpoint

The event timeline no longer treats provider tool IDs as globally unique: migration 000005 plus Go projection keys scope identity to the server Run; repeatable-read thread snapshots and 4 native PostgreSQL concurrency/tenant/rollback regressions pass. See [integration matrix and current evidence](./2026-10-13-bodyfact-integration-matrix.md) and [bounded implementation checkpoint](./2026-10-13-bodyfact-vertical-contract.md). SF-30 remains open for true singleton Chat/retention/structured-answer migration.
