# BodySense V2 Track D — V12 / React merge blocker handoff (2026-10-14)

Status: **P0 OPEN** / integration blocked. This is a focused QA handoff, **not** a release sign-off or a substitute for `v2-handoff-state.json`.

## Reproduction (exact commits)

- Backend V12: `8bf86e7922c9d68404fd27802960e397b6cd7a24` (its writer worktree currently also has uncommitted Go test/HTTP changes; the merge preflight tests **committed SHA only**).
- React: `1c779b5e04224fb38073b15bdfc426a3ce39b9ac` (clean).
- Schema baseline: `0e287ab128bb09a8e8fae354c16bf5e49acbe7bf`.

From `/home/dev/projects/.chatgpt-worktrees/bodysense-v2-qa`:

```sh
node scripts/plan/v2-stack-merge-guard.mjs 8bf86e7922c9d68404fd27802960e397b6cd7a24 1c779b5e04224fb38073b15bdfc426a3ce39b9ac
node scripts/plan/v2-stack-merge-guard.mjs 0e287ab128bb09a8e8fae354c16bf5e49acbe7bf 1c779b5e04224fb38073b15bdfc426a3ce39b9ac
node --test scripts/plan/v2-stack-merge-guard.test.mjs
```

First comparison returns `mergeable=false`, `exitCode=1`, conflict only `apps/web/src/features/body-canvas/components/BodyFactRecords.tsx`. The Schema/React baseline returns `mergeable=true`. The QA guard has 7/7 passing unit regressions; its two source files are still uncommitted in this isolated QA worktree. Live GitHub PR #250 and #252 currently have **zero** checks (stacked base branches), and PR #253 checks target older V11 SHA, not this V12.

## Semantics that must survive the merge

**Backend V12 side**, `BodyFactRecords.tsx`:

- `savedRevision` tracks a successful receipt (including amendment `committedRevision`), and `snapshotBehindCommit` compares the loaded page revision with the durable receipt.
- When the page is behind a successful commit: hide stale records, suppress empty-state claims, block additional create/correction/withdrawal against old data, show syncing state; keep the idempotent unknown-outcome/retry path and the owner-change reset.
- On amendment commit, update `savedRevision` with `Math.max` before invalidating the workspace query.

**React/Design side**, same component:

- Keep the memoized `projectBodyFactSurface`, `projectBoundedBodyFactVM`, bounded contract/state attributes on the surface, and correct empty/partial-empty projection.
- Do not replace the bounded VM or source/evidence semantics with local ad hoc flags.

The implementation must combine both in **one** owner-authenticated record surface; selecting one side wholesale is not acceptable.

## Required regression / closure evidence

1. A successful create receipt at revision N followed by a stale GET revision < N: render **no old record and no false empty state**; disable create and amendment until a fresh GET >= N.
2. A successful correction or withdrawal receipt followed by stale GET: do not display or allow rewriting the old assertion; preserve `committedRevision` forwarding from `BodyFactAmendment`.
3. 409 revision conflict and uncertain network outcome preserve idempotency, prevent duplicate writes and allow deliberate refetch/retry.
4. Owner switch, user isolation, partial pagination changes, empty/partial-empty states, 390px mobile and desktop Chromium layout.
5. New **clean integrated exact SHA** passes Go/Python/PG18 race/migration, generated contract checks, React focused tests and real Go+Chromium E2E; green CI for its own SHA, not prior #247/#253 SHAs.

**Ownership:** Track C React owner owns `BodyFactRecords.tsx` semantics/merge; Track C backend owns receipts/negative DB evidence; Track D independently runs merge preflight and E2E before accepting. Do not touch the V12 writer worktree or flip domain/release gates as part of this handoff.

**Run evidence (2026-10-14 UTC):** QA test+preflight `gcp-dev-01::job_req_5e2c72dd500c4c6c35dfe8784a47c46c`; React focused test first invocation from repository root failed to resolve `@/` aliases (runner cwd, not a code regression), corrected rerun from `apps/web` tracked independently. PG18 native test launch was blocked by platform safety checks; there is **no new PG18 success evidence** from this run.

## 2026-10-14 01:52 UTC · Updated V12 exact-SHA recheck

The backend writer advanced the **committed** V12 head to `2396b1ebb77ef62919a993f8871f2ff553f46a4d` (`test(api): validate observation amendment receipts and replay`), 58 insertions / 5 deletions across five Go files since the prior QA comparison. The V12 working tree still has an uncommitted HTTP test change and an untracked `.venv`; neither is included in the committed merge check.

Native `git merge-tree --write-tree 2396b1ebb77ef62919a993f8871f2ff553f46a4d 1c779b5e04224fb38073b15bdfc426a3ce39b9ac` returned exit **1** with a **single content conflict** in `apps/web/src/features/body-canvas/components/BodyFactRecords.tsx`. The P0 ownership and regression requirements above therefore still apply at the new committed V12 head. Independent QA Merge Guard 7/7 passed; the separate clean-archive Go Race and further tests require their own completed job evidence and are **not** assumed green by this note. At recheck, PR #253 remains `c2076d1dd` (12 checks), PR #250/#252 have zero checks and PR #244 has four failures. No release gate may be raised from these prior PR checks.
