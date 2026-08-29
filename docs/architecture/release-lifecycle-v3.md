# BodySense Release Lifecycle V3

> Status: Target release contract accepted by ADR 0008; implementation staged behind Delivery Platform V3 rollout.
> Date: 2026-08-29

## 1. Purpose

Release Lifecycle V3 makes three different decisions explicit:

```text
Prepare Release  = decide and prepare a product version
Release Publish  = make an immutable release available
Deploy Production = choose a published release for production rollout
```

They are intentionally not one workflow.

## 2. State model

A source revision may move through these states:

```text
integrated main revision
        ↓
verified candidate
        ↓
release-prepared revision
        ↓
published release
        ↓
selected production release
        ↓
deployed production release
```

Skipping states is not allowed.

## 3. Prepare Release

### Trigger

Manual `workflow_dispatch` only during the pilot.

### Input

Current `main`.

### Preconditions

- target main revision exists and is cleanly resolvable;
- the repository release metadata is internally consistent;
- normal development CI remains independent of release preparation.

### Effects

Release preparation may:

- calculate the next semantic version using release-please;
- update version files;
- update CHANGELOG;
- update release manifest metadata owned by release-please;
- create/update a Release PR.

### Forbidden effects

Prepare Release must not:

- create or move `v*` tags;
- publish a GitHub Release;
- build production images;
- move `staging-latest` or `prod-latest`;
- deploy staging or production.

## 4. Release PR

The Release PR is an ordinary protected PR into `main`.

After merge its exact merge SHA must run full main CI. That exact SHA becomes the release candidate revision.

No release is eligible merely because a Release PR was merged.

## 5. Release Publish

### Trigger

The preferred automatic trigger is successful exact-SHA main CI for a commit that satisfies the release contract. A manual retry path may accept an existing draft release tag.

### Preconditions

Release Publish must prove:

1. the exact release SHA is reachable from `main`;
2. a successful full main CI run exists for that exact SHA;
3. the release identity/version files agree;
4. the coherent candidate set for that exact SHA exists;
5. all candidate images have the expected OCI source revision;
6. public static assets for the revision are coherent;
7. no existing immutable tag points to a different SHA/digest set.

### Draft first

Release Publish creates/resumes a Draft GitHub Release and immutable `vX.Y.Z` Git tag bound to the exact release SHA.

Draft state is resumable. A failed postflight must not expose a half-complete release as Published.

### Artifact promotion

Release Publish promotes the exact candidate digests to immutable version identities. It must not rebuild source.

Conceptually:

```text
bodysense-web:sha-<revision>        digest A
                  ↓ retag/promote
bodysense-web:vX.Y.Z                digest A
```

The same rule applies to API, AI Service and runtime.

### Canonical release manifest

The workflow generates `release-manifest.json`:

```json
{
  "schema": "bodysense.release-set/v1",
  "version": "0.9.0",
  "tag": "v0.9.0",
  "gitSha": "<full sha>",
  "ciRunId": "<id>",
  "deliveryManifestDigest": "sha256:...",
  "migrationHead": 59,
  "images": {
    "web": { "digest": "sha256:..." },
    "api": { "digest": "sha256:..." },
    "aiService": { "digest": "sha256:..." },
    "runtime": { "digest": "sha256:..." }
  },
  "staticAssets": {
    "revision": "<full sha>",
    "atlasVersion": "1.4.0"
  }
}
```

The exact schema is versioned and validated before publication.

### Publish boundary

Only after release identity, candidate digests, static assets and release manifest all pass postflight may Draft become Published/Latest.

The completion condition is:

```text
Published GitHub Release
+
canonical release-manifest.json
+
immutable release image digests
```

## 6. Deploy Production

### Trigger

Explicit `workflow_dispatch` during the pilot.

Input:

```text
release = vX.Y.Z
```

### Preconditions

The deploy selector validates:

- release exists and is Published;
- canonical release manifest exists;
- tag SHA equals manifest SHA;
- exact successful main CI provenance is present;
- all four immutable release image digests match the manifest;
- release belongs to allowed production refs;
- no ambiguous/missing component identity exists.

### Effect

The selector promotes the coherent published release set to the mutable production channel:

```text
web:vX.Y.Z        ─┐
api:vX.Y.Z         │
ai-service:vX.Y.Z  ├─→ corresponding prod-latest pointers
runtime:vX.Y.Z     │
                   ┘
```

The workflow does not SSH into production and does not run migrations itself.

### Runtime rollout

Alibaba ECS `production-deploy-watch.sh` observes the coherent `prod-latest` set and performs the existing fail-closed runtime transaction.

Release Lifecycle V3 therefore keeps a strict authority split:

```text
GitHub Release Plane → choose artifacts
Production Watcher   → mutate runtime safely
```

## 7. Rollback

### Release rollback

A published release is immutable and is never rewritten.

### Deployment rollback

Production selection can choose an earlier Published release and promote that complete set to `prod-latest`, subject to database compatibility and the production watcher's schema-aware rollback rules.

The watcher may automatically restore previous application/runtime artifacts only when it can prove the database schema boundary did not move. It must continue to refuse blind rollback after an unverifiable or incompatible schema change.

## 8. Relationship to staging

Staging normally runs the latest successfully promoted main candidate, which may be newer than production and may never become a release.

A release candidate must originate from a fully validated main revision. Release publication must reuse that exact candidate artifact set.

Therefore the desired identity relation is:

```text
release artifact digest == previously validated main candidate digest
```

not merely “built from the same source”.

## 9. Existing v0.9.0 Release PR during migration

At Delivery Platform V3 pilot start, PR #138 (`chore(main): release 0.9.0`) already exists under the legacy automatic release-please flow.

It is migration state, not evidence that V3 is active.

Until V3 release publication is implemented and validated, the migration plan must choose one explicit treatment before cutover:

- finish v0.9.0 under the legacy release path, then enable V3 for the next release; or
- close/recreate the release milestone under V3 after shadow validation.

The pilot must not silently reinterpret an already prepared release PR under a new release contract.
