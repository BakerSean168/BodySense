# BodySense V2 · Integration & Release Workstream

Status: ACTIVE / TRACK D. [Primary program](./2026-10-13-v2-parallel-master.md) · [Readiness register](./v2-handoff-state.json) · [Existing acceptance](../../product/acceptance.md).

## Mission

Integrate Product Design and Engineering **continuously for each domain**, then retire legacy consumers and qualify a full rollout. Do not postpone all UI/code contact until every backend domain is finished. Do not equate a green build, screenshot, transport response, Penpot mock, or additive migration with a working product.

## Accepted interface boundaries

Use [docs/product](../../product/README.md) for business acceptance, Track A's exact query/command projection for data semantics, Track B's editable Penpot handoff for interaction, and Track C's real Go/Python/PostgreSQL/React path for behavior. Track D verifies them jointly and owns the release gate, not another domain model.

Each domain integration gate requires:

1. **Contract match:** front-end visible fields/derived counts map to authenticated Go projections; source/evidence/absence/expired distinctions survive generated DTOs; no private revision or invented clinical grade displayed.
2. **Stateful user path:** empty/populated/loading/error/stale/forbidden/reconnect, explicit consent, conflict/retry/idempotency, resume/cancel/Ask User and pending vs committed states where applicable.
3. **Negative/security tests:** user isolation, unauthorized source, failed or partial write, unapproved Diagnosis/Training modifications, SafetyState block, erased/copy-retained source, and adverse ordering/replay.
4. **Visual/function:** editable Penpot baseline, real desktop and mobile viewer screenshots, floating window over 3D body, keyboard/focus/scroll responsiveness; final pixel spacing polish follows semantic acceptance.
5. **Proof and cleanup:** reproducible test commands/logs, exact SHA, reviewed legacy route/table/model consumers and a safe retirement/migration plan. A closure requires evidence in handoff JSON, not just a changed checkbox.

## Executable tickets

### IR-BASE-01 — Baseline matrix and UI PR repair audit

**Scope:** Map [product acceptance](../../product/acceptance.md) to SF-domain tests and active routes; inventory old BodyState/Diagnosis/Treatment/Conversation/Training consumers. Read [#244](https://github.com/BakerSean168/BodySense/pull/244) failed Governance and browser E2E logs, reproduce specific failures, review UI worktree at `3597a5f29`. **Acceptance:** fault/contract/evidence matrix with P0–P3 severity and focused repair tickets; no blind PR merge; running checks distinguish from historical checks.

### IR-BS-01 — First BodyState E2E and UI handoff

**Dependencies:** DS-BS-01/02, PD-BS-01, ENG-BS-01/02. **Scope:** user statement in real Chat → typed Python extraction → Go provenance and PostgreSQL commit → receipt/outbox → actual Person Records/Habits refresh, Episode correction/withdrawal. **Acceptance:** valid + ambiguous/negative/third-party + stale/replay + rejected write + mobile/desktop; no false 'saved'. Record exact tested API/AI/Web SHA.

### IR-PC-01 — One Chat and Person continuity

**Dependencies:** PD-PC-01 and ENG-PC-01. **Scope:** one Conversation across devices, pending Ask User recovery, retention/source transfer, no public health conversation sharing. **Acceptance:** no second concurrent Run and no source loss on pruning; no UI list/create/share remnants.

### IR-DX-01 — Diagnosis V2 approval and visual integration

**Dependencies:** PD-DX-01 and ENG-DX-01. **Scope:** patient-facing condition count, source/AI distinctions, exact proposals, stale decisions, safety interrupts, history. **Acceptance:** all decisions applied atomically once; a prior accept never authorizes a changed proposal; Q-19 visuals actually reviewed.

### IR-TP-01 — Training/Trainer/Safety vertical

**Dependencies:** PD-TP-01 and ENG-TP-01. **Scope:** accepted plan vs proposal, multi-plan isolation, per-set user actions, paused/resume recovery, fail-closed safety and timezone. **Acceptance:** no false completion or accidental automatic clearance.

### IR-ASSET-01 — Shared media and erasure

**Dependencies:** PD-ASSET-01 and ENG-ASSET-01. **Scope:** equipment, reports/media processing/admission, removal vs erasure, controlled copies and cross-domain evidence. **Acceptance:** source unavailability and deletions do not misrepresent clinical authority or 'erased' before copies verified.

### IR-FINAL-01 — Legacy retirement and full release

**Dependencies:** all integration gates ready. **Steps:** enumerate exact old tables/URLs/models/events; confirm each target consumer migrated; execute isolated native migrations and prod-like restore/recovery; review negative/security/E2E matrix; run required exact-SHA CI; qualify GCP staging; validate actual BodySense host/storage/database ownership for any authorized destructive reset; promote a tested immutable artifact via existing protected workflow; verify production health and rollback. **Acceptance:** no obsolete domain route/schema still serving target product except an explicitly documented transitional consumer, all blockers closed, independent production evidence. **No production reset is implied by writing a plan.**

## Existing CI and deployment architecture

- PR verification: `.github/workflows/ci.yml` (delivery manifest/oracles, Go/Python/React quality lanes, PostgreSQL 18 baseline/recovery, longitudinal browser E2E).
- Repository gate: `scripts/validate-repo.sh`, `scripts/validate-migration-history.sh`, `scripts/validate-body-fact-ledger.sh`; selected contracts from `scripts/contracts`.
- Release authority: `docs/architecture/delivery-platform-v3.md`, `docs/architecture/release-lifecycle-v3.md`. Staging/dev reside on verified GCP ownership; production is Alibaba ECS; Oracle2 is not BodySense runtime. Do not infer live production from old documentation.

Baseline 2026-10-13: #247 code PR is Draft (audited first two migrations/Go/API and message provenance verified) with exact prior head's CI passed; #244 has failing Governance/Experience tests. **No domain's full V2 integration gate is ready; no V2 staging or production deployment.**

## Continuous review protocol

Review by verified root cause, not vague screenshots. Each finding names severity (P0 security/data or unsafe behavior, P1 missing required behavior, P2 robustness, P3 visual polish), exact file/route/event/flow, violated contract, reproduction, owner A/B/C and target test. Fix in small independent repair passes, rerun focused gate then affected full suite; mark fixed/partially fixed/not reproduced/deferred with reason. Do not create simultaneous writers on the same workspace.

**Next:** IR-BASE-01 may run without waiting for DS-BS-01. UI styling finalization is intentionally deferred; UI behavior and safety gate acceptance is not.

## 2026-10-13 verified checkpoint

D: #244 whitespace and training navigation/CSS regressions reproduced and fixed in the unreleased integration branch; 6 real-server E2E cases pass, including mobile geometry. See [bounded contract, changes and evidence](./2026-10-13-bodyfact-vertical-contract.md). Domain-wide gates and production release remain open.

### Explicit assertion amendments checkpoint

Migration 000004 and authenticated generated correction/withdrawal endpoints, actual Person UI and ten affected browser E2E paths are locally verified. Read [the bounded handoff and exact acceptance evidence](./2026-10-13-bodyfact-vertical-contract.md). Episode/Observation/Hypothesis and whole-domain release gates remain open. The prior failed #247 browser run is not labelled green; the next pushed SHA must pass fresh CI.

## 2026-10-13 · Run-scoped tool projection root repair

A regression first demonstrated that two different Runs sharing a provider tool ID collapsed into one timeline entry. The root fix is now independent of the new BodyFact producer's namespaced ID: migration `000005_run_scoped_tool_projection` changes the projection primary key to `(run_id, tool_call_id)`, and the event-to-read-model builder uses the server-owned Run plus original provider ID. The raw provider ID remains unchanged for tool response/Ask User identity. Distinct runs and users may legitimately use the same ID without collision or overwritten success/failure state.

Thread metadata, messages and projected tools are also read from one repeatable-read transaction. A concurrent complete snapshot replacement no longer combines old metadata with new message/tool rows. Native PostgreSQL tests cover repeated provider IDs across runs/users, failed final tool-write atomic rollback, an actual concurrent read/write barrier, and refused downgrade that would destroy distinct-run history. The browser's message-local tool result matching already has the correct scope; its regression verifies same IDs in separate assistant messages retain distinct results without introducing another ID translation layer.

Executed: full Go tests and vet pass (`projection-go-quality.log`); native PostgreSQL migration v5 full-up/down/up plus **20 BodyFact + 4 ThreadProjection cases** with race detection pass (`projection-postgres.log`); frontend thread mapping tests 8/8 pass; TypeScript and architecture boundaries pass. The four affected real-server browser spec files passed 10/10 again in `projection-e2e.log`; all isolated resources were cleaned. The exact final CI result is recorded separately against its final pushed SHA; earlier CI statuses are not recycled.

[The integration evidence matrix](./2026-10-13-bodyfact-integration-matrix.md) now maps user scenarios, source owners, regression tests and explicit limits, including remaining structured answers, Episode/Observation/Hypothesis, cross-domain erasure and final release gates. This is infrastructure correctness for the ongoing Chat migration, not completion of single-Conversation/retention or the full V2 project.

The prior amendment milestone `7abe9daa60141ac02f7c148bf91e850b16ca55ef` passed all 12 required GitHub CI checks before the projection-root-fix commit. This is evidence for that exact prior SHA, not a substitute for the next commit's CI.

### Same-Run Ask User checkpoint

The Ask User slice now has native checkpoint tests, 10 new PostgreSQL cases, four new browser paths and complete Go/Python/Web checks. Previous foreign-owner 500 and dark-title defects were repaired. Keep local evidence separate from exact-head CI and do not promote this partial V2 program as a finished application. See [bounded contract, acceptance and test evidence](./2026-10-13-ask-user-source-contract.md). The latest execution registry is [v2-handoff-state.json](./v2-handoff-state.json); all whole-domain/release gates remain open.

### 2026-10-13 · Bounded episode and Person projection acceptance

[ViewModel and Episode evidence matrix](./2026-10-13-bodystate-viewmodel-episode-handoff.md) tracks source fields, page freshness, proposed Penpot states and intentionally unimplemented capabilities. Isolated PostgreSQL v7 native full-up/down/up with race-enabled new Episode constraints and existing BodyFact/Ask User cases passed; Go status projector and Person component focused tests passed. Complete exact-SHA CI remains the authority after push. No Penpot editable sign-off, production-like Episode path, database reset or staging/production promotion has taken place. IR-BS-01 remains open.

### 2026-10-13 · Episode authenticated query acceptance

Added exact owner-scoped Episode GET/list through canonical OpenAPI → generated Go strict HTTP → Go service → PostgreSQL repeatable-read joins, and generated TypeScript/Zod adapter. Native PG18 baseline v7 forward/rollback/replay, race-enabled repository tests and direct HTTP round-trip prove explicit claim projection, page cursor, masked original excerpt, 404 for inaccessible/foreign identity, source withdrawal/block and concurrent block-after-snapshot consistency. Logs `.runtime/schema-first-v2-evidence/episode-public-final-pg18.log`; full Go/web/generated-contract logs separately recorded. For UI, complete history/current confidence, editable Penpot frames, real automatically admitted Episode, cross-domain source erasure and staging/production remain **unaccepted**. **IR-BS-01 stays IN_PROGRESS**; exact new SHA CI must pass before code PR review.

### 2026-10-13 · Native v8 Episode admission and denial acceptance

Migration `000008_episode_command_receipts` was tested on native isolated PostgreSQL 18 with latest-down/replay; new race-enabled repository and generated strict HTTP roundtrips prove (a) one semantic revision per direct-user create/link, (b) exact original receipt on retries, including concurrent same-key calls, (c) rollback when final receipt write fails, (d) foreign owner/source block/withdraw/negative and incompatible type-side denial, (e) unknown status remains unknown after linking, (f) no fake health text/clinical success across public transport, (g) privacy inventory and cascade/rollback refusal. Evidence `.runtime/schema-first-v2-evidence/episode-command-final-pg18.log`, plus generated Web adapter Zod tests. Entire V2 integration remains open pending production-style action UI, AI High/Ask User authorization, recurrence/history, Observation/Hypothesis, full source erasure, official Penpot and staging/production qualification. Exact next-head CI must pass; previous read-only PR SHA `ee092fa49` passed 12/12 but is **not** new-head acceptance.

### 2026-10-13 · IR-BS-01 Person Episode actual UI acceptance path

Added a real window-level Episode disclosure with source-backed Fact/Episode selection and true commit receipt, plus 6 pure-projection and 10 component tests. The browser longitudinal health harness now includes `apps/web/e2e/body-episode-associations.spec.ts`: synthetic registered owner → real Go Fact → Person UI create → idempotency replay → negative observation link → Episode read → reload → 390px overflow. This _must_ pass on the exact submitted SHA before bounded acceptance; mock tests alone do not qualify. Four-line handoff and explicit remaining UI/clinical permission gaps: [Episode Person UI integration checkpoint](./2026-10-13-episode-person-ui-integration.md). **Design PR #250 remains independently editable; do not force-merge/overwrite it.** All global BodyState, Diagnosis, Training, Media and V2 release gates remain open.

### 2026-10-13 · Source-error and uncertain-commit additional regressions

ENG-BS-02's provisional Person Episode implementation now protects against unavailable first GET, failed refresh with cached GET pages, and changed input/target after unknown POST outcomes. The red-before-green component tests cover all four cases; the source-safe hook and view-only forms preserve existing generated endpoints and query ownership. 27 targeted tests, full uncached Web suite (73 files / 452 tests, lint/typecheck/build), and native PostgreSQL 18 + Go/Python browser suite (15/15 cases without retry) passed. New exact-SHA CI remains independently required. No global gate changes based on UI-only behavior. See [component and safety checkpoint](./2026-10-13-episode-person-ui-integration.md); the separate Penpot PR #250 remains unaffected.
