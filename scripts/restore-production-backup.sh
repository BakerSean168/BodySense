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
#   - runs the domain + migration validators against the disposable target.
#
# Usage:
#   restore-production-backup.sh \
#     --object-key <PREFIX>/<yyyyMMdd>/bodysense-postgres-<ts>.dump \
#     --target-db <disposable_restore_db> \
#     --target-project <drill|staging|...> \
#     --restore-pg container:<disposable_restore_container> \
#     --confirm-target-isolated=yes \
#     [--validator-runner docker|golang] \
#     [--baseline-version N]
#
# Safety guards (all must pass):
#   1. --confirm-target-isolated=yes is required.
#   2. --restore-pg container:<id|name> is required and must identify a
#      disposable PostgreSQL container/server that is provably NOT the live
#      production PostgreSQL container/endpoint; equality is refused.
#   3. --target-db must differ from the production DB_NAME and must not already
#      exist on the disposable restore server; it is created fresh there and
#      never dropped or reused by this script.
#   4. --target-project must differ from the production project "bodysense".
#   5. the object key must be under the configured OFFHOST_BACKUP_PREFIX.
#   6. credentials are supplied to the S3 client only through the environment,
#      never via the process command line.
set -Eeuo pipefail

ROOT="${BODYSENSE_DEPLOY_ROOT:-/opt/bodysense}"
PUBLIC_ENV="$ROOT/.env.production"
SECRET_ENV="$ROOT/.env.production.local"
COMPOSE="$ROOT/docker/docker-compose.prod.yml"
STATE_DIR="$ROOT/.offhost-state"
WORK_DIR="$ROOT/.offhost-work"
LOCK_FILE="$STATE_DIR/offhost-restore.lock"
S3_CLIENT="$(cd "$(dirname "$(readlink -f "$0")")" && pwd)/offhost-s3.py"
TOOL_VERSION="1.1.0"

TARGET_DB=""
TARGET_PROJECT=""
OBJECT_KEY=""
RESTORE_PG=""
CONFIRM_ISOLATED=""
VALIDATOR_RUNNER="docker"
BASELINE_VERSION=""
WORK_DIR_OVERRIDE=""

usage() {
  echo "usage: restore-production-backup.sh --object-key KEY --target-db DB --target-project PROJECT --restore-pg container:<id|name> --confirm-target-isolated=yes [--validator-runner docker|golang] [--baseline-version N]" >&2
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

[ -n "$BUCKET" ] || fail 'OFFHOST_BACKUP_BUCKET is empty'
[ "$URL_STYLE" = path ] || [ "$URL_STYLE" = virtual ] || fail "OFFHOST_BACKUP_URL_STYLE must be path or virtual (got: $URL_STYLE)"
if [ -z "$ACCESS_KEY" ] || [ -z "$SECRET_KEY" ]; then
  fail 'OFFHOST_BACKUP_ACCESS_KEY and OFFHOST_BACKUP_SECRET_KEY must be set in .env.production.local'
fi

# --- Safety guards ----------------------------------------------------------
if [ "$TARGET_DB" = "$DB_NAME" ]; then
  fail "refusing to restore into the production database (--target-db equals DB_NAME=$DB_NAME)"
fi
if [ "$TARGET_PROJECT" = bodysense ]; then
  fail 'refusing a restore into the production project (--target-project must not be "bodysense")'
fi
case "$OBJECT_KEY" in
  "$PREFIX/"*) ;;
  *) fail "--object-key is outside the configured OFFHOST_BACKUP_PREFIX ($PREFIX)" ;;
esac

# --- Prove the restore target is NOT the production PostgreSQL endpoint -----
postgres_container_id() {
  if [ -n "${OFFHOST_PGCONTAINER_ID:-}" ]; then
    printf '%s' "$OFFHOST_PGCONTAINER_ID"
  else
    docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" \
      --env-file "$PUBLIC_ENV" --env-file "$SECRET_ENV" ps -q postgres
  fi
}
resolve_container_id() {
  # Canonicalise to the Docker long ID so `container:postgres` (the production
  # service name) and an id/name aliasing the same container are both caught.
  if [ -z "${OFFHOST_PG_PREFIX:-}" ]; then
    docker inspect -f '{{.Id}}' "$1" 2>/dev/null || printf '%s' "$1"
  else
    # Hermetic/test seam: identities are opaque tokens compared literally.
    printf '%s' "$1"
  fi
}

production_container=$(postgres_container_id)
[ -n "$production_container" ] || fail 'unable to identify the production postgres container (is the stack running? or set OFFHOST_PGCONTAINER_ID)'
prod_full=$(resolve_container_id "$production_container")
restore_full=$(resolve_container_id "$RESTORE_TARGET")
[ -n "$restore_full" ] || fail "unable to resolve the restore postgres container $RESTORE_TARGET"
[ "$restore_full" != "$prod_full" ] \
  || fail "refusing to restore into the live production postgres container/endpoint (--restore-pg ${RESTORE_PG} resolves to the production postgres ${production_container})"
if [ -z "${OFFHOST_PG_PREFIX:-}" ]; then
  restore_state=$(docker inspect -f '{{.State.Running}}' "$RESTORE_TARGET" 2>/dev/null || true)
  [ "$restore_state" = true ] || fail "restore postgres container $RESTORE_TARGET is not running"
fi

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

log "restore drill target: database=$TARGET_DB project=$TARGET_PROJECT restore_pg=$RESTORE_PG (production is db=$DB_NAME project=bodysense postgres=$production_container)"

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
restored_revision=$(pg psql -U "$DB_USER" -d "$TARGET_DB" -Atc \
  "SELECT version::text || ':' || dirty::text FROM schema_migrations ORDER BY version DESC LIMIT 1;" \
  2>/dev/null || true)
[ -n "$restored_revision" ] || fail 'restored database has no schema_migrations state'
case "$meta_revision" in
  unknown|uninitialized)
    log "backup metadata schema_revision=$meta_revision; restored state is $restored_revision (equality not enforced)"
    ;;
  *)
    [ "$restored_revision" = "$meta_revision" ] \
      || fail "restored schema revision $restored_revision does not match backup metadata $meta_revision"
    log "restored schema revision matches backup metadata ($restored_revision)"
    ;;
esac

# --- Run validation binaries against the disposable database ----------------------
VALIDATE_RESULT=FAIL
# shellcheck disable=SC2155
export PGPASSWORD="$DB_PASSWORD"
run_validator() {
  local bin="$1"; shift
  case "$VALIDATOR_RUNNER" in
    docker)
      docker exec api "/app/validators/$bin" "$@" || return 1
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
  # The api container resolves the disposable restore container by its Docker
  # network name/id; operators must run the restore container on the same
  # user-defined network as the api container.
  dsn_host="$RESTORE_TARGET"
else
  dsn_host=127.0.0.1
fi
dsn="postgres://$DB_USER:$DB_PASSWORD@$dsn_host:5432/$TARGET_DB?sslmode=disable"
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