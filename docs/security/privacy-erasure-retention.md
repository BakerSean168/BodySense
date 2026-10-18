# Privacy Erasure & Retention Boundary

Status: **production contract**
Owner: Go durable domain/runtime boundary
Primary implementation: `PrivacyErasureService` + migration `000054`

## 1. Two deletion semantics

BodySense deliberately exposes two different lifecycle actions:

1. **Conversation deletion** removes that conversation history and revokes its share token. It does **not** mutate the longitudinal BodyState, Diagnosis, Treatment, Training, Outcomes, or unrelated uploads.
2. **Full privacy erasure** is the privileged account-level operation exposed through `GET /api/v1/privacy/erasure-plan` + `POST /api/v1/privacy/erasure`. It is irreversible after confirmation and destroys all data attributable to the user.

The UI must never use copy that implies these operations are equivalent.

## 2. Primary-store retention matrix

| Data family              | Examples / parent tables                                                                                                           | Full privacy erasure                                       | Normal domain retention                                      |
| ------------------------ | ---------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------- | ------------------------------------------------------------ |
| Authentication identity  | `users`, profile, live sessions, refresh families                                                                                  | **Delete/revoke**                                          | Retain while account is active                               |
| Conversation history     | `conversations`, messages, runs, interactions, runtime events                                                                      | **Delete**                                                 | Retain until user deletes conversation/account               |
| Share state              | `conversation_shares`                                                                                                              | **Delete/revoke**                                          | Retain only while source conversation is shareable           |
| Longitudinal BodyState   | `body_states`, facts, observations, revisions, evidence, hypotheses, `safety_screens`, `safety_restrictions`, `safety_resolutions` | **Delete**                                                 | Durable source-of-truth while account exists                 |
| Diagnosis                | analyses, candidates, assessments, freshness, rollout observations                                                                 | **Delete**                                                 | Immutable/revision-pinned while account exists               |
| Treatment                | treatment aggregate, revisions, interventions, rollout observations                                                                | **Delete**                                                 | Immutable revisions while account exists                     |
| Training and outcomes    | plans, logs, outcomes, assessment reports                                                                                          | **Delete**                                                 | Retain while account exists                                  |
| User uploads             | `user_uploads` plus the per-user object prefix                                                                                     | **Delete DB + physical objects**                           | Retain while referenced by active account                    |
| AI review / job payloads | `jobs`, `job_events`, `ai_output_reviews`                                                                                          | **Delete**                                                 | Retain for product/runtime traceability while account exists |
| Global knowledge corpus  | source registry/publication batches/chunks                                                                                         | **Retain**                                                 | Managed by Knowledge lifecycle; not owned by one user        |
| Erasure audit            | `privacy_erasure_requests`                                                                                                         | **Retain only anonymized status/timestamps/report counts** | No health payload; `subject_user_id` is nulled at completion |

Migration `000054` adds missing `users(id) ON DELETE CASCADE` ownership constraints for `conversations`, `runs`, `jobs`, and `ai_output_reviews`, and makes user-derived Diagnosis rollout rows cascade with their analysis. The normal `TreatmentRevision -> Diagnosis` lineage remains `ON DELETE RESTRICT`; the privileged erasure transaction explicitly deletes the user-owned Treatment aggregate first, then deletes the user. This preserves domain immutability during ordinary operation without blocking privacy erasure.

## 3. Erasure ordering and failure recovery

A confirmed request is persisted before destructive work begins. The durable state machine is:

`pending -> running -> completed`

and failures become:

`running -> retryable -> running ...`

Execution order is fixed:

1. atomically create a Redis `user_auth_revoked:<userID>` tombstone and revoke all live sessions;
2. delete all refresh-token families known to those sessions;
3. delete the entire per-user upload object prefix;
4. delete the `users` row in one PostgreSQL transaction and let FK cascades erase user-owned domain/runtime rows;
5. mark the erasure request completed and set `subject_user_id = NULL`.

Session creation uses a Redis Lua script that checks the user tombstone atomically before creating a new session. This closes the race where a login/refresh that began immediately before erasure could otherwise re-arm authentication after global revocation.

The erasure worker leases requests. An HTTP disconnect or a process crash therefore cannot silently abandon a persisted request; stale/retryable work is reclaimed.

## 4. Upload/object semantics

The privacy boundary depends on `UserObjectCleaner`, not directly on the local filesystem. The current `LocalUserObjectCleaner` removes the complete `uploads/<userID>/` prefix, including orphan files not represented by a DB row. BS-PROD-013 replaces this implementation with durable object storage while preserving the same privacy port and prefix-deletion contract.

## 5. Backups

Full erasure is immediate for the live primary store and user object prefix. Immutable disaster-recovery backups are not rewritten in place because doing so would destroy their integrity. Existing pre-deploy local DB backups are pruned after **14 days** by `production-deploy-watch.sh`.

Off-host backups (BS-PROD-012) have an explicit bounded retention policy: objects under the configured `OFFHOST_BACKUP_PREFIX` are pruned to `OFFHOST_BACKUP_RETENTION_DAYS` (default **30 days**, keeping the newest day directory), independent of the watcher's local 14-day retention. Access is restricted to host-only least-privilege OSS keys. The off-host restore operator (interactive, operator-only) must run the erasure recovery/tombstone reconciliation before any restored environment may serve traffic; `restore-production-backup.sh` enforces the isolation and validation gates that make this verifiable, and the passed restore is only ever a disposable drill database.

## 6. Verification contract

`TestPrivacyErasureSyntheticUserPostgres` is the production-shaped synthetic erasure check. It seeds a user with authentication, conversation/share, runtime/job/review, upload, BodyState, Diagnosis + rollout comparison, Treatment revision, and Outcome data; then verifies:

- the user and all attributable DB rows are gone;
- share rows are gone;
- the physical upload prefix is gone;
- the access-session authority is revoked;
- the old refresh credential cannot be reused;
- exactly one anonymized `privacy_erasure_requests` audit row remains.

Run it through `scripts/validate-privacy-erasure.sh` so the entire migration history is replayed before the erasure test.

## 7. Per-asset erasure (AS-002)

Per-asset erasure sits between the two semantics above. It is narrower than full privacy erasure (one owned asset, not the account) and stronger than source removal (`DELETE /api/v1/assets/{id}`, PC-D7), which only removes the private original and keeps derived evidence readable as `source_unavailable`.

`POST /api/v1/assets/{id}/erasure` (idempotency key) admits one erasure per asset; `GET` reads its receipt. The asset becomes unavailable at once, but `availability = erased` and receipt `status = erased` are written only after every inventoried target verified. Until then the receipt lists the outstanding targets with a closed failure code. A SQL trigger refuses `erased` while any target is unverified, and erased is terminal.

### Controlled-copy inventory `asset-erasure-inventory-v1`

Targets run in this order on every pass; each one is executed and verified on its own, so one failure never hides the state of another.

| Target                 | Copies                                                                                                                                                                                                               | Behaviour                                                                                                                                                                                                                                                                                                                |
| ---------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `source_objects`       | primary object and every replica in `user_asset_storage_cleanups` (local / OSS)                                                                                                                                      | source removal under the storage lock, then an independent `Exists` check of every replica; storage outage is `failed` and retried                                                                                                                                                                                       |
| `derived_jobs`         | OCR / posture `jobs` (results hold full model output), `job_events`, `ai_output_reviews` of those jobs                                                                                                               | deleted once no job is live; live work is `failed: derived_work_active` and retried                                                                                                                                                                                                                                      |
| `posture_analyses`     | `posture_analysis_runs`                                                                                                                                                                                              | deleted once terminal                                                                                                                                                                                                                                                                                                    |
| `diagnosis_records`    | adopted condition entries citing the asset (an evidence link to one of its indicator reviews, or a `clinical_document` claim naming it)                                                                              | the pin: `blocked: diagnosis_review_required` while any adopted condition cites it; revising the Diagnosis or withdrawing the condition releases it; nothing in Diagnosis is changed                                                                                                                                     |
| `document_extractions` | `document_extraction_runs` (raw text, indicators) and `document_indicator_reviews`                                                                                                                                   | `blocked: diagnosis_evidence_pinned` while pinned; then rows nothing links to are deleted and rows a withdrawn condition still links to become content-free tombstones (`erased_at`, payloads and hashes cleared; readers skip them; terminal by trigger)                                                                |
| `assessment_reports`   | assessment reports whose frozen input or output names the asset                                                                                                                                                      | `blocked: assessment_replay_frozen` while pinned; then deleted whole (a frozen replay input is never rewritten)                                                                                                                                                                                                          |
| `runtime_checkpoints`  | LangGraph consultation runtime state of the owner's conversations: the inline image of a turn that attached the asset, and derived copies naming it (posture summary, tool results) in any checkpoint, write or blob | erased by the ai-service hook and verified from checkpoint storage (see below); a conversation mid-turn or waiting on Ask User is `failed: runtime_thread_busy` and retried; no receipt is `failed: runtime_unavailable`; a receipt that does not prove every conversation clean is `failed: runtime_erasure_unverified` |
| `asset_manifest`       | the `user_assets` row                                                                                                                                                                                                | finalized last, in one transaction that re-checks every database family under the asset row lock; tombstone keeps identity, kind, size and MIME type; clears the content hash and redacts the file name                                                                                                                  |

Not copies, by design: chat messages keep only the `upload_id` pointer (it resolves to the tombstone and fails closed); Redis holds no asset data (sessions and rate limits only); the knowledge index is the operator-curated global corpus and holds no user-owned rows. `TestUserAssetErasureInventoryCoversSchemaPostgres` fails when a new table references assets by column or foreign key without being inventoried.

### Runtime checkpoints (slice 2)

Go hands the ai-service (`POST /runtime/assets/{asset}/erasure`) every Conversation of the owner (a consultation thread is its Conversation), the ones a message attached the asset to, and the asset's `content_sha256`. Inline image copies are recognised by the SHA-256 of their decoded `data:` URL bytes (the bytes Go resolved from the source), derived copies by the asset id; a record that names the asset goes whole (a posture summary is dropped, a tool result is replaced by a marker, an image block becomes a text marker).

For each thread that was attached or whose stored bytes name the asset, and only when it is idle (not mid-step, not waiting on an interrupt, no pending writes), the ai-service rewrites the latest checkpoint without the copies, deletes every older checkpoint, write and superseded blob, and verifies from storage: the latest checkpoint carries no copy, it is the only one left, and no stored byte of the thread names the asset. Other threads are reported `absent` and left untouched. A turn racing the rewrite wins (`checkpoint_conflict`) and the next pass finishes. Go verifies the target only on a receipt for this asset that reports every listed thread `absent` or `erased`.

Not runtime copies: chat messages keep only the `upload_id` pointer and assistant replies written about an image are conversation records (chat retention / conversation deletion). Finalize does not re-count runtime state: no new copy can appear once the source is unavailable (images are resolved from it) and the posture runs are deleted.

### Lifecycle

`pending -> running -> outstanding -> running ... -> erased`. A pass is leased; the worker resumes expired leases and retries `failed` targets. `blocked` and `unsupported` targets are re-evaluated when the command is repeated (any key) or a later slice adds their handler; they never complete silently. A copy that reappears after its target verified reopens it (`copy_reappeared`).

### Review signal and the pin rule (slice 3)

A source stays **pinned** while an _adopted_ Diagnosis condition cites it. The receipt carries `reviews`, computed live on every read: each adopted condition citing the source (`diagnosis_condition`), each active or paused Training plan whose current accepted revision rests on such a condition or on a record derived from the source (`training_plan`, with the revision), and, for a statement, each derived record that is still asserted (`body_fact`, `body_observation`, `body_hypothesis`). The erasure never edits a Diagnosis, a plan or a record to clear them: the user revises the Diagnosis, withdraws the condition, revises or ends the plan, or withdraws the record, then repeats the command. Withdrawn (inactive) conditions do not pin; the Diagnosis decision trail keeps ids and AI rationale, and its links resolve to tombstones.

### Web flow: source removal versus erasure (slice 5)

The profile drawer's file list offers both actions on each upload and never conflates them. **移除原件** is source removal (`DELETE /api/v1/uploads/{id}`): after a confirmation that says only the original goes, the upload stays listed with the public CD-D4 marker `source_availability: source_unavailable` ("原件已移除 · 整理出的记录保留"); the lifecycle state itself is never exposed. **彻底删除** is erasure: after an explicit confirmation the web sends the command and renders the receipt as _in progress_, _outstanding_ (automatically retried targets with their reason), _review needed_ (a blocked target or a review; nothing is edited for the user) and _done_ only for `erased`. An erased asset is no longer listed. Chromium E2E `asset-erasure.spec.ts` covers the real source removal and erasure paths and the remaining receipt states.

## 8. Per-statement erasure (AS-002)

`POST /api/v1/body-state/statements/{id}/erasure` (idempotency key; `id` is the `source_evidence_id` of a BodyState record) admits one erasure per statement; `GET` reads its receipt (owner-masked 404 otherwise; `409 IDEMPOTENCY_KEY_REUSED` for a key used on another statement; `200` once terminal, `202` otherwise). Same lease, worker and verification model as §7, inventory `statement-erasure-inventory-v1`:

| Target                | Copies                                                                                                                                                                                                                                                                   | Behaviour                                                                                                                                                                                                                                                                                                                |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `diagnosis_records`   | adopted conditions citing the statement (user claim) or a derived fact / observation (evidence link)                                                                                                                                                                     | the pin: `blocked: diagnosis_review_required`                                                                                                                                                                                                                                                                            |
| `training_plans`      | active / paused plans whose current revision rests on a pinning condition or a derived record                                                                                                                                                                            | `blocked: training_plan_review_required`; the plan is never changed                                                                                                                                                                                                                                                      |
| `body_records`        | `body_facts`, `body_observations`, `body_hypotheses` derived from the statement                                                                                                                                                                                          | `blocked: body_record_active` while any is asserted, `diagnosis_evidence_pinned` / `training_plan_review_required` while pinned; then their text is cleared in place (`erased_at`; identity, kind and lineage kept because withdrawn conditions, episodes and accepted plan revisions link to them; terminal by trigger) |
| `assessment_reports`  | reports naming the statement or a derived record                                                                                                                                                                                                                         | `blocked: assessment_replay_frozen` while pinned; then deleted whole                                                                                                                                                                                                                                                     |
| `decision_records`    | the immutable decision trail naming the statement or a derived record: analysis runs and events, proposals, decisions, settlements, revisions, condition entries / evidence / relations / disposition changes / direct edits, plan proposals and accepted plan revisions | **never rewritten**: terminal `retained_by_policy` with `retention_reason: replay_input_frozen` (see below); never retried, never a failure                                                                                                                                                                              |
| `runtime_checkpoints` | consultation runtime state naming the statement or a derived record id                                                                                                                                                                                                   | `POST /runtime/statements/{id}/erasure` with every Conversation of the owner, the one the statement was said in, and the derived record ids; verified like §7                                                                                                                                                            |
| `statement_manifest`  | the `statement_evidence` row                                                                                                                                                                                                                                             | finalized last under the statement row lock (Diagnosis and plan admission qualify a statement under a share lock): `availability = erased`, excerpt and question excerpt cleared. Readers already hide facts whose statement is not `available`                                                                          |

### Frozen replay inputs are retained by policy

Diagnosis runs, revisions and proposals, and plan proposal snapshots and accepted plan revisions freeze the BodyState they were decided on so a decision can always be replayed exactly. Rewriting them would silently change what a past decision was based on, so statement erasure never touches them. Instead `decision_records` ends in the explicit, terminal state `retained_by_policy` with the closed reason `replay_input_frozen`, and the request ends **`erased_with_retention`**: the statement and every other copy are erased except those frozen inputs, which the receipt lists in `retained_targets`. It is never reported as `erased`. A SQL trigger allows `erased` only with all seven targets verified and `erased_with_retention` only with the other six verified and `decision_records` retained, both only once the statement itself is erased; a CHECK allows retention only on `decision_records` and only with a reason. **Full privacy erasure (§1–§3) removes these inputs** together with the account.

Not copies, by design: the user's own chat message the statement was quoted from and assistant replies are conversation records (chat retention / conversation deletion); commit receipts, episode links and safety lift bases hold ids only. `TestBodyFactStatementErasureInventoryCoversSchemaPostgres` fails when a new table references statements without being inventoried.

Later slice: the web deletion flows and their Chromium E2E.
