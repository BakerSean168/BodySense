# BodySense Delivery Platform V3 Pilot

> Date: 2026-08-29
> Status: ACTIVE — BodySense pilot
> Owner boundary: GitHub CI/release plane + GCP staging + Alibaba production promotion contract
> ADR: `docs/adr/0008-adopt-delivery-platform-v3.md`
> Architecture: `docs/architecture/delivery-platform-v3.md`
> Release contract: `docs/architecture/release-lifecycle-v3.md`
> Rollout runbook: `docs/runbooks/delivery-platform-v3-rollout.md`

## 1. Mission

Converge the strongest delivery properties already proven across BodySense and MemoFlow into one BodySense pilot without weakening the current production safety model.

The pilot must establish:

```text
one long-lived main
+ affected PR validation
+ exhaustive main validation
+ one versioned scope/risk manifest
+ stable fail-closed Oracles
+ immutable exact-SHA main candidates
+ canonical staging revision
+ Prepare Release / Release Publish / Deploy Production separation
+ build-once/promote-many release artifacts
+ existing coherent Alibaba production watcher
+ versioned delivery evidence
```

## 2. Explicit non-goals

This plan does not:

- introduce a permanent `dev`/`develop` branch;
- remove full main validation;
- weaken migration/DR/E2E gates;
- deploy production directly from a feature branch;
- make PR jobs capable of production mutation;
- implement partial production service release based only on affected scope;
- replace the existing production deploy watcher with Watchtower or direct GitHub SSH rollout;
- change model/provider routing or BodyState/Diagnosis/Treatment domain authority.

## 3. Current baseline

At pilot start:

```text
main = fb8ca2656145fa955704da403f7cfac949acb8c5
staging = healthy but manually/repository-built and not a canonical main-revision channel
production = v0.8.1 / revision 7f82bd2f9f11058159de11f0e025e29a04cedc4f
open release PR = #138 chore(main): release 0.9.0
```

Current BodySense strengths that are hard invariants:

- PostgreSQL 18 current-history and production-baseline migration scenarios;
- exact-SHA CI eligibility before production image publication;
- revision-scoped R2 assets and Web↔CDN coherence;
- coherent Web/API/AI/runtime production pointers;
- ACR runtime bundle;
- production deploy preflight/backup/migration/health/rollback/block state;
- off-host durability and restore isolation;
- SHA-pinned GitHub Actions.

MemoFlow patterns adopted conceptually:

- versioned delivery manifest;
- one scope/risk interpretation;
- affected PR lanes;
- stable Oracles;
- artifact/evidence promotion contracts;
- delivery observation;
- Prepare Release separated from Release Publish;
- platform fault audit.

## 4. Global invariants

Every implementation ticket must preserve these rules:

1. `main` remains the sole permanent integration source of truth.
2. PR selection may optimize work; `main` may not skip required full validation.
3. no lane owns its own scope interpretation after a manifest exists.
4. unknown/invalid scope is upgraded to safer/full work, never downgraded to docs-only.
5. expected-but-missing execution fails closed.
6. candidate/release/deploy identities bind exact Git SHA.
7. staging/prod mutable pointers are never trusted without coherent revision verification.
8. Release Publish must not rebuild source after candidate cutover.
9. Published Release does not imply production deployment.
10. production runtime mutation remains owned by the existing watcher transaction.

## 5. Phase 0 — Architecture contract and drift cleanup

### DLV-3001 — ADR + architecture + release lifecycle + rollout docs

Status: COMPLETE — repository

Deliver:

- ADR 0008;
- Delivery Platform V3 architecture;
- Release Lifecycle V3 contract;
- rollout/rollback runbook;
- this Active Plan;
- navigation updates.

Acceptance:

- documents agree on one-main branching;
- documents distinguish candidate / staging / release / deploy;
- existing production safety invariants are explicitly retained.

### DLV-3002 — Remove stale permanent-dev assumptions

Status: COMPLETE — repository

Deliver:

- BodySense CI triggers `main` only for long-lived branch push/PR targets;
- canonical deployment docs no longer state `main / dev` as equivalent target branches;
- no `dev` branch is created.

Acceptance:

- workflow YAML validates;
- existing protected check names remain unchanged in Phase 0.

## 6. Phase 1 — Shadow Delivery Control Plane

### DLV-3101 — Versioned manifest generator

Status: COMPLETE — repository; shadow Actions evidence pending

Create deterministic tooling under `scripts/delivery/`.

Inputs:

```text
base SHA
head SHA
event type
changed paths from Git
```

Outputs:

```text
delivery-manifest-v1.json
```

Initial lane set:

```text
governance
web
api
ai
contracts
database
experience
full
```

Risk mapping must cover docs, Web, Go API, Python AI, contracts, migrations, Docker/runtime, CI/root and release files.

Safety behavior:

- `main` push forces full;
- root/CI/release/runtime ambiguity forces full;
- deleted/renamed files participate in path evaluation;
- invalid SHA boundary fails rather than producing false docs-only.

### DLV-3102 — Manifest validation and canonical digest

Status: COMPLETE — repository; 15-fixture delivery test suite green

Deliver canonical self-digest verification and schema/policy validation.

Acceptance:

- same semantic input produces same digest;
- tampered manifest fails;
- malformed lanes/risk fail;
- base/head identity is mandatory.

### DLV-3103 — Stable Oracle engine

Status: COMPLETE — repository; fail-closed fixture matrix green

Deliver a generic Oracle evaluator with fail-closed semantics.

Acceptance fixtures:

```text
disabled + skipped → PASS
enabled + success → PASS
enabled + failure → FAIL
enabled + skipped → FAIL
enabled + cancelled → FAIL
enabled + missing → FAIL
invalid detector/manifest → FAIL
```

### DLV-3104 — Delivery observation summary

Status: COMPLETE — initial v1 repository implementation

Produce a versioned run summary from manifest + lane results/evidence.

Initial implementation may be local/workflow-artifact based; GitHub runner-minutes API integration can be added after the schema is stable.

### DLV-3105 — Shadow CI workflow/jobs

Status: IMPLEMENTED — GitHub shadow run validation pending

Run detector/validation/Oracle/observation in parallel with current authoritative CI.

Hard rule:

> shadow results may fail the pilot branch but may not cause existing required production checks to be skipped.

## 7. Phase 2 — Affected PR, exhaustive main, stable protected Oracles

### DLV-3201 — Split logical quality lanes

Status: TODO

Expose Web/API/AI/contracts as independently selectable logical lanes while preserving repository-level release validation semantics for full runs.

### DLV-3202 — PR affected execution

Status: TODO

PR jobs consume the manifest instead of recalculating scope.

Representative acceptance matrix:

```text
docs-only
Web-only
AI-only
Go-only
migration
shared contract
Docker/runtime
CI/toolchain/root
release workflow
```

### DLV-3203 — Full main override

Status: TODO

Prove every `main` push executes all required quality/database/experience lanes regardless of the diff category.

### DLV-3204 — Stable Oracle GitHub contexts

Status: TODO

Introduce:

```text
Governance Oracle
Quality Oracle
Database Oracle
Experience Oracle
Delivery Observation
```

### DLV-3205 — Branch protection cutover

Status: TODO / EXTERNAL GOVERNANCE CHANGE

Only after shadow evidence:

- change repository governance to require stable Oracles + commit lint;
- administrator bypass remains disabled;
- force push/deletion remain disabled;
- old concrete contexts are removed from required protection only after equivalence is proven.

## 8. Phase 3 — Exact-SHA candidate and canonical staging

### DLV-3301 — Candidate release-set manifest

Status: TODO

Define a candidate manifest binding:

```text
git SHA
CI run
Web/API/AI/runtime image digests
static asset revision
migration head
```

### DLV-3302 — Build main candidate once

Status: TODO

After full main CI, publish four immutable candidate images with `sha-<revision>` identity.

No production pointer change.

### DLV-3303 — Preserve static asset ordering

Status: TODO

Revision-scoped assets must be published/verified before the candidate Web image that references them becomes eligible.

### DLV-3304 — Promote coherent staging channel

Status: TODO

Move all four validated candidate artifacts to `staging-latest` only after complete candidate publication.

### DLV-3305 — GCP staging deploy watcher

Status: TODO

Deliver a fail-closed watcher that:

- checks four revision labels;
- rejects mixed candidate pointers;
- validates Compose/runtime bundle;
- locks concurrent deployment;
- health-gates service rollout;
- records exact deployed revision.

### DLV-3306 — Staging runtime cutover

Status: TODO / RUNTIME CHANGE

Acceptance:

```text
staging revision == promoted successful main candidate revision
```

Keep current source-build staging command as an explicitly non-canonical emergency fallback during pilot.

## 9. Phase 4 — Release Lifecycle V3

### DLV-3401 — Manual Prepare Release

Status: TODO

Change release-please ownership so normal successful main CI does not automatically prepare every release milestone.

Workflow name/contract:

```text
Prepare Release
workflow_dispatch
```

### DLV-3402 — Exact release identity contract

Status: TODO

Validate version/tag/release PR/main SHA consistency.

### DLV-3403 — Draft-first Release Publish

Status: TODO

Release Publish must:

- resolve exact successful main CI;
- require exact candidate set;
- create/resume Draft release/tag;
- promote existing candidate digests to immutable `vX.Y.Z` identities;
- generate canonical release manifest;
- publish only after postflight.

### DLV-3404 — Existing PR #138 disposition

Status: BLOCKED UNTIL CUTOVER DECISION

Before Release V3 becomes authoritative, explicitly choose:

```text
A. finish v0.9.0 with legacy lifecycle, then V3 starts next release
or
B. close/recreate v0.9.0 after V3 activation
```

No automatic closure/merge of #138 is allowed merely as a side effect of this implementation branch.

## 10. Phase 5 — Production Deploy separation

### DLV-3501 — Deploy Production selector

Status: TODO

Add explicit workflow that accepts a Published release tag and validates:

```text
release published
canonical manifest present
tag SHA == manifest SHA
exact successful main CI
release image digests == manifest
allowed production ref
```

### DLV-3502 — Coherent prod-latest promotion

Status: TODO

Only Deploy Production moves the four `prod-latest` pointers.

Release Publish must not move production pointers.

### DLV-3503 — Production watcher provenance extension

Status: TODO

Preserve the current watcher transaction and optionally record:

```text
release=<vX.Y.Z>
release_manifest_digest=<sha256>
revision=<git SHA>
```

in `.deploy-state` after successful rollout.

## 11. Phase 6 — Audit and cleanup

### DLV-3601 — Delivery Platform audit

Status: TODO

Scheduled/manual non-mutating fault matrix covering:

- manifest tampering;
- Oracle missing child;
- candidate revision mismatch;
- staging split revision;
- release identity mismatch;
- production coherent-pointer check-only path;
- backup/release evidence negative cases.

### DLV-3602 — Release health observation

Status: TODO

Non-blocking release backlog visibility:

```text
latest release age
commits since latest release
pending release PR age/count
staging ahead of production revision count/age
```

### DLV-3603 — Legacy path removal

Status: TODO

Only after V3 evidence exists:

- delete obsolete automatic release coupling;
- delete obsolete source-built canonical staging assumptions;
- archive compatibility paths;
- update canonical deployment architecture;
- archive this plan with runtime evidence.

## 12. Validation matrix

Repository validation during implementation must include, as applicable:

```text
node --test scripts/delivery/*.test.mjs
manifest fixture matrix
digest tamper negative test
Oracle fail-closed matrix
workflow YAML parser/actionlint equivalent
scripts/validate-repo.sh
PostgreSQL current-history replay
PostgreSQL production-baseline validation
Browser E2E when runtime behavior changes
local deploy validation for watcher/release changes
git diff --check
```

No test may be removed or weakened to make the platform migration green.

## 13. Commit strategy

Prefer reviewable phase commits:

```text
docs(delivery): define delivery platform v3
feat(ci): add shadow delivery control plane
refactor(ci): adopt affected pr and full main policy
feat(delivery): publish exact-sha candidates and staging channel
refactor(release): separate prepare and publish lifecycle
feat(deploy): separate production release promotion
chore(delivery): close v3 rollout evidence
```

Do not combine production pointer cutover with the first control-plane implementation commit.

## 14. Definition of Done

This plan is COMPLETE only when all are true:

- one permanent `main` branch model is reflected in code and docs;
- PR affected scope is deterministic and tested;
- main full CI is proven;
- stable Oracles own branch protection;
- exact-SHA candidate set exists;
- staging reports/deploys an exact promoted main revision;
- Prepare Release is explicit;
- Release Publish reuses verified candidate digests and emits canonical release manifest;
- Published Release does not itself move production;
- Deploy Production explicitly promotes a published coherent release;
- production watcher retains all safety/rollback gates and records exact identity;
- delivery observation and audit evidence exist;
- legacy paths are removed only after replacement validation;
- plan is moved to `docs/plan/archive/` with final evidence.
