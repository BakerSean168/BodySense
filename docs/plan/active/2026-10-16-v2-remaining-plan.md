# BodySense V2 · Remaining work plan (code-verified) · 2026-10-16

Status: ACTIVE / EXECUTION PLAN. Subordinate to the [V2 Master](./2026-10-13-v2-parallel-master.md) (§0 big-bang single-writer policy) and to `docs/product/**` (business authority). The execution DAG remains [`queue.json`](../tickets/v2/queue.json); domain gates remain in [`v2-handoff-state.json`](./v2-handoff-state.json). This plan raises **no** gate.

**Verified against main:** `adaa02607` (`adaa02607a208a1a7d4cc9ba5b70845d6a77f351`: af3e1993b code + #299 status sync + #274 TP-PROP-001); main then advanced to `6687a9e26` with #249 (GitHub Actions pin bump only, no product code). Every "exists" claim below was checked in that tree (baseline `apps/api/migrations/000001_vnext_baseline.up.sql`, `packages/contracts/openapi/bodysense.v1.openapi.yaml`, `apps/api/internal/**`, `apps/web/src/**`, `apps/ai-service/src/**`).

## 0. What is already done (do not rebuild)

| Area                          | Delivered on main                                                                                                                                                                                                                                                                                                                | Evidence                   |
| ----------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------- |
| BodyState ledger              | `body_facts`/`body_fact_commits`, `body_observations`(+commits, V12 correct/withdraw), `symptom_episodes` (+status/recurrence/relations), `body_state_revisions`/`body_state_outbox`, `statement_evidence`                                                                                                                       | PR #279–#281, BS-OBS-001   |
| Hypothesis / ConcernGroup     | user-only `body_hypotheses` + `body_hypothesis_fact_links`, `GET/POST /body-state/hypothesis-records`, `…/{id}/withdraw`, derived `GET /body-state/concern-groups` (no table, provisional id = UUIDv5 of earliest member, `model/concern_group.go`) ; AI hypotheses only in `diagnosis_analyses.decision_trace.agent_hypotheses` | PR #283                    |
| Equipment                     | `equipment_assets` + `/api/v1/equipment[/{id}]`; Treatment generation reads usable equipment                                                                                                                                                                                                                                     | PR #285                    |
| Freshness / review            | relevance-based Diagnosis freshness (`diagnosis_analysis_freshness`) and Treatment review                                                                                                                                                                                                                                        | PR #286                    |
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

#### DX-002 · Diagnosis aggregate + ConditionEntry + atomic decision — **L**

- **Exists:** legacy per-run `diagnosis_analyses`, `diagnosis_candidates` (AI `confidence`), `diagnosis_candidate_assessments`, freshness; `diagnosis_application_service.go`, `diagnosis_decision_policy.go`.
- **Missing:** tables `diagnoses` (one per user, `current_revision`), `condition_entries` (concept ref, level suspected|likely, source_kind, adopted|inactive), `condition_entry_evidence` (typed EvidenceRef + version), `diagnosis_proposals` (per analysis run, content fingerprint, base revision), `diagnosis_decisions` (+ receipts, idempotency), `diagnosis_revisions`; Go repo/service with one-transaction batch commit; read projection `GET /diagnosis`; delete candidate-assessment legacy path.
- **Slices:** (1) docs+api: frozen contract + baseline tables + repository; (2) api: decision transaction (partial selection, atomic commit, stale/foreign/replay/urgent-zero-commit negatives); (3) api: OpenAPI read/decide + delete legacy candidate-assessment routes.
- **Deps:** none (BodyState done). **Acceptance:** decision batch commits once or none; lineage retained.

#### DX-003 · Diagnosis Go/Agent decision API — **M**

- **Missing:** diagnosis agent output mapped to `diagnosis_proposals` (AI level read-only, provenance), user accept/reject/direct-correct/withdraw commands, freshness hooks to ConditionEntry, safety interruption.
- **Slices:** (1) api+ai: proposal ingestion + tools; (2) api: direct user correction/withdrawal (§3.5).
- **Deps:** DX-002. **Acceptance:** no partial/unauthorized commit; confirmed only after receipt.

#### UI-DX-001 · Diagnosis window on decision API — **M**

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

- **Exists:** legacy `training_plans` (single active via `idx_training_plans_user_active`, `phases` JSON, `treatment_id`), `training_logs` (`is_checked_in`, date by server `localDate(time.Now())`), `treatments`/`treatment_revisions` lifecycle, `/treatments/*` and `/training/*` routes, dormant `WithIndependentTrainingPlans` + owner lock (`FOR NO KEY UPDATE`), `domain/trainingplan` precommit policy.
- **Missing:** `training_plans` reshaped (title, goal, status active|paused|completed, multi-active, `current_accepted_revision_id`, related condition/body refs), `training_plan_revisions` (immutable `TrainingPlanContent`: sessions[], frequency, schedule), `plan_proposals` (create|update, 7-day TTL, fingerprints) with accept transaction calling `EvaluateAcceptance` under the owner lock + receipt; remove `treatments`/`treatment_revisions` coupling and `SupersedePlansExcept`.
- **Slices:** (1) api: baseline tables + repo + proposal accept transaction; (2) api: OpenAPI `/training-plans`, `/plan-proposals/{id}/accept|reject`; treatment generation → PlanProposal(create); (3) api: delete Treatment lifecycle/routes.
- **Deps:** TP-001 (gate in accept path), TP-PROP-001 (done). **Acceptance:** plan A never supersedes plan B; no stale safety allow.

#### TP-003 · Trainer runtime + user action receipt — **M**

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

| Ticket       | Missing scope (short)                                                          | Size | Deps               | Order      |
| ------------ | ------------------------------------------------------------------------------ | ---- | ------------------ | ---------- |
| PC-001       | singleton conversation + per-user run + delete share stack + single-thread web | M    | —                  | 1          |
| TP-001       | levels/scopes/lift policies + pure TrainerGate                                 | M    | —                  | 2          |
| BS-HYP-001   | observation links, correct, stable group id                                    | S    | —                  | 3          |
| UI-001       | hypothesis/concern UI + Penpot states                                          | M    | BS-HYP-001         | 4          |
| DX-001       | folded into DX-002 s1                                                          | S    | —                  | 5          |
| DX-002       | Diagnosis/ConditionEntry/proposal/decision tables + atomic commit              | L    | —                  | 5          |
| BS-AGENT-001 | chat-sourced suggestion envelope → AskUser confirm → Go commit                 | M    | BS-HYP-001         | 6          |
| DX-003       | agent→proposal ingestion, direct correction                                    | M    | DX-002             | 7          |
| TP-002       | plans/revisions/proposals, drop Treatment lifecycle                            | L    | TP-001             | 8          |
| PC-RET-001   | 3 cursors + pruning + checkpoint cleanup                                       | M    | PC-001             | 9          |
| UI-DX-001    | Diagnosis window on decision API                                               | M    | DX-003             | 10         |
| TP-003       | sessions + per-set actions + gate                                              | M    | TP-002             | 11         |
| AS-001       | user_assets identity + source removal                                          | M    | —                  | 12         |
| UI-TP-001    | plan/trainer windows on new API                                                | M    | TP-003             | 13         |
| AS-002       | controlled-copy erasure + receipts                                             | M    | AS-001, PC-RET-001 | 14         |
| QA-001       | per-domain exact-SHA matrix                                                    | M    | each domain        | continuous |
| REL-001      | six gates × five domains, staging wipe/canary                                  | L    | all                | last       |
| OPS-001      | superseded by single-writer policy                                             | —    | —                  | close      |
| TP-PROP-001  | done (#274)                                                                    | —    | —                  | done       |

**Next three tasks:** (1) PC-001 slice 1 (api singleton + per-user Run + delete share stack); (2) TP-001 slice 1 (Safety levels/scopes + pure TrainerGate); (3) BS-HYP-001 (observation links, hypothesis correct, stable ConcernGroup id).
