# BodySense Delivery Platform V3

> Status: Target architecture accepted by ADR 0008; BodySense pilot in implementation.
> Date: 2026-08-29
> Scope: source integration, CI policy, immutable candidates, staging promotion, release publication, production deployment and delivery evidence.

## 1. North Star

BodySense delivery must preserve one source truth while separating four different concerns:

```text
source integration
      ↓
verification
      ↓
release publication
      ↓
production deployment
```

The key invariants are:

1. `main` is the only long-lived development branch.
2. PR validation may be affected/risk-based; every `main` push is fully validated.
3. scope is interpreted once and serialized as a versioned delivery manifest.
4. branch protection depends on stable Oracles, not dynamic child jobs.
5. a successful main revision produces one immutable coherent candidate set.
6. staging deploys a promoted main candidate and records its exact revision.
7. release preparation, release publication, and production deployment are separate operations.
8. release publication promotes previously verified artifacts; it does not rebuild source.
9. production promotion remains a coherent release-set transaction and the existing Alibaba watcher remains fail closed.
10. every important transition is bound to source SHA, artifact digest and versioned evidence.

## 2. End-to-end topology

```text
short-lived branch
       │
       ▼
PR → main
       │
       ▼
Delivery Scope Detector
       │
       ├── Governance
       ├── Quality
       ├── Database
       └── Experience
       │
       ▼
Stable Oracles
       │
       ▼
merge main
       │
       ▼
FULL exact-SHA CI
       │
       ├── tests/evals/migrations/e2e
       ├── immutable static assets
       └── coherent candidate images
               │
               ▼
       staging-latest channel
               │
               ▼
          GCP staging
               │
               ▼
         product validation

          explicit milestone
               │
               ▼
        Prepare Release
               │
               ▼
          Release PR
               │
               ▼
      exact-SHA main CI
               │
               ▼
        Release Publish
               │
               ├── Draft → Published GitHub Release
               ├── vX.Y.Z
               ├── immutable image tags
               └── release-manifest.json

          explicit rollout
               │
               ▼
       Deploy Production
               │
               ▼
          prod-latest
               │
               ▼
 Alibaba coherent deploy watcher
   backup → preflight → migrations
     health → rollback/block state
               │
               ▼
          production
```

## 3. Git and environment model

### 3.1 Git

Only `main` is permanent.

Normal branches are short-lived:

```text
feature/*
fix/*
refactor/*
docs/*
chore/*
```

They must originate from current `origin/main` or record an explicit pinned base SHA when parallel workers require deterministic integration.

### 3.2 Environments are channels, not branches

```text
main HEAD                    source integration truth
staging-latest               latest promoted validated main candidate
vX.Y.Z                       immutable published release identity
prod-latest                  currently selected production release channel
production .deploy-state     actually deployed revision/release identity
```

`staging` and `production` are therefore runtime projections of artifact channels rather than Git branches.

## 4. Delivery control plane

### 4.1 Manifest

The detector writes `delivery-manifest-v1.json` once per run.

Minimum shape:

```json
{
  "schema": "bodysense.delivery-manifest/v1",
  "baseSha": "...",
  "headSha": "...",
  "event": "pull_request",
  "risk": "runtime",
  "policyVersion": "delivery-policy-v1",
  "changedPaths": [],
  "lanes": {
    "governance": true,
    "web": true,
    "api": false,
    "ai": false,
    "contracts": true,
    "database": false,
    "experience": true,
    "full": false
  },
  "digest": "sha256:..."
}
```

The digest is computed over a canonical representation excluding the `digest` field itself.

### 4.2 Risk classes

Initial policy classes:

| Risk | Typical changes | PR policy |
| --- | --- | --- |
| `docs` | `docs/**`, prose-only metadata | governance only |
| `web` | React/UI/static frontend code | Web + contracts as required |
| `api` | Go service/application/domain | API + contracts |
| `ai` | Python Agent/runtime/prompts/evals | AI + contracts/eval |
| `database` | migrations, durable schemas, storage semantics | API + database + required cross-service tests |
| `runtime` | Docker, deployment, model gateway, shared config | full |
| `contract` | shared API/SSE/tool/domain contracts | Web + API + AI + experience |
| `root` | lockfiles, Nx/toolchain, CI control plane | full |
| `release` | release/deploy/runtime packaging | full + release contract validation |

Multiple categories collapse upward to the safest applicable risk.

### 4.3 Full-main override

For `push` to `main`, every required lane is enabled regardless of changed paths.

The detector still emits the manifest because observation, candidate publication and release provenance need the exact run contract.

## 5. Execution plane

Initial logical lanes:

```text
governance
quality-web
quality-api
quality-ai
quality-contracts
database-current-history
database-production-baseline
experience-e2e
candidate-build
```

The implementation may combine lanes on one runner when that is cheaper. Logical lane identity must remain observable even if physical jobs are consolidated.

### 5.1 PR semantics

A PR should avoid expensive unaffected work.

Examples:

```text
docs-only
  → governance

React-only
  → web lint/typecheck/test/build
  → relevant contract tests

Python Agent-only
  → Ruff/Pyright/Pytest
  → Agent qualification/eval
  → relevant contract/runtime tests

migration
  → API tests
  → current-history replay
  → production-baseline upgrade
  → domain semantics

CI/Docker/release/root
  → full
```

### 5.2 Main semantics

Every main push runs the complete existing production safety set, including migration scenarios and browser E2E. Candidate publication is downstream of those gates.

## 6. Stable Oracle model

Target protected checks:

```text
Governance Oracle
Quality Oracle
Database Oracle
Experience Oracle
Delivery Observation
commit-lint
```

Each Oracle receives:

```text
manifest result
logical lane enabled/disabled state
child execution result
required evidence existence
```

Truth table:

| Policy | Child | Oracle |
| --- | --- | --- |
| disabled | skipped/not-created | PASS |
| enabled | success | PASS |
| enabled | failure | FAIL |
| enabled | cancelled | FAIL |
| enabled | skipped | FAIL |
| enabled | missing | FAIL |
| detector/manifest invalid | any | FAIL |

The first migration stage runs Oracles in shadow mode while existing required checks remain authoritative.

## 7. Artifact and candidate plane

### 7.1 Candidate identity

After successful full main CI:

```text
sha-<full git revision>
```

is the immutable candidate identity.

The coherent set contains:

```text
bodysense-web
bodysense-api
bodysense-ai-service
bodysense-runtime
```

Each artifact records:

```text
Git revision
candidate identity
build timestamp
content/image digest
source CI run
```

The runtime bundle continues to carry tracked non-secret production runtime files.

### 7.2 Public assets

Existing revision-scoped CDN identity remains authoritative:

```text
https://assets.bakersean.top/web/<full-git-revision>/...
```

Assets are published and verified before the Web candidate that references them becomes eligible.

### 7.3 Build once, promote many

The target lifecycle is:

```text
main exact SHA
   ↓ build once
candidate digest
   ├─ staging channel
   ├─ vX.Y.Z release identity
   └─ prod-latest selected release
```

Retagging/promotion must preserve digest identity. A release workflow must not silently rebuild source into a different artifact.

## 8. Staging plane

### 8.1 Promotion

After candidate publication and complete main success, a staging promotion moves all four `staging-latest` pointers to that candidate set.

Partial registry pointer updates may occur transiently. The staging watcher treats any mixed revision set as ineligible and waits.

### 8.2 GCP watcher

The staging watcher is intentionally simpler than production but must:

1. pull Web/API/AI/runtime `staging-latest`;
2. require non-empty identical OCI source revisions;
3. validate the runtime bundle and staging Compose configuration;
4. avoid concurrent deployment using a host lock;
5. deploy in a deterministic dependency order;
6. wait for service health;
7. verify the staging application health endpoint;
8. write an exact revision state file.

Canonical state example:

```text
revision=<sha>
channel=staging-latest
deployed_at=<UTC timestamp>
```

A later phase may add candidate/release manifest digest and CI run ID.

## 9. Release Lifecycle V3

Detailed contract: `docs/architecture/release-lifecycle-v3.md`.

The three operations are:

```text
Prepare Release
Release Publish
Deploy Production
```

No operation may silently perform the next operation's authority.

## 10. Production plane

BodySense keeps the existing production watcher architecture.

The deploy workflow's only new responsibility is to select a published release and promote its already verified coherent artifact set to `prod-latest`.

The host watcher continues to own:

- pointer coherence checks;
- runtime bundle extraction;
- Compose validation;
- active Consultation execution preflight;
- PostgreSQL backup and archive validation;
- optional off-host DR gate;
- PostgreSQL 18 boundary handling;
- service ordering;
- health checks;
- `.deploy-state`;
- blocked-revision state;
- schema-aware rollback.

## 11. Observation plane

Every CI run should eventually produce `delivery-run-summary-v1.json` containing at least:

```text
manifest digest
base/head SHA
risk
selected lanes
lane outcomes
wall-clock timing
runner timing when available
evidence/artifact identities
candidate identity when produced
failure classification
```

Observation must fail closed when required evidence is missing. It must not reinterpret test success.

## 12. Scheduled platform audit

A non-production-mutating scheduled audit validates delivery mechanics such as:

- invalid manifest digest rejection;
- expected lane missing/skipped → Oracle failure;
- artifact/revision mismatch rejection;
- staging mixed revision detection;
- production mixed `prod-latest` detection in check-only mode;
- deployment backup validator negative paths;
- release identity mismatch rejection.

The audit reports platform health but does not automatically deploy or repair production.

## 13. Security boundaries

- PR workflows use read-only repository permissions unless a narrowly scoped write is required.
- production credentials remain GitHub Environment / host-only secrets.
- R2 credentials remain publication-plane only.
- production host secrets are not part of runtime artifacts.
- third-party Actions stay pinned to immutable commit SHAs.
- artifacts and manifests contain identities/digests, not credentials.
- release/deploy workflows fail closed on ambiguous identity.

## 14. Migration strategy

Migration is incremental:

```text
Phase 0 docs + drift cleanup
Phase 1 shadow detector/oracles/observation
Phase 2 affected PR + full main
Phase 3 immutable candidate + automatic staging
Phase 4 release lifecycle separation
Phase 5 explicit production deploy promotion
Phase 6 audit/cleanup
```

Production behavior is unchanged until its dedicated cutover phase.

The current automatic release-please path and existing production image workflow remain rollback references until their replacements have passed shadow/exact-SHA validation.
