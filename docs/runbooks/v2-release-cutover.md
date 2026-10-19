# BodySense V2 Release Cutover (REL-001)

> Issue: [#317](https://github.com/BakerSean168/BodySense/issues/317) · Ticket: `docs/plan/tickets/v2/REL-001.md`
> Status: **prepared, not executed.** Nothing in this runbook has been run against
> staging or production. Every staging/production step waits for an explicit user
> approval (§2).

V2 is a big-bang release: the single in-place `000001_vnext_baseline` migration is
applied to **fresh** databases. Data is not migrated. Staging and production are
both reset by **volume replacement**, the mechanism already proven by the vNext
rebaseline ([vnext-schema-rebaseline-cutover.md](./vnext-schema-rebaseline-cutover.md)):
the live PostgreSQL 18 volume is swapped for an empty one, the previous volume is
kept only as a short rollback seam, and it is deleted only after every health gate
passes.

Related runbooks: [delivery-platform-v3-rollout.md](./delivery-platform-v3-rollout.md),
[production-postgres-dr.md](./production-postgres-dr.md),
[../architecture/release-lifecycle-v3.md](../architecture/release-lifecycle-v3.md),
[../architecture/delivery-platform-v3.md](../architecture/delivery-platform-v3.md) §8.1.1.

## 1. Order of operations

```text
repository gates green on exact main SHA (§3, §4)
→ V2 readiness register admissible (five domains × six gates + foundations)
→ [A2] arm production reset on main (POSTGRES_DATA_VOLUME change) → new candidate SHA
→ re-qualify that exact SHA (§4)
→ [A1] staging wipe + promote that SHA to staging (§5)
→ staging smoke + canary soak (§6.1) + rollback drill (§7.1)
→ [A3] canary promote: Release Publish + Deploy Production of the same SHA (§8)
→ production watcher performs the transactional volume reset + health gates
→ production canary window (§6.3) → sign-off record (§10)
```

A2 comes before A1 because the production volume name lives in the committed
`.env.production`, which ships inside the runtime image. The SHA that is qualified
on staging must be the exact SHA that later resets production; arming on main has
no runtime effect until a Release containing it is deployed with Deploy Production.

## 2. Human approval points

Only the user can give these approvals. An agent stops at each one, reports the
dry-run output and the evidence collected so far, and waits.

| Id     | Approval            | What it authorizes                                                                                                                                                                                                               | What it does not authorize                         |
| ------ | ------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------- |
| **A1** | **Staging wipe**    | Pause the GCP staging watcher, preserve and replace the staging PostgreSQL volume, set `BODYSENSE_V2_STAGING_PROMOTION_ENABLED=true`, deploy the exact candidate to staging, run the canary soak and the rollback drill.         | Any production action.                             |
| **A2** | **Production wipe** | Merge the PR that changes `.env.production` `POSTGRES_DATA_VOLUME` (`bodysense-postgres-vnext` → `bodysense-postgres-v2`) and the matching `validate-production-postgres18.sh` expectation, and take the final DR backup (§8.1). | Publishing or deploying a release.                 |
| **A3** | **Canary promote**  | Publish the Release from the staging-qualified SHA and run Deploy Production, which executes the armed volume reset on the Alibaba host.                                                                                         | A different SHA than the one qualified on staging. |

Each approval names the exact candidate SHA. A new commit on main after an
approval invalidates it for the new SHA.

## 3. Release gates

Every gate below runs locally (or on a throwaway PostgreSQL 18 container) and in
CI. On `main` the manifest always selects the full lane, so all of them run on the
exact candidate SHA.

| Gate                                                                        | Local command                                                                                      | CI job (required context)                                    |
| --------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- | ------------------------------------------------------------ |
| Migration history + baseline checksums                                      | `bash scripts/validate-migration-history.sh`                                                       | Affected quality child → `validate-repo.sh` (Quality Oracle) |
| Baseline up → down → replay + domain semantics                              | `go run ./cmd/migration-validator`, `go run ./cmd/domain-validator` (apps/api)                     | PostgreSQL 18 vNext baseline child (Database Oracle)         |
| Baseline schema snapshot drift                                              | `bash scripts/schema/check-snapshot.sh` (`pnpm schema:check`)                                      | PostgreSQL 18 vNext baseline child (Database Oracle)         |
| Dump/restore + off-host DR algorithm                                        | see `recovery-migrations-child`                                                                    | PostgreSQL 18 vNext recovery child (Database Oracle)         |
| PostgreSQL 18 production contract                                           | `bash scripts/validate-production-postgres18.sh`                                                   | `validate-repo.sh` (Quality Oracle)                          |
| Contracts drift (lint, generated, breaking, mutation, conformance, Postman) | `pnpm contracts:verify`                                                                            | `validate-repo.sh` / contracts lane (Quality Oracle)         |
| Supply chain                                                                | `bash scripts/test-validate-supply-chain.sh && bash scripts/validate-supply-chain.sh`              | `validate-repo.sh` (Quality Oracle)                          |
| Web chunk budgets                                                           | `pnpm nx run @bodysense/web:build` (the `bodysense-web-chunk-budgets` Vite plugin fails the build) | `validate-repo.sh` `pnpm build` (Quality Oracle)             |
| Validation capacity / architecture                                          | `pnpm quality:capacity`, `pnpm quality:verify`                                                     | `validate-repo.sh` (Quality Oracle)                          |
| Delivery policy + V2 plan register                                          | `pnpm test:delivery`, `pnpm plan:check`                                                            | Governance child (Governance Oracle)                         |
| Release dry-run guard                                                       | `bash scripts/test-v2-release-dry-run.sh`                                                          | `validate-repo.sh` (Quality Oracle)                          |
| Browser E2E matrix                                                          | `pnpm e2e` (Chromium, `BODYSENSE_E2E_STUB_AI=1`, PG18 + Redis + Go API + AI)                       | Browser longitudinal health E2E child (Experience Oracle)    |
| Commit hygiene                                                              | —                                                                                                  | commit-lint                                                  |

**E2E matrix pointer.** The executable matrix is every `apps/web/e2e/*.spec.ts`
run by the Browser longitudinal health E2E child on the exact SHA. The
cross-domain owner-masking / replay / stale-revision / second-device / 390px
matrix and its per-SHA evidence are owned by QA-001
([#316](https://github.com/BakerSean168/BodySense/issues/316)); REL-001 consumes
that evidence and does not redefine it.

**Readiness register.** `scripts/delivery/v2-staging-readiness.mjs` evaluates
`docs/plan/active/v2-handoff-state.json` (owned by the plan owner). The staging
promotion job stays closed until all five domains × six gates and the five
foundations are `ready` with proof **and** the repository variable is set (A1).

## 4. Exact-SHA qualification checklist

Fill one copy of §10 per candidate. Every line refers to the same full SHA.

- [ ] Candidate is the current `origin/main` HEAD (`git rev-parse origin/main`).
- [ ] All six required contexts succeeded on that SHA: `commit-lint`, `Governance Oracle`,
      `Quality Oracle`, `Database Oracle`, `Experience Oracle`, `Delivery Observation`.
- [ ] The main push run selected the full lane (delivery manifest `lanes.full=true`) and
      every child job concluded `success` (no skipped enabled child).
- [ ] Local gates in §3 re-run on a clean checkout of the SHA, including
      `scripts/schema/check-snapshot.sh` against a throwaway PG18.
- [ ] `bash scripts/v2-release-dry-run.sh --sha <sha>` prints `V2_RELEASE_DRY_RUN=READY`
      (exit 0). Exit 3 means the readiness register is still blocked.
- [ ] `Publish Main Candidate` succeeded for the SHA: four `sha-<full sha>` images,
      candidate manifest and revision-scoped static assets; image digests recorded.
- [ ] QA-001 matrix evidence exists for this SHA (or a SHA it is an ancestor-equivalent of,
      with only docs changes in between, stated explicitly).
- [ ] No open user/clinical decision blocks a shipped feature (Q-17 diagnosis TTL and clinical
      thresholds stay out of implementation authority).
- [ ] Staging: `staging-deploy-state` `revision=` equals the SHA, `schema_migrations=1:false`,
      health green, canary soak and rollback drill recorded (§6.1, §7.1).
- [ ] Release manifest binds version/tag/SHA/CI/image digests (Release Publish postflight).
- [ ] Production: `.deploy-state` revision equals the SHA, `.postgres18-reset-state`
      `status=committed release_revision=<sha>`, `schema_migrations=1:false`, public health green.

Never reuse evidence from another SHA (including V11 / PR #247 runs).

## 5. Staging wipe (volume replacement) — requires A1

Host: GCP `gcp-dev-01`, user systemd, compose project `bodysense-staging`,
runtime `~/.local/share/bodysense/staging-runtime`, secrets
`~/.config/bodysense/staging.env`, state `~/.local/state/bodysense/staging-deploy-state`.
The staging compose file declares the fixed volume key `postgres-staging-vnext-data`
(Docker name `bodysense-staging_postgres-staging-vnext-data`), so the replacement
keeps that name and preserves the old contents in a separate rollback volume.

1. Record the current staging revision and its immutable image tags
   (`cat ~/.local/state/bodysense/staging-deploy-state`; images `sha-<revision>`).
   This is the rollback target for §7.1.
2. Pause the watcher: `systemctl --user stop bodysense-staging-deploy-watch.timer`.
3. Quiesce writers, then the database:
   `docker compose -p bodysense-staging -f <runtime>/docker/docker-compose.staging.yml --env-file <staging.env> stop web ai-service api document-service postgres`.
4. Preserve the volume: create `bodysense-staging-v2-rollback-<sha12>` and copy the
   stopped volume into it (`cp -a` from a read-only mount).
5. Replace: `compose rm -sf postgres`, then `docker volume rm bodysense-staging_postgres-staging-vnext-data`.
6. Admit the candidate: set the Actions variable
   `BODYSENSE_V2_STAGING_PROMOTION_ENABLED=true`, then re-run the `Publish Main Candidate`
   run of the SHA (it promotes `staging-latest` only while the SHA is still main HEAD and the
   readiness register passes). Never re-tag `staging-latest` by hand.
7. Resume: `systemctl --user start bodysense-staging-deploy-watch.timer` and
   `bodysense-staging-deploy-watch --force`. Compose recreates an empty volume; the API
   bootstraps `000001_vnext_baseline` before AI and Web start.
8. Verify: state `revision=` equals the SHA; `schema_migrations` reports `1:false`; every
   service healthy; `curl -fsS http://127.0.0.1:20150/api/health` succeeds.
9. Keep the rollback volume until §6.1 and §7.1 pass; then delete it (commit).

## 6. Canary plan

BodySense runs one coherent revision per environment; there is no request-level
traffic split for application releases. The canary is therefore **staged by
environment**: the exact SHA soaks on staging first, and production gets a bounded
observation window while the previous volume still exists. The Diagnosis model
rollout (`DIAGNOSIS_CANARY_BPS`) is independent and stays at its current champion
setting during this release.

### 6.1 Staging canary soak

Proposed defaults (the user may change them at A1):

- duration: at least 24 hours and at least two staging watcher timer cycles with no
  redeploy of a different revision;
- journeys on a fresh account, desktop and 390 px, plus a second device/session:
  Conversation/Person, BodyState facts + AI suggestion decision, Diagnosis decision window,
  TrainingPlan/Trainer with Safety fail-closed, asset upload + verified erasure;
- owner isolation spot check: a foreign id returns an owner-masked 404;
- error budget: zero 5xx on `/api/*` attributable to the release, zero container restarts,
  zero dirty migration state, no Safety fail-open.

### 6.2 Promote criteria (input to A3)

All of §4 up to the staging lines is checked, §6.1 passed, §7.1 drill recorded, and the
SHA is unchanged. Any failure returns to a fix on main and a new candidate SHA.

### 6.3 Production canary window

After Deploy Production, for at least 2 hours (proposed): watch
`.postgres18-reset-state` reach `committed`, `.deploy-state` revision, container health,
API/AI logs for 5xx and Safety errors, and repeat the §6.1 smoke on a fresh production
account. Before `committed`, a failed health gate triggers the watcher's automatic
rollback (§7.2).

## 7. Rollback

### 7.1 Staging rollback drill

Run once after the staging canary, with the watcher paused:

1. Stop the stack services; move the fresh volume aside (copy to
   `bodysense-staging-v2-drill-<sha12>`), restore the preserved volume contents into
   `bodysense-staging_postgres-staging-vnext-data`.
2. Start the previous revision with its immutable images
   (`STAGING_WEB_IMAGE/STAGING_API_IMAGE/STAGING_AI_IMAGE=…:sha-<previous revision>`) and
   verify health and the previous schema state.
3. Roll forward: restore the drill copy (or re-run §5 steps 5–8) and verify the candidate
   is healthy again with `1:false`.
4. Record timings and outcomes in §10. Resume the watcher only after roll-forward.

### 7.2 Production before commit (automatic)

`production-deploy-watch.sh` prepares rollback images and, if any health gate fails
after `production-postgres18-reset.sh cutover`, calls `rollback`, which re-mounts the
untouched source volume (`bodysense-postgres-vnext`) and the previous runtime. Verify
`.postgres18-reset-state` and `.deploy-state` afterwards and stop; do not retry by hand.

### 7.3 Production after commit

After `commit` the source volume is deleted. There is **no data rollback** and the
previous release cannot run on the V2 baseline. Options: roll forward with a fix on a
new qualified SHA, or (user decision only) restore the §8.1 off-host backup with the
previous release per [production-postgres-dr.md](./production-postgres-dr.md).

## 8. Production wipe and promote

### 8.1 Final backup (part of A2)

Take and verify an off-host DR backup of the current production database with the
existing DR tooling before arming. V2 does not migrate this data; the backup only
keeps the §7.3 escape hatch available.

### 8.2 Arm (A2)

One PR on main: `.env.production` `POSTGRES_DATA_VOLUME=bodysense-postgres-v2` and the
matching expectation in `scripts/validate-production-postgres18.sh`. The compose file
already reads `name: ${POSTGRES_DATA_VOLUME:-…}`. Nothing changes on the host until a
Release containing this SHA is deployed. The resulting SHA is the candidate for §4–§6.

### 8.3 Promote (A3)

1. Prepare Release (`release-please.yml`) and `Release Publish` from the qualified SHA.
   No source rebuild; the release manifest binds tag/SHA/digests.
2. `Deploy Production` with `release_tag=vX.Y.Z` moves `prod-latest`.
3. The Alibaba watcher's deploy preflight waits for zero leased Consultation runs, then
   `production-postgres18-reset.sh cutover` sees source volume ≠ `POSTGRES_DATA_VOLUME`,
   stops Caddy/API/AI, starts PostgreSQL 18 on the new empty volume, the API bootstraps
   the baseline, health gates run, `commit` deletes the old volume, Caddy is exposed.
4. Verify per [vnext-schema-rebaseline-cutover.md](./vnext-schema-rebaseline-cutover.md)
   "Verification", with `bodysense-postgres-v2` as the active volume, then run §6.3.

## 9. Dry run

```bash
bash scripts/v2-release-dry-run.sh [--target staging|production|all] [--sha <commit>]
```

Read-only. It checks local preconditions (clean tree, candidate = `origin/main`, baseline
pair + checksums, PG18 pin, current/planned volumes, staging volume key, readiness register
with a hypothetical opt-in) and prints every planned step with the A1/A2/A3 stops. It never
calls ssh, docker, gh, curl, psql or systemctl; `scripts/test-v2-release-dry-run.sh`
enforces that with stub binaries. Exit codes: `0` ready, `1` local precondition failed,
`2` usage, `3` preconditions pass but the readiness register is blocked.

## 10. Evidence record (one per candidate)

```text
candidate_sha=
required_contexts=commit-lint:… Governance:… Quality:… Database:… Experience:… Observation:…
main_ci_run=  candidate_publish_run=  image_digests=web:… api:… ai:… runtime:…
local_gates=PASS (date, host)  schema_snapshot=PASS  dry_run=READY
qa_001_matrix=<link>
A2 approved_by/at=  dr_backup_id=  arming_pr=
A1 approved_by/at=  staging_revision=  staging_schema=1:false  canary_window=  drill=
A3 approved_by/at=  release_tag=  deploy_run=  reset_state=committed  prod_revision=
production_canary_window=  sign_off=
```
