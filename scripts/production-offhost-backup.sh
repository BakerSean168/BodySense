#!/usr/bin/env bash
# BS-PROD-012: operator-owned scheduled off-host PostgreSQL backups.
#
# Creates a custom-format pg_dump, records its SHA-256 and metadata, uploads the
# trio to a private OSS/S3-compatible destination through scripts/offhost-s3.py,
# prunes objects older than the independent off-host retention window, and keeps
# a freshness state file that the separate freshness check consumes.
#
# Security contract:
#   - the source database is read through the ordinary postgres network protocol
#     (docker compose exec postgres pg_dump), never by reading the DB volume;
#   - least-privilege OSS credentials live only in .env.production.local and are
#     passed to offhost-s3.py via the process environment (OFFHOST_BACKUP_*),
#     never on the command line and never written into artifacts;
#   - pruning is strictly scoped under the configured object prefix.
#
# Usage:
#   production-offhost-backup.sh --backup            create + upload + prune
#   production-offhost-backup.sh --check-freshness   alert when backups are stale
#
# Environment (see .env.example/.env.production and BS-PROD-012):
#   non-secret: OFFHOST_BACKUP_ENABLED OFFHOST_BACKUP_BUCKET OFFHOST_BACKUP_ENDPOINT
#               OFFHOST_BACKUP_REGION OFFHOST_BACKUP_PREFIX OFFHOST_BACKUP_URL_STYLE
#               OFFHOST_BACKUP_RETENTION_DAYS OFFHOST_BACKUP_FRESHNESS_HOURS
#               OFFHOST_BACKUP_FRESHNESS_PROBE OFFHOST_BACKUP_ALERT_CMD
#   secret:     OFFHOST_BACKUP_ACCESS_KEY OFFHOST_BACKUP_SECRET_KEY (.env.production.local)
set -Eeuo pipefail

ROOT="${BODYSENSE_DEPLOY_ROOT:-/opt/bodysense}"
PUBLIC_ENV="$ROOT/.env.production"
SECRET_ENV="$ROOT/.env.production.local"
COMPOSE="$ROOT/docker/docker-compose.prod.yml"
STATE_DIR="$ROOT/.offhost-state"
WORK_DIR="$ROOT/.offhost-work"
LOCK_FILE="$STATE_DIR/offhost.lock"
S3_CLIENT="$(cd "$(dirname "$(readlink -f "$0")")" && pwd)/offhost-s3.py"
TOOL_VERSION="1.0.0"

MODE=""
for arg in "$@"; do
  case "$arg" in
    --backup) MODE=backup ;;
    --check-freshness) MODE=freshness ;;
    --help|-h)
      sed -n '2,25p' "$0"
      exit 0
      ;;
    *) echo "unknown argument: $arg" >&2; exit 2 ;;
  esac
done
if [ -z "$MODE" ]; then
  echo "usage: production-offhost-backup.sh --backup|--check-freshness" >&2
  exit 2
fi

log() { printf '[%s] %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }
fail() { log "ERROR: $*" >&2; exit 1; }

mkdir -p "$ROOT" "$STATE_DIR" "$WORK_DIR"
chmod 700 "$STATE_DIR" "$WORK_DIR"
exec 9>"$LOCK_FILE"
flock -n 9 || { log 'another off-host backup operation is already running'; exit 0; }

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
# Process environment wins over env files so operators/tests can override.
cfg() {
  local key="$1" default="${2:-}" value
  if [ -n "${!key:-}" ]; then
    printf '%s' "${!key}"
  else
    value=$(read_public_env "$key" "$default")
    printf '%s' "$value"
  fi
}
secret_cfg() {
  local key="$1" default="${2:-}"
  if [ -n "${!key:-}" ]; then
    printf '%s' "${!key}"
  else
    printf '%s' "$(read_secret_env "$key" "$default")"
  fi
}

OFFHOST_BACKUP_ENABLED=$(cfg OFFHOST_BACKUP_ENABLED true)
if [ "$OFFHOST_BACKUP_ENABLED" != true ]; then
  log "off-host backups disabled (OFFHOST_BACKUP_ENABLED=$OFFHOST_BACKUP_ENABLED)"
  exit 0
fi

BUCKET=$(cfg OFFHOST_BACKUP_BUCKET)
ENDPOINT=$(cfg OFFHOST_BACKUP_ENDPOINT https://oss-cn-hangzhou.aliyuncs.com)
REGION=$(cfg OFFHOST_BACKUP_REGION cn-hangzhou)
PREFIX=$(cfg OFFHOST_BACKUP_PREFIX bodysense/postgres)
URL_STYLE=$(cfg OFFHOST_BACKUP_URL_STYLE path)
RETENTION_DAYS=$(cfg OFFHOST_BACKUP_RETENTION_DAYS 30)
FRESHNESS_HOURS=$(cfg OFFHOST_BACKUP_FRESHNESS_HOURS 30)
FRESHNESS_PROBE=$(cfg OFFHOST_BACKUP_FRESHNESS_PROBE object)
ALERT_CMD=$(cfg OFFHOST_BACKUP_ALERT_CMD)
ACCESS_KEY=$(secret_cfg OFFHOST_BACKUP_ACCESS_KEY)
SECRET_KEY=$(secret_cfg OFFHOST_BACKUP_SECRET_KEY)
DB_USER=$(cfg DB_USER bodysense)
DB_NAME=$(cfg DB_NAME bodysense)
COMPOSE_FILE="${BODYSENSE_COMPOSE_FILE:-$COMPOSE}"
COMPOSE_PROJECT="${BODYSENSE_COMPOSE_PROJECT:-docker}"

[ -n "$BUCKET" ] || fail 'OFFHOST_BACKUP_BUCKET is empty'
[ "$URL_STYLE" = path ] || [ "$URL_STYLE" = virtual ] || fail "OFFHOST_BACKUP_URL_STYLE must be path or virtual (got: $URL_STYLE)"
case "$FRESHNESS_PROBE" in object|state) ;; *) fail "OFFHOST_BACKUP_FRESHNESS_PROBE must be object or state (got: $FRESHNESS_PROBE)" ;; esac
if [ "${RETENTION_DAYS:-0}" -lt 1 ] 2>/dev/null; then fail "OFFHOST_BACKUP_RETENTION_DAYS must be >= 1 (got: $RETENTION_DAYS)"; fi
if [ "${FRESHNESS_HOURS:-0}" -lt 1 ] 2>/dev/null; then fail "OFFHOST_BACKUP_FRESHNESS_HOURS must be >= 1 (got: $FRESHNESS_HOURS)"; fi

# Host-side postgres access.  Everything goes through this one seam so hermetic
# tests can stub it with OFFHOST_PG_PREFIX (e.g. a fake pg_dump/psql/pg_restore).
pg() {
  if [ -n "${OFFHOST_PG_PREFIX:-}" ]; then
    # shellcheck disable=SC2086
    $OFFHOST_PG_PREFIX "$@"
  else
    docker compose -p "$COMPOSE_PROJECT" -f "$COMPOSE_FILE" \
      --env-file "$PUBLIC_ENV" --env-file "$SECRET_ENV" exec -T postgres "$@"
  fi
}

s3() {
  OFFHOST_BACKUP_ACCESS_KEY="$ACCESS_KEY" OFFHOST_BACKUP_SECRET_KEY="$SECRET_KEY" \
    python3 "$S3_CLIENT" --endpoint "$ENDPOINT" --bucket "$BUCKET" --region "$REGION" \
      --url-style "$URL_STYLE" "$@"
}

db_schema_state() {
  local exists value
  if ! exists=$(pg psql -U "$DB_USER" -d "$DB_NAME" -Atc \
    "SELECT to_regclass('public.schema_migrations') IS NOT NULL;" 2>/dev/null); then
    printf '%s' unknown
    return 0
  fi
  if [ "$exists" != t ]; then
    printf '%s' uninitialized
    return 0
  fi
  if ! value=$(pg psql -U "$DB_USER" -d "$DB_NAME" -Atc \
    "SELECT version::text || ':' || dirty::text FROM schema_migrations ORDER BY version DESC LIMIT 1;" \
    2>/dev/null); then
    printf '%s' unknown
    return 0
  fi
  printf '%s' "${value:-uninitialized}"
}

fail_freshness() {
  local reason="$1" last_at="${2:-}" age_h="${3:-}"
  printf 'OFFHOST_BACKUP_FRESH=FAIL reason=%s last_success_at=%s age_hours=%s threshold_hours=%s object_key=%s\n' \
    "$reason" "$last_at" "$age_h" "$FRESHNESS_HOURS" "${LAST_OBJECT_KEY:-}"
  log "ERROR: off-host backup freshness check failed ($reason)"
  if [ -n "$ALERT_CMD" ]; then
    OFFHOST_BACKUP_FRESH=FAIL reason="$reason" last_success_at="$last_at" \
      age_hours="$age_h" threshold_hours="$FRESHNESS_HOURS" bash -c "$ALERT_CMD" || true
  fi
  exit 1
}

run_backup() {
  local ts datedir object_key dump checksum size schema pg_dump_version listing \
    verify_dump verification remote_sha verify_sha

  if [ -z "$ACCESS_KEY" ] || [ -z "$SECRET_KEY" ]; then
    fail 'OFFHOST_BACKUP_ACCESS_KEY and OFFHOST_BACKUP_SECRET_KEY must be set in .env.production.local'
  fi

  ts=$(date -u +%Y%m%d-%H%M%SZ)
  datedir=$(date -u +%Y%m%d)
  object_key="$PREFIX/$datedir/bodysense-postgres-$ts.dump"
  dump="$WORK_DIR/bodysense-postgres-$ts.dump"

  schema=$(db_schema_state)
  log "creating off-host custom-format dump schema=$schema"
  if ! pg pg_dump -U "$DB_USER" -d "$DB_NAME" -Fc > "$dump" 2>/dev/null; then
    rm -f "$dump"
    fail 'pg_dump failed; no off-host backup object was written'
  fi
  [ -s "$dump" ] || { rm -f "$dump"; fail 'off-host dump is empty; no object was written'; }
  size=$(wc -c < "$dump")
  checksum=$(sha256sum "$dump" | awk '{print $1}')
  printf '%s  %s\n' "$checksum" "$(basename "$dump")" > "$dump.sha256"
  pg_dump_version=$(pg pg_dump --version 2>/dev/null | head -1 || true)
  [ -n "$pg_dump_version" ] || pg_dump_version=unknown

  cat > "$dump.meta.json" <<META
{
  "format": 1,
  "tool": "production-offhost-backup.sh",
  "tool_version": "$TOOL_VERSION",
  "backup_kind": "offhost-postgres",
  "object_key": "$object_key",
  "checksum_sha256": "$checksum",
  "schema_revision": "$schema",
  "created_at_utc": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "archive_bytes": $size,
  "archive_format": "custom",
  "pg_dump_version": "$pg_dump_version",
  "source": {
    "project": "bodysense",
    "db_name": "$DB_NAME",
    "db_user": "$DB_USER",
    "host": "$(hostname)"
  },
  "retention_days": "$RETENTION_DAYS"
}
META
  python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$dump.meta.json" \
    || { rm -f "$dump" "$dump.sha256" "$dump.meta.json"; fail 'metadata object is not valid JSON'; }

  log "uploading off-host backup $object_key"
  s3 put --key "$object_key" --file "$dump"
  s3 put --key "$object_key.sha256" --file "$dump.sha256"
  s3 put --key "$object_key.meta.json" --file "$dump.meta.json"

  # Re-fetch checksum + archive and recompute locally so the upload is proven
  # end-to-end, not just accepted.  Operators may skip the (bandwidth-heavy)
  # archive re-download via OFFHOST_BACKUP_SKIP_REDOWNLOAD_VERIFY=true, but the
  # checksum file round-trip and object listing are always enforced.
  verification="verified"
  remote_sha="$WORK_DIR/verify-$ts.dump.sha256"
  s3 get --key "$object_key.sha256" --file "$remote_sha"
  verify_sha=$(awk -F'  ' '{print $1}' "$remote_sha")
  [ "$verify_sha" = "$checksum" ] || { rm -f "$remote_sha"; fail "remote checksum round-trip mismatch ($verify_sha != $checksum)"; }
  if [ "$(cfg OFFHOST_BACKUP_SKIP_REDOWNLOAD_VERIFY false)" != true ]; then
    verify_dump="$WORK_DIR/verify-$ts.dump"
    s3 get --key "$object_key" --file "$verify_dump"
    if [ "$(sha256sum "$verify_dump" | awk '{print $1}')" != "$checksum" ]; then
      rm -f "$remote_sha" "$verify_dump"
      fail 're-downloaded archive checksum does not match the uploaded dump'
    fi
    rm -f "$verify_dump"
  fi
  rm -f "$remote_sha"

  s3 list --prefix "$PREFIX/$datedir/" > "$WORK_DIR/verify-$ts.list" \
    || { rm -f "$WORK_DIR/verify-$ts.list"; fail 'upload listing verification failed'; }
  listing=$(cat "$WORK_DIR/verify-$ts.list")
  rm -f "$WORK_DIR/verify-$ts.list"
  for needle in "$object_key" "$object_key.sha256" "$object_key.meta.json"; do
    printf '%s\n' "$listing" | awk -F'\t' '{print $1}' | grep -qxF "$needle" \
      || { fail "upload listing is missing $needle"; }
  done
  log "off-host upload verified: three objects present, checksum round-trip ok ($verification)"

  prune_old_objects

  umask 077
  cat > "$STATE_DIR/last-success.json.tmp" <<STATE
{
  "last_success_at_utc": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "object_key": "$object_key",
  "checksum_sha256": "$checksum",
  "schema_revision": "$schema",
  "retention_days": "$RETENTION_DAYS",
  "backup_kind": "offhost-postgres"
}
STATE
  python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$STATE_DIR/last-success.json.tmp" >/dev/null
  mv -f "$STATE_DIR/last-success.json.tmp" "$STATE_DIR/last-success.json"
  chmod 600 "$STATE_DIR/last-success.json"
  umask 022

  log "off-host backup complete key=$object_key schema=$schema bytes=$size"
  printf 'OFFHOST_BACKUP_OBJECT=%s\n' "$object_key"
}

prune_old_objects() {
  local boundary newest_dir keys key relative dirdate to_delete
  boundary=$(date -u -d "$RETENTION_DAYS days ago" +%Y%m%d)
  newest_dir=""
  to_delete=""
  s3 list --prefix "$PREFIX/" > "$WORK_DIR/prune-$ts.list" 2>/dev/null || true
  keys=$(awk -F'\t' '{print $1}' "$WORK_DIR/prune-$ts.list")
  rm -f "$WORK_DIR/prune-$ts.list"
  # Collect unique date directories under the prefix.
  while IFS= read -r key; do
    [ -n "$key" ] || continue
    case "$key" in
      "$PREFIX/"*) relative="${key#"$PREFIX"/}" ;;
      *) continue ;;
    esac
    dirdate=${relative%%/*}
    case "$dirdate" in
      [0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]) ;;
      *) continue ;;
    esac
    if [ -z "$newest_dir" ] || [ "$dirdate" -gt "$newest_dir" ]; then
      newest_dir=$dirdate
    fi
  done <<<"$keys"

  while IFS= read -r key; do
    [ -n "$key" ] || continue
    case "$key" in
      "$PREFIX/"*) relative="${key#"$PREFIX"/}" ;;
      *) continue ;;
    esac
    dirdate=${relative%%/*}
    case "$dirdate" in
      [0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]) ;;
      *) continue ;;
    esac
    # Never prune the newest directory even when it is outside the retention
    # window (e.g. backups were paused and restarted).
    [ "$dirdate" -lt "$boundary" ] || continue
    [ "$dirdate" != "$newest_dir" ] || continue
    to_delete="$to_delete $key"
  done <<<"$keys"

  for key in $to_delete; do
    s3 delete --key "$key"
    log "pruned off-host backup object $key (older than $RETENTION_DAYS days)"
  done
}

run_freshness() {
  local state_file last_at object_key last_checksum now_epoch last_epoch age_hours head_out
  state_file="$STATE_DIR/last-success.json"
  LAST_OBJECT_KEY=""
  if [ ! -s "$state_file" ]; then
    fail_freshness no-last-success-state
  fi
  if ! python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$state_file" >/dev/null 2>&1; then
    fail_freshness corrupt-state-file
  fi
  last_at=$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(d.get("last_success_at_utc",""))' "$state_file")
  last_checksum=$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(d.get("checksum_sha256",""))' "$state_file")
  object_key=$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); print(d.get("object_key",""))' "$state_file")
  LAST_OBJECT_KEY="$object_key"
  if [ -z "$last_at" ] || [ -z "$object_key" ]; then
    fail_freshness incomplete-state-file
  fi
  case "$object_key" in
    "$PREFIX/"*) ;;
    *) fail_freshness object-key-outside-configured-prefix ;;
  esac

  if ! last_epoch=$(date -u -d "$last_at" +%s 2>/dev/null); then
    fail_freshness unparseable-last-success-timestamp "$last_at"
  fi
  now_epoch=$(date -u +%s)
  age_hours=$(((now_epoch - last_epoch) / 3600))

  if [ "$FRESHNESS_PROBE" = object ]; then
    if [ -z "$ACCESS_KEY" ] || [ -z "$SECRET_KEY" ]; then
      fail_freshness credentials-missing-for-object-probe "$last_at" "$age_hours"
    fi
    if head_out=$(s3 head --key "$object_key" 2>/dev/null); then
      case "$head_out" in
        "HEAD 200"*) ;;
        *) fail_freshness object-probe-head-failed "$last_at" "$age_hours" ;;
      esac
    else
      fail_freshness object-probe-unreachable "$last_at" "$age_hours"
    fi
  fi

  if [ "$age_hours" -gt "$FRESHNESS_HOURS" ]; then
    fail_freshness stale "$last_at" "$age_hours"
  fi

  printf 'OFFHOST_BACKUP_FRESH=OK last_success_at=%s age_hours=%s threshold_hours=%s object_key=%s checksum=%s\n' \
    "$last_at" "$age_hours" "$FRESHNESS_HOURS" "$object_key" "${last_checksum:-}"
  log "off-host backup freshness OK (age_hours=$age_hours threshold=$FRESHNESS_HOURS)"
}

case "$MODE" in
  backup) run_backup ;;
  freshness) run_freshness ;;
esac