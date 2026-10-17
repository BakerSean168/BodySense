# AS-001 slice 1: PR #327 review repairs

Owner: sole writer in the original AS worktree. Review: GitHub review 5556227819.

1. Enforce owner scope in repository reads, remove unscoped API, verify SQL guard and foreign-owner negatives. Commit: 1712b24a1.
2. Serialize migration and source removal with an asset-scoped PostgreSQL session advisory lock on a pinned connection. Commit replica intent before external IO. Record current and candidate backend/key in `user_asset_storage_cleanups`; only forget after Delete followed by Exists=false. Retain failed/ambiguous cleanup for retry. Filter migration list and CAS to available assets. Clean old source on successful cutover, orphan target on definitive CAS failure, and resume old-source cleanup on migration retry.
3. Regression coverage: migration then removal; removal then migration; removal between listing and lock; CAS failure; transient Delete failure and retry; false-success Delete; actual PostgreSQL owner scope and concurrent lock behavior.
4. After TP-002 has a stable revision, rebase this branch onto latest main, reconcile baseline/schema/generated artifacts serially. Run bounded Go checks and isolated PostgreSQL 18 checks; push the same Draft PR branch with force-with-lease. Independent review and required checks remain merge gates.

The ledger inventories source objects only. Source removal retains asset and derived evidence, and does not issue an AS-002 all-controlled-copy verified-erasure receipt. No staging, production, real buckets, clinical thresholds or TTL policy changes are authorized here.
