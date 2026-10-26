#!/usr/bin/env bash
# BodySense V2 release cutover DRY RUN (REL-001).
#
# Prints the planned staging/production cutover steps and checks LOCAL
# preconditions only. It never executes a remote action: no ssh, no docker, no
# gh/GitHub mutation, no curl, no psql, no systemctl. Every remote step is
# printed as text for the operator. There is intentionally no --execute mode.
#
# Runbook: docs/runbooks/v2-release-cutover.md
#
# Exit codes:
#   0  local preconditions pass and the V2 readiness register is admissible
#   1  a local precondition failed (fix the repository/checkout first)
#   2  usage error
#   3  local preconditions pass but release gates are not yet admissible
set -Eeuo pipefail

usage() {
  cat <<'USAGE'
usage: scripts/v2-release-dry-run.sh [--target staging|production|all] [--sha <commit>]

Prints the V2 cutover plan and checks local preconditions. Read-only; never
contacts staging, production, the registry or GitHub.

Environment (optional):
  V2_DRY_RUN_ALLOW_DIRTY=1  downgrade the clean-tree check to a warning
  V2_PROD_TARGET_VOLUME   planned new production PostgreSQL volume
                          (default: bodysense-postgres-v2)
USAGE
}

TARGET=all
SHA_ARG=''
while [[ $# -gt 0 ]]; do
  case "$1" in
    --target) TARGET="${2:-}"; shift 2 || { usage >&2; exit 2; } ;;
    --target=*) TARGET="${1#*=}"; shift ;;
    --sha) SHA_ARG="${2:-}"; shift 2 || { usage >&2; exit 2; } ;;
    --sha=*) SHA_ARG="${1#*=}"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1 (this tool has no execute mode)" >&2; usage >&2; exit 2 ;;
  esac
done
case "$TARGET" in staging|production|all) ;; *) usage >&2; exit 2 ;; esac

ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { echo 'not inside a git checkout' >&2; exit 1; }
cd "$ROOT"

FAILURES=0
WARNINGS=0
pass() { printf '  [PASS] %s\n' "$*"; }
warn() { printf '  [WARN] %s\n' "$*"; WARNINGS=$((WARNINGS + 1)); }
failc() { printf '  [FAIL] %s\n' "$*"; FAILURES=$((FAILURES + 1)); }
section() { printf '\n== %s ==\n' "$*"; }
STEP=0
plan() {
  # plan <title> <command text...>; text is printed, never evaluated.
  local title="$1"; shift
  STEP=$((STEP + 1))
  printf '  %02d. %s\n' "$STEP" "$title"
  local line
  for line in "$@"; do printf '        $ %s\n' "$line"; done
}
approval() {
  printf '\n  >>> HUMAN APPROVAL %s: %s\n' "$1" "$2"
  printf '      Stop here. Proceed only after the user explicitly approves %s.\n' "$1"
}

if [[ -n "$SHA_ARG" ]]; then
  SHA="$(git rev-parse --verify --quiet "${SHA_ARG}^{commit}" || true)"
  [[ -n "$SHA" ]] || { echo "unknown commit: $SHA_ARG" >&2; exit 1; }
else
  SHA="$(git rev-parse HEAD)"
fi
SHORT="${SHA:0:12}"
CUR_VOLUME="$(sed -n 's/^POSTGRES_DATA_VOLUME=//p' .env.production 2>/dev/null | tail -1)"
NEW_VOLUME="${V2_PROD_TARGET_VOLUME:-bodysense-postgres-v2}"
STAGING_VOLUME_KEY=postgres-staging-vnext-data
STAGING_PROJECT=bodysense-staging
STAGING_VOLUME="${STAGING_PROJECT}_${STAGING_VOLUME_KEY}"
STAGING_PRESERVED="bodysense-staging-v2-rollback-${SHORT}"

printf 'BodySense V2 release cutover — DRY RUN (no remote action is executed)\n'
printf 'candidate=%s target=%s\n' "$SHA" "$TARGET"

section 'Local preconditions'
for tool in git node sha256sum; do
  if command -v "$tool" >/dev/null 2>&1; then pass "tool available: $tool"; else failc "missing tool: $tool"; fi
done

if [[ -z "$(git status --porcelain --untracked-files=no)" ]]; then
  pass 'tracked files are clean'
elif [[ "${V2_DRY_RUN_ALLOW_DIRTY:-}" == 1 ]]; then
  warn 'tracked files have local modifications (allowed by V2_DRY_RUN_ALLOW_DIRTY=1)'
else
  failc 'tracked files have local modifications; qualify only a committed SHA'
fi

if git rev-parse --verify --quiet origin/main >/dev/null; then
  main_sha="$(git rev-parse origin/main)"
  if [[ "$SHA" == "$main_sha" ]]; then
    pass "candidate equals local origin/main ($SHORT)"
  else
    warn "candidate is not local origin/main (${main_sha:0:12}); only an exact main SHA may be released (run git fetch first)"
  fi
else
  warn 'no local origin/main ref; cannot confirm the candidate is on main'
fi

for required in \
  apps/api/migrations/000001_vnext_baseline.up.sql \
  apps/api/migrations/000001_vnext_baseline.down.sql \
  apps/api/migrations/checksums.sha256 \
  tools/schema/database-schema.json \
  docker/docker-compose.prod.yml \
  docker/docker-compose.staging.yml \
  scripts/production-postgres18-reset.sh \
  scripts/production-deploy-watch.sh \
  scripts/staging-deploy-watch.sh \
  scripts/delivery/v2-staging-readiness.mjs \
  .github/workflows/ci.yml \
  .github/workflows/candidate-publish.yml \
  .github/workflows/release-publish.yml \
  .github/workflows/deploy-production.yml \
  docs/runbooks/v2-release-cutover.md \
  docs/runbooks/vnext-schema-rebaseline-cutover.md; do
  if [[ -s "$required" ]]; then pass "present: $required"; else failc "missing: $required"; fi
done

migration_files="$(cd apps/api/migrations && ls -1 ./*.sql 2>/dev/null | sed 's#^\./##' | sort | tr '\n' ' ')"
if [[ "$migration_files" == '000001_vnext_baseline.down.sql 000001_vnext_baseline.up.sql ' ]]; then
  pass 'only the 000001_vnext_baseline pair exists (big-bang policy)'
else
  failc "unexpected migration files: ${migration_files:-none}"
fi
if (cd apps/api/migrations && sha256sum -c --quiet checksums.sha256 >/dev/null 2>&1); then
  pass 'baseline checksums.sha256 matches'
else
  failc 'baseline checksums.sha256 does not match the SQL files'
fi

if grep -q '^POSTGRES_MAJOR=18$' .env.production 2>/dev/null; then
  pass 'production targets PostgreSQL 18'
else
  failc '.env.production must pin POSTGRES_MAJOR=18'
fi
if [[ -n "$CUR_VOLUME" ]]; then
  pass "production POSTGRES_DATA_VOLUME (committed) = $CUR_VOLUME"
else
  failc '.env.production has no POSTGRES_DATA_VOLUME'
fi
if [[ "$NEW_VOLUME" =~ ^[a-z0-9][a-z0-9_.-]+$ && "$NEW_VOLUME" != "$CUR_VOLUME" ]]; then
  pass "planned production reset volume = $NEW_VOLUME (differs from current)"
elif [[ "$NEW_VOLUME" == "$CUR_VOLUME" ]]; then
  warn "candidate already arms the production reset (POSTGRES_DATA_VOLUME=$CUR_VOLUME); confirm approval A2 was given"
else
  failc "invalid V2_PROD_TARGET_VOLUME: $NEW_VOLUME"
fi
if grep -q "^  ${STAGING_VOLUME_KEY}:" docker/docker-compose.staging.yml 2>/dev/null; then
  pass "staging compose volume key = $STAGING_VOLUME_KEY"
else
  failc "docker/docker-compose.staging.yml no longer declares $STAGING_VOLUME_KEY; update the runbook"
fi

section 'V2 readiness register (local evaluation, hypothetical operator opt-in)'
READY=false
if readiness="$(BODYSENSE_V2_STAGING_PROMOTION_ENABLED=true node scripts/delivery/v2-staging-readiness.mjs 2>&1)"; then
  printf '%s\n' "$readiness" | sed 's/^/  /'
  if printf '%s\n' "$readiness" | grep -q '^V2_STAGING_PROMOTION=PASS$'; then
    READY=true
    pass 'all five domains x six gates and foundations are ready with proof'
  else
    warn 'release is NOT admissible yet: resolve every BLOCKER above before approval A1'
  fi
else
  failc 'v2-staging-readiness evaluation failed to run'
fi

section 'Repository gates to re-run on the exact candidate (local / CI)'
plan 'Migration history + checksums' 'bash scripts/validate-migration-history.sh'
plan 'Baseline schema snapshot drift (throwaway PG18)' 'bash scripts/schema/check-snapshot.sh'
plan 'PostgreSQL 18 production contract' 'bash scripts/validate-production-postgres18.sh'
plan 'Contracts drift (lint, generated, breaking, mutation, conformance, Postman)' 'pnpm contracts:verify'
plan 'Supply chain' 'bash scripts/test-validate-supply-chain.sh' 'bash scripts/validate-supply-chain.sh'
plan 'Web chunk budgets (enforced by the production build)' 'pnpm nx run @bodysense/web:build'
plan 'Delivery policy + V2 plan register' 'pnpm test:delivery' 'pnpm test:delivery'
plan 'Full repository gate' 'pnpm verify:release'
plan 'E2E matrix: CI "Browser longitudinal health E2E child" on the exact SHA + QA-001 (#316) matrix evidence' \
  "gh api repos/BakerSean168/BodySense/commits/$SHA/check-runs --jq '.check_runs[]|[.name,.conclusion]|@tsv'"

if [[ "$TARGET" == staging || "$TARGET" == all ]]; then
  section 'Staging (GCP gcp-dev-01) — wipe by volume replacement, canary, rollback drill'
  approval A1 "staging wipe at candidate $SHORT (and enabling V2 staging promotion)"
  plan 'Pause the staging watcher (GCP host, user systemd)' \
    'systemctl --user stop bodysense-staging-deploy-watch.timer'
  plan 'Quiesce application writers, then PostgreSQL' \
    "docker compose -p $STAGING_PROJECT -f ~/.local/share/bodysense/staging-runtime/docker/docker-compose.staging.yml --env-file ~/.config/bodysense/staging.env stop web ai-service api document-service postgres"
  plan 'Preserve the current volume as the short rollback seam' \
    "docker volume create $STAGING_PRESERVED" \
    "docker run --rm -v $STAGING_VOLUME:/from:ro -v $STAGING_PRESERVED:/to alpine:3.24 sh -c 'cp -a /from/. /to/'"
  plan 'Replace the live volume with an empty one' \
    "docker compose -p $STAGING_PROJECT ... rm -sf postgres" \
    "docker volume rm $STAGING_VOLUME"
  plan 'Admit the exact candidate to staging-latest (repository variable; CI promotes only the exact main SHA)' \
    'gh variable set BODYSENSE_V2_STAGING_PROMOTION_ENABLED --body true' \
    "gh run rerun <Publish Nightly Candidate run id for $SHORT>   # promotes only while $SHORT is still main HEAD"
  plan 'Deploy through the canonical watcher (fresh PG18 volume; API bootstraps 000001_vnext_baseline)' \
    'systemctl --user start bodysense-staging-deploy-watch.timer' \
    'bodysense-staging-deploy-watch --force'
  plan 'Verify exact revision, schema state and health' \
    'cat ~/.local/state/bodysense/staging-deploy-state   # revision must equal the candidate' \
    "docker exec ${STAGING_PROJECT}-postgres-1 psql -U bodysense -d bodysense -Atc \"select version::text||':'||dirty::text from schema_migrations\"   # 1:false" \
    'curl -fsS http://127.0.0.1:20150/api/health'
  plan 'Smoke + canary soak on staging (runbook §6): five-domain journeys, 390px + second device, error budget' \
    'see docs/runbooks/v2-release-cutover.md#6-canary-plan'
  plan 'Rollback drill (runbook §7.1): restore preserved volume + previous runtime, verify, then roll forward again' \
    'see docs/runbooks/v2-release-cutover.md#71-staging-rollback-drill'
  plan 'Commit the staging wipe only after canary + drill pass' \
    "docker volume rm $STAGING_PRESERVED"
fi

if [[ "$TARGET" == production || "$TARGET" == all ]]; then
  section 'Production (Alibaba ali-bodysense) — armed volume reset, canary promote'
  approval A2 "production wipe (arm the reset: POSTGRES_DATA_VOLUME $CUR_VOLUME -> $NEW_VOLUME)"
  plan 'Take and verify a final off-host DR backup of the current production database (runbook §8.1)' \
    'see docs/runbooks/production-postgres-dr.md'
  plan 'Merge the arming PR on main (no runtime effect until a Release is deployed)' \
    "sed -i 's/^POSTGRES_DATA_VOLUME=.*/POSTGRES_DATA_VOLUME=$NEW_VOLUME/' .env.production" \
    "update scripts/validate-production-postgres18.sh expectation to $NEW_VOLUME" \
    'the merged SHA becomes the candidate; re-run every gate and the staging flow above on it'
  approval A3 "canary promote of the exact staging-qualified SHA to production"
  plan 'Prepare and publish the Release from the qualified SHA (no rebuild)' \
    'gh workflow run release-please.yml' \
    'gh workflow run release-publish.yml -f candidate_run_id=<successful-nightly-run> -f release_tag=vX.Y.Z'
  plan 'Deploy Production (moves prod-latest; the watcher performs the volume reset)' \
    'gh workflow run deploy-production.yml -f release_tag=vX.Y.Z'
  plan 'Watch the transactional reset and health gates' \
    'cat /opt/bodysense/.postgres18-reset-state   # prepared -> cutover_complete -> committed' \
    'cat /opt/bodysense/.deploy-state' \
    "docker exec docker-postgres-1 psql -U bodysense -d bodysense -Atc \"select version::text||':'||dirty::text from schema_migrations\"   # 1:false" \
    'curl -fsS https://body.bakersean.top/api/health'
  plan 'Post-promote canary window (runbook §6.3) and sign-off record' \
    'see docs/runbooks/v2-release-cutover.md#63-production-canary-window'
fi

section 'Summary'
printf '  failures=%d warnings=%d readiness=%s\n' "$FAILURES" "$WARNINGS" "$([[ $READY == true ]] && echo admissible || echo blocked)"
printf '  approval points: A1 staging wipe, A2 production wipe, A3 canary promote\n'
printf '  DRY_RUN_REMOTE_ACTIONS=0\n'
if [[ "$FAILURES" -gt 0 ]]; then
  printf 'V2_RELEASE_DRY_RUN=FAIL\n'
  exit 1
fi
if [[ "$READY" != true ]]; then
  printf 'V2_RELEASE_DRY_RUN=BLOCKED\n'
  exit 3
fi
printf 'V2_RELEASE_DRY_RUN=READY\n'
