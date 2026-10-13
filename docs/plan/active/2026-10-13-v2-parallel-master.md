# BodySense V2 · Parallel Full-Stack Rebuild Master Plan

Status: ACTIVE / PRIMARY PROGRAM. Baseline audited 2026-10-13 (UTC). Execution owner: ChatGPT Web; Agent Fabric is the authorized transport, not a second implementation writer. The hourly continuation automation has been disabled at the user's request; this plan does not reactivate it.

## 0. Decision and outcome

Implement the approved BodySense V2 business contracts using **one domain-contract foundation, two parallel delivery tracks (Product Design and Engineering), and continuous domain-by-domain integration**. Do not wait for all database tables to be complete before designing the first domain; do not use a table schema as a UI specification. Do not wait for all UI polish before implementing backend services.

Final product: a persistent interactive 3D body canvas, floating Chat/Person/Diagnosis/TrainingPlan/Trainer windows, durable BodyState, one Conversation and one Diagnosis per user, multiple independently accepted TrainingPlans, independent SafetyState, and source-bound evidence/authorization/erasure.

This document owns global priorities, dependency gates and release readiness. Each track has its own ACTIVE plan:

| Track                     | Workstream plan                                                 | Responsibility                                                                                       |
| ------------------------- | --------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| A · Domain & Schema       | [Domain & Schema](./2026-10-13-v2-domain-schema.md)             | Aggregate identity, lifecycle invariants, PostgreSQL and command/query/event contracts               |
| B · Product Design        | [Product Design](./2026-10-13-v2-product-design.md)             | Penpot flows, user-visible read models, window states, interaction review, design handoff            |
| C · Engineering           | [Engineering](./2026-10-13-v2-engineering.md)                   | Go/Python runtime, generated HTTP contracts, React data adapters and initial working UI              |
| D · Integration & Release | [Integration & Release](./2026-10-13-v2-integration-release.md) | Continuous domain E2E acceptance, Penpot-to-code review, legacy retirement, staging/production gates |

**Single status register:** [v2-handoff-state.json](./v2-handoff-state.json). Read it for per-domain gates; do not independently mark a domain complete in multiple plans. The existing [SF-00–90 detailed plan](./2026-10-13-schema-first-full-refactor.md) remains the **Engineering implementation ticket ledger and historical verification record**, not a competing master. Preserve all SF identifiers and checkpoints; do not reset SF-12 or reproduce already accepted SF-10/11 work.

## 1. Source-of-truth and boundary

| Concern                                        | Authority                                                                                                                                                                       | Handoff rule                                                                       |
| ---------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Business rules, clinical distinctions, consent | [docs/product](../../product/README.md), [domain](../../product/domain/README.md), [acceptance](../../product/acceptance.md), [open questions](../../product/open-questions.md) | Do not invent business rules in plans, prompts, Penpot or code                     |
| Existing behavior and deployed reality         | Actual repository sources, migrations, service configuration and executable tests                                                                                               | Label unimplemented specifications as TARGET, not deployed                         |
| Persistent authority / migrations              | `apps/api/internal/model`, `apps/api/internal/service`, `apps/api/migrations`                                                                                                   | Go owns transaction, tenant, revision, permission and commit receipt               |
| Public API / generated types                   | `packages/contracts/openapi/bodysense.v1.openapi.yaml` and pinned generator                                                                                                     | Go/TS/Zod clients are generated; API DTOs are not domain models                    |
| Runtime and stream events                      | `contracts/internal/agent-runtime`, `packages/contracts/schemas`                                                                                                                | Python cannot confer Go-owned user/message identity or commit success              |
| Presentation contract                          | Track A's explicit query/command projection and generated API DTO; track B's annotated component mapping                                                                        | UI derives from meaningful view models, **not a SQL table-per-panel mapping**      |
| Visual and interaction design                  | [interface specification](../../product/interface.md) + existing editable **BodySense · Product Design** Penpot Product page                                                    | Design cannot override business semantics; screenshots are not editable components |
| Release policy                                 | `.github/workflows/ci.yml`, `docs/architecture/delivery-platform-v3.md`, exact-SHA release tooling                                                                              | A successful local test or tool call is not staging/production acceptance          |

Protected invariants: one authoritative owner per persistent state; no AI claim promoted to clinical fact; user-authored statement provenance; exact proposal identity and atomic consent; serial active Chat run and resumable Ask User; verified user action for training completion; SafetyState fails closed; source deletion distinct from health-data erasure; revision-bound receipt before UI 'saved'; least-necessary health evidence; 3D body scene persists beneath floating windows. No old split-screen comeback, arbitrary clinical TTL, or silent legacy compatibility state.

## 2. Verified starting baseline (audit snapshot, not a claim of current deployments)

- GCP worktree `/home/dev/projects/bodysense`, branch `refactor/schema-first-v2-20261013`, audited HEAD `ce00bc0c177d6cb227d0aad4344314ff44034633`, clean on 2026-10-13. Main baseline at program start `4f50f14a481757b52b903a3e93c37fd2a0315e8b`; this master is authored on the existing implementation branch and must pass review/merge before becoming main's canonical file.
- Domain decisions PR #246 is merged; [docs/product](../../product/README.md) contains the current approved target semantics. Q-17 Diagnosis proposal TTL remains undecided; Q-19 confirmation presentation is a design deliverable.
- Migrations `000001` old baseline + `000002` BodyFact ledger + `000003` user-message provenance exist. PostgreSQL 18 replay/integration, Go tests and generated Go/TS/Zod contract tests passed for this limited slice. **The full replacement Schema is NOT complete.** Old BodyStateFact, DiagnosisAnalysis, Treatment, Conversation and Training owners/paths remain.
- PR [#247](https://github.com/BakerSean168/BodySense/pull/247) carries these unmerged changes; audited `ce00bc0c1` had 12/12 successful CI checks, still Draft. Do not reuse this evidence for later SHAs.
- `apps/web/src/features/profile/services/bodyFactRecordsService.ts` is implemented and tested but **not yet used by a real Person window**. Python automatic extraction/tool binding is pending; `SF-12` is in progress.
- Existing editable [Penpot Product page](https://design.penpot.app/#/view?file-id=24d9d841-759d-81bc-8008-b6423250c2b8&page-id=ed4c6ec0-8f91-80cb-8008-c1e744bad64b&section=interactions&index=2) provides a visual baseline. UI PR [#244](https://github.com/BakerSean168/BodySense/pull/244), worktree `/home/dev/projects/bodysense-penpot-ui`, is **not accepted**: Governance and browser E2E checks failed; inspect and selectively integrate after fixing them.
- There has been **no v2 staging or production promotion/reset**. The separate hourly continuation task is disabled and must remain off unless the user explicitly re-enables it.

## 3. Two independent readiness axes per domain

**Business contract settled does not mean Schema deployed. Schema migrated does not mean UI ready.** The handoff file tracks six distinct gates; a gate's `ready` state requires a linked reproducible artifact/evidence.

| Gate          | Evidence to mark ready                                                                                                                                                   | Allows                                                                             |
| ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------- |
| `business`    | Exact approved domain/decision/acceptance documents, explicit bounded open issues                                                                                        | Detailed view-model and persistence design                                         |
| `view_model`  | Named query projections, required/optional user-facing fields, source/freshness states, command preconditions/outcomes, stable typed interface and negative/error states | B: final interaction/data mapping; C: typed frontend/API work                      |
| `database`    | Additive migration and rollback/replay on isolated PostgreSQL, auth/tenant tests, revision/idempotency and provenance checks                                             | C: live persistence and full business vertical                                     |
| `design`      | Editable Penpot screens for each required state, flow semantics, desktop and narrow-screen review, contract trace                                                        | D: final visual parity acceptance; C can use provisional tested components earlier |
| `engineering` | Authenticated Go/Python/React slice connected to actual persisted data; commit receipt, retry/denial and focused E2E tests                                               | D: domain integration acceptance                                                   |
| `integration` | Production-shaped domain E2E, accessibility/UX review, cross-domain invariants, accepted CI at exact SHA and retirement matrix                                           | Release candidate preparation                                                      |

Concurrency policy: B and C may start **when the domain's `view_model` contract is stable**; only C's production persistence needs the database gate. B may explore layout beforehand but cannot sign off data states without the view-model contract. D begins testing each vertical as soon as both surfaces exist, rather than being deferred until the final project phase. A may define a later domain while B/C work on earlier domains.

The `ready` gates are **review/merge gates**, not reasons to block all unrelated work. On a tool failure: inspect → bounded retry → isolate blocker → continue independent ticket; never bypass persistent platform/security controls.

## 4. Domain waves and dependency graph

1. **BodyState first working vertical**: A finishes the full BodyFact/Episode/Source and query projection; B produces Person Records/Habits states; C completes SF-12 Python→Go→DB and Person UI, then SF-20 Episode; D tests factual, ambiguous, negative, retry, conflict and refresh behavior.
2. **Person/Chat single conversation**: A defines serial Run, Ask User, retention and minimal Profile; B designs Chat and identity popover flows; C implements SF-30/70; D verifies cross-device recovery, old sharing retirement and minimum-source preservation.
3. **Diagnosis V2**: A defines ConditionEntry/proposal/decision/evidence projection; B designs Diagnosis and precise confirmation flows; C implements SF-40; D verifies multi-proposal atomic consent, stale rejection, safety blocking and audit.
4. **TrainingPlan/Trainer/SafetyState**: A defines independent plan/trainer identities and fail-closed risk gates; B designs Plan, Trainer and restrictions; C implements SF-50; D verifies per-set user action, timezones and scope matching.
5. **Person assets, Equipment, Media/Reports and cross-domain erasure**: A defines shared UserAsset/source semantics; B designs file and disclosure flows; C implements SF-60 and dependent cleanups; D tests source removal vs erasure, copies/checkpoints and unaffected evidence.

Overlap permitted: when BodyState A has a stable view-model contract, B and C run concurrently; A can move to Chat/Diagnosis; after each vertical passes its gate, D verifies and records it. The sequence above is **dependency order, not a global ban on parallel work**. Training safety depends on qualified BodyState evidence; Diagnosis uses stable source references; UserAsset admission/erasure intersects all.

## 5. Implementation ownership and merge policy

- **One writer per mutable checkout.** ChatGPT directly owns work unless the user explicitly changes execution mode. Agent Fabric is transport. Read `git status`, HEAD, worktrees, active jobs and `.git/bodysense-v2-writer.lock` before a write.
- The schema-first worktree is the current Engineering integration branch; no additional worktree is created per iteration. Existing UI worktree/PR #244 stays independent pending review. Parallelism means independent deliverables/paths and controlled reviews, **not two agents simultaneously editing a shared branch or schema**.
- A owns business-read-model contract drafts and migration handoff; C owns production migration/Go/Agent/API implementation; A reviews database/domain semantics. B owns Penpot and design handoff; C owns React code; D owns integration evidence/merge/release qualification. Shared generated source and migrations require a serialized change/approval sequence.
- Treat `docs/product` as product authority, this master as plan authority, handoff JSON as readiness authority, per-track plans as executable queues, SF plan as engineering checkpoint history. Never create duplicate business-rule authorities.
- No merge of #244 purely for its visuals: reproduce/fix failed checks and compare behavior to current target. Do not merge #247 as 'whole refactor completed' merely because its first vertical has green CI.

## 6. Milestones and checkable exit criteria

| Milestone                          | Acceptance                                                                                                                                                     | Current                                              |
| ---------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------- |
| P0 · Parallel foundation           | Master + 4 linked ACTIVE plans; verified current inventory; per-domain six-gate handoff register; validator/tests and index registered; single-writer protocol | IN PROGRESS until committed, published and validated |
| P1 · First BodyState vertical      | BodyFact/Episode source truth, validated extraction/atomic receipt, Person Records/Habits real API rendering, native PostgreSQL and browser negative-path E2E  | NOT COMPLETE                                         |
| P2 · Conversation & Person         | One serial/resumable Chat, user-safe retention, profile and identity UI, no public health conversation sharing                                                 | NOT COMPLETE                                         |
| P3 · Diagnosis V2                  | Stable condition identities, exact user decisions, atomic proposals, diagnosis window and safety interruption                                                  | NOT COMPLETE                                         |
| P4 · Training & Safety             | Live plan/trainer logs, explicit per-set action, independent fail-closed safety and recovered sessions                                                         | NOT COMPLETE                                         |
| P5 · Assets & cross-domain privacy | Equipment/Media/Reports, admission, source withdrawal and verified health-data erasure                                                                         | NOT COMPLETE                                         |
| P6 · Final migration and release   | Obsolete consumers retired, schema transition proven, exact-SHA CI + real staging/production acceptance                                                        | NOT COMPLETE                                         |

Risk/decision register: Q-17 Diagnosis fixed expiry **must not be invented**; Q-19 diagnosis confirmation visuals need design work; unresolved medical/clinical release thresholds are out of software's authority; legacy multi-conversation/sharing routes are to be removed only after new flow acceptance; UserAsset source deletion != all controlled-copy erasure; UI PR #244 has failing gates.

## 7. Immediate work from this foundation checkpoint

1. **A / DS-BS-01:** inventory BodyState old/new persistence and produce the canonical Records/Habits display projection plus operation/error contract, covering valid/negative/ambiguous statements, Episode linkage, minimum source and revision ownership; record evidence in the handoff register. Keep `000001` immutable.
2. **B / PD-BS-01:** map the approved Records/Habits projection to existing Penpot Person windows; enumerate populated/empty/loading/error/stale, field visibility and confirm/rollback. Do not manufacture a visual snapshot as acceptance.
3. **C / ENG-BS-01:** finish SF-12 actual Python extraction/tool binding using server-owned run/message identity and Person real API usage; continue independently on contract/test work while a separate task is blocked.
4. **D / IR-BASE-01:** establish domain E2E matrix and reproduce #244's Governance/browser failures before UI integration; do not block A/C on unrelated visual polish.

Each ticket completion updates **one gate in the handoff JSON with concrete evidence**, its workstream checkpoint, and the SF ledger where applicable. A finished script or passing isolated schema test alone never closes the entire domain.
