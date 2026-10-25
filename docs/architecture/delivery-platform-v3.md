# BodySense Delivery Platform V3

> Status: ADOPTED — production-proven on v0.9.0; governed by ADR 0008.
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

The detector writes `delivery-manifest-v4.json` once per run.

Minimum shape:

```json
{
  "schema": "bodysense.delivery-manifest/v4",
  "baseSha": "...",
  "headSha": "...",
  "event": "pull_request",
  "risk": "runtime",
  "policyVersion": "delivery-policy-v3",
  "changedPaths": [],
  "lanes": {
    "governance": true,
    "web": true,
    "api": false,
    "ai": false,
    "contracts": true,
    "database": false,
    "experience": true,
    "component": true,
    "full": false
  },
  "laneDetails": {
    "web": {
      "required": true,
      "files": ["apps/web/src/example.tsx"],
      "targets": ["job:quality-web-child", "nx:@bodysense/web:test"],
      "skipReason": null
    },
    "api": {
      "required": false,
      "files": [],
      "targets": ["job:quality-api-child", "nx:api:test"],
      "skipReason": "no-matching-paths"
    }
  },
  "digest": "sha256:..."
}
```

Since schema v2, every lane carries a `laneDetails` entry:

- `required` mirrors `lanes.<lane>`;
- `files` lists the changed paths that require the lane (governance inspects
  the whole diff, so its `files` is the full change set);
- `targets` lists the executable units the lane gates (CI jobs, Nx targets,
  scripts), fixed by policy in `scripts/delivery/lib.mjs`;
- `skipReason` is `null` for a required lane and a machine-readable code
  otherwise: `no-changed-paths` (empty diff) or `no-matching-paths` (no
  changed path selects the lane). An Oracle can therefore distinguish a
  legitimately skipped lane from a required lane that never ran.

The digest is computed over a canonical representation excluding the `digest`
field itself, and `laneDetails` is inside the digest. Validators additionally
cross-check `laneDetails` against `lanes` and `changedPaths`, so recomputing
a digest after tampering does not launder an inconsistent contract.

Version compatibility: `validateManifest` accepts
`bodysense.delivery-manifest/v1` (validated under the v1 rules, without
`laneDetails`), v2, v3, and v4. Lane sets, lane target policies, and policy
versions are pinned per schema version: v1/v2 manifests validate against
the eight-lane set and the v1-era target policy (`LANE_TARGETS_V2`) with
`delivery-policy-v1`; v3 adds the `component` lane, moves contract
verification into `contracts-child`, and carries `delivery-policy-v2`
(`LANE_TARGETS_V3`); v4 splits the quality lanes into per-lane children,
adds the `e2e-report-child` receipt to the experience and full lanes, and
carries `delivery-policy-v3`. Generators only emit v4; `laneDetails` on a
v1 manifest is rejected. Consumers that re-validate stored manifests
(candidate publication, platform audit) therefore keep working across the
transition.

The Scope job publishes the manifest digest as a job output. Every consumer
that downloads the manifest artifact (governance child, the quality
children, contracts child, component child, the four Oracles, Delivery
Observation)
verifies the artifact against that output
with `--expect-digest`, and each Oracle derives its expected child outcomes
from the digest-validated manifest (`ORACLE_GROUPS` in
`scripts/delivery/lib.mjs`) instead of re-deriving lane expressions in
workflow YAML.

### 4.2 Risk classes

Each changed path is classified by the first matching rule below (the table
mirrors `classifySinglePath` in `scripts/delivery/lib.mjs`, in evaluation
order). Governance always runs. The manifest risk is the highest-priority
risk across all paths, with priority
`docs < web < api < ai < contract < database < runtime < release < root`.
If any path selects the `full` lane, every lane is enabled.

| Path pattern | Risk | Lanes enabled |
| ------------ | ---- | ------------- |
| `docs/**`, any `*.md`, `README.md`, `CONTRIBUTING.md` | `docs` | governance only |
| `.github/**`, `scripts/delivery/**`, `docker/**`, `deploy/**`, `.env.production` | `runtime` | all |
| `release-please-config.json`, `.release-please-manifest.json` | `release` | all |
| `package.json`, `pnpm-lock.yaml`, `pnpm-workspace.yaml`, `nx.json`, `tsconfig*`, `.npm*`, `.node-version`, `.python-version`, `uv.lock` | `root` | all |
| `apps/api/migrations/**` | `database` | api, database, experience |
| `apps/api/**` | `api` | api, experience |
| `apps/ai-service/**` | `ai` | ai, experience |
| `apps/web/**` | `web` | web, experience, component |
| `packages/contracts/**` | `contract` | web, api, ai, contracts, experience, component |
| `postman/**`, `scripts/postman/**` | `contract` | contracts |
| `packages/utils/**` | `root` | all (shared-utility fail-safe) |
| `scripts/static-assets/**`, `scripts/anatomy/**` | `runtime` | all (public-asset runtime) |
| any other `scripts/**` | `runtime` | all (repository script) |
| `tests/**`, `e2e/**` | `contract` | contracts, experience |
| anything else | `root` | all (unknown-path fail-safe) |

Special cases:

- Empty diff: risk `docs`, governance only, reason `no-changed-paths`. An
  empty diff is never interpreted as docs-only beyond governance, and the
  manifest records the explicit reason.
- Forced full (push to `main`, `workflow_dispatch`, or
  `DELIVERY_FORCE_FULL=1`): risk `root`, every lane enabled, reason
  `full-main-policy`, and every lane's `laneDetails.files` is the full
  change set.

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
experience-component
candidate-build
```

Physical mapping: `quality-contracts` runs as its own CI job
(`contracts-child`) with an independent Quality Oracle receipt, separate
from the web/api/ai unit lanes; `experience-component` runs as
`component-child` (static Storybook build + test-runner, no API/AI/
database services) under the Experience Oracle, separate from the
full-stack `experience-e2e` child.

Since manifest v4, the quality lanes also run as separate jobs:
`quality-web-child`, `quality-api-child`, and `quality-ai-child` execute
one lane each in parallel with independent Quality Oracle receipts, and
the cross-cutting `quality:architecture` checks run in
`quality-arch-child` whenever any of those lanes is selected. The `full`
lane keeps the `validate-repo.sh` monolith in `quality-full-child`; the
per-lane children stand down in full runs (the Quality Oracle derives this
from the manifest via `{ anyOf, noneOf }` child rules), so no check
executes twice outside the documented full/contracts overlap.

The `experience-e2e` child is sharded: `e2e-child` runs as a matrix over
the duration-balanced partition committed in `apps/web/e2e/shards.json`
(generated by `scripts/e2e/plan-shards.mjs` from measured per-file
durations, LPT greedy), each shard booting its own full stack and
uploading a Playwright blob report. `e2e-report-child` merges the blobs
and verifies the merged results cover exactly the partitioned spec set
with zero unexpected results (`scripts/e2e/verify-merged-report.mjs`);
its outcome is the `experience` receipt the Experience Oracle checks, so
a silently dropped shard fails the run even if every executed test
passed. Governance additionally validates partition completeness
(`plan-shards.mjs --check`) on every run.

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

### 5.3 Stage matrix

Every stage reuses the same manifest, lane, and Oracle machinery; stages
differ in trigger, lane selection, and which evidence they consume.

| Stage | Trigger | What runs | Evidence | What does not run, and why |
| --- | --- | --- | --- | --- |
| Local | husky hooks on commit; `pnpm verify:pr` before push | commit-msg: commitlint. pre-commit: Oxfmt `--check` + Oxlint on staged files. `verify:pr`: governance format/lint gates, delivery + plan contract tests, E2E shard-partition check, and `nx affected` lint/typecheck/test against `origin/main` | Local exit codes only; nothing is uploaded | E2E, database migrations, and the full validate-repo.sh set: they need CI services and are covered by the PR stage on the same diff |
| Pull request | `ci.yml` on `pull_request` | Manifest-scoped lanes selected by risk (sections 4.1-4.2): governance always; web/api/ai/arch/contracts/component/database/experience children only when their paths or risk require them; Oracles aggregate child receipts | `delivery-manifest-v4` artifact (digest-bound) + `delivery-run-summary` artifact. Required checks: commit-lint, the four Oracles, Delivery Observation (strict) | Unaffected lanes: the manifest records a machine-readable skip reason per lane, and Oracles fail closed if a required lane is missing |
| Main | `ci.yml` on push to `main` | The complete production safety set: the manifest force-selects every lane, including the full validate-repo.sh child, migration baseline + recovery, sharded browser E2E, and the component lane | Same artifacts as PR, bound to the exact main SHA | Nothing in the lane set is skipped; path scoping does not apply to main |
| Nightly | `ci.yml` on `schedule` (daily 18:23 UTC) or manual `workflow_dispatch` | Identical to a main push: base == head, the `schedule` event force-selects every lane, same children, Oracles, and observation | Same artifacts as a main run, retained 30 days for the run summary | Not a required check and never gates a merge. It exists to catch drift that path-scoped PR selection and quiet main periods would hide (toolchain or upstream changes, flaky lanes). Failures surface as a red run plus the Delivery Observation summary, matching the release-health reporting style |
| Release | release-please PR, then `release-publish`; production deploy is an explicit operator action | Candidate publication runs downstream of a successful main CI run, bound to the exact SHA: it downloads that run's delivery manifest, validates it, and embeds its digest in the candidate set | Candidate set + image digests trace back to the source CI run and its manifest digest | Lint/test/typecheck/E2E are **not** re-run: the exact SHA was already verified by the main run, and re-running could only test a different tree. Promotion consumes evidence; it does not recreate it |

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

| Policy                    | Child               | Oracle |
| ------------------------- | ------------------- | ------ |
| disabled                  | skipped/not-created | PASS   |
| enabled                   | success             | PASS   |
| enabled                   | failure             | FAIL   |
| enabled                   | cancelled           | FAIL   |
| enabled                   | skipped             | FAIL   |
| enabled                   | missing             | FAIL   |
| detector/manifest invalid | any                 | FAIL   |

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

Retagging/promotion must preserve digest identity. A release workflow must not silently rebuild source into a different artifact. Because Buildx defaults `imagetools create` to `--prefer-index=true`, every single-platform channel/release promotion explicitly uses `--prefer-index=false`; otherwise a carbon-copy candidate manifest would be wrapped in a new one-member OCI index with a different outer digest.

## 8. Staging plane

### 8.1 Promotion

After candidate publication and complete main success, a staging promotion moves all four `staging-latest` pointers to that candidate set.

Partial registry pointer updates may occur transiently. The staging watcher treats any mixed revision set as ineligible and waits.

### 8.1.1 V2 staging-promotion admission (2026-10-14)

**Main integration is not an authorization to deploy an unfinished V2 product.**
`Publish Main Candidate` still runs after an exact-SHA successful main CI and
publishes the immutable revision-scoped atlas/Web assets and all four candidate
images/manifests. The separate `staging-readiness` job evaluates the
**same checkout's** `docs/plan/active/v2-handoff-state.json` with
`scripts/delivery/v2-staging-readiness.mjs`. Its `eligible` output gates **only**
`promote-staging`, the job that changes the four mutable `staging-latest`
tags. The existing exact-main freshness recheck remains in place as a second
independent guard.

Promotion requires **both** (a) all five approved V2 domains' six gates
(`business/view_model/database/design/engineering/integration`) marked
`ready` with reproducible proof, plus the non-circular Product/BodyFact/
Contract/Penpot/Acceptance foundations, and (b) the explicit repository
Actions variable `BODYSENSE_V2_STAGING_PROMOTION_ENABLED=true`. Undefined,
malformed, partial, or not-approved status **fails closed**. Do not enable the
variable until the unique handoff register has been independently accepted
and the actual GCP staging resources and rollback are qualified.

A successful PR/main CI or `main` merge **does not** flip that variable or
promote staging. To re-enable future promotion, complete the approved domain
acceptance and repository review, then set the explicit Actions variable.
Before such a change, inspect the exact-SHA CI and candidate manifest; never
temporarily bypass the readiness job or re-tag `staging-latest` out-of-band.
Production remains governed by its separate published Release and explicit
`Deploy Production` workflow.

**Visual sign-off on staging (2026-10-19).** F-PENPOT is the user's visual
sign-off (Q-19), and that sign-off is done on staging. The second Actions
variable `BODYSENSE_V2_STAGING_VISUAL_SIGNOFF_ON_STAGING=true` lets promotion
proceed while F-PENPOT is explicitly `blocked`; the job prints it as `WAIVED`.
It waives no other foundation or domain gate, never replaces
`BODYSENSE_V2_STAGING_PROMOTION_ENABLED`, and has no production effect.
F-PENPOT is still marked `ready` only after the user signs off.

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

This weekly audit is complementary to the nightly CI run in section 5.3: the audit exercises delivery *mechanics* (fail-closed negative paths), while the nightly run exercises the product lanes themselves on current main.

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

The pilot completed on v0.9.0. Transitional shadow CI, legacy required-context aliases, and the old manual production-image fallback were removed only after exact-SHA candidate, staging, Published Release, and explicit production-deploy evidence were all green. Immutable releases are never rewritten as a rollback mechanism.

## 15. VNext acceptance (B7, 2026-10-25)

Batches B1–B6 reworked the test and CI/CD system batch by batch (issues
#446/#448/#450/#452/#454/#457, PRs #447/#449/#451/#453/#455/#458). This
section is the consolidated acceptance record: what changed, that no check
was lost, how the pipeline behaves under injected faults, what it costs,
and what is known to remain open.

### 15.1 Batch inventory

| Batch | Change |
| --- | --- |
| B1 | Format and lint become gates: Oxfmt full-repo check (58-file debt cleared), Oxlint + type-aware Oxlint, gofmt, Ruff format, husky pre-commit; pre-existing zod supply-chain failure fixed |
| B2 | Manifest contract v2: per-lane `required`/`files`/`targets`/`skipReason`, digest verified by every consumer, Oracle expectations derived from the validated manifest instead of hand-written expressions |
| B3 | Nx graph honesty: fake `^build` edges removed, typecheck cache-input false-green fixed, outputs/inputs corrected, ai-service coverage target repaired |
| B4 | Component browser lane (Storybook test-runner) and contracts child split out of the quality monolith with independent Oracle receipts; manifest v3 |
| B5 | E2E sharded by measured duration (3 shards, per-shard databases, merged-report completeness check), quality lanes split into parallel children; manifest v4 |
| B6 | Stage matrix (section 5.3), nightly full-run safety net on `schedule`, `pnpm verify:pr` local preflight, root `verify` false-green vector removed, shared Nx cache measured at zero task hits and reverted |
| B7 | This acceptance: end-to-end fault-injection drill, performance comparison, release-evidence audit, child-receipt evaluation |

### 15.2 Check reconciliation

The 23 checks inventoried in B4 are all retained 1:1 (per-check table in
issue #452); no batch dropped, merged away, or downgraded a check. Pure
additions since that inventory: the four format gates and two Oxlint gates
(B1), the component browser lane with 41 story tests (B4), the E2E
merged-report completeness check (B5), the nightly full run (B6), and the
local `verify:pr` preflight (B6). Required checks are unchanged throughout:
commit-lint, the four Oracles, and Delivery Observation.

### 15.3 Performance comparison (same scope: full safety set)

Medians of three runs per side, from the GitHub Actions API. Baseline:
main pushes at the pre-VNext architecture (runs 39910452549, 39905968499,
39900065797; main force-selects every lane). Current: full-scope runs on
the B5/B6 branches (runs 40143436887, 40149500738, 40150801347; the
manifest selected the full lane set because CI/delivery paths changed).

| Metric (median) | Baseline | Current | Change |
| --- | ---: | ---: | --- |
| Whole-run wall | 961s | 456s | −53% |
| Critical path | E2E child 928s | Full quality child 421s | E2E no longer dominates |
| E2E longest shard | 928s (single job) | 362s (+33s merged report) | −57% |
| E2E total job time | 928s | ≈1049s (3 shards) | +13% job time buys the wall win |
| Quality execution | 513s (one monolith child) | 421s full child, or per-lane children in parallel on scoped runs | split in B4/B5 |
| Contracts child | (inside quality monolith) | 174s | independent receipt |
| Component child | (did not exist) | 67s | new lane |
| Governance child | 11s | 43s | now carries the B1 format/lint gates for four stacks |
| PG baseline child | 176s | 176s | unchanged |
| Jobs per run | 12 | 21 | more parallelism, same required checks |

### 15.4 Fault-injection drill (CI, end to end)

Each fault was injected as a single commit on the B7 acceptance branch,
observed in a real PR run, and reverted before the next injection. All
five faults failed exactly the gates listed below and nothing else; no
defect was found by the drill. The injection commits are not part of
this PR's net diff (branch history was reset after the drill).

| # | Injected fault | Expected | Observed (run, jobs) |
| --- | --- | --- | --- |
| a | `any` in production web code (`apps/web/src/lib/utils.ts`) | Lint gates fail | Run [40152143094](https://github.com/BakerSean168/BodySense/actions/runs/40152143094): Governance child + Governance Oracle + Web quality child + Quality Oracle all failed; unrelated jobs green. The same commit was first stopped locally by the B1 pre-commit hook; the drill used `--no-verify` because CI, not the local hook, was the system under test |
| b | Type error in a web test file (`displayValue.test.ts`) | Typecheck fails (B3 probe, CI form) | Run [40152712708](https://github.com/BakerSean168/BodySense/actions/runs/40152712708): Web quality child + Quality Oracle failed; Governance stayed green, isolating the failure to typecheck |
| c | contracts-child forced to skip (`if: … && false && …`) while its lane is enabled | Quality Oracle fails | Run [40153188328](https://github.com/BakerSean168/BodySense/actions/runs/40153188328): Contract verification child skipped, Quality Oracle failed with `contracts: expected success, got skipped`; every other job green |
| d | Deterministic failure in one E2E spec (`legacy-share-retired.spec.ts`, shard 1) | Its shard fails, merged-report check fails, Experience Oracle fails | Run [40153828687](https://github.com/BakerSean168/BodySense/actions/runs/40153828687): shard 1 + E2E merged report child + Experience Oracle failed; shards 2/3 and all other jobs green |
| e | Format violation in one source file (`api-url.ts`, single quotes) | Governance format gate fails | Run [40154338364](https://github.com/BakerSean168/BodySense/actions/runs/40154338364): Governance child + Governance Oracle failed; Quality Oracle green, isolating the failure to the format gate |

### 15.5 Release-evidence audit (read-only)

| Link in the chain | Verdict | Evidence |
| --- | --- | --- |
| Exact-SHA binding | Complete | candidate-publish triggers only on a successful `push` CI run on `main` and checks out `workflow_run.head_sha`; release-publish revalidates candidate `gitSha` == checkout HEAD, source CI run `head_sha` == candidate `gitSha`, candidate SHA is an ancestor of `origin/main`, and the release tag points at the release SHA |
| Manifest digest embedding | Complete | candidate-manifest downloads the `delivery-manifest-v4` artifact from the exact source CI run, validates it (self-digest + policy), and embeds its digest into `candidate-set-v1`; the candidate set's own digest is format-checked and validated again in release-publish |
| Image digest verification | Complete | Every component verifies existing-vs-built digest equality and the OCI revision label at build time, promotes by digest without rebuilding, and re-verifies registry and pulled `RepoDigests` after promotion; immutable-tag collisions fail closed |
| Release-health retention | Gap (low) | The weekly release-health job writes only to the run's step summary and emits a soft-threshold `::warning::`; nothing is uploaded as an artifact or filed, so the signal survives only within Actions run retention. It is advisory (backlog/lag), never a gate |
| Candidate artifact lifetime | Gap (low) | release-publish downloads the `candidate-set-*` artifact (90-day retention) from the candidate run; a release cut more than 90 days after candidate publication would first need candidate-publish re-dispatched for that SHA |

No release-chain code was changed in this batch; both gaps are advisory
and recorded here for a future decision.

### 15.6 Child-receipt evaluation (B2 leftover)

Question: is the platform's `needs.*.result` a sufficient truth source
for the Oracles, or must each child also produce its own signed receipt
file?

Conclusion: the platform result is sufficient as the gating truth, and a
separate child-receipt system is not required. Reasoning:

- The result value is produced by the Actions scheduler, not by the
  child; a child cannot forge it. It can only exit non-zero (fail) or
  exit zero. The manifest side of the contract — which lanes must run —
  is independently digest-bound and re-validated by every Oracle (B2),
  so "lane should have run" and "lane result" come from two independent
  sources and are cross-checked.
- The truth table (section 6) fails closed on skipped, cancelled, and
  missing children. Across B1–B6 there is no recorded instance of a
  child reporting success while its lane did not execute; the failures
  that did occur (B2 manifest/format interaction, B3 oracle mapping,
  B5 schema gate) were all caught precisely because child results and
  Oracle expectations disagreed.
- The residual risk a receipt file would address is a child that exits
  zero without doing its work (a runner script swallowing its own
  errors). That is a runner-correctness risk, not a receipt-forgery
  risk, and the delivery test suites already pin the runners' fail-loud
  behavior. If it is ever addressed, the minimal step is to have the
  Oracles also verify the existing `delivery-run-summary` artifact's
  lane coverage; that design note is recorded here and intentionally
  not implemented in this batch.

### 15.7 Known limitations

- The nightly full run (B6) is verified locally and by contract tests;
  its first real `schedule` execution can only happen after the batch
  chain merges to main, and is tracked as an open item in issue #457.
- Oxlint warnings stand at 29 (plain) / 98 (type-aware) with zero
  errors; warnings remain warn-level and are burned down individually,
  never by disabling rules.
- The shared Nx cache was measured at zero task-level hits in CI and
  reverted (B6); local Nx caching is unaffected.
- E2E sharding trades +13% total job time for a −57% longest-shard
  wall; shard balance depends on the duration table in
  `apps/web/e2e/shards.json`, which must be regenerated when the
  distribution drifts materially.
- Full-scope runs are now bounded below by the full quality child
  (validate-repo.sh, median 421s); splitting it further is possible
  (B5 scoped runs already use per-lane children) but was not needed
  to meet the acceptance targets.

## 16. Oxc evaluation and lint-gate decision (B8, 2026-10-25)

Question (the Q8 analogue): can Oxlint replace ESLint as this repo's lint
gate? Method: the effective ESLint configuration was dumped per project
(`eslint --print-config`, 474 files across `apps/web`, `packages/contracts`,
`packages/utils`), and a temporary probe suite (13 files, ~90 deliberate
violations, including the scoped cases: data-boundary assertion, test-file
exemption, `^_` unused exemptions) was run under the old ESLint, the
then-current Oxlint config, and a candidate config with every mapped rule
raised to `error`. Every verdict below is a probe result, not
documentation. The probe files were deleted after the experiment; the
suite is reproducible from the recipe in issue #461.

### 16.1 Equivalence verdict (70 checks)

| Class | Count | Detail |
| --- | ---: | --- |
| Oxlint equivalent at `error` | 67 | Identical probe hits rule for rule (core eslint rules plus typescript-eslint rules, incl. `no-unused-vars` with the `^_` exemptions and `no-unused-expressions`) |
| Scoped `no-unsafe-type-assertion` | 1 | Equivalent via the existing override + `--type-aware`: boundary files error, test files exempt. One asymmetry: outside the boundary Oxlint additionally *warns* where ESLint is silent — warn-only, gate unaffected |
| `no-delete-var` | 1 | The oxc parser rejects `delete <identifier>` in strict mode as a hard error — equal or stronger than the rule. Side effect: a parse error suppresses the file's other diagnostics |
| `preserve-caught-error` | 1 | **Gap.** Oxlint 1.89 ships the rule but never fires under this repo's options (`{ requireCatchParameter: false }`), verified on both probe variants; ESLint fires. The only genuine non-equivalence found |

`no-octal` is not a configured rule; octal literals and `\8` escapes are
parse errors under both parsers (probe-verified). In the other direction,
the new Oxlint config enables no error rule beyond this mapping, so no
unaccounted gate was added.

### 16.2 Decision: B — Oxlint primary, ESLint as single-rule residual

Not C (full retirement): the `preserve-caught-error` gap has no
equivalent carrier. Not A (unchanged dual track): with 69 of 70 checks
equivalent, the full type-aware ESLint pass is pure duplication.

Landed changes:

- `.oxlintrc.json` now lists the 68 mapped rules explicitly at `error`
  (categories stay `warn` for the incremental warning programme) and adds
  one scoped override: `no-unsafe-optional-chaining` is `warn` in test
  files. Reason: Oxlint sees through `as` assertions where the ESLint
  core rule is blind; the only three hits are test files whose patterns
  are guarded by call-count assertions (re-verified from the 2022
  review). Production code stays `error`, so the gate is at least
  ESLint-strict everywhere ESLint could see.
- `lint:oxlint` / `lint:oxlint:types` widened from `apps/web/src` to the
  former ESLint scope: plain run over `apps/web` + `packages` (474
  files), type-aware over the three `src` trees (471 files). Governance
  (the always-on lane) carries Oxlint; `validate-repo.sh` picks the same
  scripts up unchanged.
- `eslint.config.mjs` reduced to the residual rule only, without type
  information (no `projectService`): the Nx `lint` targets now run a
  syntactic single-rule pass. The `@nx` plugin block was dropped — it
  contributed no enabled rules.
- `packages/utils/tsconfig.json` gained an explicit `"rootDir": "src"`:
  tsgolint could not build a program for the stub package (its single
  file is comment-only) and emitted a `tsconfig-error`; `tsc` itself was
  always green.
- Five `no-unnecessary-type-assertion` warnings in real code were removed
  as a separate commit (portal-scope, popover, select x2,
  BodyRegionNavigator); web typecheck and the 716 web tests pass, and
  the rule now reports 0.

Check reconciliation: the former 70 ESLint-carried checks are now
carried by Oxlint (68), the residual ESLint config (1), and the parser
(1). No check was dropped or downgraded in production code. Required
checks and lane semantics are unchanged; the probe suite was re-run
against the landed configuration as a regression test (Oxlint: 77 plain
+ 3 boundary errors across the probes, each mapped; residual ESLint:
exactly the one `preserve-caught-error` hit).

### 16.3 Performance (Oracle2, medians of 3, same tree)

| Pass | Before | After |
| --- | ---: | ---: |
| ESLint full (474 files, type-aware config) | 28.9s | 9.0s (residual single rule, no type info) |
| Oxlint plain | 0.3s (`apps/web/src` only) | 2.4s (full scope, via pnpm) |
| Oxlint `--type-aware` | 3.6s (`apps/web/src` only) | 7.5s (three `src` trees, via pnpm) |
| **Total lint wall** | **~32.8s** | **~18.9s (-42%)** |

In CI terms the quality web lane's lint segment shrinks by roughly 20s
gross, about 13s net after the wider Oxlint runs in Governance. Oxfmt was
re-verified in passing: `format:check` exit 0 over 677 files, median
2.81s — the formatter migration remains complete and is not reopened.

### 16.4 Warning stock (after the change: 33 plain / 96 type-aware)

All remaining diagnostics are `warn` by design; none was silenced by
disabling a rule. Classification (locations in issue #461):

| Category | Count | Verdict |
| --- | ---: | --- |
| `consistent-function-scoping` | 19 | Style noise (React component-scope helpers) |
| `no-unsafe-type-assertion` (type-aware) | 33 | By-design scope signal: assertions outside the data boundary, individually reviewed in 2022; boundary stays `error` |
| `consistent-return` | 15 | Framework idiom / exhaustive-switch false positives |
| `no-floating-promises` | 10 | Event-handler and router `navigate()` idiom; callee-side error handling verified in 2022 — remains the standing review item when this count *changes* |
| `no-base-to-string` | 4 | Test-only false positives (string bodies) |
| Plain remainder (`no-unsafe-optional-chaining` 3, `no-unmodified-loop-condition` 2, `no-shadow` 2, misc 4) | 11 | Guarded test patterns and 2022-verified false positives |
| Type-aware remainder (`await-thenable` 1, `no-unnecessary-type-parameters` 1, misc) | ~15 | Known harmless idioms, itemised in #461 |

Digestion path: categories stay at `warn`; new warnings in a PR are
reviewed against this table, and any category whose count grows gets a
fresh per-item review (the B8 method), not a config exemption.

### 16.5 Exit condition for the residual ESLint

When an Oxlint release enforces `preserve-caught-error` under
`{ requireCatchParameter: false }` (re-run the #461 probe recipe to
confirm), delete `eslint.config.mjs`, point the Nx lint targets at the
Oxlint scripts, and this section's decision collapses to full retirement
(option C) with no other changes.
