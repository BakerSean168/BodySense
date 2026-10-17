# AS-001 slice 1: PR #327 review repairs

Owner: sole writer in the original AS worktree. Review: GitHub review 5556227819.

1. Enforce owner scope in repository reads, remove unscoped API, verify SQL guard and foreign-owner negatives. Commit before rebase: 1712b24a1; after rebase: 7ef6eec78.
2. Serialize migration and source removal with an asset-scoped PostgreSQL session advisory lock on a pinned connection. Commit replica intent before external IO. Record current and candidate backend/key in `user_asset_storage_cleanups`; only forget after Delete followed by Exists=false. Retain failed/ambiguous cleanup for retry. Filter migration list and CAS to available assets. Clean old source on successful cutover, orphan target on definitive CAS failure, and resume old-source cleanup on migration retry.
3. Regression coverage: migration then removal; removal then migration; removal between listing and lock; CAS failure; transient Delete failure and retry; false-success Delete; actual PostgreSQL owner scope and concurrent lock behavior.
4. After TP-002 has a stable revision, rebase this branch onto latest main, reconcile baseline/schema/generated artifacts serially. Run bounded Go checks and isolated PostgreSQL 18 checks; push the same Draft PR branch with force-with-lease. Independent review and required checks remain merge gates.

The ledger inventories source objects only. Source removal retains asset and derived evidence, and does not issue an AS-002 all-controlled-copy verified-erasure receipt. No staging, production, real buckets, clinical thresholds or TTL policy changes are authorized here.

## Verified checkpoint

- TP-002 stable commit `db014d9f7` was observed clean and rebased onto `0256ffaf3d4d3aee22cf24a9dc734ac30c873c49`; AS then rebased onto that main commit. No TP/UI/main worktree was modified.
- Source cleanup implementation: `c841521cb` after rebase, followed by schema/contract and failure-outcome corrections. The owner-keyed ledger primary key also prevents a foreign-owner conflict from being silently treated as an idempotent insert.
- Isolated pinned PG18, 1 CPU / 512 MiB: baseline up/down/up, domain-validator, `TestUserAssetStorageMigrationRemovalPostgres`, and `TestUserAssetSourceRemovalPostgres` passed. Synthetic fixtures verify owner masking, ledger composite FK, durable intent before Put, lock contention, availability list/CAS guards, and no remaining old/new source objects. The service test is included in the CI PG selection.
- Final local checks passed: `GOMAXPROCS=2 go vet -p 1 ./...`, `GOMAXPROCS=2 go test -p 1 ./... -count=1`, migration sequence/checksums, contracts generation/lint/generated consistency/breaking, Postman generation/verification (99 routes), web typecheck/lint, and `pnpm plan:check`.
- Fake-store regressions cover migration/removal order, lock-time revalidation, rejected CAS orphan cleanup, cleanup retry, ambiguous/failed DB writes, failed or falsely successful Delete, both old/new replicas after failed cutover cleanup, and erased-state source cleanup without changing erasure state.
- Local full Go race and Playwright were not run in this repair round because shared-host Swap remained approximately 8/8 GiB used. New-head required CI and independent ChatGPT Web review remain pending; old-head CI is not evidence for these repairs. Keep PR #327 Draft and #306 open.
