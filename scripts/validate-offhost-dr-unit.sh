#!/usr/bin/env bash
# BS-PROD-012 hermetic disaster-recovery unit tests.
#
# Runs without docker, without PostgreSQL and without a real object store:
#   - database activity is stubbed through OFFHOST_PG_PREFIX (fake pg_dump /
#     psql / pg_restore / rm script);
#   - the S3 surface is scripts/test_offhost_s3.py's in-process fake S3 server
#     (the same one used by the signature-verified wire tests), extended with a
#     per-request corruption control file;
#   - the api container validators are stubbed with a fake `docker` on PATH.
#
# This proves the off-host pipeline logic (schedule artifact layout, metadata,
# retention scoping, freshness alerting and the restore safety guards) against
# a real, signature-verified S3 wire client. The docker-backed integration test
# (validate-offhost-dr.sh) re-proves the flow against real PostgreSQL.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

TMP="$(mktemp -d)"
ROOT="$TMP/root"
BIN="$TMP/bin"
CORRUPT_FILE="$TMP/corrupt.on"
PORT_FILE="$TMP/server.port"
export TMP ROOT BIN CORRUPT_FILE

PASS=0
FAIL=0
SERVER_PID=""
cleanup() {
  if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$TMP"
}
trap cleanup EXIT

# shellcheck disable=SC2155
export S3LIBS="$(pwd)/scripts/test_offhost_s3.py"
export S3CLI="$(pwd)/scripts/offhost-s3.py"
export ACCESS="AKID0123456789TESTKEY"
export SECRET="S3cr3t+S3cr3t/K7MDENG/bPxRfiCYEXAMPLESecret"
export PREFIX="bodysense/postgres"
export BUCKET="testbucket"
export REGION="cn-hangzhou"

report() {
  if [ "$1" = 0 ]; then
    echo "ok   - $2"
    PASS=$((PASS + 1))
  else
    echo "FAIL - $2"
    echo "       $3"
    FAIL=$((FAIL + 1))
  fi
}

mkdir -p "$ROOT" "$BIN"

# --- fake Postgres tool stub -------------------------------------------------
cat > "$BIN/fake-pg" <<'PGSTUB'
#!/usr/bin/env bash
# OFFHOST_PG_PREFIX stub: pg_dump / psql / pg_restore / rm
tool="$1"
shift
case "$tool" in
  pg_dump)
    if [[ "$*" == *--version* ]]; then
      echo "pg_dump (PostgreSQL) 18.0"
    else
      cat "${FAKEPG_DUMP:-/dev/null}"
    fi
    ;;
  psql)
    case "$*" in
      *"SELECT to_regclass('public.schema_migrations') IS NOT NULL"*) echo "${FAKEPG_HAS_MIGRATIONS:-t}" ;;
      *"FROM schema_migrations ORDER BY version DESC LIMIT 1"*) echo "${FAKEPG_SCHEMA:-49:false}" ;;
      *"SELECT 1 FROM pg_database WHERE datname"*) echo "${FAKEPG_DB_EXISTS:-}" ;;
      *) : ;;
    esac
    ;;
  pg_restore)
    [ "${FAKEPG_RESTORE_FAIL:-0}" = 1 ] && exit 1
    ;;
  rm) ;;
  *) ;;
esac
exit 0
PGSTUB
chmod +x "$BIN/fake-pg"

# --- fake docker stub (validators + docker cp + compose ps) ------------------
cat > "$BIN/docker" <<'DOCKSTUB'
#!/usr/bin/env bash
case "$1" in
  cp) exit 0 ;;
  exec) exit "${FAKEPG_VALIDATOR_EXIT:-0}" ;;
  compose) exit 0 ;;
  *) exit 0 ;;
esac
DOCKSTUB
chmod +x "$BIN/docker"
export PATH="$BIN:$PATH"

# --- fake S3 server lifecycle -------------------------------------------------
start_server() {
  rm -f "$PORT_FILE" "$CORRUPT_FILE"
  python3 - "$PORT_FILE" "$CORRUPT_FILE" <<'PY' &
import importlib.util, os, sys
spec = importlib.util.spec_from_file_location("test_offhost_s3", os.environ["S3LIBS"])
T = importlib.util.module_from_spec(spec)
sys.modules["test_offhost_s3"] = T
spec.loader.exec_module(T)
T.FakeS3Handler.store = {}
Handler = type("CorruptibleHandler", (T.FakeS3Handler,), {
    "corrupt_file": sys.argv[2],
    "corrupt_suffix": ".dump",
})
srv = T.FakeServer(handler=Handler)
host, port = srv.server.server_address
with open(sys.argv[1], "w") as fh:
    fh.write("http://127.0.0.1:%d" % port)
srv.thread.start()
srv.server.serve_forever()
PY
  SERVER_PID=$!
  for _ in $(seq 1 50); do
    [ -s "$PORT_FILE" ] && break
    sleep 0.1
  done
  [ -s "$PORT_FILE" ] || { echo "fake S3 server did not start"; exit 1; }
  ENDPOINT=$(cat "$PORT_FILE")
}

stop_server() {
  if kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  SERVER_PID=""
}

set_corrupt() { : > "$CORRUPT_FILE"; }
unset_corrupt() { rm -f "$CORRUPT_FILE"; }

# --- helpers -------------------------------------------------------------------
write_env() {
  mkdir -p "$ROOT/docker"
  cat > "$ROOT/.env.production" <<ENV
OFFHOST_BACKUP_ENABLED=true
OFFHOST_BACKUP_BUCKET=$BUCKET
OFFHOST_BACKUP_ENDPOINT=$ENDPOINT
OFFHOST_BACKUP_REGION=$REGION
OFFHOST_BACKUP_PREFIX=$PREFIX
OFFHOST_BACKUP_URL_STYLE=path
OFFHOST_BACKUP_RETENTION_DAYS=30
OFFHOST_BACKUP_FRESHNESS_HOURS=30
OFFHOST_BACKUP_FRESHNESS_PROBE=object
DB_USER=bodysense
DB_NAME=bodysense
ENV
  umask 077
  cat > "$ROOT/.env.production.local" <<ENV
OFFHOST_BACKUP_ACCESS_KEY=$ACCESS
OFFHOST_BACKUP_SECRET_KEY=$SECRET
DB_PASSWORD=0123456789abcdef
ENV
  chmod 600 "$ROOT/.env.production.local"
  chmod 644 "$ROOT/.env.production"
  umask 022
}

run_backup() {
  FAKEPG_DUMP="$(fake_dump)" BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" \
    bash scripts/production-offhost-backup.sh --backup
}

run_freshness() {
  FAKEPG_DUMP="$(fake_dump)" BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" \
    bash scripts/production-offhost-backup.sh --check-freshness
}

fake_dump() {
  local f="$TMP/fake.dump"
  printf 'custom\nformat\narchive\nfixture\n' > "$f"
  printf '%s' "$f"
}

s3put() {
  python3 "$S3CLI" put --endpoint "$ENDPOINT" --bucket "$BUCKET" --region "$REGION" \
    --access-key "$ACCESS" --secret-key "$SECRET" --key "$1" --file "$2"
}

last_key() {
  python3 -c 'import json;print(json.load(open("'"$ROOT"'/.offhost-state/last-success.json"))["object_key"])'
}

count_objects() {
  python3 "$S3CLI" list --endpoint "$ENDPOINT" --bucket "$BUCKET" --region "$REGION" \
    --access-key "$ACCESS" --secret-key "$SECRET" --prefix "$1" 2>/dev/null \
    | awk -F'\t' 'NF { n++ } END { print n+0 }'
}

# ==============================================================================
# 1. backup uploads three verified signed objects + freshness state, freshness OK
# ==============================================================================
start_server
write_env
run_backup > "$TMP/backup1.out"
[ -f "$ROOT/.offhost-state/last-success.json" ] || { echo "no last-success.json after backup"; exit 1; }
OBJKEY=$(last_key)
n=$(count_objects "$PREFIX/")
meta_ok=bad
python3 "$S3CLI" get --endpoint "$ENDPOINT" --bucket "$BUCKET" --region "$REGION" \
  --access-key "$ACCESS" --secret-key "$SECRET" --key "$OBJKEY.meta.json" --file "$TMP/m1.json"
meta_ok=$(python3 -c 'import json;d=json.load(open("'"$TMP"'/m1.json"));print("ok" if d["schema_revision"]=="49:false" and d["backup_kind"]=="offhost-postgres" and d["archive_format"]=="custom" else "bad")')
if [ "$n" -eq 3 ] && [ "$meta_ok" = ok ]; then
  report 0 "backup uploads exactly 3 verified objects with correct metadata"
else
  report 1 "backup uploads exactly 3 verified objects with correct metadata" "objects=$n meta=$meta_ok"
fi

out=$(run_freshness 2>&1) && rc=0 || rc=$?
if [ $rc -eq 0 ] && [[ "$out" == *OFFHOST_BACKUP_FRESH=OK* ]]; then
  report 0 "freshness check passes right after the backup"
else
  report 1 "freshness check passes right after the backup" "rc=$rc out=$out"
fi

# ==============================================================================
# 2. retention prunes old directories but never the newest
# ==============================================================================
old=$(fake_dump)
s3put "$PREFIX/20260701/bodysense-postgres-20260701T000000Z.dump" "$old" >/dev/null
s3put "$PREFIX/20260701/bodysense-postgres-20260701T000000Z.dump.sha256" "$old" >/dev/null
s3put "$PREFIX/20260701/bodysense-postgres-20260701T000000Z.dump.meta.json" "$old" >/dev/null
s3put "$PREFIX/20990101/bodysense-postgres-20990101T000000Z.dump" "$old" >/dev/null
run_backup > "$TMP/backup2.out"
after_prune=$(count_objects "$PREFIX/")
remaining_old=$(count_objects "$PREFIX/20260701/")
remaining_future=$(count_objects "$PREFIX/20990101/")
keep=$(count_objects "$(python3 -c 'import json,sys;print(json.load(open("'"$ROOT"'/.offhost-state/last-success.json"))["object_key"].rsplit("/",1)[0]+"/")')")
if [ "$after_prune" -ge 4 ] && [ "$remaining_old" -eq 0 ] && [ "$remaining_future" -eq 1 ] \
  && [ "$keep" -ge 3 ]; then
  report 0 "retention prunes old objects and keeps the newest directory"
else
  report 1 "retention prunes old objects and keeps the newest directory" \
    "total=$after_prune old=$remaining_old newest=$remaining_future keep_dir=$keep"
fi

# ==============================================================================
# 3. freshness failures are loud and non-zero
# ==============================================================================
python3 - "$ROOT/.offhost-state/last-success.json" <<'PY'
import json, sys, datetime
path = sys.argv[1]
d = json.load(open(path))
old = (datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(days=40)).strftime("%Y-%m-%dT%H:%M:%SZ")
d["last_success_at_utc"] = old
json.dump(d, open(path, "w"))
PY
out=$(run_freshness 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *OFFHOST_BACKUP_FRESH=FAIL* ]] && [[ "$out" == *reason=stale* ]]; then
  report 0 "stale backup triggers a freshness alert with non-zero exit"
else
  report 1 "stale backup triggers a freshness alert with non-zero exit" "rc=$rc out=$out"
fi
rm -f "$ROOT/.offhost-state/last-success.json"
out=$(run_freshness 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *OFFHOST_BACKUP_FRESH=FAIL* ]] && [[ "$out" == *reason=no-last-success-state* ]]; then
  report 0 "missing freshness state triggers a freshness alert"
else
  report 1 "missing freshness state triggers a freshness alert" "rc=$rc out=$out"
fi

# ==============================================================================
# 4. corrupted remote archive is detected during backup verification
# ==============================================================================
stop_server
start_server          # clean store
write_env
set_corrupt
out=$(run_backup 2>&1) && rc=0 || rc=$?
unset_corrupt
if [ $rc -ne 0 ] && [[ "$out" == *"re-downloaded archive checksum does not match"* ]] \
  && [ ! -f "$ROOT/.offhost-state/last-success.json" ]; then
  report 0 "backup verification fails loudly when the remote archive is corrupted"
else
  report 1 "backup verification fails loudly when the remote archive is corrupted" "rc=$rc out=$out"
fi

# clean slate for the restore group
stop_server
start_server
write_env
run_backup > /dev/null
OBJKEY=$(last_key)

# ==============================================================================
# 5. restore safety guards
# ==============================================================================
RESTORE="scripts/restore-production-backup.sh"
run_restore_guard() {
  local expect="$1"; shift
  local out rc
  out=$(BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" \
    OFFHOST_PGCONTAINER_ID=pg1 bash "$RESTORE" "$@" 2>&1) && rc=0 || rc=$?
  if [ $rc -ne 0 ] && [[ "$out" == *"$expect"* ]]; then
    report 0 "guard: $expect"
  else
    report 1 "guard: $expect" "rc=$rc out=$out"
  fi
}
run_restore_guard "refusing to restore into the production database" \
  --object-key "$PREFIX/20260824T000000Z/bodysense-postgres-20260824T000000Z.dump" \
  --target-db bodysense --target-project drill --confirm-target-isolated=yes
run_restore_guard 'must not be "bodysense"' \
  --object-key "$PREFIX/20260824T000000Z/bodysense-postgres-20260824T000000Z.dump" \
  --target-db drill_db --target-project bodysense --confirm-target-isolated=yes
run_restore_guard "--confirm-target-isolated=yes" \
  --object-key "$PREFIX/20260824T000000Z/bodysense-postgres-20260824T000000Z.dump" \
  --target-db drill_db --target-project drill
run_restore_guard "outside the configured" \
  --object-key "other/prefix/20260824T000000Z/x.dump" \
  --target-db drill_db --target-project drill --confirm-target-isolated=yes

# ==============================================================================
# 6. restore happy path with validator invocations
# ==============================================================================
FAKEPG_DB_EXISTS="" FAKEPG_SCHEMA="49:false"
out=$(BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" OFFHOST_PGCONTAINER_ID=pg1 \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db drill_db --target-project drill \
  --confirm-target-isolated=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
if [ $rc -eq 0 ] && [[ "$out" == *RESTORE_RESULT=PASS* ]] \
  && [[ "$out" == *"restored schema revision matches backup metadata"* ]]; then
  report 0 "restore drill restores the verified archive and runs validators"
else
  report 1 "restore drill restores the verified archive and runs validators" "rc=$rc out=$out"
fi

# ==============================================================================
# 7. restore refuses a target database that already exists
# ==============================================================================
out=$(FAKEPG_DB_EXISTS=1 BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" \
  OFFHOST_PGCONTAINER_ID=pg1 \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db used_db --target-project drill \
  --confirm-target-isolated=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"already exists"* ]]; then
  report 0 "restore refuses an existing target database"
else
  report 1 "restore refuses an existing target database" "rc=$rc out=$out"
fi

# ==============================================================================
# 8. restore fails when the downloaded archive checksum is corrupted in transit
# ==============================================================================
set_corrupt
out=$(FAKEPG_DB_EXISTS="" BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" \
  OFFHOST_PGCONTAINER_ID=pg1 \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db drill_db2 --target-project drill \
  --confirm-target-isolated=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
unset_corrupt
if [ $rc -ne 0 ] && [[ "$out" == *"SHA-256 mismatch"* ]]; then
  report 0 "restore fails when the downloaded archive checksum mismatches"
else
  report 1 "restore fails when the downloaded archive checksum mismatches" "rc=$rc out=$out"
fi

echo
echo "offhost DR unit tests: PASS=$PASS FAIL=$FAIL"
[ "$FAIL" -eq 0 ]