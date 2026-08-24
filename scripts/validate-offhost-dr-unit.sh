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
# retention scoping, freshness alerting, env-only credential handling, the
# restore isolation/enforcement guards, the SHA-256 sidecar verification and the
# resolved api-container validation path) against a real, signature-verified S3
# wire client. The docker-backed integration test (validate-offhost-dr.sh)
# re-proves the flow against real PostgreSQL.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

TMP="$(mktemp -d)"
ROOT="$TMP/root"
BIN="$TMP/bin"
CORRUPT_FILE="$TMP/corrupt.on"
LIST_FAIL_FILE="$TMP/listfail.on"
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

# --- fake docker stub (validators via --env-file + docker cp + compose ps +
#     docker inspect for the restore isolation proof) ------------------------
# FAKEDOCKER_INSPECT_DIR/<container>.json overrides the default per-container
# inspect JSON; the default treats every container as a running, disposable
# restore candidate on its own network (Id = container name).  Tests write a
# file to force a refusal scenario (shared network, missing label, non-running,
# production Compose membership, equal IDs).
cat > "$BIN/docker" <<'DOCKSTUB'
#!/usr/bin/env bash
{
  printf '%s\n' "$*"
} >> "${DOCKER_LOG:-/dev/null}"
case "$1" in
  cp) exit 0 ;;
  exec)
    # Emulate `docker exec --env-file <file> ...`: the real CLI reads the file
    # and sends its variables to the daemon, so the secret value must not appear
    # in argv. Record the file contents for the argv-leak proof.
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "--env-file" ]; then
        shift
        if [ -n "${1:-}" ] && [ -f "$1" ]; then
          {
            printf 'ENV_FILE %s\n' "$1"
            sed 's/^/  /' "$1"
          } >> "${DOCKER_ENVLOG:-/dev/null}"
        fi
        shift
        continue
      fi
      shift
    done
    exit "${FAKEPG_VALIDATOR_EXIT:-0}" ;;
  inspect)
    name="${2:-}"
    if [ -n "${FAKEDOCKER_INSPECT_DIR:-}" ]; then
      if [ -f "$FAKEDOCKER_INSPECT_DIR/$name.json" ]; then
        cat "$FAKEDOCKER_INSPECT_DIR/$name.json"
      else
        printf '[{"Id":"%s","State":{"Running":true},"Config":{"Labels":{"bodysense.restore-project":"drill","bodysense.disposable-restore":"yes"}},"NetworkSettings":{"Networks":{"%s-net":{}}}}]' \
          "$name" "$name"
      fi
      exit 0
    fi
    echo '[]'
    exit 0 ;;
  compose) exit 0 ;;
  *) exit 0 ;;
esac
DOCKSTUB
chmod +x "$BIN/docker"
export PATH="$BIN:$PATH"
FAKEDOCKER_INSPECT_DIR="$TMP/docker-inspect"
mkdir -p "$FAKEDOCKER_INSPECT_DIR"
DOCKER_LOG="$TMP/docker.log"
DOCKER_ENVLOG="$TMP/docker-env.log"
export FAKEDOCKER_INSPECT_DIR DOCKER_LOG DOCKER_ENVLOG

# write_inspect <container> <running> <labels-as-json> <networks-as-json>
write_inspect() {
  local name="$1" running="$2" labels="$3" networks="$4"
  cat > "$FAKEDOCKER_INSPECT_DIR/$name.json" <<JSON
[{"Id":"$name","State":{"Running":$running},"Config":{"Labels":$labels},"NetworkSettings":{"Networks":$networks}}]
JSON
}

# --- fake S3 server lifecycle -------------------------------------------------
start_server() {
  rm -f "$PORT_FILE" "$CORRUPT_FILE" "$LIST_FAIL_FILE"
  python3 - "$PORT_FILE" "$CORRUPT_FILE" "$LIST_FAIL_FILE" <<'PY' &
import importlib.util, os, sys
spec = importlib.util.spec_from_file_location("test_offhost_s3", os.environ["S3LIBS"])
T = importlib.util.module_from_spec(spec)
sys.modules["test_offhost_s3"] = T
spec.loader.exec_module(T)
T.FakeS3Handler.store = {}
_orig_list = T.FakeS3Handler._list
def _guarded_list(self, query_raw):
    # A list-failure control file: when it holds a prefix, any ListObjectsV2
    # request for exactly that prefix fails with a 500 so tests can prove the
    # retention listing path fails loudly instead of being swallowed.
    lff = getattr(self, "list_fail_file", None)
    if lff and os.path.exists(lff):
        params = dict(T.urllib.parse.parse_qsl(query_raw))
        if params.get("prefix", "") == open(lff).read().strip():
            self._reject(500, "ListObjectsFailed", "simulated list failure")
            return
    return _orig_list(self, query_raw)
Handler = type("CorruptibleHandler", (T.FakeS3Handler,), {
    "corrupt_file": sys.argv[2],
    "corrupt_suffix": ".dump",
    "list_fail_file": sys.argv[3],
    "_list": _guarded_list,
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

rewrite_state_age() {
  # Rewrite last_success_at_utc to now - $1 seconds (negative = future-dated).
  local seconds="$1"
  python3 - "$ROOT/.offhost-state/last-success.json" "$seconds" <<'PY'
import json, sys, datetime
path, seconds = sys.argv[1], int(sys.argv[2])
d = json.load(open(path))
t = datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(seconds=seconds)
d["last_success_at_utc"] = t.strftime("%Y-%m-%dT%H:%M:%SZ")
json.dump(d, open(path, "w"))
PY
}

fake_dump() {
  local f="$TMP/fake.dump"
  printf 'custom\nformat\narchive\nfixture\n' > "$f"
  printf '%s' "$f"
}

s3put() {
  OFFHOST_BACKUP_ACCESS_KEY="$ACCESS" OFFHOST_BACKUP_SECRET_KEY="$SECRET" \
    python3 "$S3CLI" put --endpoint "$ENDPOINT" --bucket "$BUCKET" --region "$REGION" \
      --key "$1" --file "$2"
}

last_key() {
  python3 -c 'import json;print(json.load(open("'"$ROOT"'/.offhost-state/last-success.json"))["object_key"])'
}

count_objects() {
  OFFHOST_BACKUP_ACCESS_KEY="$ACCESS" OFFHOST_BACKUP_SECRET_KEY="$SECRET" \
    python3 "$S3CLI" list --endpoint "$ENDPOINT" --bucket "$BUCKET" --region "$REGION" \
      --prefix "$1" 2>/dev/null \
      | awk -F'\t' 'NF { n++ } END { print n+0 }'
}

meta_get() {
  OFFHOST_BACKUP_ACCESS_KEY="$ACCESS" OFFHOST_BACKUP_SECRET_KEY="$SECRET" \
    python3 "$S3CLI" get --endpoint "$ENDPOINT" --bucket "$BUCKET" --region "$REGION" \
      --key "$1" --file "$2"
}

sha256sidecar_rewrite() {
  # Rewrite the .sha256 sidecar object for $OBJKEY using a sed expression.
  local key="$1" expression="$2" f="$TMP/sidecar-work"
  OFFHOST_BACKUP_ACCESS_KEY="$ACCESS" OFFHOST_BACKUP_SECRET_KEY="$SECRET" \
    python3 "$S3CLI" get --endpoint "$ENDPOINT" --bucket "$BUCKET" --region "$REGION" \
      --key "$key.sha256" --file "$f" >/dev/null
  sed -e "$expression" "$f" > "$f.new"
  s3put "$key.sha256" "$f.new" >/dev/null
  rm -f "$f" "$f.new"
}

sidecar_restore() {
  # Put the correct sidecar back: <metadata checksum> + two spaces + the object
  # basename, which is exactly what production-offhost-backup.sh writes.
  local key="$1" chk f="$TMP/sidecar-fix"
  meta_get "$key.meta.json" "$TMP/s1.json"
  chk=$(python3 -c 'import json;print(json.load(open("'"$TMP"'/s1.json"))["checksum_sha256"])')
  printf '%s  %s\n' "$chk" "${key##*/}" > "$f"
  s3put "$key.sha256" "$f" >/dev/null
  rm -f "$f" "$TMP/s1.json"
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
meta_get "$OBJKEY.meta.json" "$TMP/m1.json"
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
# 3b. freshness policy is enforced in whole seconds and rejects future dates
#     (a 30h59m-old backup used to truncate to a healthy "30h"; future-dated
#     state used to produce a negative age that was accepted as fresh)
# ==============================================================================
run_backup > /dev/null   # restore a valid last-success state
rewrite_state_age 106200              # 29h30m: below the 30h policy
out=$(run_freshness 2>&1) && rc=0 || rc=$?
if [ $rc -eq 0 ] && [[ "$out" == *OFFHOST_BACKUP_FRESH=OK* ]]; then
  report 0 "freshness below the threshold is OK"
else
  report 1 "freshness below the threshold is OK" "rc=$rc out=$out"
fi
rewrite_state_age 111540              # 30h59m: truncates to 30h but exceeds policy
out=$(run_freshness 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *reason=stale* ]]; then
  report 0 "freshness fails when age exceeds policy even below the next whole hour (no truncation)"
else
  report 1 "freshness fails when age exceeds policy even below the next whole hour (no truncation)" "rc=$rc out=$out"
fi
rewrite_state_age -3600               # future-dated last-success
out=$(run_freshness 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *reason=future-dated-last-success* ]]; then
  report 0 "future-dated last-success is rejected, never treated as fresh"
else
  report 1 "future-dated last-success is rejected, never treated as fresh" "rc=$rc out=$out"
fi

# ==============================================================================
# 4. corrupted remote archive is detected during backup verification
# ==============================================================================
stop_server
start_server          # clean store
write_env
rm -f "$ROOT/.offhost-state/last-success.json"
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
# 4b. a retention-listing failure must abort the backup (apply-or-fail) and must
#     never record last-success.json, otherwise unbounded sensitive retention is
#     possible while freshness keeps reporting OK
# ==============================================================================
BEFORE_KEY=$(last_key)
printf '%s' "$PREFIX/" > "$LIST_FAIL_FILE"
out=$(run_backup 2>&1) && rc=0 || rc=$?
rm -f "$LIST_FAIL_FILE"
if [ $rc -ne 0 ] && [[ "$out" == *"retention listing failed"* ]] && [ "$(last_key)" = "$BEFORE_KEY" ]; then
  report 0 "retention-listing failure aborts the backup and keeps the previous last-success"
else
  report 1 "retention-listing failure aborts the backup and keeps the previous last-success" "rc=$rc out=$out"
fi

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
  --target-db bodysense --target-project drill --restore-pg container:restore-pg \
  --confirm-target-isolated=yes
run_restore_guard 'must not be "bodysense"' \
  --object-key "$PREFIX/20260824T000000Z/bodysense-postgres-20260824T000000Z.dump" \
  --target-db drill_db --target-project bodysense --restore-pg container:restore-pg \
  --confirm-target-isolated=yes
run_restore_guard "--confirm-target-isolated=yes" \
  --object-key "$PREFIX/20260824T000000Z/bodysense-postgres-20260824T000000Z.dump" \
  --target-db drill_db --target-project drill --restore-pg container:restore-pg
run_restore_guard "outside the configured" \
  --object-key "other/prefix/20260824T000000Z/x.dump" \
  --target-db drill_db --target-project drill --restore-pg container:restore-pg \
  --confirm-target-isolated=yes
run_restore_guard '--restore-pg container:<id|name> is required' \
  --object-key "$PREFIX/20260824T000000Z/bodysense-postgres-20260824T000000Z.dump" \
  --target-db drill_db --target-project drill --confirm-target-isolated=yes
run_restore_guard 'must be container:<id|name>' \
  --object-key "$PREFIX/20260824T000000Z/bodysense-postgres-20260824T000000Z.dump" \
  --target-db drill_db --target-project drill \
  --restore-pg tcp:127.0.0.1:5433 --confirm-target-isolated=yes
run_restore_guard "refusing to restore into the live production postgres container/endpoint" \
  --object-key "$PREFIX/20260824T000000Z/bodysense-postgres-20260824T000000Z.dump" \
  --target-db drill_db --target-project drill --restore-pg container:pg1 \
  --confirm-target-isolated=yes

# 5b. the isolation proof (docker inspect) must refuse any target that is not a
#     provably disposable drill container:
#     right network set, labels, running state, exclusivity from the production
#     container/endpoint and from the production Compose project.
write_inspect restore-shared-net true \
  '{"bodysense.restore-project":"drill","bodysense.disposable-restore":"yes"}' \
  '{"pg1-net":{}}'
run_restore_guard "attached to the production postgres network" \
  --object-key "$OBJKEY" --target-db drill_net --target-project drill \
  --restore-pg container:restore-shared-net --confirm-target-isolated=yes
write_inspect restore-wrong-project true \
  '{"bodysense.restore-project":"staging","bodysense.disposable-restore":"yes"}' \
  '{"restore-wrong-project-net":{}}'
run_restore_guard "does not declare bodysense.restore-project=drill" \
  --object-key "$OBJKEY" --target-db drill_proj --target-project drill \
  --restore-pg container:restore-wrong-project --confirm-target-isolated=yes
write_inspect restore-not-disposable true \
  '{"bodysense.restore-project":"drill","bodysense.disposable-restore":"no"}' \
  '{"restore-not-disposable-net":{}}'
run_restore_guard "refusing a non-disposable target" \
  --object-key "$OBJKEY" --target-db drill_disp --target-project drill \
  --restore-pg container:restore-not-disposable --confirm-target-isolated=yes
write_inspect restore-stopped false \
  '{"bodysense.restore-project":"drill","bodysense.disposable-restore":"yes"}' \
  '{"restore-stopped-net":{}}'
run_restore_guard "is not running" \
  --object-key "$OBJKEY" --target-db drill_run --target-project drill \
  --restore-pg container:restore-stopped --confirm-target-isolated=yes
write_inspect pg1 true \
  '{"com.docker.compose.project":"docker"}' \
  '{"pg1-net":{}}'
write_inspect restore-prod-compose true \
  '{"com.docker.compose.project":"docker","bodysense.restore-project":"drill","bodysense.disposable-restore":"yes"}' \
  '{"restore-prod-compose-net":{}}'
run_restore_guard "belongs to the production compose project 'docker'" \
  --object-key "$OBJKEY" --target-db drill_compose --target-project drill \
  --restore-pg container:restore-prod-compose --confirm-target-isolated=yes
rm -f "$FAKEDOCKER_INSPECT_DIR/pg1.json"

# ==============================================================================
# 6. restore happy path with validator invocations
# ==============================================================================
FAKEPG_DB_EXISTS="" FAKEPG_SCHEMA="49:false"
: > "$DOCKER_LOG"
: > "$DOCKER_ENVLOG"
out=$(BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" OFFHOST_PGCONTAINER_ID=pg1 \
  OFFHOST_API_CONTAINER=fake-api-1 DOCKER_LOG="$DOCKER_LOG" DOCKER_ENVLOG="$DOCKER_ENVLOG" \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db drill_db --target-project drill \
  --restore-pg container:restore-pg --confirm-target-isolated=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
if [ $rc -eq 0 ] && [[ "$out" == *RESTORE_RESULT=PASS* ]] \
  && [[ "$out" == *"restored schema revision matches backup metadata"* ]]; then
  report 0 "restore drill restores the verified archive and runs validators"
else
  report 1 "restore drill restores the verified archive and runs validators" "rc=$rc out=$out"
fi
# Production Compose names the api container "<project>-api-1" (docker-api-1),
# never a literal "api"; the restore path must exec validators on the resolved
# container, and the OFFHOST_API_CONTAINER seam must win over any lookup.
if grep -q '^exec --env-file [^ ]* fake-api-1 /app/validators/migration-validator ' "$DOCKER_LOG" \
  && grep -q '^exec --env-file [^ ]* fake-api-1 /app/validators/domain-validator ' "$DOCKER_LOG" \
  && ! grep -q '^exec --env-file [^ ]* api /app/validators/' "$DOCKER_LOG"; then
  report 0 "validators exec via the resolved api container (fake-api-1), never a literal 'api'"
else
  report 1 "validators exec via the resolved api container (fake-api-1), never a literal 'api'" "docker_log=$(tr '\n' '|' < "$DOCKER_LOG")"
fi
# The database password (fixed test secret 0123456789abcdef) must never appear
# on any recorded docker argv (host side) or psql/fake-pg argv; it may only be
# present in the --env-file the stub read on behalf of the daemon.  The DSN in
# -database-url must carry no password at all.
if ! grep -q '0123456789abcdef' "$DOCKER_LOG" \
  && grep -q 'postgres://bodysense@restore-pg:5432/drill_db?sslmode=disable' "$DOCKER_LOG" \
  && grep -q 'PGPASSWORD=0123456789abcdef' "$DOCKER_ENVLOG"; then
  report 0 "database password never appears in argv; delivered only via PGPASSWORD in an --env-file"
else
  report 1 "database password never appears in argv; delivered only via PGPASSWORD in an --env-file" \
    "docker_log=$(tr '\n' '|' < "$DOCKER_LOG") env_log=$(tr '\n' '|' < "$DOCKER_ENVLOG")"
fi
# Without an explicit OFFHOST_API_CONTAINER the restore path must fall back to
# Compose's default "<project>-api-1" naming (docker-api-1 for project "docker"),
# which is exactly what production runs — the original P1 blocker.
: > "$DOCKER_LOG"
: > "$DOCKER_ENVLOG"
out=$(BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" OFFHOST_PGCONTAINER_ID=pg1 \
  DOCKER_LOG="$DOCKER_LOG" DOCKER_ENVLOG="$DOCKER_ENVLOG" \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db drill_db_b --target-project drill \
  --restore-pg container:restore-pg --confirm-target-isolated=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
if [ $rc -eq 0 ] && grep -q '^exec --env-file [^ ]* docker-api-1 /app/validators/migration-validator ' "$DOCKER_LOG"; then
  report 0 "without OFFHOST_API_CONTAINER the restore uses Compose's '<project>-api-1' naming (docker-api-1)"
else
  report 1 "without OFFHOST_API_CONTAINER the restore uses Compose's '<project>-api-1' naming (docker-api-1)" \
    "rc=$rc docker_log=$(tr '\n' '|' < "$DOCKER_LOG")"
fi

# ==============================================================================
# 7. restore refuses a target database that already exists
# ==============================================================================
out=$(FAKEPG_DB_EXISTS=1 BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" \
  OFFHOST_PGCONTAINER_ID=pg1 \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db used_db --target-project drill \
  --restore-pg container:restore-pg --confirm-target-isolated=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
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
  --restore-pg container:restore-pg --confirm-target-isolated=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
unset_corrupt
if [ $rc -ne 0 ] && [[ "$out" == *"does not match the checksum sidecar"* ]]; then
  report 0 "restore fails when the downloaded archive checksum mismatches"
else
  report 1 "restore fails when the downloaded archive checksum mismatches" "rc=$rc out=$out"
fi

# ==============================================================================
# 9. restore refuses checksum sidecars that are syntactically invalid
# ==============================================================================
sha256sidecar_rewrite "$OBJKEY" 's/.*/not-a-sha256-sidecar!/
s/^[a-z0-9-]*$/garbage/'
out=$(FAKEPG_DB_EXISTS="" BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" \
  OFFHOST_PGCONTAINER_ID=pg1 \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db drill_db3 --target-project drill \
  --restore-pg container:restore-pg --confirm-target-isolated=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"not in '<sha256>  <filename>' format"* ]]; then
  report 0 "restore refuses a checksum sidecar that is not in '<sha256>  <filename>' format"
else
  report 1 "restore refuses a checksum sidecar that is not in '<sha256>  <filename>' format" "rc=$rc out=$out"
fi
sidecar_restore "$OBJKEY"

# ==============================================================================
# 10. restore refuses a checksum sidecar whose attested filename is wrong
# ==============================================================================
sha256sidecar_rewrite "$OBJKEY" 's/  .*/  wrong-object-name.dump/'
out=$(FAKEPG_DB_EXISTS="" BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" \
  OFFHOST_PGCONTAINER_ID=pg1 \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db drill_db4 --target-project drill \
  --restore-pg container:restore-pg --confirm-target-isolated=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"does not match object key basename"* ]]; then
  report 0 "restore refuses a checksum sidecar attesting the wrong object name"
else
  report 1 "restore refuses a checksum sidecar attesting the wrong object name" "rc=$rc out=$out"
fi
sidecar_restore "$OBJKEY"

# ==============================================================================
# 11. restore refuses a checksum sidecar whose digest disagrees with metadata
# ==============================================================================
sha256sidecar_rewrite "$OBJKEY" 's/^[0-9a-f]\{64\}/ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff/'
out=$(FAKEPG_DB_EXISTS="" BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" \
  OFFHOST_PGCONTAINER_ID=pg1 \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db drill_db5 --target-project drill \
  --restore-pg container:restore-pg --confirm-target-isolated=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"does not match metadata checksum_sha256"* ]]; then
  report 0 "restore refuses a checksum sidecar whose digest disagrees with backup metadata"
else
  report 1 "restore refuses a checksum sidecar whose digest disagrees with backup metadata" "rc=$rc out=$out"
fi

# ==============================================================================
# 12. offhost credentials are env-only: no operator script passes them as CLI args
# ==============================================================================
if grep -lE -- '--(access-key|secret-key)' scripts/*.sh >/dev/null 2>&1; then
  report 1 "offhost credentials are never passed through process argv (argv-leak guard)"
elif [ "$(grep -cE 'OFFHOST_BACKUP_ACCESS_KEY=' scripts/production-offhost-backup.sh scripts/restore-production-backup.sh 2>/dev/null | awk -F: '{s+=$2} END {print s+0}')" -lt 2 ]; then
  report 1 "offhost credentials are never passed through process argv (argv-leak guard)" "missing env-based credential wiring"
else
  report 0 "offhost credentials are never passed through process argv (argv-leak guard)"
fi

# ==============================================================================
# 13. restore never hard-codes a literal 'api' container name for the validators
# ==============================================================================
if grep -nE 'docker[[:space:]]+exec[[:space:]]+api([[:space:]]|$)' "$RESTORE" >/dev/null 2>&1; then
  report 1 "restore runs validators on the resolved api container, never a literal 'api'" \
    "found a hard-coded 'docker exec api' in scripts/restore-production-backup.sh"
else
  report 0 "restore runs validators on the resolved api container, never a literal 'api'"
fi

# ==============================================================================
# 14. systemd timers pin the timezone inside OnCalendar (a bare Timezone= line in
#     [Timer] is non-standard and is ignored/overridden by the system); every
#     off-host timer must carry the timezone embedded in its schedule expression
# ==============================================================================
tz_ok=0
tz_fail=""
for timer in deploy/systemd/bodysense-offhost-backup.timer deploy/systemd/bodysense-offhost-freshness.timer; do
  if [ ! -f "$timer" ]; then
    tz_fail="$tz_fail missing $timer;"
    continue
  fi
  if grep -Eq '^[[:space:]]*Timezone=' "$timer"; then
    tz_fail="$tz_fail $timer uses non-standard [Timer] Timezone=;"
  fi
  schedules=$(sed -n '/^\[Timer\]/,/^\[/p' "$timer" | sed -nE 's/^[[:space:]]*OnCalendar=//p')
  if [ -z "$schedules" ]; then
    tz_fail="$tz_fail $timer has no OnCalendar=;"
  fi
  while IFS= read -r sched; do
    case "$sched" in
      *"Asia/Shanghai"|*"UTC"|*"Etc/UTC") ;;
      *) tz_fail="$tz_fail $timer OnCalendar='$sched' has no explicit timezone;"
    esac
  done <<EOF
$schedules
EOF
done
if [ -n "$tz_fail" ]; then
  report 1 "systemd timers pin the timezone inside OnCalendar expressions" "$tz_fail"
else
  report 0 "systemd timers pin the timezone inside OnCalendar expressions"
fi

echo
echo "offhost DR unit tests: PASS=$PASS FAIL=$FAIL"
[ "$FAIL" -eq 0 ]