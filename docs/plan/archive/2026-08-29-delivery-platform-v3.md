# BodySense Delivery Platform V3 Pilot

> Date: 2026-08-29
> Status: COMPLETE — production-proven on v0.9.0; archived after cleanup merge
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

Status: COMPLETE — repository + shadow Actions run 33236292914

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

Status: COMPLETE — Delivery Observation proven in authoritative CI (including main run 33238855280)

Produce a versioned run summary from manifest + lane results/evidence.

Initial implementation may be local/workflow-artifact based; GitHub runner-minutes API integration can be added after the schema is stable.

### DLV-3105 — Shadow CI workflow/jobs

Status: COMPLETE — GitHub Actions run 33236292914 passed Scope → Policy Contract → Oracle → Observation

Run detector/validation/Oracle/observation in parallel with current authoritative CI.

Hard rule:

> shadow results may fail the pilot branch but may not cause existing required production checks to be skipped.

## 7. Phase 2 — Affected PR, exhaustive main, stable protected Oracles

### DLV-3201 — Split logical quality lanes

Status: COMPLETE — GitHub CI run 33237118453 passed affected quality + database + experience children and all compatibility contexts

Expose Web/API/AI/contracts as independently selectable logical lanes while preserving repository-level release validation semantics for full runs.

### DLV-3202 — PR affected execution

Status: COMPLETE — manifest-selected PR quality/database/experience execution validated in GitHub CI run 33237118453

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

Status: COMPLETE — main run 33238855280 emitted `full=true` with every lane enabled and completed the exhaustive safety set

Prove every `main` push executes all required quality/database/experience lanes regardless of the diff category.

### DLV-3204 — Stable Oracle GitHub contexts

Status: COMPLETE — Oracles passed CI; migration compatibility contexts were later removed after branch-protection cutover and production proof

Introduce:

```text
Governance Oracle
Quality Oracle
Database Oracle
Experience Oracle
Delivery Observation
```

### DLV-3205 — Branch protection cutover

Status: COMPLETE — Repository Governance run 33239175031 requires Governance/Quality/Database/Experience Oracle + Delivery Observation + commit-lint

Only after shadow evidence:

- change repository governance to require stable Oracles + commit lint;
- administrator bypass remains disabled;
- force push/deletion remain disabled;
- old concrete contexts are removed from required protection only after equivalence is proven.

## 8. Phase 3 — Exact-SHA candidate and canonical staging

### DLV-3301 — Candidate release-set manifest

Status: COMPLETE — repository schema/validator + mixed-revision negative tests

Define a candidate manifest binding:

```text
git SHA
CI run
Web/API/AI/runtime image digests
static asset revision
migration head
```

### DLV-3302 — Build main candidate once

Status: COMPLETE — candidate run 33239156530 (397dbd25) and release candidate run 33240373618 (e61326b4) both succeeded

After full main CI, publish four immutable candidate images with `sha-<revision>` identity.

No production pointer change.

### DLV-3303 — Preserve static asset ordering

Status: COMPLETE — revision CDN publication and Web↔CDN coherence passed in candidate runs 33239156530 and 33240373618

Revision-scoped assets must be published/verified before the candidate Web image that references them becomes eligible.

### DLV-3304 — Promote coherent staging channel

Status: COMPLETE — `staging-latest` promoted coherent exact-SHA sets; tag-copy now carbon-copies OCI manifests with `--prefer-index=false`

Move all four validated candidate artifacts to `staging-latest` only after complete candidate publication.

### DLV-3305 — GCP staging deploy watcher

Status: COMPLETE — GCP user-systemd watcher rejected split/missing labels in tests and deployed coherent candidates 397dbd25 then e61326b4

Deliver a fail-closed watcher that:

- checks four revision labels;
- rejects mixed candidate pointers;
- validates Compose/runtime bundle;
- locks concurrent deployment;
- health-gates service rollout;
- records exact deployed revision.

### DLV-3306 — Staging runtime cutover

Status: COMPLETE — timer enabled; staging reached e61326b4 with Web/API/AI healthy, Tailnet/local API health green, schema 59:false, and subsequent polls idempotent

Acceptance:

```text
staging revision == promoted successful main candidate revision
```

Keep current source-build staging command as an explicitly non-canonical emergency fallback during pilot.

## 9. Phase 4 — Release Lifecycle V3

### DLV-3401 — Manual Prepare Release

Status: COMPLETE — manual Prepare Release run 33239754662 created replacement PR #140 after legacy #138 was closed

Change release-please ownership so normal successful main CI does not automatically prepare every release milestone.

Workflow name/contract:

```text
Prepare Release
workflow_dispatch
```

### DLV-3402 — Exact release identity contract

Status: COMPLETE — repository release/version/CHANGELOG/merge-shape contract with negative tests

Validate version/tag/release PR/main SHA consistency.

### DLV-3403 — Draft-first Release Publish

Status: COMPLETE — recovery run 33241966081 published v0.9.0 from candidate run 33240373618; release manifest digest `sha256:448b57191020a8a1bf9cc4b94d940e2dbfd203d87891b075b097f1d99e127f0a`

Release Publish must:

- resolve exact successful main CI;
- require exact candidate set;
- create/resume Draft release/tag;
- promote existing candidate digests to immutable `vX.Y.Z` identities;
- generate canonical release manifest;
- publish only after postflight.

### DLV-3404 — Existing PR #138 disposition

Status: COMPLETE — legacy #138 closed as superseded; explicit Prepare Release created #140, merged as e61326b49237aa7c55b3a1b12c26e6dd977b1095

Before Release V3 becomes authoritative, explicitly choose:

```text
A. finish v0.9.0 with legacy lifecycle, then V3 starts next release
or
B. close/recreate v0.9.0 after V3 activation
```

No automatic closure/merge of #138 is allowed merely as a side effect of this implementation branch.

## 10. Phase 5 — Production Deploy separation

### DLV-3501 — Deploy Production selector

Status: COMPLETE — Deploy Production run 33242163051 validated Published v0.9.0 + canonical manifest + exact CI/tag/main/image provenance

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

Status: COMPLETE — Release Publish left prod-latest unchanged; Deploy Production run 33242163051 moved all four pointers to the exact v0.9.0 digests

Only Deploy Production moves the four `prod-latest` pointers.

Release Publish must not move production pointers.

### DLV-3503 — Production watcher provenance extension

Status: DEFERRED OPTIONAL — existing watcher remains revision-authoritative; GitHub deployment history + release manifest carry release identity until a registry-side selector metadata contract is justified

Preserve the current watcher transaction and optionally record:

```text
release=<vX.Y.Z>
release_manifest_digest=<sha256>
revision=<git SHA>
```

in `.deploy-state` after successful rollout.

## 11. Phase 6 — Audit and cleanup

### DLV-3601 — Delivery Platform audit

Status: COMPLETE — manual Delivery Platform Audit run 33238888664 passed the deterministic negative-path matrix; scheduled workflow remains enabled

Scheduled/manual non-mutating fault matrix covering:

- manifest tampering;
- Oracle missing child;
- candidate revision mismatch;
- staging split revision;
- release identity mismatch;
- production coherent-pointer check-only path;
- backup/release evidence negative cases.

### DLV-3602 — Release health observation

Status: COMPLETE — manual Release Health run 33238889568 succeeded; scheduled non-blocking observation remains enabled

Non-blocking release backlog visibility:

```text
latest release age
commits since latest release
pending release PR age/count
staging ahead of production revision count/age
```

### DLV-3603 — Legacy path removal

Status: COMPLETE — transitional shadow workflow, legacy concrete-context aliases, and manual legacy production-image fallback removed after v0.9.0 production proof

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
docs(docs): define delivery platform v3
feat(ops): add shadow delivery control plane
refactor(ops): adopt affected pr and full main policy
feat(ops): publish exact-sha candidates and staging channel
refactor(ops): separate release preparation and publication
feat(ops): add explicit production release promotion
chore(ops): close v3 rollout evidence
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
## 15. Closure evidence — 2026-08-29

### Source / CI / governance

```text
V3 implementation PR       #139
V3 merge main              397dbd25f6becda424de85fac8ca0e82eb24b8b2
main exhaustive CI         33238855280 (SUCCESS)
main manifest              full=true; every lane enabled
branch protection cutover  33239175031 (SUCCESS)
required contexts           Governance Oracle / Quality Oracle / Database Oracle / Experience Oracle / Delivery Observation / commit-lint
platform audit              33238888664 (SUCCESS)
release health              33238889568 (SUCCESS)
```

### Candidate / staging

First production-shaped candidate proof:

```text
revision                    397dbd25f6becda424de85fac8ca0e82eb24b8b2
candidate run               33239156530 (SUCCESS)
candidate manifest digest   sha256:617ae46c8c8033fcbca35ff22175a44d9024d41a534d97bdf2f09248fe3e273a
staging deploy              PASS
schema                      59:false
```

Release candidate proof:

```text
release merge revision      e61326b49237aa7c55b3a1b12c26e6dd977b1095
full-main CI                33240111227 (SUCCESS)
candidate run               33240373618 (SUCCESS)
candidate manifest digest   sha256:200e926b5a782a8631c1dac275b8208ed35d46e8e17241ff81159c6f89df2d73
staging deploy              PASS @ e61326b4
local + Tailnet health      PASS
schema                      59:false
```

The GCP staging watcher is enabled as a user systemd timer. Repeated polling after deployment reports the revision as already deployed rather than recreating the stack.

### Release lifecycle

Legacy PR #138 was closed rather than mixed into the new lifecycle. Manual Prepare Release run `33239754662` created PR #140, which merged as `e61326b4`.

The first Release Publish run `33240662731` **failed closed** during ACR promotion: Buildx's default `imagetools create --prefer-index=true` wrapped the single-platform Web candidate manifest in a one-member OCI index, changing the outer digest. The candidate config/layers/revision were unchanged, but the release contract correctly rejected the digest mismatch. PR #141 fixed all channel/release promotion paths to carbon-copy single-platform manifests with `--prefer-index=false` and permits recovery only when an existing wrapper index has exactly one child equal to the expected candidate digest.

```text
registry identity hotfix    PR #141 / merge 5fee232c5274feeb7b7b5664caf481253aebb815
hotfix main CI              33241602414 (SUCCESS)
Release Publish recovery    33241966081 (SUCCESS)
Published Release           v0.9.0
release revision            e61326b49237aa7c55b3a1b12c26e6dd977b1095
release manifest digest     sha256:448b57191020a8a1bf9cc4b94d940e2dbfd203d87891b075b097f1d99e127f0a
```

All four `v0.9.0` ACR refs are OCI image manifests whose top-level digests equal the candidate manifest digests exactly. Release publication was separately proven not to change any `prod-latest` pointer.

### Explicit production rollout

```text
Deploy Production selector  33242163051 (SUCCESS)
selected release            v0.9.0
production revision         e61326b49237aa7c55b3a1b12c26e6dd977b1095
deploy state source         acr
production schema           59:false
public API health           PASS
blocked revision            none
```

The Alibaba watcher performed its own transaction:

1. coherent four-pointer verification;
2. active-run preflight;
3. custom-format PostgreSQL backup + SHA-256 + `pg_restore --list` validation (`bodysense-pre-e61326b49237-20260829-080536.dump`);
4. confirmed the PostgreSQL 18 reset boundary was already committed and required no destructive action;
5. health-gated AI → API → Web rollout;
6. public HTTPS health validation;
7. atomic deploy-state update to `e61326b4`.

A subsequent check-only run reported current Web/API/AI/runtime/managed revisions all equal to `e61326b4`, proving the watcher is idempotent after the rollout. `prod-latest` Web/API/AI/runtime digests equal the canonical v0.9.0 release-manifest digests.

### Backup caveat / separate active durability plan

The periodic Alibaba OSS off-host backup timer is **not** accepted by this closeout: the production host currently lacks `OFFHOST_BACKUP_ACCESS_KEY` and `OFFHOST_BACKUP_SECRET_KEY`, so that path fails closed and freshness has no successful state. This is inherited work already tracked by `docs/plan/active/data-durability-backup-2026-08-25.md` (parked by owner), not silently folded into Delivery Platform V3.

For the v0.9.0 production rollout, an independent GCP off-host custom-format snapshot was created and validated before production selection:

```text
file    .runtime/backups/production-pre-v0.9.0-20260829T074729Z.dump
sha256  53332400201e78bdb5c7321e5480a447899901b9cfec029d1f05ff67d6ae1858
size    185391 bytes
verify  PostgreSQL 18 pg_restore --list PASS
```

This one-time snapshot does not replace the parked long-term durability work.

### Final cleanup

After replacement proof, the closeout removes:

- `.github/workflows/delivery-shadow.yml`;
- `.github/workflows/docker-deploy.yml`;
- old concrete-context compatibility alias jobs from `ci.yml`.

The authoritative delivery surface is now: manifest-selected CI children + stable Oracles, exact-SHA candidate publication, canonical GCP staging watcher, explicit Prepare Release, Draft-first Release Publish, explicit Deploy Production, scheduled Delivery Platform Audit, and Release Health observation.
