#!/usr/bin/env bash
# BS-PROD-012 restore operator: retrieve, verify and restore an off-host backup.
#
# The restore target is an explicitly isolated, disposable database on an
# explicitly supplied disposable PostgreSQL container that is independent from
# the production PostgreSQL container/endpoint.  The script:
#   - refuses to touch the production database or production PostgreSQL server;
#   - refuses any target that already exists;
#   - re-checks the SHA-256 sidecar (syntax, filename, digest) and the metadata,
#     and proves the downloaded archive matches all three;
#   - validates the archive with pg_restore --list;
#   - restores with --no-owner/--no-privileges;
#   - confirms the restored schema revision matches the backup metadata;
#   - runs the domain + migration validators against the disposable target in a
#     SEPARATE disposable validator container derived from the API image and
#     attached ONLY to the declared drill network (never on the production api
#     container, never on any network that also carries production traffic).
#
# Usage (normal drill):
#   restore-production-backup.sh \
#     --object-key <PREFIX>/<yyyyMMdd>/bodysense-postgres-<ts>.dump \
#     --target-db <disposable_restore_db> \
#     --target-project <drill|staging|...> \
#     --restore-pg container:<disposable_restore_container> \
#     --confirm-target-isolated=yes \
#     [--validator-runner docker|golang] \
#     [--baseline-version N]
#
# Usage (recovery, when the production Postgres container cannot be inspected
# because production is down):
#   restore-production-backup.sh \
#     --object-key <PREFIX>/<yyyyMMdd>/bodysense-postgres-<ts>.dump \
#     --target-db <recovery_db> \
#     --target-project <recovery_project> \
#     --restore-pg container:<recovery_postgres_container> \
#     --confirm-target-isolated=yes \
#     --recovery-mode=yes \
#     [--recovery-production-project bodysense] \
#     [--validator-runner docker|golang] \
#     [--baseline-version N]
#
# Safety guards (all must pass):
#   1. --confirm-target-isolated=yes is required.
#   2. --restore-pg container:<id|name> is required and must identify a running
#      disposable PostgreSQL container that is provably isolated from the live
#      production PostgreSQL container/endpoint.  The proof (via docker inspect)
#      is: different container ID; NOT a member of the production Compose
#      project; NOT using host or `none` networking; attached to NO Docker
#      network shared with the production postgres container; actually attached
#      to an operator-declared dedicated non-host drill network
#      `bodysense.restore-network=<network>` that is the container's ONLY
#      network; publishing NO host ports; and operator-declared labels
#      `bodysense.restore-project=<--target-project>` and
#      `bodysense.disposable-restore=yes` on the container itself.  The declared
#      drill network must then also contain ONLY the disposable restore
#      container: every container attached to the drill network is enumerated and
#      the restore is refused unless the membership is exactly the restore
#      container itself — any member that is the production postgres container,
#      carries the production Compose project, or is an unrelated/compromised
#      container is a refusal.  Docker bridge connectivity is bidirectional, so
#      any container joined to the drill network could reach the disposable
#      restore database.  Any docker inspect / network enumeration failure, an
#      empty member set, or a member set that does not include the restore
#      container itself is fail-closed (refused), never treated as an isolated
#   3. --target-db must differ from the production DB_NAME and must not already
#      exist on the disposable restore server; it is created fresh there and
#      never dropped or reused by this script.
#   4. --target-project must differ from the production project ("bodysense", or
#      --recovery-production-project in recovery mode).
#   5. the object key must be under the configured OFFHOST_BACKUP_PREFIX.
#   6. credentials are supplied to the S3 client only through the environment,
#      never via the process command line.
#   7. the database password reaches the validators only via PGPASSWORD in the
#      process environment (docker: injected through an --env-file, never on a
#      command line; golang: inherited), and is never packed into -database-url.
#   8. the validators run (docker runner) inside a SEPARATE disposable validator
#      container derived from the API image — taken from OFFHOST_VALIDATOR_IMAGE
#      or read off the resolved api container's Config.Image — attached ONLY to
#      the declared drill network and removed when it exits.  The production api
#      container is never joined to the drill network (a production container on
#      the drill network is refused by guard 2's membership proof).
#   9. backup metadata must carry an exact, verifiable schema revision
#      (<version>:false, i.e. a proven-clean revision); `unknown`
#      `uninitialized`, empty, or dirty/malformed metadata is refused
#      (fail-closed), and the restored database revision must equal the metadata
#      revision — the gate is never skipped.
#   10. recovery mode (--recovery-mode=yes, or OFFHOST_RECOVERY_MODE=true) is ONLY
#      for actual recovery from a production outage, when the production
#      Postgres container cannot be inspected.  In recovery mode the proof that
#      production and the target cannot share a container/network is NOT claimed
#      (production is down and cannot be enumerated): isolation rests entirely on
#      the target's own provable declarations (its dedicated non-host drill
#      network as its ONLY network, no published host ports, the disposable
#      labels, and a compose-project label distinct from the named production
#      project).  The operator-declared production project name defaults to
#      "bodysense".
set -Eeuo pipefail

ROOT="${BODYSENSE_DEPLOY_ROOT:-/opt/bodysense}"
PUBLIC_ENV="$ROOT/.env.production"
SECRET_ENV="$ROOT/.env.production.local"
COMPOSE="$ROOT/docker/docker-compose.prod.yml"
STATE_DIR="$ROOT/.offhost-state"
WORK_DIR="$ROOT/.offhost-work"
LOCK_FILE="$STATE_DIR/offhost-restore.lock"
S3_CLIENT="$(cd "$(dirname "$(readlink -f "$0")")" && pwd)/offhost-s3.py"
TOOL_VERSION="1.5.0"

TARGET_DB=""
TARGET_PROJECT=""
OBJECT_KEY=""
RESTORE_PG=""
CONFIRM_ISOLATED=""
VALIDATOR_RUNNER="docker"
BASELINE_VERSION=""
WORK_DIR_OVERRIDE=""
RECOVERY_MODE_FLAG=""
RECOVERY_PRODUCTION_PROJECT_FLAG=""

usage() {
  echo "usage: restore-production-backup.sh --object-key KEY --target-db DB --target-project PROJECT --restore-pg container:<id|name> --confirm-target-isolated=yes [--recovery-mode=yes] [--recovery-production-project PROJECT] [--validator-runner docker|golang] [--baseline-version N]" >&2
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --object-key) OBJECT_KEY="${2:-}"; shift 2 ;;
    --target-db) TARGET_DB="${2:-}"; shift 2 ;;
    --target-project) TARGET_PROJECT="${2:-}"; shift 2 ;;
    --restore-pg) RESTORE_PG="${2:-}"; shift 2 ;;
    --confirm-target-isolated) CONFIRM_ISOLATED="${2:-}"; shift 2 ;;
    --confirm-target-isolated=yes) CONFIRM_ISOLATED=yes; shift ;;
    --validator-runner) VALIDATOR_RUNNER="${2:-}"; shift 2 ;;
    --baseline-version) BASELINE_VERSION="${2:-}"; shift 2 ;;
    --workdir) WORK_DIR_OVERRIDE="${2:-}"; shift 2 ;;
    --recovery-mode) RECOVERY_MODE_FLAG="${2:-}"; shift 2 ;;
    --recovery-mode=yes) RECOVERY_MODE_FLAG=yes; shift ;;
    --recovery-mode=no) RECOVERY_MODE_FLAG=no; shift ;;
    --recovery-mode=*) echo "--recovery-mode must be yes or no (got: ${1#*=})" >&2; usage; exit 2 ;;
    --recovery-production-project) RECOVERY_PRODUCTION_PROJECT_FLAG="${2:-}"; shift 2 ;;
    --help|-h) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage; exit 2 ;;
  esac
done

log() { printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }
fail() { log "ERROR: $*" >&2; exit 1; }

[ -n "$OBJECT_KEY" ] || { usage; fail '--object-key is required'; }
[ -n "$TARGET_DB" ] || { usage; fail '--target-db is required'; }
[ -n "$TARGET_PROJECT" ] || { usage; fail '--target-project is required'; }
[ -n "$RESTORE_PG" ] || { usage; fail '--restore-pg container:<id|name> is required (a disposable restore PostgreSQL container distinct from production)'; }
[ "$CONFIRM_ISOLATED" = yes ] || fail '--confirm-target-isolated=yes is required before any restore operation'
case "$VALIDATOR_RUNNER" in docker|golang) ;; *) fail "--validator-runner must be docker or golang (got: $VALIDATOR_RUNNER)" ;; esac
case "$TARGET_DB" in
  *[!a-z_0-9]*|"") fail "invalid --target-db (use lowercase letters, digits, underscores): $TARGET_DB" ;;
  [0-9]*) fail "invalid --target-db (must not start with a digit): $TARGET_DB" ;;
esac
case "$RESTORE_PG" in
  container:*) RESTORE_TARGET="${RESTORE_PG#container:}" ;;
  *) fail "--restore-pg must be container:<id|name> (got: $RESTORE_PG)" ;;
esac
[ -n "$RESTORE_TARGET" ] || fail '--restore-pg requires a container id or name after "container:"'
[ -n "$BASELINE_VERSION" ] && [[ "$BASELINE_VERSION" =~ ^[0-9]+$ ]] || [ -z "$BASELINE_VERSION" ] \
  || fail "--baseline-version must be a positive integer"

mkdir -p "$ROOT" "$STATE_DIR" "${WORK_DIR_OVERRIDE:-$WORK_DIR}"
chmod 700 "$STATE_DIR" "${WORK_DIR_OVERRIDE:-$WORK_DIR}"
exec 9>"$LOCK_FILE"
flock -n 9 || { log 'another off-host restore operation is already running'; exit 1; }

[ -f "$S3_CLIENT" ] || fail "missing $S3_CLIENT"
[ -s "$PUBLIC_ENV" ] || fail "missing $PUBLIC_ENV"
[ -s "$SECRET_ENV" ] || fail "missing $SECRET_ENV"

read_public_env() {
  local key="$1" default="${2:-}" value
  value=$(sed -n "s/^${key}=//p" "$PUBLIC_ENV" | tail -1)
  printf '%s' "${value:-$default}"
}
read_secret_env() {
  local key="$1" default="${2:-}" value
  value=$(sed -n "s/^${key}=//p" "$SECRET_ENV" | tail -1)
  printf '%s' "${value:-$default}"
}
cfg() {
  local key="$1" default="${2:-}"
  if [ -n "${!key:-}" ]; then printf '%s' "${!key}"; else printf '%s' "$(read_public_env "$key" "$default")"; fi
}
secret_cfg() {
  local key="$1" default="${2:-}"
  if [ -n "${!key:-}" ]; then printf '%s' "${!key}"; else printf '%s' "$(read_secret_env "$key" "$default")"; fi
}

BUCKET=$(cfg OFFHOST_BACKUP_BUCKET)
ENDPOINT=$(cfg OFFHOST_BACKUP_ENDPOINT https://oss-cn-hangzhou.aliyuncs.com)
REGION=$(cfg OFFHOST_BACKUP_REGION cn-hangzhou)
PREFIX=$(cfg OFFHOST_BACKUP_PREFIX bodysense/postgres)
URL_STYLE=$(cfg OFFHOST_BACKUP_URL_STYLE path)
ACCESS_KEY=$(secret_cfg OFFHOST_BACKUP_ACCESS_KEY)
SECRET_KEY=$(secret_cfg OFFHOST_BACKUP_SECRET_KEY)
DB_USER=$(cfg DB_USER bodysense)
DB_NAME=$(cfg DB_NAME bodysense)
DB_PASSWORD=$(secret_cfg DB_PASSWORD)
COMPOSE_FILE="${BODYSENSE_COMPOSE_FILE:-$COMPOSE}"
COMPOSE_PROJECT="${BODYSENSE_COMPOSE_PROJECT:-docker}"

# Recovery mode is opt-in, only for actual recovery from a production outage.
# CLI: --recovery-mode=yes | --recovery-mode no; env: OFFHOST_RECOVERY_MODE=true.
RECOVERY_MODE=no
if [ -n "${OFFHOST_RECOVERY_MODE:-}" ]; then
  case "${OFFHOST_RECOVERY_MODE,,}" in
    true|yes|1) RECOVERY_MODE=yes ;;
    false|no|0|"") ;;
    *) fail "OFFHOST_RECOVERY_MODE must be true or false (got: $OFFHOST_RECOVERY_MODE)" ;;
  esac
fi
if [ -n "$RECOVERY_MODE_FLAG" ]; then
  case "$RECOVERY_MODE_FLAG" in
    yes) RECOVERY_MODE=yes ;;
    no) RECOVERY_MODE=no ;;
    *) fail "--recovery-mode must be yes or no (got: $RECOVERY_MODE_FLAG)" ;;
  esac
fi
if [ -n "$RECOVERY_PRODUCTION_PROJECT_FLAG" ]; then
  RECOVERY_PRODUCTION_PROJECT="$RECOVERY_PRODUCTION_PROJECT_FLAG"
else
  # The operator-declared name of the production Compose/app project.  Defaults
  # to "bodysense" (the production project name used by the safety guards).
  RECOVERY_PRODUCTION_PROJECT=$(cfg OFFHOST_RECOVERY_PRODUCTION_PROJECT bodysense)
fi
[ -n "$RECOVERY_PRODUCTION_PROJECT" ] || fail 'the production project name (--recovery-production-project / OFFHOST_RECOVERY_PRODUCTION_PROJECT) cannot be empty'

[ -n "$BUCKET" ] || fail 'OFFHOST_BACKUP_BUCKET is empty'
[ "$URL_STYLE" = path ] || [ "$URL_STYLE" = virtual ] || fail "OFFHOST_BACKUP_URL_STYLE must be path or virtual (got: $URL_STYLE)"
if [ -z "$ACCESS_KEY" ] || [ -z "$SECRET_KEY" ]; then
  fail 'OFFHOST_BACKUP_ACCESS_KEY and OFFHOST_BACKUP_SECRET_KEY must be set in .env.production.local'
fi

# --- Safety guards ----------------------------------------------------------
if [ "$TARGET_DB" = "$DB_NAME" ]; then
  fail "refusing to restore into the production database (--target-db equals DB_NAME=$DB_NAME)"
fi
if [ "$TARGET_PROJECT" = "$RECOVERY_PRODUCTION_PROJECT" ]; then
  fail "refusing a restore into the production project (--target-project must differ from the production project \"$RECOVERY_PRODUCTION_PROJECT\")"
fi
case "$OBJECT_KEY" in
  "$PREFIX/"*) ;;
  *) fail "--object-key is outside the configured OFFHOST_BACKUP_PREFIX ($PREFIX)" ;;
esac

# --- Prove the restore target is an isolated, disposable container ----------
postgres_container_id() {
  if [ -n "${OFFHOST_PGCONTAINER_ID:-}" ]; then
    printf '%s' "$OFFHOST_PGCONTAINER_ID"
  else
    docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" \
      --env-file "$PUBLIC_ENV" --env-file "$SECRET_ENV" ps -q postgres
  fi
}

# The api service container's image hosts the validator binaries
# (/app/validators) and the migrations at the same revision; the drill derives
# its disposable validator container image from it (see validator_image below) —
# the api container itself is read-only input and is NEVER joined to the drill
# network. docker-compose.prod.yml does NOT set container_name, so with
# the configured Compose project the running container is normally
# "<project>-api-1" (e.g. "docker-api-1"), never a literal "api". Resolve it the
# same way production postgres is found; operators/tests can pin it explicitly
# with OFFHOST_API_CONTAINER.
api_container_name() {
  if [ -n "${OFFHOST_API_CONTAINER:-}" ]; then
    printf '%s' "$OFFHOST_API_CONTAINER"
    return 0
  fi
  local cid name
  cid=$(docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" \
    --env-file "$PUBLIC_ENV" --env-file "$SECRET_ENV" ps -q api 2>/dev/null || true)
  if [ -n "$cid" ]; then
    name=$(docker inspect -f '{{.Name}}' "$cid" 2>/dev/null || true)
    [ -n "$name" ] || name=""
    printf '%s' "${name#/}"
    return 0
  fi
  # Compose default container naming when container_name is unset.
  printf '%s' "$COMPOSE_PROJECT-api-1"
}

# inspect_str prints the value at <key...> inside `docker inspect <container>[0]`.
# Fail-closed: on any daemon/parse error or missing key it prints nothing and
# the caller treats that as a refusal.  Works against the real Docker daemon and
# the hermetic fake `docker` used by the unit tests alike.
inspect_str() {
  local container="$1"; shift
  docker inspect "$container" 2>/dev/null | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)[0]
    for key in sys.argv[2:]:
        d = d[key]
    if isinstance(d, (dict, list)):
        print(json.dumps(d))
    else:
        print(d)
except Exception:
    pass
' "_" "$@" || true
}

# container_networks prints the (newline-separated) Docker network names the
# container is attached to.  FAIL-CLOSED: any docker inspect or JSON parsing
# error returns non-zero (with no output), never an empty "proof of isolation".
# An unprovable network claim is treated as an unsafe/shared target, not as
# evidence that the target and production share nothing.
container_networks() {
  local container="$1"
  docker inspect "$container" 2>/dev/null | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)[0]
    for name in d.get("NetworkSettings", {}).get("Networks", {}):
        print(name)
except Exception:
    sys.exit(1)
'
}

production_proof_skipped=no
if [ "$RECOVERY_MODE" = yes ]; then
  # Recovery mode: production is expected to be down, so NOTHING on the
  # production side is inspected or claimed (no container-ID comparison, no
  # shared-network intersection).  Isolation is proven entirely by the restore
  # target's own declarations below plus the operator-declared production
  # project name.  The recovery-mode proof is strictly weaker than the live
  # drill proof and is only used when production cannot be inspected.
  production_proof_skipped=yes
  production_container="<recovery-mode: production inspection skipped>"
  log "RECOVERY MODE: production postgres container inspection is skipped; target isolation is proven by the restore container's own declarations and the declared production project \"$RECOVERY_PRODUCTION_PROJECT\""
else
  production_container=$(postgres_container_id)
  [ -n "$production_container" ] || fail 'unable to identify the production postgres container (is the stack running? or set OFFHOST_PGCONTAINER_ID)'
  prod_id=$(inspect_str "$production_container" Id)
  [ -n "$prod_id" ] || fail "unable to inspect the production postgres container $production_container (is the stack running? or set OFFHOST_PGCONTAINER_ID)"
fi
restore_id=$(inspect_str "$RESTORE_TARGET" Id)
[ -n "$restore_id" ] || fail "unable to resolve the restore postgres container $RESTORE_TARGET"
if [ "$production_proof_skipped" != yes ]; then
  [ "$restore_id" != "$prod_id" ] \
    || fail "refusing to restore into the live production postgres container/endpoint (--restore-pg ${RESTORE_PG} resolves to the production postgres ${production_container})"
else
  log "RECOVERY MODE: the container-ID-difference proof against production is skipped (production is down and cannot be inspected)"
fi

restore_running=$(inspect_str "$RESTORE_TARGET" State Running)
[ "$restore_running" = True ] || fail "restore postgres container $RESTORE_TARGET is not running"

# The drill target must not be part of the production Compose project: a
# container owned by the production project is traffic-reachable from
# production regardless of its name.
restore_compose=$(inspect_str "$RESTORE_TARGET" Config Labels com.docker.compose.project)
if [ "$production_proof_skipped" = yes ]; then
  # Recovery mode: the operator-declared production project name is the only
  # production-side fact available, and the drill target must not claim it.
  [ "$restore_compose" != "$RECOVERY_PRODUCTION_PROJECT" ] \
    || fail "refusing a restore container labeled with the production compose/project name '$RECOVERY_PRODUCTION_PROJECT'"
  log "RECOVERY MODE: target compose-project '$restore_compose' differs from the declared production project '$RECOVERY_PRODUCTION_PROJECT'"
else
  prod_compose=$(inspect_str "$production_container" Config Labels com.docker.compose.project)
  if [ -n "$prod_compose" ] && [ "$restore_compose" = "$prod_compose" ]; then
    fail "refusing a restore container that belongs to the production compose project '$prod_compose'"
  fi
fi

# Host networking (the `host` driver) or `none` cannot be proven isolated from
# the production host: a host-network container shares the host network stack
# and can reach host-published production endpoints even when it has no Docker
# network name in common with the production postgres container.  Such a target
# is never an acceptable drill server.
restore_mode=$(inspect_str "$RESTORE_TARGET" HostConfig NetworkMode)
case "$restore_mode" in
  host)
    fail "refusing a restore container using host networking ($RESTORE_TARGET): a host-network target is not provably isolated from the production host"
    ;;
  none)
    fail "refusing a restore container with no networking (NetworkMode=none): a dedicated non-host drill network is required"
    ;;
esac

# A dedicated drill network must be declared on the container itself.  This is
# part of the operator proof: the drill server runs on its OWN non-host network,
# never merely "not attached to a network the production postgres happens to be
# on" — an incidental non-overlap provides no isolation guarantee.
restore_network=$(inspect_str "$RESTORE_TARGET" Config Labels bodysense.restore-network)
[ -n "$restore_network" ] \
  || fail "restore postgres container $RESTORE_TARGET does not declare bodysense.restore-network=<dedicated drill network name>; refusing a target that is not provably on a dedicated drill network"
case "$restore_network" in
  host|none)
    fail "refusing declared restore network '$restore_network': host/none networking is not an isolated drill network"
    ;;
esac

# Network enumeration is fail-closed: if either side's network set cannot be
# inspected, the shared-network isolation claim cannot be proven and the restore
# is refused (an inspection failure is never treated as an empty result).  In
# recovery mode the production side cannot be enumerated; that comparison is
# skipped and the target's OWN network proof below remains authoritative.
if [ "$production_proof_skipped" = yes ]; then
  log "RECOVERY MODE: the shared-network intersection with production is skipped (production is down and cannot be enumerated); the restore target still proves its own dedicated-network isolation below"
else
  prod_networks=$(container_networks "$production_container") \
    || fail "unable to inspect the production postgres container network(s) ($production_container); refusing the restore because isolation cannot be proven"
fi
restore_networks=$(container_networks "$RESTORE_TARGET") \
  || fail "unable to inspect the restore postgres container network(s) ($RESTORE_TARGET); refusing the restore because isolation cannot be proven"

# The declared drill network must actually be a network the restore container is
# attached to, and the container must not sit on the host/`none` pseudo
# networks.
if ! printf '%s\n' "$restore_networks" | awk 'NF' | grep -Fxq "$restore_network"; then
  fail "restore postgres container $RESTORE_TARGET is not attached to its declared bodysense.restore-network '$restore_network'; refusing the restore"
fi
while IFS= read -r n; do
  [ -n "$n" ] || continue
  case "$n" in
    host|none)
      fail "refusing a restore container attached to the $n network driver: a dedicated non-host drill network is required"
      ;;
  esac
done <<<"$restore_networks"

# The drill target must not be attached to ANY network shared with the
# production postgres container, so it can neither reach production services nor
# be reached by them.  An empty result from the intersection is only accepted
# because both sides were enumerated above without error.  (In recovery mode the
# production side is down and cannot be enumerated; the intersection is skipped
# and the sole-network proof below is the target's isolation claim.)
if [ "$production_proof_skipped" != yes ]; then
  shared_networks=$(comm -12 \
    <(printf '%s\n' "$prod_networks" | awk 'NF' | sort -u) \
    <(printf '%s\n' "$restore_networks" | awk 'NF' | sort -u) | tr '\n' ' ')
  [ -z "$shared_networks" ] \
    || fail "refusing a restore container attached to the production postgres network(s): $shared_networks"
fi

# The declared dedicated drill network must be the container's ONLY network: an
# incidental "declared + not shared with production" proof is not enough, because
# a target attached to a second ingress/application network is traffic-reachable
# from that network even when it has nothing in common with the production
# postgres container.  The isolation contract is only proven when the network
# set is exactly {bodysense.restore-network}.  (Both sides were enumerated
# without error above, so an empty "no other networks" result is a real proof.)
other_networks=$(printf '%s\n' "$restore_networks" | awk 'NF' | grep -Fvx "$restore_network" || true)
if [ -n "$other_networks" ]; then
  other_list=$(printf '%s' "$other_networks" | tr '\n' ' ')
  fail "refusing a restore container attached to networks beyond its declared drill network '$restore_network': $other_list"
fi

# --- Require the drill network to contain EXACTLY the restore container -----------
# Docker bridge connectivity is bidirectional: a container joined to the drill
# network could reach the disposable restore database and be reached by it.  The
# restore container's own network/port/label proofs above do not cover OTHER
# containers on that network, so every member of the declared drill network is
# enumerated and the restore is refused unless the membership is EXACTLY the
# disposable restore container itself.  This is strictly stronger than refusing
# only production-labelled members: it also refuses an unrelated, compromised or
# mistakenly-attached non-production container, and it refuses the unsafe
# topology of running validators on the production api container joined to the
# drill network.  The restore container must itself be present among the members
# (it is attached to its declared network), and:
#   - any member that is the production postgres container,
#   - any member that carries the production Compose project, or
#   - any member that is not the disposable restore container at all (whether
#     labelled or unlabelled, production or unrelated)
# is a refusal.  An un-inspectable or empty membership set is refused too, never
# treated as an empty "no stray member" proof.
# Applicable in recovery mode too: the membership of the drill network is a
# target-side fact that is always inspectable and must equal the restore target.
drill_members=$(docker network inspect "$restore_network" | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)[0]
    containers = d.get("Containers") or {}
    for cid in containers:
        print(cid)
except Exception:
    sys.exit(1)
') || fail "unable to inspect the drill network $restore_network container membership; refusing the restore because isolation cannot be proven"
drill_seen_restore=no
drill_issues=""
while IFS= read -r member; do
  [ -n "$member" ] || continue
  member_id=$(inspect_str "$member" Id)
  [ -n "$member_id" ] || fail "unable to inspect drill network member $member on '$restore_network'; refusing the restore because isolation cannot be proven"
  if [ "$member_id" = "$restore_id" ]; then
    drill_seen_restore=yes
    continue
  fi
  member_compose=$(inspect_str "$member" Config Labels com.docker.compose.project)
  if [ "$production_proof_skipped" != yes ] && [ "$member_id" = "$prod_id" ]; then
    drill_issues="$drill_issues the production postgres container (${member_id}):${member_compose:-}"
  elif [ -n "$member_compose" ]; then
    drill_issues="$drill_issues a production-project member (${member_id}, compose project '$member_compose'):"
  else
    drill_issues="$drill_issues an unrelated container ($member):"
  fi
done <<<"$drill_members"
if [ -n "$drill_issues" ]; then
  fail "refusing a restore whose drill network '$restore_network' is NOT the disposable restore container's exclusive network; it also contains $drill_issues a non-restore container on the drill network can reach the disposable restore database (Docker bridge connectivity is bidirectional)"
fi
[ "$drill_seen_restore" = yes ] \
  || fail "refusing a restore whose drill network '$restore_network' does not contain the disposable restore container itself; isolation cannot be proven unless the drill network is the restore container's exclusive network"

# Published host ports make the target reachable from the host (and any host
# ingress) even though it is on a dedicated Docker network: a drill server must
# be attached ONLY to its drill network and publish NO host ports, otherwise it
# is not provably isolated from traffic.  Docker reports no bindings as an empty
# object; an absent key (hermetic fakes) is also treated as no bindings.
restore_port_bindings=$(inspect_str "$RESTORE_TARGET" HostConfig PortBindings)
case "$restore_port_bindings" in
  ""|"{}"|null) ;;
  *)
    fail "refusing a restore container that publishes host ports ($RESTORE_TARGET): a host-published target is not provably isolated from traffic"
    ;;
esac

# Disposability and drill-project ownership are declared on the container itself
# at creation time and must match --target-project.  A plain running PostgreSQL
# container (staging, or any other non-disposable database) is refused here.
restore_project=$(inspect_str "$RESTORE_TARGET" Config Labels bodysense.restore-project)
restore_disposable=$(inspect_str "$RESTORE_TARGET" Config Labels bodysense.disposable-restore)
[ "$restore_project" = "$TARGET_PROJECT" ] \
  || fail "restore postgres container $RESTORE_TARGET does not declare bodysense.restore-project=$TARGET_PROJECT; refusing a target that is not provably disposable for --target-project=$TARGET_PROJECT"
[ "$restore_disposable" = yes ] \
  || fail "restore postgres container $RESTORE_TARGET does not declare bodysense.disposable-restore=yes; refusing a non-disposable target"

# Postgres tooling always runs against the disposable restore container (or the
# hermetic OFFHOST_PG_PREFIX seam in tests), never against production.
pg() {
  if [ -n "${OFFHOST_PG_PREFIX:-}" ]; then
    # The seam may need to route at the restore container (rather than the
    # production one), so expose the resolved identity to it.
    OFFHOST_PG_RESTORE_CONTAINER="$RESTORE_TARGET" \
      $OFFHOST_PG_PREFIX "$@"
  else
    docker exec -i "$RESTORE_TARGET" "$@"
  fi
}

s3() {
  OFFHOST_BACKUP_ACCESS_KEY="$ACCESS_KEY" OFFHOST_BACKUP_SECRET_KEY="$SECRET_KEY" \
    python3 "$S3_CLIENT" --endpoint "$ENDPOINT" --bucket "$BUCKET" --region "$REGION" \
      --url-style "$URL_STYLE" "$@"
}

workdir="${WORK_DIR_OVERRIDE:-$WORK_DIR}"
base="${OBJECT_KEY##*/}"
base="${base%.dump}"
dump="$workdir/$base.dump"
shafile="$workdir/$base.dump.sha256"
metafile="$workdir/$base.dump.meta.json"
container_tmp="/tmp/$(basename "$dump")"

if [ "$production_proof_skipped" = yes ]; then
  log "restore drill target: database=$TARGET_DB project=$TARGET_PROJECT restore_pg=$RESTORE_PG (RECOVERY MODE: production db=$DB_NAME project=$RECOVERY_PRODUCTION_PROJECT, assumed down)"
else
  log "restore drill target: database=$TARGET_DB project=$TARGET_PROJECT restore_pg=$RESTORE_PG (production is db=$DB_NAME project=bodysense postgres=$production_container)"
fi

# --- Retrieve -----------------------------------------------------------------
s3 get --key "$OBJECT_KEY.meta.json" --file "$metafile"
python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$metafile" || fail 'backup metadata is not valid JSON'
meta_object_key=$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(d.get("object_key",""))' "$metafile")
meta_revision=$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(d.get("schema_revision",""))' "$metafile")
meta_kind=$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(d.get("backup_kind",""))' "$metafile")
meta_checksum=$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(d.get("checksum_sha256",""))' "$metafile")
[ "$meta_object_key" = "$OBJECT_KEY" ] \
  || fail "metadata object_key ($meta_object_key) does not match the requested key ($OBJECT_KEY)"
[ "$meta_kind" = offhost-postgres ] || fail "metadata backup_kind is $meta_kind, expected offhost-postgres"
# The schema-revision gate is fail-closed from the start: a backup whose
# metadata does not carry an exact clean `<version>:false` revision (including
# `unknown`/`uninitialized`, empty, or a dirty/malformed value) is refused
# outright (no archive download/restore happens), and after the restore the
# restored revision is required to equal it.  Unverifiable metadata is never
# accepted and never skips the gate.
[[ "$meta_revision" =~ ^[0-9]+:false$ ]] \
  || fail "backup metadata declares an unverifiable schema revision '$meta_revision'; refusing a restore drill without an exact clean <version>:false revision"

s3 get --key "$OBJECT_KEY.sha256" --file "$shafile"
s3 get --key "$OBJECT_KEY" --file "$dump"
[ -s "$dump" ] || fail 'downloaded archive is empty'
[ -s "$shafile" ] || fail 'downloaded checksum sidecar is empty'
[ -n "$meta_checksum" ] || fail 'backup metadata has no checksum_sha256'

# --- Verify the SHA-256 sidecar, metadata and archive form one linked triple ---
# The sidecar is the pairing artifact for the archive: its syntax, the object
# name it attests and its digest are all checked, and every digest must agree.
sidecar_line=$(sed -n '1p' "$shafile" | tr -d '\r')
[[ "$sidecar_line" =~ ^([0-9a-f]{64})[[:space:]]+(.+)$ ]] \
  || fail "checksum sidecar is not in '<sha256>  <filename>' format (got: $sidecar_line)"
sidecar_checksum=${BASH_REMATCH[1]}
sidecar_name=${BASH_REMATCH[2]}
[ "$sidecar_name" = "$(basename "$OBJECT_KEY")" ] \
  || fail "checksum sidecar filename ($sidecar_name) does not match object key basename ($(basename "$OBJECT_KEY"))"
[ "$sidecar_checksum" = "$meta_checksum" ] \
  || fail "checksum sidecar digest ($sidecar_checksum) does not match metadata checksum_sha256 ($meta_checksum)"
actual=$(sha256sum "$dump" | awk '{print $1}')
[ "$actual" = "$sidecar_checksum" ] \
  || fail "downloaded archive SHA-256 ($actual) does not match the checksum sidecar ($sidecar_checksum)"
[ "$actual" = "$meta_checksum" ] \
  || fail "downloaded archive SHA-256 ($actual) does not match metadata checksum_sha256 ($meta_checksum)"
log "off-host archive verified: sidecar syntax/name/digest ok and sha256($dump)=metadata"

# --- Validate archive readability over the network protocol ---------------------
docker cp "$dump" "$RESTORE_TARGET:$container_tmp" >/dev/null
if ! pg pg_restore --list "$container_tmp" >/dev/null 2>&1; then
  pg rm -f "$container_tmp" >/dev/null 2>&1 || true
  fail 'off-host archive failed pg_restore archive validation'
fi
pg rm -f "$container_tmp" >/dev/null 2>&1 || true
log "off-host archive validated with pg_restore --list"

# --- Create disposable target on the disposable restore server ------------------
existing=$(pg psql -U "$DB_USER" -d postgres -Atc \
  "SELECT 1 FROM pg_database WHERE datname = '$TARGET_DB';" 2>/dev/null || true)
[ "$existing" != 1 ] || fail "target database $TARGET_DB already exists; refusing to reuse or drop it"
pg psql -U "$DB_USER" -d postgres -c "CREATE DATABASE \"$TARGET_DB\";" >/dev/null \
  || fail "failed to create disposable target database $TARGET_DB on restore postgres $RESTORE_TARGET"

cleanup_container_copy() {
  pg rm -f "$container_tmp" >/dev/null 2>&1 || true
}
trap cleanup_container_copy EXIT

# --- Restore --------------------------------------------------------------------
docker cp "$dump" "$RESTORE_TARGET:$container_tmp" >/dev/null
if ! pg pg_restore -U "$DB_USER" -d "$TARGET_DB" --no-owner --no-privileges -j 2 "$container_tmp" >/dev/null 2>&1; then
  fail "pg_restore into $TARGET_DB on $RESTORE_TARGET failed"
fi
pg rm -f "$container_tmp" >/dev/null 2>&1 || true
log "restored archive into disposable database $TARGET_DB on $RESTORE_TARGET"

# --- Verify restored schema revision vs backup metadata ---------------------------
# The gate is never skipped: backup metadata always carries an exact clean
# revision (any unverifiable `unknown`/`uninitialized`/empty/dirty backup was
# refused above), and the restored database must carry an exact clean revision
# that matches it exactly.
restored_revision=$(pg psql -U "$DB_USER" -d "$TARGET_DB" -Atc \
  "SELECT version::text || ':' || dirty::text FROM schema_migrations ORDER BY version DESC LIMIT 1;" \
  2>/dev/null || true)
[ -n "$restored_revision" ] || fail 'restored database has no schema_migrations state'
[[ "$restored_revision" =~ ^[0-9]+:false$ ]] \
  || fail "restored schema revision '$restored_revision' is not an exact clean <version>:false revision; refusing to certify a dirty or unverifiable restore"
[ "$restored_revision" = "$meta_revision" ] \
  || fail "restored schema revision $restored_revision does not match backup metadata $meta_revision"
log "restored schema revision matches backup metadata ($restored_revision)"

# --- Run validation binaries against the disposable database ----------------------
VALIDATE_RESULT=FAIL
# shellcheck disable=SC2155
export PGPASSWORD="$DB_PASSWORD"
# The database password reaches the validators ONLY through the process
# environment (PGPASSWORD), never through the -database-url argument or any
# process command line, so the secret cannot appear in /proc/*/cmdline.
# `docker run --env-file`/`docker exec --env-file` does not inherit host env
# vars, so the secret is injected with an --env-file (mode 0600) rather than
# `-e PGPASSWORD=...` which would leak it through the docker CLI process argv.
# The golang runner inherits the exported PGPASSWORD directly.
VALIDATOR_ENV_FILE="$WORK_DIR/validator-pgpw.env"
umask 077
printf 'PGPASSWORD=%s\n' "$DB_PASSWORD" > "$VALIDATOR_ENV_FILE"
chmod 600 "$VALIDATOR_ENV_FILE"
umask 022
cleanup_on_exit() {
  cleanup_container_copy
  rm -f "$VALIDATOR_ENV_FILE"
}
trap cleanup_on_exit EXIT

# The validators run inside SEPARATE disposable validator containers derived
# from the API image and attached ONLY to the restore target's declared drill
# network — never on the production api container and never on a network that
# also carries production traffic.  Docker bridge networking is bidirectional,
# so joining the production api container to the drill network would let the
# disposable restore database reach production (and vice versa); the
# drill-network-membership guard above already refuses that topology.  The api
# container is read-only input here (its image is taken as the validator image);
# it is never joined to the drill network.
validator_image() {
  if [ -n "${OFFHOST_VALIDATOR_IMAGE:-}" ]; then
    printf '%s' "$OFFHOST_VALIDATOR_IMAGE"
    return 0
  fi
  local c img
  c=$(api_container_name)
  [ -n "$c" ] || fail 'unable to resolve the api container whose image hosts the validators (set OFFHOST_API_CONTAINER or OFFHOST_VALIDATOR_IMAGE)'
  img=$(inspect_str "$c" Config Image)
  [ -n "$img" ] || fail "unable to read the api container $c image (the source of the validator image); set OFFHOST_VALIDATOR_IMAGE explicitly"
  printf '%s' "$img"
}

VALIDATOR_IMAGE=""
if [ "$VALIDATOR_RUNNER" = docker ]; then
  VALIDATOR_IMAGE=$(validator_image)
  log "running validators in disposable containers derived from the api image $VALIDATOR_IMAGE, attached only to the drill network $restore_network"
fi
run_validator() {
  local bin="$1"; shift
  case "$VALIDATOR_RUNNER" in
    docker)
      # A fresh disposable container derived from the API image, attached only
      # to the drill network (the image's WORKDIR /app carries /app/migrations),
      # running the validator binary, and removed when it exits.
      docker run --rm --network "$restore_network" \
        -l bodysense.disposable-restore=yes \
        --env-file "$VALIDATOR_ENV_FILE" \
        --entrypoint "/app/validators/$bin" \
        "$VALIDATOR_IMAGE" "$@" || return 1
      ;;
    golang)
      # Development/hermetic runner; requires a source checkout at the repo root.
      local repo_root
      repo_root=$(git rev-parse --show-toplevel 2>/dev/null)
      [ -n "$repo_root" ] || return 1
      (cd "$repo_root" && go run "./apps/api/cmd/$bin" "$@") || return 1
      ;;
  esac
}

if [ "$VALIDATOR_RUNNER" = docker ]; then
  # The validator container resolves the disposable restore container by its
  # Docker network name on the shared drill network.
  dsn_host="$RESTORE_TARGET"
else
  dsn_host=127.0.0.1
fi
# No password in the URL: lib/pq (migration-validator) and pgx (domain-validator)
# both read PGPASSWORD from the process environment, keeping DB_PASSWORD out of
# every command line (host and container).
dsn="postgres://$DB_USER@$dsn_host:5432/$TARGET_DB?sslmode=disable"
migration_args=("-database-url" "$dsn" "-migrations" "file://migrations")
[ -z "$BASELINE_VERSION" ] || migration_args+=("-baseline-version" "$BASELINE_VERSION")
if run_validator migration-validator "${migration_args[@]}"; then
  if run_validator domain-validator "-database-url" "$dsn"; then
    VALIDATE_RESULT=PASS
  else
    fail 'domain validator failed against the disposable restore target'
  fi
else
  fail 'migration validator failed against the disposable restore target'
fi

printf 'RESTORE_RESULT=%s database=%s project=%s restore_pg=%s object_key=%s\n' \
  "$VALIDATE_RESULT" "$TARGET_DB" "$TARGET_PROJECT" "$RESTORE_PG" "$OBJECT_KEY"
log "restore drill PASS (database=$TARGET_DB project=$TARGET_PROJECT restore_pg=$RESTORE_PG object_key=$OBJECT_KEY)"