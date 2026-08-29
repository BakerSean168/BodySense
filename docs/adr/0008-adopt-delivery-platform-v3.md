# ADR 0008: Adopt Delivery Platform V3 with affected PR validation, exact-SHA candidates, and release/deploy separation

- Status: Accepted for BodySense pilot
- Date: 2026-08-29
- Related: ADR 0002, ADR 0004, ADR 0005, ADR 0006, ADR 0007
- Supersedes: the implicit coupling between every successful `main` CI run, release-please processing, release publication, and production pointer promotion

## Context

BodySense and MemoFlow evolved complementary delivery strengths.

MemoFlow's CI/CD Platform V2 has a mature delivery control plane: a versioned scope/risk manifest, affected lanes, stable Oracles, immutable build/test evidence, release contracts, and delivery observation. It deliberately separates release preparation from release publication.

BodySense has a stronger production delivery plane: revision-scoped public assets, coherent Web/API/AI/runtime promotion, Alibaba ACR as the runtime artifact plane, a fail-closed production deploy watcher, PostgreSQL migration validation, pre-deploy backup, active Consultation-run preflight, schema-aware rollback, blocked-revision handling, off-host durability and production health checks.

The previous BodySense flow is safe but has five limitations:

1. pull requests mostly pay the cost of full-repository validation regardless of change scope;
2. branch protection is coupled to implementation job names instead of stable policy Oracles;
3. staging is rebuilt imperatively from a repository worktree, so its exact deployed source revision is not a durable delivery contract;
4. release publication rebuilds source after CI instead of promoting the exact artifact set already validated for that main revision;
5. `release-please` is automatically considered after every successful main CI, which makes normal trunk integration and an explicit product release milestone less clearly separated.

BodySense is currently maintained primarily by one developer plus multiple AI implementation agents. The repository therefore needs a delivery model that is safe under high parallelism without introducing a permanent `dev` branch or GitFlow synchronization burden.

## Decision

### 1. Keep one long-lived trunk

`main` is the only long-lived development branch.

Normal work uses short-lived `feature/*`, `fix/*`, `refactor/*`, `docs/*`, or equivalent task branches. They are created from current `origin/main`, validated through pull requests, and deleted after merge.

There is no permanent `dev`/`develop` branch. Environment identity is represented by immutable revisions and release channels, not by permanent Git branches.

### 2. Use affected validation for pull requests and full validation for main

Every pull request generates one versioned delivery manifest containing at minimum:

```text
schema version
base SHA
head SHA
risk class
changed paths
selected lanes
policy version
self digest
```

PR jobs consume that manifest. They do not independently reinterpret change scope.

`main` remains the safety boundary: every push to `main` runs the complete required validation set regardless of affected scope.

This establishes the invariant:

> PR validation may be selective; main validation is exhaustive.

### 3. Protect stable Oracles, not dynamic implementation jobs

Dynamic child jobs may run, skip, shard or evolve according to the delivery manifest. Branch protection depends only on stable policy Oracles.

An Oracle passes only when every lane that policy says must execute succeeded. A legally unaffected lane is treated as satisfied. An expected lane that was skipped, cancelled, missing or failed causes the Oracle to fail closed.

The target stable checks are:

```text
Governance Oracle
Quality Oracle
Database Oracle
Experience Oracle
Delivery Observation
commit-lint
```

The first pilot may keep legacy required checks in parallel until the new Oracles prove equivalent.

### 4. Build the main candidate once and promote it through environments

After full exact-SHA main CI succeeds, the delivery plane creates one coherent candidate set:

```text
bodysense-web:sha-<full revision>
bodysense-api:sha-<full revision>
bodysense-ai-service:sha-<full revision>
bodysense-runtime:sha-<full revision>
```

Each image records the full Git revision in OCI labels. Public Vite assets remain revision-scoped under the existing immutable CDN namespace.

The candidate set is immutable. A later release does not rebuild source into a semantically new artifact. It promotes/retags the already validated candidate digests.

### 5. Make staging an environment channel, not a branch

A successful main candidate may be promoted to the mutable `staging-latest` channel.

The GCP staging runtime deploys only when Web/API/AI/runtime resolve to a coherent identical revision. It records the deployed revision and deployment time in durable local state.

The staging contract is:

> staging runs the latest successfully promoted and successfully deployed main candidate, not an arbitrary working-tree build.

Manual source-build staging remains available only as an emergency/developer diagnostic path during migration and must not be confused with canonical staging.

### 6. Separate Prepare Release, Release Publish, and Deploy Production

The release lifecycle has three explicit boundaries.

#### Prepare Release

A manually triggered release-preparation workflow updates version metadata and CHANGELOG and creates/updates a Release PR. It does not create a release tag, publish a GitHub Release, move production pointers, or deploy production.

#### Release Publish

After the Release PR merge SHA passes full exact-SHA main CI and its coherent candidate set exists, Release Publish:

- validates release identity;
- creates or resumes a Draft GitHub Release;
- creates the immutable `vX.Y.Z` tag for the exact release SHA;
- records the exact candidate image digests and static-asset identity in a canonical `release-manifest.json`;
- promotes immutable release tags to the same digests rather than rebuilding source;
- publishes the GitHub Release only after postflight verification succeeds.

A tag alone is not a completed release. A published GitHub Release plus its canonical release manifest is the release completion contract.

#### Deploy Production

Production deployment is a separate explicit workflow/operation. It selects an already published release, verifies its release manifest and exact-SHA provenance, and only then promotes the coherent release set to `prod-latest`.

The existing Alibaba production deploy watcher continues to own runtime rollout, backup, migration ordering, health gates and safe rollback.

Therefore:

> Release availability does not imply production deployment.

### 7. Preserve BodySense production safety invariants

Delivery Platform V3 must not weaken:

- coherent Web/API/AI/runtime revision validation;
- assets-before-HTML publication and CDN coherence;
- PostgreSQL 18 current-history and production-baseline migration gates;
- migration history immutability;
- active Consultation execution preflight;
- pre-deploy database backup and archive validation;
- runtime bundle provenance;
- schema-aware automatic rollback;
- failed revision blocking;
- off-host durability and restore-drill boundaries;
- production Environment restrictions;
- exact-SHA-pinned third-party GitHub Actions.

### 8. Do not use affected scope to create partial production releases in the pilot

Affected scope is initially a CI optimization and policy input only.

Every candidate/release remains a complete coherent release set containing Web, API, AI Service and runtime artifacts. The pilot does not publish only the changed service to production.

A future release-set manifest may legally reuse an unchanged prior component digest, but that optimization requires its own evidence and policy decision.

### 9. Make delivery evidence first-class

Every run emits a versioned delivery summary with selected lanes, timings, test/evidence counts where available, cache information, artifact identities and failure classification.

A scheduled delivery-platform audit exercises fail-closed behavior such as manifest mismatch, missing evidence, split revision pointers and other control-plane faults without mutating production.

## Consequences

### Positive

- one durable Git truth (`main`) with no permanent integration branch drift;
- cheaper/faster PR feedback while retaining exhaustive main safety;
- stable branch-protection contexts independent of CI implementation details;
- exact traceability from source revision to staging candidate, release manifest and production deploy state;
- build-once/promote-many semantics reduce release rebuild ambiguity;
- staging becomes a reproducible deployment channel rather than a manual repository state;
- release milestones become explicit product decisions;
- production rollout remains independently controllable and recoverable;
- the same conceptual platform can later converge with MemoFlow without forcing identical runtime implementations.

### Costs

- a delivery control plane, versioned schemas and Oracle tests must be maintained;
- candidate image storage increases because every successful main revision may create immutable images;
- staging requires a small deploy watcher/channel mechanism;
- migration from legacy required checks must run in shadow mode before protection switches;
- release tooling becomes more explicit and therefore has more states (candidate, prepared release PR, draft release, published release, deployed release).

## Rejected alternatives

### Permanent `dev` + `main`

Rejected because the repository does not need a second long-lived integration truth. Environment promotion provides the staging boundary without branch synchronization overhead.

### Keep full CI for every PR forever

Rejected because it scales cost with repository size rather than change risk and provides poorer feedback latency. Full validation remains mandatory on main.

### Let every successful main CI automatically create/release a version

Rejected because integration cadence and product release cadence are different concerns.

### Rebuild source during Release Publish

Rejected because a second build can diverge from the tested main candidate and weakens artifact provenance.

### Update only changed production containers

Rejected for the pilot because Go/Python/Web contracts, runtime configuration, migrations and static asset identity form a coherent release unit. Optimization may later reuse component digests through an explicit release-set contract.

## Rollout

The completed pilot evidence is archived at `docs/plan/archive/2026-08-29-delivery-platform-v3.md`. The rollout used shadow-mode migration before branch-protection and production-promotion cutover, then removed the transitional shadow/compatibility paths only after v0.9.0 production proof.
