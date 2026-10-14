# BodySense V2 · Execution Reorganization and Integration Control

Status: ACTIVE / EXECUTION CONTROL (subordinate to [V2 Master](./2026-10-13-v2-parallel-master.md)).
Started: 2026-10-14. Implementation owner: ChatGPT Web, GCP `gcp-dev-01`.
This is an execution ledger, **not a new product authority, domain gate, or second Master Plan**.

## 1. Objective and authorities

Convert existing local V2 work into one clean, reproducible integration baseline. Then run independently scoped domain tickets in durable T3 projects/worktrees, with ForgeFlow preflight as an optional dispatch policy and ChatGPT Web owning review/integration. No staging/production without the normal repository release gates.

Authorities: `docs/product/**` for approved medical/business behavior; `2026-10-13-v2-parallel-master.md` for global dependencies; `v2-handoff-state.json` for six per-domain readiness gates; `2026-10-13-schema-first-full-refactor.md` for existing SF-00..90 ticket continuity; `docs/plan/tickets/v2/queue.json` for **execution intent only**.

Protect source provenance, tenant/revision/idempotency, atomic decision receipts, original-Run AskUser, Safety fail-closed, controlled-copy erasure, persistent 3D scene, and the rule that unconfirmed model output is not a clinical fact. Never invent Diagnosis Q-17 TTL or clinical thresholds.

## 2. Inspected inputs (2026-10-14 UTC; recheck before every write)

| Lane                    | Source revision at discovery               | Worktree/ownership                                                     |
| ----------------------- | ------------------------------------------ | ---------------------------------------------------------------------- |
| Canonical schema #247   | `0e287ab128bb09a8e8fae354c16bf5e49acbe7bf` | `/home/dev/projects/bodysense`, contains unrelated `.zcode/`           |
| Backend Observation V12 | `2396b1ebb77ef62919a993f8871f2ff553f46a4d` | Existing lock `ChatGPT-Web/ENG-BS-04`; one dirty HTTP test and `.venv` |
| Domain                  | `e823931390cee700891b35c73cf26bc4b2374e46` | Existing Track A worktree has unfinished Training/Safety files         |
| Design #250             | `a3b8db87e1ca190af08d95f648b540486022962f` | Independent, clean                                                     |
| React #252              | `1c779b5e04224fb38073b15bdfc426a3ce39b9ac` | Independent, clean                                                     |
| QA                      | `86b2189841ce80a8ee73a8479d1581dbf5937009` | QA Merge Guard 7/7 PASS, commit saved                                  |
| Legacy UI #244          | `3597a5f29bc1085f319c91f6b51c1bdaa481f084` | Separate PR with failed CI; selective reuse only                       |

Dedicated integration writer/worktree: `/home/dev/projects/.chatgpt-worktrees/bodysense-v2-integration`, branch `integration/v2-execution-20261014`, initially created from `0e287ab12`. No other writer may edit it. Domain and QA committed slices can be cherry-picked by SHA. Design/React/V12 may be merged only with conflict review and exact-SHA tests. This execution does **not** alter their original worktrees, and uncommitted owner changes are never silently imported.

## 3. Phased integration gates

### INT-P0 · Reconcile the existing work (active)

1. Preserve and review each checkout's dirty diff, writer lock and current job/thread. Do not reset, clean, force-push or steal another writer's changes.
2. Integrate committed source snapshots in an isolated worktree; record origin SHA and merge commits. Never claim a source checkout's uncommitted modifications were accepted.
3. Resolve `apps/web/src/features/body-canvas/components/BodyFactRecords.tsx` retaining **both** V12 `savedRevision/snapshotBehindCommit` stale-read suppression and the Design/React `useMemo/projectBoundedBodyFactVM` presentation projection.
4. Run focused React + merge guard + domain contract + Go race tests; then generated contract parity, PG18 replay and cross-process browser suite as environment permits.
5. Commit a clean integration candidate with independent verification; do **not** merge to `main` or deploy based on incomplete tests.

Exit: clean exact integration SHA; no unmerged paths; provenance of merged SHAs; required tests recorded as PASS/FAIL/BLOCKED with exact command and evidence; identified failures owned by one ticket.

### INT-P1 · Execution-ready DAG (active)

Ticket documents and `queue.json` are under `docs/plan/tickets/v2/`. Do not reclassify legacy SF tickets as done without proof. Each ticket specifies dependencies, allowable write scope, protected contract, negative tests and acceptance evidence. Missing dependency proof means `blocked`, not an invitation to improvise.

### INT-P2 · T3/ForgeFlow three-writer pilot (admission gated)

`t3_thread_create` does **not** accept a cwd/worktree; its project ID must map to an independently registered absolute project root. `t3 project add <verified-worktree-path> --title ...` is available on GCP. Register durable worktrees, verify project root and native process cwd, then create ONE writer thread per lane. A thread title is not isolation proof. T3 model enumeration lists DeepSeek Pi/DSH routes, but GLM 5.3 is presently through the ZCode external ACP profile, not a proven T3 provider route.

ForgeFlow V1 is a **deterministic policy preflight**, not a second scheduler; Pi Planner may refine already-approved tickets; pi-subagents owns Pi worker lifecycle. Current ForgeFlow inventory marks `pi-fast-writer` `modelRegistered=false`; do not dispatch it until physical model resolution and one bounded real-health call pass. `zcode-fast-writer` (WindHub GLM 5.3) is inventory-eligible on prepared worktrees but needs live inference/auth validation. Do not assume either is healthy from inventory.

Pilot max 3 independently checked out writers. Reviewers read only. If a provider cannot reliably write, ChatGPT Web uses the SAME admitted worktree after the original writer is stopped; never double-write.

### INT-P3 · Domain parallelism

Use `queue.json` dependencies **and exclusive resources** to schedule waves. Eligible domain-contract drafts may run concurrently in distinct persistent worktrees. The `apps/api/migrations`, canonical OpenAPI, generated clients and `v2-handoff-state.json` are serialized shared resources; only a nominated integration owner can accept those changes. Separate business domain readiness from available implementation.

### INT-P4 · Exact release gates

Every accepted ticket: changed paths + source/target SHA + focused test + relevant cross-layer tests + independent review + accepted integration SHA. Integration Gate only after authenticated UI→Go→PostgreSQL→Python/Agent (where used) and Browser/PG18 negative tests. Product or Design sign-off does not establish DB correctness. Do not publish old PR CI as new candidate CI. Staging/production require existing deployment policy, exact environment ownership, health and rollback.

## 4. Ongoing risks and owners

- P0: unresolved source merge or loss of stale snapshot/VM behavior → INT-001 (Integration).
- P0: unsafe SafetyState JSON overwrite without restriction-specific revision → TP-001/TP-002 (Domain/Backend).
- P1: singleton Conversation, retention, Diagnosis decision and asset erasure are not implemented → respective tickets.
- P1: PR #250/#252 stacked bases get no `pull_request` checks under CI restricted to main; no false green → INT-002.
- P1: platform safety write denials sometimes block commits; classify actual gate result, retry boundedly only for transient failures; never bypass persistent permissions.
- P1: T3 project-level worktree binding and live model quota unresolved → OPS-001.

## 5. Execution protocol

- Before dispatch: read the chosen ticket, `docs/product`, `queue.json`, git SHA/status/worktree list, active T3 jobs, lock ownership, provider health.
- Reserve one durable project+worktree per writer lane; never spawn a new checkout each hour. Write the T3 thread ID and verified project/worktree binding to a single execution ledger; keep `v2-handoff-state.json` solely for accepted domain gates.
- Check exact dependencies, lock tokens, and target paths; send one Ticket at a time with the instructions in its document. Commit tested work; use the normal `git`/`gh` CLI.
- On result: inspect diff, run focused tests, compare protected contracts, merge to integration candidate, run real integration tests, record exact integration SHA and release blockers.
- Continue the existing hourly Track A/C/D tasks unless intentionally replaced after a verified pilot. React hourly automation was disabled as of this checkpoint; do not assume all four tasks are active.

## 6. Immediate actions

Finish INT-001 reconciliation, commit the integration candidate after real tests, and close no whole-domain gates. Then verify OPS-001 (T3 native project roots/provider health) and admit a small three-writer wave of independent tickets. Generate no empty implementation threads and never start a worker without a path-specific ticket.

## 7. 2026-10-14 observed first execution checkpoint

- Created persistent isolated `integration/v2-execution-20261014` worktree from canonical Schema `0e287ab12`; cherry-picked Domain `ab65cfd65`/`e82393139` and QA `86b218984`, then merged Design `a3b8db87` and React `1c779b5e0`. Original source worktrees were not edited.
- The merge of Backend V12 committed HEAD `2396b1ebb` exposed one conflict in `BodyFactRecords.tsx`. The integration-only resolution keeps both `savedRevision/snapshotBehindCommit` protections and memoized `projectBodyFactSurface/projectBoundedBodyFactVM`. It does not include the owner worktree's uncommitted HTTP test.
- QA 7/7, focused React four files/33 tests, Web TypeScript typecheck, V2 plan 33/33 and ticket queue 8/8 passed before the final merge commit. Go race and generated contract parity are separate pending jobs; never present a candidate as accepted until their exits are verified.
- Ticket queue currently contains 21 concrete documents and disjoint resources with a maximum three-writer admission gate. No domain gate is raised by creating these documents.
- Native `t3 project add` registered project `67a21bd5-5929-468b-b6ea-e252a8b11e27` at the integration path, but Agent Fabric T3 gateway returned `t3_project_denied` and its authorized project listing did not include the project. The local T3 CLI build `2955` is older than service build `2994`, and reported an unknown recorded migration ID. **No T3 writer is admitted**; this access/version mismatch is OPS-001 and must be resolved via supported normal project authorization, not raw API or database bypass. Details in `docs/plan/tickets/v2/dispatch-ledger.json`.

## 8. 2026-10-14 integration QA checkpoint

**Passed (local evidence only, not full V2 release):**

- Committed V12/React merge `6e59dd0db`; execution DAG `0563f626c`, admission guard `cbca5375c`; mobile E2E selector fix `4539f8192`. The V12 owner's uncommitted changes were never imported.
- Native Go `-race` Model/Repository/HTTP (Observation/BodyFact/Episode/InteractionResume) **three packages PASS**, exit 0, `job_req_1418ce9e5f708ba28e4dfc221adb45c2`.
- React BodyFact focused **33/33 PASS**; independent `BusinessWindows.test.tsx` **21/21 PASS** after a timing failure in the full suite; TypeScript check PASS.
- `pnpm contracts:check-generated && pnpm postman:verify` **PASS**, Postman **110 operations**, exit 0, `job_req_eca8ace187f9c2397aa0575a4c79a5f8`. Plan checks, queue dependency/collision validator and QA Merge Guard pass.
- Real isolated **PG18 + Redis + Go + Python + React + Chromium** mobile Ask User, `BODY_FACT_TEST_GREP='Ask User mobile replacement composer' BODY_FACT_TEST_RETRIES=0 bash scripts/validate-body-fact-vertical.sh`: **1/1 PASS without retry**, exit 0, `job_req_16315bfd9f7fdd9ef416b2199c9c4032`. Evidence: `.runtime/schema-first-v2-evidence/vertical-e2e-1640595/browser.log`. Synthetic containers cleaned.

**Unaccepted / reproducible QA findings:**

- First full React Vitest had **491/492 PASS** with one 5-second repeated-Dock test timeout under concurrent work; isolated 21/21 then passed. The subsequent full Web rerun completed **80 files / 492 tests PASS**, exit 0, job `gcp-dev-01::job_req_7f7d3af0195870e0b4e3b9dc2f214f92`. The earlier Dock timeout appears load-sensitive; exact-head CI remains independently required.
- The first real 17-case E2E run was not successful: one Ask User foreign-owner/cancel path failed on its first attempt (composer unavailable at 8 seconds), passed on retry; the mobile test failed twice because old `.bc-business-window-title` selector ignored the accepted V2 Chat contextual title `.bc-chat-context-title`; and the Python extraction/Person Habits path ran too long in the overloaded environment. That obsolete-source batch was deliberately cancelled; its two synthetic PG18/Redis containers were inspected and removed. No green full-suite claim.
- The mobile test fix `4539f8192` preserves narrow-screen overflow, dark/light title color, source binding, draft and true commit assertions. The reusable native harness now admits focused `BODY_FACT_TEST_GREP` and `BODY_FACT_TEST_RETRIES`, defaulting to the original full suite; all arguments use arrays, retries are validated, and termination follows cleanup.
- `INT-001` stays **IN_PROGRESS** until a clean exact-SHA integrated CI + broad negative E2E acceptance. `OPS-001` and all new T3 writers stay gated because newly registered project ID `67a21bd5-5929-468b-b6ea-e252a8b11e27` was denied by Agent Fabric T3 (`t3_project_denied`). Do not bypass permissions, infer current model health, raise `v2-handoff-state.json` Gates, or deploy.

## 9. 2026-10-14 final focused cross-process retests

After the source-compatible Chat header E2E repair and focused-harness commit `37148991ef39b8ea8fbf91e9a975cae615a9ec7c`, two previously unstable cases were independently rerun on fresh isolated PG18/Redis/Go/Python/React/Chromium resources with **retries=0**. Job `gcp-dev-01::job_req_a535d3927f41ca2632aac9c7003e9f85` returned **exit=0, 2/2 PASS**: (1) foreign-owner, forged-choice and cancelled Ask User refusal; (2) Python extraction -> exact Go commit receipt -> PG18 -> actual Person Habits with replay. Reproducible log: `.runtime/schema-first-v2-evidence/vertical-e2e-1656743/browser.log`. Its two labelled synthetic containers are gone, and the integration Git worktree is clean at the tested HEAD.

Together with the preceding separate mobile Ask User zero-retry **1/1 PASS**, and second complete React rerun **80/80 files, 492/492 tests PASS**, this closes these three bounded local regression findings. It does **not** close the 17-case full E2E or any whole-domain integration/release gate; exact-head GitHub CI, full migrations/rollback, remaining clinical/safety/privacy domains, T3 admission and design approval are still required. No production/staging deployment or shared infrastructure mutation occurred.

## 10. 2026-10-14 PR #255 first exact-head CI repair

At PR #255 head `758d9ac15d4c0f963b1f28e18ae13e781b85101d`, the actual `pull_request` CI detected two issues, not an acceptable release candidate:

1. The Web production chunk `ConsultationPage` was **503,848 raw bytes** against the unchanged **500,000-byte budget**. The failure was reproducible locally. Root cause: UI barrel exports in the consultation/workspace/training dependency graph pulled an unused legacy `TrainingExecutionPanel` and other modules into the route entry.
2. Longitudinal Browser E2E used the removed V1/V5 `<h2>综合诊断` expectation while the approved V2 component exposes a toolbar selector. The Observation public API test independently attempted another cookie-rotating `/auth/refresh` after browser reload, racing browser bootstrap and causing intermittent 401; API idempotency/owner denial were not the cause.

The integration-only candidate corrects the dependencies by using leaf Workspace/Training service and query-key modules and loads the legacy Trainer UI only through `React.lazy` **after all four Go/Plan/Safety/explicit-launch UI admission predicates**. It preserves fail-closed gates and leaves the global Web budget at 500 KB; a fresh production Vite build **PASS** (job `job_req_896ec42e0f40ff0bc5ebd9b2777db5d5`), with `ConsultationPage` **481.26 KB** and separate `TrainingExecutionPanel` **23.82 KB**. Three focused Vitest files, **29/29 PASS** (job `job_req_e36da0a8f3c3be3c23bad47f3deff40d`).

E2E now tests the actual labeled V2 Diagnosis combobox, absence of invented adopted conditions, and visible empty state. Observation tests reuse the still-valid already authenticated owner's bearer token across page reload rather than unnecessarily attempting a _second_ refresh; the 401 unauthenticated/foreign-owner denial assertions remain. A fresh 17-case real PostgreSQL 18 + Go + Python + React + Chromium suite with zero retries was started separately; **do not claim it passed until the job terminates successfully**. New GitHub CI must run at the subsequent exact pushed commit. These remain bounded acceptance steps only; the V2 six-domain Release Gate and staging/production are not approved.

## 11. 2026-10-14 exact regression reproduction and retest

At PR #255 candidate **75fb200b9cdf089ed800218eecd37fc539f4ab6b**, GitHub `Affected quality child` was **PASS** (run `38512449205`, job `115683844356`; all PostgreSQL, governance and commit-lint gates seen were PASS). This independently establishes the production 500 KB bundle fix; the exact-head Browser job was still running at this entry.

The isolated full real stack (Go + PG18 + Python + React + Chromium) `BODY_FACT_TEST_RETRIES=0 bash scripts/validate-body-fact-vertical.sh` exited **1**, **16/17 PASS** (job `job_req_8fac45d38ef01eea657e4b30071030cb`, log `.runtime/schema-first-v2-evidence/vertical-e2e-2070826/browser.log`). The sole failure was Playwright expecting a closed native `<option value="">暂无已确认病情</option>` to be visible. The diagnosis dialog/body itself and its disabled selector were present. No loss of Go data or medical safety policy was involved.

The follow-up changes only that assertion: use V2 `role=region, name=当前诊断内容`, assert the named Diagnosis combobox is disabled, verify its native `option` text using `toHaveText` rather than `toBeVisible`, and retain the visible adopted-condition empty state and Close action. This aligns with the independently authored QA Track D change `a89a80900` without importing another writer's uncommitted work.

Exact failed-case isolated replay:
`BODY_FACT_TEST_GREP='register -> profile -> durable BodyState fact survives reload' BODY_FACT_TEST_RETRIES=0 bash scripts/validate-body-fact-vertical.sh` returned **1/1 PASS without retry**, job `job_req_73564cfc585fe295d4c079730622fc0b`, `.runtime/schema-first-v2-evidence/vertical-e2e-2116124/browser.log`, exit 0 and test containers cleaned. The remaining 16 independent cases passed the immediately preceding complete run. **A full 17/17 on the new SHA and exact-head GitHub required checks remain the final integration admission tests; do not equate 16+1 across different SHAs to one full-run acceptance.**

GCP's infrastructure-owned T3 allowlist drift has been reconciled and merged to `personal-infrastructure/main` at `4e25ae12`; the only desired/live config difference was the new registered project. The host's config was atomically staged with mode 0600, but active Agent Fabric `t3_orchestrator_describe` still advertises only seven older projects. `OPS-001` remains blocked until safe approved Worker reload and real project/model/checkout verification. This work does not modify `v2-handoff-state.json` or authorize V2 release.
