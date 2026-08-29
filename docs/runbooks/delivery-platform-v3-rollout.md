# BodySense Delivery Platform V3 Rollout Runbook

> Date: 2026-08-29
> Safety rule: no phase may weaken the currently working production release/deploy path before the replacement proves equivalent or stronger.

## 1. Rollout philosophy

Delivery infrastructure is changed with the same discipline as application runtime migrations:

```text
introduce new path
→ run in shadow
→ compare evidence
→ switch policy authority
→ retain rollback seam
→ remove legacy path last
```

The production host is not used as an experimental validation target.

## 2. Preconditions

Before each phase:

```bash
git fetch origin --prune
git status --short --branch
git rev-parse HEAD
git rev-parse origin/main
```

Implementation happens on a short-lived branch/worktree derived from current `origin/main`.

Production must remain healthy. Any phase that changes release/promotion semantics requires a successful repository validation plus workflow syntax/action validation before merge.

## 3. Phase 0 — Documentation and drift cleanup

Changes:

- add ADR 0008;
- add Delivery Platform V3 architecture and Release Lifecycle V3 docs;
- add active implementation plan;
- remove nonexistent `dev` branch trigger from BodySense CI;
- update deployment/architecture navigation to single-trunk terminology.

Acceptance:

- current legacy required CI jobs retain their names and semantics;
- production workflow remains unchanged;
- no branch-protection mutation occurs.

Rollback: revert documentation/trigger commit.

## 4. Phase 1 — Shadow control plane

Introduce:

```text
scripts/delivery/generate-manifest.mjs
scripts/delivery/validate-manifest.mjs
scripts/delivery/oracle.mjs
scripts/delivery/observe-run.mjs
```

plus deterministic tests and a shadow workflow/job.

Rules:

- the shadow detector may not skip existing required CI;
- Oracles are observational and not yet protected contexts;
- manifest generation must fail closed on unknown event/SHA boundaries rather than silently mark everything docs-only;
- `main` manifests always enable the full policy.

Acceptance:

- docs-only, Web-only, API-only, AI-only, migration, contract, root/CI and release path fixtures produce the expected lane matrix;
- digest tampering fails validation;
- an enabled missing/skipped child causes Oracle failure;
- a legitimately disabled child is accepted;
- existing CI remains green independently.

Rollback: remove the shadow workflow and scripts; no production effect.

## 5. Phase 2 — Affected PR / full main cutover

Change CI so PR child work is selected from the manifest while `main` always runs full.

Introduce stable Oracle jobs while preserving legacy job execution during initial comparison.

Observe at least representative runs for:

```text
docs-only
Web-only
Python Agent
Go API
migration
root/CI
shared contract
```

Only after equivalence is demonstrated may repository governance switch required status checks to stable Oracles.

Repository-protection switch must be idempotent and itself validated in dry/runbook form before applying.

Rollback: restore old required contexts and full PR jobs. Main full CI remains available throughout.

## 6. Phase 3 — Immutable main candidate and staging channel

### 6.1 Candidate publication

After successful full main CI, publish exact revision candidate images:

```text
sha-<full revision>
```

Do not move production pointers.

Existing static publication order remains:

```text
publish/verify revision CDN bytes
→ build/publish Web candidate that references them
```

Candidate metadata must include the exact Git revision.

### 6.2 Staging promotion

Move all candidate images to `staging-latest` only after the coherent set exists.

A partial tag move is tolerated only because the GCP watcher rejects mixed revisions.

### 6.3 GCP staging watcher

Install the staging watcher and state file without deleting the current manual staging script.

Expected state:

```text
revision=<sha>
channel=staging-latest
deployed_at=<timestamp>
```

Acceptance:

- mixed revisions are rejected without runtime mutation;
- coherent revisions deploy successfully;
- health failure does not record successful state;
- deployed staging revision can be read without inferring it from container age;
- current manual staging remains an emergency fallback during the pilot.

Rollback: disable staging watcher/channel and use the existing `staging-runtime.sh` source build.

## 7. Phase 4 — Release Lifecycle V3

Convert release preparation to manual `Prepare Release`.

Implement Release Publish as exact-SHA candidate promotion with Draft-first postflight.

Hard constraints:

- no source rebuild during release publication;
- immutable `vX.Y.Z` tag conflict fails closed;
- release manifest binds version/tag/SHA/CI/image digests/static assets;
- Published state occurs only after postflight passes.

### Existing PR #138

The cutover decision is option B: do not merge legacy PR #138. After V3 reaches `main` and candidate/staging evidence is green, close #138 as superseded and invoke `Prepare Release` explicitly so 0.9.0 is recreated under V3 semantics. Do not silently mix lifecycle semantics.

Rollback before a V3 release is published: restore the legacy release-preparation workflow while candidate publication remains harmless. After a V3 release is Published, immutable tags/manifests are never rewritten; rollback means selecting an earlier compatible Published release.

## 8. Phase 5 — Explicit Deploy Production

Split production pointer movement out of Release Publish.

`Deploy Production` accepts an already Published `vX.Y.Z` release, validates the canonical release manifest, and promotes its four immutable images to `prod-latest`.

The Alibaba watcher remains unchanged except for optionally recording release/manifest identity in `.deploy-state`.

Acceptance:

- publishing a release alone does not move production pointers;
- invalid/unpublished/mismatched release cannot promote;
- coherent promotion is observed by the existing watcher;
- existing backup, migration, health and rollback gates still pass;
- production deployment state identifies release + revision.

Rollback: repoint `prod-latest` to the previous published release set, subject to database compatibility. Do not bypass watcher safety checks.

## 9. Phase 6 — Observation, audit and cleanup

Add structured delivery summary and scheduled platform audit.

After stable operation:

- remove legacy workflow paths that no longer own authority;
- remove compatibility/staging source-build assumptions from canonical docs;
- archive the active plan with final evidence;
- apply the proven platform concepts to MemoFlow, preserving MemoFlow-specific Desktop/PowerSync/runtime needs.

## 10. Required evidence before archiving

The active plan may close only when evidence includes:

```text
PR affected-scope matrix tests
main full-CI evidence
stable Oracle branch protection
candidate digest/revision evidence
staging exact revision state
Prepare Release evidence
Published release manifest evidence
explicit Deploy Production evidence
production coherent deploy state
rollback/fail-closed negative-path evidence
platform audit result
```

No item may be marked complete solely because repository code exists; runtime/external acceptance must be distinguished explicitly.
