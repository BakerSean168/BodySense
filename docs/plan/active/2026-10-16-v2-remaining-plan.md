# BodySense V2 · Remaining work plan (code-verified) · 2026-10-16

Status: ACTIVE / EXECUTION PLAN · V2 implementation complete 2026-10-19 (§10); REL-001 awaits the user. Subordinate to the [V2 Master](./2026-10-13-v2-parallel-master.md) (§0 big-bang single-writer policy) and to `docs/product/**` (business authority). The execution DAG remains [`queue.json`](../tickets/v2/queue.json); domain gates remain in [`v2-handoff-state.json`](./v2-handoff-state.json). This plan raises **no** gate. **2026-10-17:** execution runs in parallel lanes (§7); §4's single-writer interleaving is historical. **2026-10-18:** domain-closeout sync on main `3dd993a02` (§8).

**Verified against main:** `adaa02607` (`adaa02607a208a1a7d4cc9ba5b70845d6a77f351`: af3e1993b code + #299 status sync + #274 TP-PROP-001); main then advanced to `6687a9e26` with #249 (GitHub Actions pin bump only, no product code). Every "exists" claim below was checked in that tree (baseline `apps/api/migrations/000001_vnext_baseline.up.sql`, `packages/contracts/openapi/bodysense.v1.openapi.yaml`, `apps/api/internal/**`, `apps/web/src/**`, `apps/ai-service/src/**`).

## 0. What is already done (do not rebuild)

| Area                          | Delivered on main                                                                                                                                                                                                                                                                                                                | Evidence                   |
| ----------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------- |
| BodyState ledger              | `body_facts`/`body_fact_commits`, `body_observations`(+commits, V12 correct/withdraw), `symptom_episodes` (+status/recurrence/relations), `body_state_revisions`/`body_state_outbox`, `statement_evidence`                                                                                                                       | PR #279–#281, BS-OBS-001   |
| Hypothesis / ConcernGroup     | user-only `body_hypotheses` + `body_hypothesis_fact_links`, `GET/POST /body-state/hypothesis-records`, `…/{id}/withdraw`, derived `GET /body-state/concern-groups` (no table, provisional id = UUIDv5 of earliest member, `model/concern_group.go`) ; AI hypotheses only in `diagnosis_analyses.decision_trace.agent_hypotheses` | PR #283                    |
| Equipment                     | `equipment_assets` + `/api/v1/equipment[/{id}]`; Treatment generation reads usable equipment                                                                                                                                                                                                                                     | PR #285                    |
| Freshness / review            | relevance-based Diagnosis freshness (`diagnosis_analysis_freshness`) and Treatment review (**both removed 2026-10-18**: Treatment by #348, legacy freshness by #359; see §8)                                                                                                                                                     | PR #286                    |
| Agent body context            | `agent-body-context-v1` built in Go (`service/agent_body_context.go`), Go-only safety_state never sent                                                                                                                                                                                                                           | PR #287                    |
| Safety                        | `safety_screens`, native `safety_restrictions` (scope `all_training`, lift `professional_clearance` only), source-bound `safety_resolutions`, `GET /body-state/safety/restrictions`, `POST …/{id}/resolve`, fail-closed consumers                                                                                                | PR #281/#282, #294–#296    |
| Legacy retirement             | all `body_state_*` legacy tables/model/API removed; Web reads native V2; full Chromium 25/25                                                                                                                                                                                                                                     | PR6 A–E (#284–#298)        |
| PlanProposal precommit policy | pure `apps/api/internal/domain/trainingplan/proposal_admission.go` `EvaluateAcceptance` (owner, pending, content/context fingerprint, base revision, 7-day TTL); **dormant, no caller**                                                                                                                                          | PR #274 (TP-PROP-001 done) |
| Conversation partial          | `idx_runs_one_active_per_conversation`, run leases, `GetSoleLiveByUserID` read-only seam (no caller), `BODYSENSE_V2_RETIRE_CHAT_SHARES` kill-switch on share routes                                                                                                                                                              | #260–#266, PC-D8 gate      |

## 1. Invariants (hold in every slice)

1. **Owner isolation.** Every table is keyed/FK'd by `user_id`; every read/write is owner-scoped; foreign ids return owner-masked 404; composite FKs `(user_id, id)` for cross-table links.
2. **Only user words become facts.** BodyFact/Observation/Episode/Hypothesis/Safety-clearance rows require a user-authored source (`statement_evidence`, user message, structured AskUser answer) bound by id + version.
3. **AI never becomes a fact.** Agent output is a proposal/suggestion or audit trace (`decision_trace`, `agent_hypotheses`) until an explicit user decision is committed by Go with a receipt.
4. **Go owns medical and Safety decisions.** Python/LLM never decides a restriction, a clearance, a Diagnosis adoption or a TrainerGate result; Python receives the Go-built context only.
5. **Safety fails closed.** Missing/unscreened/unavailable/stale Safety ⇒ block (never allow); urgent ⇒ zero-commit for open Diagnosis proposals.
6. **Plans are never auto-modified.** Review/safety changes produce review states or PlanProposals; only an accepted user decision writes a new immutable TrainingPlanRevision.
7. **Replay inputs are frozen.** `replay_input`/`execution_provenance` captured at run time are immutable; replay never re-reads live state.
8. Commit receipt before success (UI shows "saved/confirmed" only after Go receipt); idempotency key + expected revision on every command; no invented clinical thresholds or Diagnosis Q-17 TTL.

## 2. Dev conventions

- **Big-bang mode (Master §0):** no data migration, no rollback/down migration work, no compatibility shims. Single designated writer (lock `.git/bodysense-v2-writer.lock`, own worktree under `~/projects/.hark-worktrees/`). Never touch other worktrees, schedulers, staging or production. DBs are wiped only at V2 deploy time.
- **Schema changes:** edit `apps/api/migrations/000001_vnext_baseline.up.sql` (and `.down.sql`) in place, then regenerate `apps/api/migrations/checksums.sha256` (`sha256sum 000001_vnext_baseline.*.sql`) — `scripts/validate-migration-history.sh` enforces it. No new numbered migrations.
- **Contracts:** edit `packages/contracts/openapi/bodysense.v1.openapi.yaml`, then `pnpm contracts:generate` and `pnpm contracts:check-generated` (Go oapi, TS client, Zod); agent runtime proto in `contracts/internal/agent-runtime/**` (buf).
- **Verification per PR:** `go test -race ./...` in `apps/api` (PG18 integration via `pgvector/pgvector:pg18`), `go run ./cmd/migration-validator` + `./cmd/domain-validator` against a fresh PG18, `uv run pytest` in `apps/ai-service`, `pnpm --filter web test` / typecheck / lint, Playwright Chromium against real Go+Python stub (`BODYSENSE_E2E_STUB_AI=1`), `pnpm plan:check`, prettier (`node_modules/.bin/prettier --check`) on changed docs/JSON.
- **Commitlint:** conventional type ∈ {feat, fix, docs, style, refactor, test, chore, perf, ci, build, revert, merge}; scope ∈ {web, api, ai, assessment, docker, docs, deps, main, ops} or empty; header ≤ 120.
- **Merge rules:** main requires strict up-to-date; rebase + `--force-with-lease` only on Hark-owned branches (others: GitHub update-branch merge); `gh pr merge <n> --merge --match-head-commit <sha>`, never `--admin`. Known flakes: setup-go download → `gh run rerun <id> --failed`; `ask-user-body-facts.spec.ts:92` cold Vite → rerun failed.
- **Docs per merge:** update the ticket doc, `queue.json` (`status`, `acceptedSha`, `mergedPrs`), `v2-handoff-state.json` `latestExecution` (gates only with evidence), and this plan's table.

## 3. Remaining tickets (code-verified)

Size: S ≤ 1 PR/≈½ day, M 2–3 PRs, L 4+ PRs.

### Conversation / Person

#### PC-001 · One Conversation per owner + one serial Run — **M**

- **Exists:** `conversations` (multi per user: title/pinned/deleted_at), `runs` with leases and `idx_runs_one_active_per_conversation`, `conversation_shares` table + `/conversations/{id}/share` + `/shared-conversations/{token}` (behind kill-switch), `ConversationService.CreateConversation/ListConversations/DeleteConversation/PinConversation/RenameTitle`, web `useConversationsQuery` sidebar.
- **Missing:** DB uniqueness `UNIQUE (user_id)` on live conversations; one active Run **per user** (index on `runs(user_id)` for `running|waiting_user`); `GET /api/v1/conversation` (get-or-create singleton, serialized by owner lock) replacing list/create/delete/pin/title routes; delete `conversation_shares`, share routes, `share_service.go`, kill-switch flag; web single-thread consultation (remove conversation list/pin/rename/share UI).
- **Slices:** (1) api: baseline singleton + per-user run index + `GET /conversation` + delete multi routes and share stack; tests: second-device create race returns the same id, second concurrent Run → 409, pending AskUser resumes original Run. (2) web: single conversation UI, remove list/share; E2E second tab sees pending AskUser.
- **Deps:** none (INT-001 done). **Acceptance:** one conversation enforced at DB+API; one active Run per user; no public share path remains.

#### PC-RET-001 · Three-cursor retention + controlled pruning — **M**

> **2026-10-18 closeout:** all four scope items merged (#343 #354 #355, last merge `1f2db35c0`). Blocked on an owner decision about the residual acceptance #355 lists (§8). The bullets below are historical.

- **Exists:** spec models `scripts/plan/v2-chat-retention-*.mjs` (DS-PC-01); Python only truncates model context; LangGraph checkpointer keeps everything.
- **Missing:** cursor columns on `conversations` (`processed_through_message_id`, `context_start_message_id`, `pruned_through_message_id` + revision), Go pruning job (via `jobs`) that deletes `messages`/`thread_projection_*` only below min(processed, context) and only when every referencing `statement_evidence`/AskUser source is preserved; Python checkpoint cleanup endpoint keyed by the pruned cursor; context assembly reads from `context_start`.
- **Slices:** (1) api: cursors + ordering invariants + context window read; (2) api+ai: prune job + checkpoint cleanup with interrupted-resume/concurrent-message tests.
- **Deps:** PC-001. **Acceptance:** no cursor skip, no loss of evidence-referenced source text, no ghost resume.

#### AS-001 · Shared UserAsset admission — **M**

- **Exists:** `user_uploads` (mixes file identity with `ocr_*`/`analysis_*` state), `document_extraction_runs`, `document_indicator_reviews`, upload/source routes, `equipment_assets`, `user_profiles` (gender, birth_date only).
- **Missing:** `user_assets` (owner, kind photo/report/document, storage identity, `availability` available|source_removed|erased, sha256) as sole file identity; OCR/posture analysis moved to derived run tables referencing `(user_id, asset_id)`; `DELETE /assets/{id}` = **source removal** (PC-D7) that marks unavailable but keeps adopted evidence history (CD-D4); Profile optional display fields/preferences.
- **Slices:** (1) api: baseline `user_assets` + rewire uploads/extractions + source-removal command; (2) web: asset views by category + "source unavailable" state; (3, S) profile fields.
- **Deps:** none. **Acceptance:** single owner-scoped asset identity; source removal never revokes historical adopted evidence.

#### AS-002 · Verified erasure of source and controlled copies — **M**

- **Exists:** `privacy_erasure_requests` + `/privacy/erasure-plan` + `/privacy/erasure` (account-level, leased worker).
- **Missing:** per-asset/per-health-data erasure command distinct from source removal; controlled-copy inventory (DB rows, outbox minimal refs, LangGraph checkpoints, Redis cache, object storage, knowledge index, derived records) with per-target verification; receipt with outstanding partial failures; dependent Diagnosis/Training review trigger.
- **Slices:** (1) api: inventory + multi-step erasure state machine + receipt; (2) ai: checkpoint/cache erasure hooks + verification; (3) web+E2E deletion flows.
- **Deps:** AS-001, PC-RET-001. **Acceptance:** no premature "erased"; unrelated sources/decisions untouched.

### BodyState / Agent

#### BS-HYP-001 · Hypothesis/ConcernGroup finalization — **S (rescoped)**

- **Exists:** see §0 (ledger, records, withdraw, derived groups). `RelatedObservationIDs` is always empty ("no V2 observation link"); group id is provisional (DC-032); no correction/supersede of a hypothesis; no web UI.
- **Missing:** `body_hypothesis_observation_links` (and observation links in derivation); hypothesis `correct` (supersede with new revision); DC-032 stable group identity (persist `concern_group_identities` keyed by first member, carried through merge/split by majority-overlap rule — or document provisional id as final); contract doc of ViewModel/error states.
- **Slices:** (1) api: observation links + correct + stable identity; (2) web part moves to UI-001.
- **Deps:** none. **Acceptance:** observation-linked groups; ids stable across unrelated revisions; still no authority/FK target.

#### BS-AGENT-001 · Source-bound AI suggestions → Go decisions — **M**

> **2026-10-18:** slice 1 merged (#347, hypothesis suggestions only). Observation/episode kinds, ai emission and the web card remain (§8).

- **Exists:** runtime proto events `ExtractedInfoUpsert`, `LifestyleContextUpsert`, `InteractionRequired` (AskUser), `RedFlagDetected`; Go `consultation/body_fact.go` commits facts only from accepted structured answers; frozen `agent-body-context-v1`.
- **Missing:** proposal envelope for chat-sourced **hypothesis / observation / episode** suggestions (new proto `BodyStateSuggestion` with cited user `message_id`+span), Go validation (owner, revision, source span exists in the user's own message), surfacing as AskUser confirmation, commit only on user accept; withdrawn-source and stale-revision rejection; bounded retry with no model-text commit.
- **Slices:** (1) contracts+api: proto + Go validator/committer + negative tests; (2) ai: emit suggestions from consultation graph; (3) web: confirmation card + E2E.
- **Deps:** BS-HYP-001 (slice 1). **Acceptance:** extraction stays proposal until Go receipt; auth/replay/cancel negatives reproducible.

#### UI-001 · Person / Episode / Hypothesis presentation — **M**

- **Exists:** `PersonWindow.tsx`, `BodyFactRecords`, `BodyFactAmendment`, `BodyEpisode*` on native V2 reads (#288).
- **Missing:** web for user hypotheses (create/withdraw/correct via `hypothesis-records`) and ConcernGroup view; Episode state badges per Penpot; 390px geometry and empty/pending/error states; Penpot visual acceptance.
- **Slices:** (1) hypotheses + concern groups panel; (2) Penpot states + responsive + E2E.
- **Deps:** BS-HYP-001. **Acceptance:** no false "saved"; Canvas persists while windows change.

### Diagnosis

#### DX-001 → folded into DX-002 slice 1 — **S**

- **Exists:** `2026-10-14-ds-dx-01-diagnosis-contract-handoff.md`, `docs/product/domain/diagnosis.md` §3.0–3.9.
- **Change:** under big-bang there is no separate contract-only PR; the frozen ConditionEntry/Proposal/Decision field list is the first commit of DX-002. Keep Q-17 TTL undecided.

#### DX-002 - Diagnosis aggregate, decisions and native API - **L / DONE (ticket) 2026-10-17**

> **2026-10-17 closeout:** #336 (`GET /api/v1/diagnosis`), #337 (`POST /api/v1/diagnosis/analysis-runs/{runId}/decisions` + `/settlement`) and #338 (Run/Proposal recovery reads) completed the native API; #305 is closed. Legacy candidate-assessment retirement moved to **DX-RETIRE-001 / #341** (after DX-003 and UI-DX-001). The "Remaining" bullet below is historical. No domain gate raised.

- **Merged:** #322 storage/frozen contract; #332 durable user decision/event receipt; #334 whole-run atomic settlement (`74b19b19b7ef9c09eba3a4466c6bf3f670c99dd0`). The kernel now validates exact proposal content/source versions, rechecks accepts and rejects, preserves partial choices, and writes one atomic Diagnosis patch/receipt or none. Urgent risk is a zero-adoption terminal audit.
- **Remaining:** native public read/decide/settle endpoints and error/receipt contracts, generated Go/TS/Zod clients, authenticated consumer migration, then removal of legacy candidate-assessment routes/models/tables. A service-level test is not UI/Agent acceptance.
- **Evidence:** #334 exact head `18e56eb1b9c53a93784c207fe3eb7558f4bfef00`, [12/12 CI](https://github.com/BakerSean168/BodySense/actions/runs/39019387899), full isolated PG18 race/vet/staticcheck, 18 targeted Diagnosis groups, 94/94 plan checks and reproducible existing generated contracts.
- **Dependencies:** BodyState/UserAsset source authorities already merged. **Issue #305 remains OPEN** and still blocks full DX-003 acceptance. No Diagnosis TTL or clinical threshold was invented; scheduler remains user-paused.

#### DX-003 · Diagnosis Go/Agent decision API — **M**

> **2026-10-18 closeout:** both slices merged (#345 #353, last merge `614dcdad2`). Blocked on an owner decision: no ConditionEntry-level freshness hook exists after #359 deleted the legacy freshness (§8).

- **Missing:** diagnosis agent output mapped to `diagnosis_proposals` (AI level read-only, provenance), user accept/reject/direct-correct/withdraw commands, freshness hooks to ConditionEntry, safety interruption.
- **Slices:** (1) api+ai: proposal ingestion + tools; (2) api: direct user correction/withdrawal (§3.5).
- **Deps:** none open (DX-002 native API merged; `#310 blocked by #305` removed 2026-10-17). **Acceptance:** no partial/unauthorized commit; confirmed only after receipt.

#### DX-RETIRE-001 · Delete legacy candidate assessment / diagnosis_analyses / diagnosis_candidates — **L**

> **2026-10-18:** slice 1 web (#356) and slice 2 api/tables (#359, `3dd993a02`) merged. Slice 3 (ai) and the grep acceptance remain (§8).

- **Exists:** legacy baseline tables, Go model/repository/service/HTTP, OpenAPI `/diagnosis-analyses/**` + generated clients, Web consultation Diagnosis panel and AI candidate structures (~28–37 non-test files on `1abcc9f16`).
- **Missing:** delete all of it once DX-003 moves Agent output to native proposals and UI-DX-001 moves the window to the native API; no compatibility layer.
- **Deps:** DX-003, UI-DX-001. **Issue:** [#341](https://github.com/BakerSean168/BodySense/issues/341). **Acceptance:** legacy grep empty outside tests/historical docs; full checks green.

#### UI-DX-001 · Diagnosis window on decision API — **M**

> **2026-10-18:** #349 #350 #351 merged (binding, states, 390px, E2E, Penpot alignment). Blocked on the user's Q-19 visual sign-off (§8).

- **Exists:** `DiagnosisWindow.tsx` (162 lines), `useDiagnosisActions.ts` on legacy analyses.
- **Missing:** bind to `GET /diagnosis` + proposal decisions; distinct pending/undecided/changed/rejected/committed states; Q-19 visual sign-off.
- **Deps:** DX-003.

### Training / Safety

#### TP-001 · Restriction-specific Safety model completion — **M**

- **Exists:** native restrictions with only `risk_level='restricted'`, `scope_kind='all_training'`, `lift_policy='professional_clearance'` (CHECK constraints).
- **Missing (per `safety.md`):** `advisory|urgent` levels, scope `plan`/`exercise` (+ `safety_restriction_exercises`), `on_expiry` (`valid_until`) and `user_reverification` lift with AskUser interaction id; pure Go `TrainerGate(restrictions, planId, exerciseIds) → allow|warn|block + matchedRestrictionIds`; per-restriction expected-version release.
- **Slices:** (1) api: baseline widening + pure TrainerGate with overlap/stale/unavailable negatives; (2) api: reverification/expiry lift commands.
- **Deps:** none. **Acceptance:** fail-closed restriction-level behaviors executable; no invented clinical cutoff.

#### TP-002 · Independent TrainingPlan persistence — **L**

> **2026-10-18 closeout: DONE (ticket).** #326 #344 #346 #348 merged, last merge `2f3f41ee9`; #311 is ready to close. The bullets below are historical. No domain gate raised.

- **Exists:** legacy `training_plans` (single active via `idx_training_plans_user_active`, `phases` JSON, `treatment_id`), `training_logs` (`is_checked_in`, date by server `localDate(time.Now())`), `treatments`/`treatment_revisions` lifecycle, `/treatments/*` and `/training/*` routes, dormant `WithIndependentTrainingPlans` + owner lock (`FOR NO KEY UPDATE`), `domain/trainingplan` precommit policy.
- **Missing:** `training_plans` reshaped (title, goal, status active|paused|completed, multi-active, `current_accepted_revision_id`, related condition/body refs), `training_plan_revisions` (immutable `TrainingPlanContent`: sessions[], frequency, schedule), `plan_proposals` (create|update, 7-day TTL, fingerprints) with accept transaction calling `EvaluateAcceptance` under the owner lock + receipt; remove `treatments`/`treatment_revisions` coupling and `SupersedePlansExcept`.
- **Slices:** (1) api: baseline tables + repo + proposal accept transaction; (2) api: OpenAPI `/training-plans`, `/plan-proposals/{id}/accept|reject`; treatment generation → PlanProposal(create); (3) api: delete Treatment lifecycle/routes.
- **Deps:** TP-001 (gate in accept path), TP-PROP-001 (done). **Acceptance:** plan A never supersedes plan B; no stale safety allow.

#### TP-003 · Trainer runtime + user action receipt — **M**

> **2026-10-18 closeout: DONE (ticket).** #352 #357 #358 #360 merged, last merge `a7826b2b0`; #313 is ready to close. Full Trainer UI is UI-TP-001. No domain gate raised.

- **Missing:** `training_sessions` (user IANA timezone snapshot, started_at), `training_set_actions` (planExerciseId+setNumber+sessionId, performed|skipped, idempotency), TrainerGate check against exact restriction versions on each action, feedback → review projection only.
- **Deps:** TP-002. **Acceptance:** every performed/skipped action user-originated and receipted.

#### UI-TP-001 · TrainingPlan/Trainer windows — **M**

- **Exists:** `TrainingPlanWindow.tsx`, `TrainerEntryGuard.tsx`, `PlanProposals.tsx` (on Treatment revisions), `TrainingExecutionPanel` (name-keyed state).
- **Missing:** bind to plan/proposal/session APIs, per-set actions, gate-disabled states, 390px.
- **Deps:** TP-003.

### Execution / QA / Release

#### OPS-001 · T3 worktree/model health — **close (superseded)**

Master §0 single designated writer removes the need for T3 parallel writers; the blocker is external provider auth. Recommend `status: superseded`; reopen only if parallel agents are re-enabled.

#### QA-001 · Cross-domain exact-SHA acceptance matrix — **M, continuous**

- **Exists:** Chromium suite 25/25 (PR6 E), CI PG18 baseline/recovery/browser lanes.
- **Missing:** owner-masking/replay/stale-revision matrix per domain added as each domain merges; 390px + second-device cases; evidence log per SHA.
- **Deps:** runs after each domain vertical.

#### REL-001 · Release qualification — **L, last**

All six gates per domain with evidence; DB wipe + baseline apply on staging; canary/health/rollback drill. **Deps:** everything above.

## 4. Order and parallel tracks

Single writer ⇒ tracks are an interleaving order, not concurrent writers. Exclusive resource is the baseline migration + OpenAPI, so one schema-touching PR in flight at a time.

| #   | Track A · Conversation/Person                  | Track B · Diagnosis | Track C · Training/Safety | Track D · Assets/Privacy |
| --- | ---------------------------------------------- | ------------------- | ------------------------- | ------------------------ |
| 1   | PC-001 (s1 api, s2 web)                        |                     | TP-001 (s1)               |                          |
| 2   | BS-HYP-001 → UI-001 s1                         | DX-002 s1–s2        |                           |                          |
| 3   | BS-AGENT-001                                   | DX-002 s3 → DX-003  | TP-002 s1–s3              | AS-001                   |
| 4   | PC-RET-001                                     | UI-DX-001           | TP-003 → UI-TP-001        | AS-002                   |
| 5   | QA-001 per domain after each row; REL-001 last |                     |                           |                          |

Domain order: Conversation/Person → Diagnosis → Training/Safety → Assets, then QA/REL. TP-001 s1 is pure Go and may interleave early because TP-002 depends on it.

## 5. Summary table

| Ticket        | Missing scope (short)                                                          | Size | Deps               | Order      |
| ------------- | ------------------------------------------------------------------------------ | ---- | ------------------ | ---------- |
| PC-001        | singleton conversation + per-user run + delete share stack + single-thread web | M    | —                  | 1          |
| TP-001        | levels/scopes/lift policies + pure TrainerGate                                 | M    | —                  | 2          |
| BS-HYP-001    | observation links, correct, stable group id                                    | S    | —                  | 3          |
| UI-001        | hypothesis/concern UI + Penpot states                                          | M    | BS-HYP-001         | 4          |
| DX-001        | done — folded into DX-002 s1 (#322)                                            | S    | —                  | done       |
| DX-002        | done (#322 #332 #334 #336–#338); legacy retirement → DX-RETIRE-001             | L    | —                  | done       |
| BS-AGENT-001  | slice 1 merged (#347); observation/episode, ai, web remain                     | M    | BS-HYP-001         | 6          |
| DX-003        | merged (#345 #353); freshness-hook decision pending                            | M    | —                  | lane A     |
| DX-RETIRE-001 | web + api/tables merged (#356 #359); slice 3 ai remains                        | L    | DX-003, UI-DX-001  | lane A     |
| TP-002        | done (#326 #344 #346 #348)                                                     | L    | TP-001             | done       |
| PC-RET-001    | merged (#343 #354 #355); residual acceptance decision pending                  | M    | PC-001             | 9          |
| UI-DX-001     | merged (#349 #350 #351); Q-19 visual sign-off pending                          | M    | DX-003             | 10         |
| TP-003        | done (#352 #357 #358 #360)                                                     | M    | TP-002             | done       |
| AS-001        | user_assets identity + source removal                                          | M    | —                  | 12         |
| UI-TP-001     | plan/trainer windows on new API                                                | M    | TP-003             | 13         |
| AS-002        | controlled-copy erasure + receipts                                             | M    | AS-001, PC-RET-001 | 14         |
| QA-001        | per-domain exact-SHA matrix                                                    | M    | each domain        | continuous |
| REL-001       | six gates × five domains, staging wipe/canary                                  | L    | all                | last       |
| OPS-001       | superseded by single-writer policy                                             | —    | —                  | close      |
| TP-PROP-001   | done (#274)                                                                    | —    | —                  | done       |

**Next three tasks:** (1) PC-001 slice 1 (api singleton + per-user Run + delete share stack); (2) TP-001 slice 1 (Safety levels/scopes + pure TrainerGate); (3) BS-HYP-001 (observation links, hypothesis correct, stable ConcernGroup id).

## 6. GitHub issues and parallel waves · 2026-10-16

Epic [#301](https://github.com/BakerSean168/BodySense/issues/301) tracks every remaining ticket as a sub-issue with GitHub `blocked by` links; each issue carries scope, slices, invariants and verification. Per user direction (2026-10-16) implementation now runs in **parallel waves**: a ticket may start as soon as its blockers are merged, which replaces the single-writer interleaving of §4 for scheduling. `queue.json` statuses and `maxParallelWriters` are unchanged by this section; PC-001 s1, TP-001 s1 and BS-HYP-001 are already being implemented on `feat/v2-pc001-*`, `feat/v2-tp001-*`, `feat/v2-bshyp001-*`.

| Ticket        | Issue                                                                    | Wave | Blocked by                                                                                                                                              |
| ------------- | ------------------------------------------------------------------------ | ---- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PC-001        | [#302](https://github.com/BakerSean168/BodySense/issues/302)             | 1    | —                                                                                                                                                       |
| TP-001        | [#303](https://github.com/BakerSean168/BodySense/issues/303)             | 1    | —                                                                                                                                                       |
| BS-HYP-001    | [#304](https://github.com/BakerSean168/BodySense/issues/304)             | 1    | —                                                                                                                                                       |
| DX-002        | [#305](https://github.com/BakerSean168/BodySense/issues/305)             | 1    | —                                                                                                                                                       |
| AS-001        | [#306](https://github.com/BakerSean168/BodySense/issues/306)             | 1    | —                                                                                                                                                       |
| PC-RET-001    | [#307](https://github.com/BakerSean168/BodySense/issues/307)             | 2    | PC-001 #302                                                                                                                                             |
| BS-AGENT-001  | [#308](https://github.com/BakerSean168/BodySense/issues/308)             | 2    | BS-HYP-001 #304                                                                                                                                         |
| UI-001        | [#309](https://github.com/BakerSean168/BodySense/issues/309)             | 2    | BS-HYP-001 #304                                                                                                                                         |
| DX-003        | [#310](https://github.com/BakerSean168/BodySense/issues/310)             | 2    | — (was DX-002 #305; removed 2026-10-17)                                                                                                                 |
| TP-002        | [#311](https://github.com/BakerSean168/BodySense/issues/311)             | 2    | TP-001 #303                                                                                                                                             |
| UI-DX-001     | [#312](https://github.com/BakerSean168/BodySense/issues/312)             | 3    | DX-003 #310                                                                                                                                             |
| TP-003        | [#313](https://github.com/BakerSean168/BodySense/issues/313)             | 3    | TP-002 #311                                                                                                                                             |
| AS-002        | [#314](https://github.com/BakerSean168/BodySense/issues/314)             | 3    | AS-001 #306, PC-RET-001 #307                                                                                                                            |
| UI-TP-001     | [#315](https://github.com/BakerSean168/BodySense/issues/315)             | 4    | TP-003 #313                                                                                                                                             |
| QA-001        | [#316](https://github.com/BakerSean168/BodySense/issues/316)             | 5    | UI-001 #309, BS-AGENT-001 #308, PC-RET-001 #307, UI-DX-001 #312, UI-TP-001 #315, AS-002 #314                                                            |
| REL-001       | [#317](https://github.com/BakerSean168/BodySense/issues/317)             | 5    | QA-001 #316, BS-AGENT-001 #308, PC-RET-001 #307, DX-003 #310, TP-003 #313, AS-002 #314, UI-001 #309, UI-DX-001 #312, UI-TP-001 #315, DX-RETIRE-001 #341 |
| DX-001        | folded into [#305](https://github.com/BakerSean168/BodySense/issues/305) | 1    | —                                                                                                                                                       |
| DX-RETIRE-001 | [#341](https://github.com/BakerSean168/BodySense/issues/341)             | 3    | DX-003 #310, UI-DX-001 #312                                                                                                                             |

Waves: **1** PC-001, TP-001, BS-HYP-001, DX-002, AS-001 · **2** PC-RET-001, BS-AGENT-001, UI-001, DX-003, TP-002 · **3** UI-DX-001, TP-003, AS-002 · **4** UI-TP-001 · **5** QA-001 (slices after each domain, closes last), then REL-001. Issue #268 is closed as superseded by TP-002.

**Shared-hotspot rules for parallel PRs:**

- `000001_vnext_baseline.{up,down}.sql` + `checksums.sha256`: rebase on latest main, regenerate checksums and rerun migration-validator before merge; add your own table/index blocks, never rewrite another ticket's block.
- OpenAPI / runtime proto + generated code: rebase, then `pnpm contracts:generate` + `pnpm contracts:check-generated`; regenerate instead of hand-merging generated conflicts.
- `docs/plan/**`, `queue.json`, `v2-handoff-state.json`: implementation PRs do not edit them; the plan owner syncs after each merge.
- One PR per slice; `gh pr merge <n> --merge --match-head-commit <sha>`, never `--admin`; never touch staging/production or another writer's worktree/branch.

## 7. Parallel lanes · 2026-10-17

At the user's request (2026-10-17), Hark implements the remaining V2 work in **parallel lanes** with at most four simultaneous writers. This supersedes the single designated writer of Master §0 and §2/§4 for scheduling only; big-bang mode, invariants (§1) and merge rules are unchanged. Queue baseline: main `1abcc9f1634bb6039e00c6f9654817118ae5cf4c`.

| Lane | Domain       | Order                                                                      | Active now      |
| ---- | ------------ | -------------------------------------------------------------------------- | --------------- |
| docs | Plan/queue   | phase-0 sync; the **only** writer of `docs/plan/**`                        | this sync       |
| A    | Diagnosis    | DX-003 #310 → UI-DX-001 #312 → DX-RETIRE-001 #341                          | DX-003 #310     |
| B    | Training     | TP-002 #311 → TP-003 #313 → UI-TP-001 #315                                 | TP-002 #311     |
| C    | Interleaved  | PC-RET-001 #307, BS-AGENT-001 #308, AS-001 #306 → AS-002 #314, UI-001 #309 | PC-RET-001 #307 |
| —    | QA / release | QA-001 #316 slices after each domain closeout; REL-001 #317 last           | —               |

**Breaking the Diagnosis cycle.** #310 needed #305 closed, but #305's last item (deleting the legacy candidate-assessment stack) needs #310's native Agent ingestion and #312's native window first. The legacy deletion is now DX-RETIRE-001 (#341, blocked by #310 and #312, blocks #317); `#310 blocked by #305` is removed and #305 is closed.

**Rules.**

- **Status lives in GitHub.** The GitHub issue checkboxes (ticked after each merged slice) are the live status. `docs/plan/**`, `queue.json` and `v2-handoff-state.json` are synced **per domain closeout, not per slice**, by the docs lane only; implementation PRs never edit them.
- **Writers.** `maxParallelWriters: 4`. The queue validator (`scripts/plan/validate-v2-ticket-queue.mjs`) still rejects a fifth writer, a shared checkout, an unaccepted prerequisite and any shared exclusive resource that names domain files. The shared generated hotspots (`database-migrations` = baseline migration + checksums, `public-openapi` = OpenAPI + generated Go/TS/Zod + Postman, `domain-contract`) are allowed concurrently and resolved by the rebase rule.
- **Rebase rule.** Whoever merges second rebases onto latest `origin/main`, takes main's version of generated files, regenerates (`scripts/contracts/generate-public-rest.sh`, `pnpm postman:generate`, baseline `checksums.sha256`), and re-runs the checks before merging.
- One slice = one PR; own worktree under `~/projects/.hark-worktrees/`; `gh pr merge <n> --merge --match-head-commit <sha>`, never `--admin`; no staging/production operations.

## 8. Domain closeout · 2026-10-18

Synced on main `3dd993a023c50a0de82c8602a179f09e0338d547` (#359 merge). GitHub issue checkboxes were ticked for merged work first; this section, [`queue.json`](../tickets/v2/queue.json), the ticket docs and [`v2-handoff-state.json`](./v2-handoff-state.json) `latestExecution` follow them. No domain gate is raised and no issue is closed by this sync.

| Ticket        | Issue                                                            | Merged PRs (merge SHA)                                                 | Queue status             | Remaining                                                                                                                                                         |
| ------------- | ---------------------------------------------------------------- | ---------------------------------------------------------------------- | ------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| TP-002        | [#311](https://github.com/BakerSean168/BodySense/issues/311) 6/6 | #326, #344 `cde83efbb`, #346 `e6bf1b2af`, #348 `2f3f41ee9`             | done                     | none; issue ready to close                                                                                                                                        |
| TP-003        | [#313](https://github.com/BakerSean168/BodySense/issues/313) 4/4 | #352 `1f2196746`, #357 `eb1922639`, #358 `e69b4c0b4`, #360 `a7826b2b0` | done                     | none; full Trainer UI is UI-TP-001                                                                                                                                |
| PC-RET-001    | [#307](https://github.com/BakerSean168/BodySense/issues/307) 4/4 | #343 `bf2fea6e9`, #354 `100f14686`, #355 `1f2db35c0`                   | blocked (owner decision) | residual from #355: Ask User expiry settle trigger, cross-domain deletion-scope proof, `interaction_resumes` cascade review, PC3-19 share retirement, browser E2E |
| DX-003        | [#310](https://github.com/BakerSean168/BodySense/issues/310) 2/3 | #345 `3e0931b21`, #353 `614dcdad2`                                     | blocked (owner decision) | ConditionEntry-level freshness/review hook (legacy freshness deleted by #359; open runs are fenced by revision only)                                              |
| UI-DX-001     | [#312](https://github.com/BakerSean168/BodySense/issues/312) 2/3 | #349 `e86e0ff2d`, #350 `78f04e997`, #351 `576201e85`                   | blocked (user sign-off)  | Q-19 visual sign-off                                                                                                                                              |
| DX-RETIRE-001 | [#341](https://github.com/BakerSean168/BodySense/issues/341) 4/6 | #356 `28398ddd6`, #359 `3dd993a02`                                     | queued                   | slice 3 ai: legacy candidate output, `diagnosis_analysis` → `diagnosis` rename, evals, grep acceptance                                                            |
| BS-AGENT-001  | [#308](https://github.com/BakerSean168/BodySense/issues/308) 1/4 | #347 `d0037a291`                                                       | queued                   | observation/episode kinds, typed proto (optional), ai emission + bounded retry, web confirmation card + E2E                                                       |

**Lanes after this sync.** A: DX-RETIRE-001 slice 3 next, with DX-003 and UI-DX-001 waiting on decisions. B: UI-TP-001 #315. C: AS-002 #314, BS-AGENT-001 slices 2–3, AS-001 profile slice, UI-001 residual. UI-TP-001 is admitted in the queue (worktree `bodysense-uitp001`, base `3dd993a02`). An AS-002 writer is also running, but its queue admission waits until AS-001 and PC-RET-001 are accepted. The queue `mainBaselineSha` moves to `3dd993a02`.

**Removed on main since §0:** Treatment lifecycle and Treatment review (#348), legacy `training_logs` and `/training/*` (#360), the legacy Diagnosis analysis/candidate/freshness stack (#359). IG-01 and IG-02 in `docs/product/implementation-gaps.md` are resolved by the Trainer runtime (#352, #360).

## 9. Slice sync · 2026-10-19

Synced on main `f2e52c79ccadc4a868650ac43f02469ff13bc542` (#362 merge), the new queue `mainBaselineSha`.

| Ticket    | Issue                                                        | Merged PRs (merge SHA)             | Queue status | Remaining                                                                                                                                   |
| --------- | ------------------------------------------------------------ | ---------------------------------- | ------------ | ------------------------------------------------------------------------------------------------------------------------------------------- |
| UI-TP-001 | [#315](https://github.com/BakerSean168/BodySense/issues/315) | #363 `fcdc025ed`, #364 `b8b7c1740` | in_progress  | slice 3: corrections, exercise skip, Ended feedback, Home trainer state, retire web Treatment naming                                        |
| AS-002    | [#314](https://github.com/BakerSean168/BodySense/issues/314) | #362 `f2e52c79c`                   | queued       | slice 2 ai checkpoint hook, slice 3 review projection + per-health-data erasure, slice 4 web+E2E; admission waits for AS-001 and PC-RET-001 |

TP-002 #311 and TP-003 #313 are closed. No domain gate is raised.

## 10. Final closeout · 2026-10-19

Synced on main `da14c3435ea3b70bd63bb3a845519722b40a159f` (#388 merge), the new queue `mainBaselineSha`. Every implementation ticket is done and
its issue closed; all five domains have view_model/database/design/engineering/integration gates **ready with
proof** in [`v2-handoff-state.json`](./v2-handoff-state.json), as do F-BODYFACT, F-CONTRACT and F-ACCEPTANCE.

| Ticket        | Issue                                                        | Merged PRs               | Queue status               |
| ------------- | ------------------------------------------------------------ | ------------------------ | -------------------------- |
| PC-RET-001    | [#307](https://github.com/BakerSean168/BodySense/issues/307) | #343 #354 #355 #368      | done                       |
| AS-001        | [#306](https://github.com/BakerSean168/BodySense/issues/306) | #327 #374                | done                       |
| BS-AGENT-001  | [#308](https://github.com/BakerSean168/BodySense/issues/308) | #347 #372 #379           | done                       |
| UI-001        | [#309](https://github.com/BakerSean168/BodySense/issues/309) | #330 #331 #380 #381      | done                       |
| DX-003        | [#310](https://github.com/BakerSean168/BodySense/issues/310) | #345 #353 #366           | done                       |
| UI-DX-001     | [#312](https://github.com/BakerSean168/BodySense/issues/312) | #349 #350 #351 #370 #375 | done                       |
| DX-RETIRE-001 | [#341](https://github.com/BakerSean168/BodySense/issues/341) | #356 #359 #365           | done                       |
| AS-002        | [#314](https://github.com/BakerSean168/BodySense/issues/314) | #362 #367 #371 #376 #377 | done                       |
| UI-TP-001     | [#315](https://github.com/BakerSean168/BodySense/issues/315) | #363 #364 #369           | done                       |
| QA-001        | [#316](https://github.com/BakerSean168/BodySense/issues/316) | #385 #386 #387 #388      | done                       |
| REL-001       | [#317](https://github.com/BakerSean168/BodySense/issues/317) | #383 (prep)              | blocked: awaiting the user |

**QA-001 evidence** ([ticket](../tickets/v2/QA-001.md)): Go race 18 packages and 156 PG18 integration tests, PG18 schema/recovery/DR validators, ai pytest 787 and 7 evals, web vitest 613/613, pnpm verify:release PASS; Chromium 53 tests at 1440 and at 390 with --retries=0 including the API-restart test (environmental failures re-run in isolation, desktop-only specs pinned); main CI green.

**Awaiting the user (nothing executed, never faked):**

1. **F-PENPOT / Q-19:** visual sign-off of the Penpot-aligned UI and the final V2 acceptance.
2. **REL-001 A1** staging wipe, **A2** production wipe, **A3** canary promote
   ([runbook](../../runbooks/v2-release-cutover.md)). `pnpm release:v2:dry-run` on `da14c3435`: failures=0, DRY_RUN_REMOTE_ACTIONS=0, approval points A1/A2/A3; after this sync the only readiness blocker is F-PENPOT (user Q-19).

No staging or production resource, deployment or scheduler was changed. #301 closes with this sync; #317 stays open.
