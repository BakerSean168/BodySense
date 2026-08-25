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

| Data family | Examples / parent tables | Full privacy erasure | Normal domain retention |
| --- | --- | --- | --- |
| Authentication identity | `users`, profile, live sessions, refresh families | **Delete/revoke** | Retain while account is active |
| Conversation history | `conversations`, messages, runs, interactions, runtime events | **Delete** | Retain until user deletes conversation/account |
| Share state | `conversation_shares` | **Delete/revoke** | Retain only while source conversation is shareable |
| Longitudinal BodyState | `body_states`, facts, observations, revisions, evidence, hypotheses | **Delete** | Durable source-of-truth while account exists |
| Diagnosis | analyses, candidates, assessments, freshness, rollout observations | **Delete** | Immutable/revision-pinned while account exists |
| Treatment | treatment aggregate, revisions, interventions, rollout observations | **Delete** | Immutable revisions while account exists |
| Training and outcomes | plans, logs, outcomes, assessment reports | **Delete** | Retain while account exists |
| User uploads | `user_uploads` plus the per-user object prefix | **Delete DB + physical objects** | Retain while referenced by active account |
| AI review / job payloads | `jobs`, `job_events`, `ai_output_reviews` | **Delete** | Retain for product/runtime traceability while account exists |
| Global knowledge corpus | source registry/publication batches/chunks | **Retain** | Managed by Knowledge lifecycle; not owned by one user |
| Erasure audit | `privacy_erasure_requests` | **Retain only anonymized status/timestamps/report counts** | No health payload; `subject_user_id` is nulled at completion |

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
