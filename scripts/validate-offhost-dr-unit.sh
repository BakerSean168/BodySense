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
PUBLIC_ACL_FILE="$TMP/public-acl.on"
ACL_UNREADABLE_FILE="$TMP/acl-unreadable.on"
POLICY_PUBLIC_FILE="$TMP/policy-public.on"
POLICY_UNAVAILABLE_FILE="$TMP/policy-unavailable.on"
POLICY_UNSUPPORTED_FILE="$TMP/policy-unsupported.on"
PUT_LOG_FILE="$TMP/put.log"
export TMP ROOT BIN CORRUPT_FILE PUBLIC_ACL_FILE ACL_UNREADABLE_FILE
export POLICY_PUBLIC_FILE POLICY_UNAVAILABLE_FILE POLICY_UNSUPPORTED_FILE PUT_LOG_FILE

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
      *"SELECT to_regclass('public.schema_migrations') IS NOT NULL"*)
        [ "${FAKEPG_TOREGCLASS_FAIL:-0}" = 1 ] && exit 1
        echo "${FAKEPG_HAS_MIGRATIONS:-t}" ;;
      *"FROM schema_migrations ORDER BY version DESC LIMIT 1"*)
        [ "${FAKEPG_SCHEMA_FAIL:-0}" = 1 ] && exit 1
        echo "${FAKEPG_SCHEMA:-49:false}" ;;
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

# --- fake docker stub (validators via `docker run --rm` disposable containers +
#     docker network inspect membership + docker cp + compose ps + docker inspect
#     for the restore isolation proof) ------------------------------------------
# FAKEDOCKER_INSPECT_DIR/<container>.json overrides the default per-container
# inspect JSON; the default treats every container as a running, disposable
# restore candidate on its own network (Id = container name, Image =
# example-registry/bodysense-api:test).  Tests write a file to force a refusal
# scenario (shared network, missing label, non-running, production Compose
# membership, equal IDs, an api container whose Config.Image is the validator
# image source).  FAKEDOCKER_NETWORK_DIR/<network>.json overrides the
# `docker network inspect <network>` Containers membership; the default is an
# empty membership set, and a file can add a "production container on the drill
# network" member the membership guard must refuse.
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
  run)
    # Emulate `docker run --rm --network <net> -l ... --env-file <file>
    # --entrypoint <bin> <image> <args>`: same --env-file recording as `exec`
    # (secret never in argv), and the container just "runs the validator".
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
  network)
    if [ "${2:-}" = inspect ]; then
      name="${3:-}"
      if [ -n "${FAKEDOCKER_NETWORK_DIR:-}" ] && [ -n "$name" ] && [ -f "$FAKEDOCKER_NETWORK_DIR/$name.json" ]; then
        cat "$FAKEDOCKER_NETWORK_DIR/$name.json"
      elif [ -n "${FAKEDOCKER_INSPECT_DIR:-}" ]; then
        # NO explicit membership override: model the REAL docker network by
        # scanning the inspect store for every container whose
        # NetworkSettings.Networks declares this network and emit those
        # containers as members.  This reflects reality (the disposable restore
        # container is attached to its declared drill network, so it appears as
        # a member) and lets the stricter "membership must equal exactly the
        # restore container" guard be exercised hermetically.
        RESTORE_MEMBERS="$(python3 - "$FAKEDOCKER_INSPECT_DIR" "$name" <<'PYINFER'
import json, os, sys
d, net = sys.argv[1], sys.argv[2]
members = {}
if d:
    for f in os.listdir(d):
        if not f.endswith(".json"):
            continue
        try:
            c = json.load(open(os.path.join(d, f)))[0]
        except Exception:
            continue
        ns = (c.get("NetworkSettings", {}) or {}).get("Networks", {}) or {}
        if net in ns:
            cid = c.get("Id", f[:-5])
            members[cid] = {"Name": c.get("Name", cid)}
print(json.dumps([{"Name": net, "Containers": members}]))
PYINFER
)"
        printf '%s' "$RESTORE_MEMBERS"
      else
        printf '[{"Name":"%s","Containers":{}}]' "$name"
      fi
    fi
    exit 0 ;;
  inspect)
    name="${2:-}"
    if [ -n "${FAKEDOCKER_INSPECT_DIR:-}" ]; then
      if [ -f "$FAKEDOCKER_INSPECT_DIR/$name.json" ]; then
        cat "$FAKEDOCKER_INSPECT_DIR/$name.json"
      else
        # Persist the default inspect so the network-membership inference above
        # can later find this container attached to its `<name>-net` network.
        printf '[{"Id":"%s","State":{"Running":true},"HostConfig":{"NetworkMode":"%s-net","PortBindings":{}},"Config":{"Image":"example-registry/bodysense-api:test","Name":"%s","Labels":{"bodysense.restore-project":"drill","bodysense.disposable-restore":"yes","bodysense.restore-network":"%s-net"}},"NetworkSettings":{"Networks":{"%s-net":{}}}}]' \
          "$name" "$name" "$name" "$name" "$name" \
          > "$FAKEDOCKER_INSPECT_DIR/$name.json"
        cat "$FAKEDOCKER_INSPECT_DIR/$name.json"
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
FAKEDOCKER_NETWORK_DIR="$TMP/docker-networks"
mkdir -p "$FAKEDOCKER_NETWORK_DIR"
DOCKER_LOG="$TMP/docker.log"
DOCKER_ENVLOG="$TMP/docker-env.log"
export FAKEDOCKER_INSPECT_DIR FAKEDOCKER_NETWORK_DIR DOCKER_LOG DOCKER_ENVLOG

# write_inspect <container> <running> <labels-as-json> <networks-as-json> [netmode] [portbindings] [image]
# The default netmode is the first key of <networks-as-json> (or "none" if empty).
# The default portbindings is an empty {} object (no host ports published).  An
# image (a JSON string like "registry/bodysense-api:v1") is only added when given.
write_inspect() {
  local name="$1" running="$2" labels="$3" networks="$4" netmode="${5:-}" ports="${6:-}" image="${7:-}"
  [ -n "$ports" ] || ports='{}'
  if [ -z "$netmode" ]; then
    netmode=$(printf '%s' "$networks" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(next(iter(d or {}), "none"))')
  fi
  if [ -n "$image" ]; then
    cat > "$FAKEDOCKER_INSPECT_DIR/$name.json" <<JSON
[{"Id":"$name","State":{"Running":$running},"HostConfig":{"NetworkMode":"$netmode","PortBindings":$ports},"Config":{"Image":$image,"Labels":$labels},"NetworkSettings":{"Networks":$networks}}]
JSON
  else
    cat > "$FAKEDOCKER_INSPECT_DIR/$name.json" <<JSON
[{"Id":"$name","State":{"Running":$running},"HostConfig":{"NetworkMode":"$netmode","PortBindings":$ports},"Config":{"Labels":$labels},"NetworkSettings":{"Networks":$networks}}]
JSON
  fi
}

# --- fake S3 server lifecycle -------------------------------------------------
start_server() {
  rm -f "$PORT_FILE" "$CORRUPT_FILE" "$LIST_FAIL_FILE" "$PUT_LOG_FILE"
  rm -f "$PUBLIC_ACL_FILE" "$ACL_UNREADABLE_FILE" "$POLICY_PUBLIC_FILE" "$POLICY_UNAVAILABLE_FILE" "$POLICY_UNSUPPORTED_FILE"
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
# ACL / policy-status control files so the private-destination preflight can be
# made to fail (public or unreadable ACL / public policy) without editing the
# fake server.
_acl_unreadable = os.environ.get("ACL_UNREADABLE_FILE", "")
_public_acl = os.environ.get("PUBLIC_ACL_FILE", "")
_policy_unavailable = os.environ.get("POLICY_UNAVAILABLE_FILE", "")
_policy_public = os.environ.get("POLICY_PUBLIC_FILE", "")
_policy_unsupported = os.environ.get("POLICY_UNSUPPORTED_FILE", "")
def _flag(path):
    return bool(path) and os.path.exists(path)
_orig_acl_reply = T.FakeS3Handler._acl_reply
def _guarded_acl_reply(self):
    self.acl_unreadable = _flag(_acl_unreadable)
    self.public_acl = _flag(_public_acl)
    return _orig_acl_reply(self)
_orig_policy_reply = T.FakeS3Handler._policy_status_reply
def _guarded_policy_reply(self):
    self.policy_unavailable = _flag(_policy_unavailable)
    self.policy_public = _flag(_policy_public)
    self.policy_unsupported = _flag(_policy_unsupported)
    return _orig_policy_reply(self)
_orig_put = T.FakeS3Handler.do_PUT
def _guarded_put(self):
    _orig_put(self)
    # Record every PUT's wire ACL/SSE headers so tests can prove the backup
    # script uploads objects with x-amz-acl=private and
    # x-amz-server-side-encryption=AES256.
    rec = "%s acl=%s sse=%s" % (
        self._key_from_path(),
        self.headers.get("x-amz-acl", "-"),
        self.headers.get("x-amz-server-side-encryption", "-"),
    )
    with open(os.environ.get("PUT_LOG_FILE", "/dev/null"), "a") as fh:
        fh.write(rec + "\n")
Handler = type("CorruptibleHandler", (T.FakeS3Handler,), {
    "corrupt_file": sys.argv[2],
    "corrupt_suffix": ".dump",
    "list_fail_file": sys.argv[3],
    "_list": _guarded_list,
    "_acl_reply": _guarded_acl_reply,
    "_policy_status_reply": _guarded_policy_reply,
    "do_PUT": _guarded_put,
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
# 3c. backup and freshness use SEPARATE lock domains (review finding: the old
#     single lock let a running/hung backup suppress the freshness alert path).
#     A backup that loses the backup lock must still skip cleanly (exit 0, no
#     duplicate upload), while a freshness run that loses its OWN lock must exit
#     non-zero (never silently 0), and neither domain may block the other.
# ==============================================================================
run_backup > /dev/null            # valid last-success state first
LOCKDIR="$ROOT/.offhost-state"
exec 9>"$LOCKDIR/offhost-backup.lock"
flock -n 9 || { echo "test harness could not hold the backup lock"; exit 1; }
out=$(run_backup 2>&1) && rc=0 || rc=$?
if [ $rc -eq 0 ] && [[ "$out" == *"backup operation is already running; skipping this run"* ]]; then
  report 0 "a backup that loses the backup lock skips cleanly (exit 0), never duplicate-runs"
else
  report 1 "a backup that loses the backup lock skips cleanly (exit 0), never duplicate-runs" "rc=$rc out=$out"
fi
out=$(run_freshness 2>&1) && rc=0 || rc=$?
if [ $rc -eq 0 ] && [[ "$out" == *OFFHOST_BACKUP_FRESH=OK* ]]; then
  report 0 "freshness alerting is NOT suppressed while a backup holds the backup lock"
else
  report 1 "freshness alerting is NOT suppressed while a backup holds the backup lock" "rc=$rc out=$out"
fi
exec 9>&-

exec 8>"$LOCKDIR/offhost-freshness.lock"
flock -n 8 || { echo "test harness could not hold the freshness lock"; exit 1; }
out=$(run_freshness 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"freshness lock held"* ]]; then
  report 0 "a freshness run that loses the freshness lock fails loudly, never exits 0"
else
  report 1 "a freshness run that loses the freshness lock fails loudly, never exits 0" "rc=$rc out=$out"
fi
out=$(run_backup 2>&1) && rc=0 || rc=$?
if [ $rc -eq 0 ] && [[ "$out" == *"uploading off-host backup"* ]]; then
  report 0 "a backup is NOT blocked while a freshness check holds the freshness lock"
else
  report 1 "a backup is NOT blocked while a freshness check holds the freshness lock" "rc=$rc out=$out"
fi
exec 8>&-

# ==============================================================================
# 3d. the private-destination preflight is fail-closed (review finding: privacy
#     used to be assumed, never proven): a backup must abort BEFORE any upload
#     when the bucket ACL is public, unreadable, or the bucket policy status is
#     public, and must never record last-success.json in those cases.  By default
#     the acl+policy proof is fail-closed even when a store cannot answer
#     GetBucketPolicyStatus (BucketPolicyStatusUnsupported refuses the backup);
#     OFFHOST_BACKUP_PRIVACY_PROOF=acl explicitly opts into ACL-only proof for
#     stores like Alibaba OSS that do not implement policy status, prints a
#     warning, and still succeeds.
# ==============================================================================
BEFORE_KEY=$(last_key)
: > "$PUBLIC_ACL_FILE"
out=$(run_backup 2>&1) && rc=0 || rc=$?
rm -f "$PUBLIC_ACL_FILE"
if [ $rc -ne 0 ] && [[ "$out" == *"private-destination preflight failed"* ]] \
  && [[ "$out" == *BucketPublicAcl* ]] && [ "$(last_key)" = "$BEFORE_KEY" ]; then
  report 0 "backup fails closed when the destination bucket ACL is public"
else
  report 1 "backup fails closed when the destination bucket ACL is public" "rc=$rc out=$out"
fi
: > "$ACL_UNREADABLE_FILE"
out=$(run_backup 2>&1) && rc=0 || rc=$?
rm -f "$ACL_UNREADABLE_FILE"
if [ $rc -ne 0 ] && [[ "$out" == *"private-destination preflight failed"* ]] \
  && [[ "$out" == *BucketAclUnreadable* ]] && [ "$(last_key)" = "$BEFORE_KEY" ]; then
  report 0 "backup fails closed when the destination bucket ACL is unreadable"
else
  report 1 "backup fails closed when the destination bucket ACL is unreadable" "rc=$rc out=$out"
fi
: > "$POLICY_PUBLIC_FILE"
out=$(run_backup 2>&1) && rc=0 || rc=$?
rm -f "$POLICY_PUBLIC_FILE"
if [ $rc -ne 0 ] && [[ "$out" == *"private-destination preflight failed"* ]] \
  && [[ "$out" == *BucketPublicPolicy* ]] && [ "$(last_key)" = "$BEFORE_KEY" ]; then
  report 0 "backup fails closed when the destination bucket policy status is public"
else
  report 1 "backup fails closed when the destination bucket policy status is public" "rc=$rc out=$out"
fi
: > "$POLICY_UNSUPPORTED_FILE"
out=$(run_backup 2>&1) && rc=0 || rc=$?
rm -f "$POLICY_UNSUPPORTED_FILE"
if [ $rc -ne 0 ] && [[ "$out" == *"private-destination preflight failed"* ]] \
  && [[ "$out" == *BucketPolicyStatusUnsupported* ]] && [ "$(last_key)" = "$BEFORE_KEY" ]; then
  report 0 "policy status that cannot be answered is a FAILURE in the default acl+policy mode"
else
  report 1 "policy status that cannot be answered is a FAILURE in the default acl+policy mode" "rc=$rc out=$out"
fi
: > "$POLICY_UNSUPPORTED_FILE"
out=$(OFFHOST_BACKUP_PRIVACY_PROOF=acl run_backup 2>&1) && rc=0 || rc=$?
rm -f "$POLICY_UNSUPPORTED_FILE"
if [ $rc -eq 0 ] && [[ "$out" == *"PRIVATE_PREFLIGHT=PASS proof=acl"* ]] \
  && [[ "$out" == *OFFHOST_S3_WARNING* ]]; then
  report 0 "acl-only proof passes with a warning when the store has no policy status"
else
  report 1 "acl-only proof passes with a warning when the store has no policy status" "rc=$rc out=$out"
fi
out=$(OFFHOST_BACKUP_PRIVACY_PROOF=bogus run_backup 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"OFFHOST_BACKUP_PRIVACY_PROOF must be 'acl+policy' or 'acl'"* ]]; then
  report 0 "OFFHOST_BACKUP_PRIVACY_PROOF refuses any value other than 'acl+policy' or 'acl'"
else
  report 1 "OFFHOST_BACKUP_PRIVACY_PROOF refuses any value other than 'acl+policy' or 'acl'" "rc=$rc out=$out"
fi

# ==============================================================================
# 3f. a disabled backup pipeline is fail-closed on the freshness side (review
#     finding: a disabled pipeline used to exit 0 for --check-freshness, letting
#     a STOPPED backup pipeline masquerade as protected).  The scheduled
#     freshness check must exit non-zero with reason=backups-disabled so the
#     alert path fires; only OFFHOST_BACKUP_MAINTENANCE_SUPPRESS=true
#     acknowledges an intentional maintenance window with a clean exit.  A
#     disabled pipeline in --backup mode skips cleanly.
# ==============================================================================
rm -f "$ROOT/.offhost-state/last-success.json"
out=$(OFFHOST_BACKUP_ENABLED=false run_freshness 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *OFFHOST_BACKUP_FRESH=FAIL* ]] \
  && [[ "$out" == *reason=backups-disabled* ]]; then
  report 0 "disabled backups fail the freshness check with reason=backups-disabled"
else
  report 1 "disabled backups fail the freshness check with reason=backups-disabled" "rc=$rc out=$out"
fi
out=$(OFFHOST_BACKUP_ENABLED=false OFFHOST_BACKUP_MAINTENANCE_SUPPRESS=true run_freshness 2>&1) && rc=0 || rc=$?
if [ $rc -eq 0 ] && [[ "$out" == *"intentionally suppressed"* ]]; then
  report 0 "explicit maintenance suppression acknowledges the disabled pipeline with a clean exit"
else
  report 1 "explicit maintenance suppression acknowledges the disabled pipeline with a clean exit" "rc=$rc out=$out"
fi
out=$(OFFHOST_BACKUP_ENABLED=false run_backup 2>&1) && rc=0 || rc=$?
if [ $rc -eq 0 ] && [[ "$out" == *"off-host backups disabled"* ]]; then
  report 0 "a disabled pipeline in backup mode skips cleanly"
else
  report 1 "a disabled pipeline in backup mode skips cleanly" "rc=$rc out=$out"
fi
out=$(OFFHOST_BACKUP_ENABLED=false OFFHOST_BACKUP_MAINTENANCE_SUPPRESS=maybe run_freshness 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"OFFHOST_BACKUP_MAINTENANCE_SUPPRESS must be true or false"* ]]; then
  report 0 "OFFHOST_BACKUP_MAINTENANCE_SUPPRESS refuses any value other than true or false"
else
  report 1 "OFFHOST_BACKUP_MAINTENANCE_SUPPRESS refuses any value other than true or false" "rc=$rc out=$out"
fi
run_backup > /dev/null   # restore a valid last-success state for later sections

# ==============================================================================
# 3e. object ACL/SSE configuration is fail-closed: public object ACLs are never
#     accepted, unknown SSE modes are refused, and successful backups upload
#     every object with x-amz-acl=private and x-amz-server-side-encryption=
#     AES256 on the real signed wire request.
# ==============================================================================
out=$(OFFHOST_BACKUP_OBJECT_ACL=public-read run_backup 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"must be 'private' or empty"* ]]; then
  report 0 "OFFHOST_BACKUP_OBJECT_ACL refuses any non-private value"
else
  report 1 "OFFHOST_BACKUP_OBJECT_ACL refuses any non-private value" "rc=$rc out=$out"
fi
out=$(OFFHOST_BACKUP_SSE=magic run_backup 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"must be AES256, aws:kms or empty"* ]]; then
  report 0 "OFFHOST_BACKUP_SSE refuses unknown encryption modes"
else
  report 1 "OFFHOST_BACKUP_SSE refuses unknown encryption modes" "rc=$rc out=$out"
fi
: > "$PUT_LOG_FILE"
run_backup > /dev/null
OBJKEY_LATEST=$(last_key)
if grep -q "^$OBJKEY_LATEST acl=private sse=AES256$" "$PUT_LOG_FILE" \
  && grep -q "^$OBJKEY_LATEST.sha256 acl=private sse=AES256$" "$PUT_LOG_FILE" \
  && grep -q "^$OBJKEY_LATEST.meta.json acl=private sse=AES256$" "$PUT_LOG_FILE" \
  && ! grep -q "acl=public-read" "$PUT_LOG_FILE"; then
  report 0 "backup uploads every object with x-amz-acl=private and x-amz-server-side-encryption=AES256"
else
  report 1 "backup uploads every object with x-amz-acl=private and x-amz-server-side-encryption=AES256" "put_log=$(tr '\n' '|' < "$PUT_LOG_FILE")"
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
# 4c. the schema-revision gate is fail-closed on the backup side: a backup whose
#     revision cannot be verified (no schema_migrations table, a failed
#     existence probe, or a failed query) must abort before any object is
#     uploaded and before last-success.json is written, so no backup is ever
#     recorded as successful while carrying an unverified revision
# ==============================================================================
BEFORE_KEY=$(last_key)
out=$(FAKEPG_HAS_MIGRATIONS=f run_backup 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"no schema_migrations table"* ]] && [ "$(last_key)" = "$BEFORE_KEY" ]; then
  report 0 "backup aborts when the source has no schema_migrations table"
else
  report 1 "backup aborts when the source has no schema_migrations table" "rc=$rc out=$out"
fi
out=$(FAKEPG_TOREGCLASS_FAIL=1 run_backup 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"to_regclass query failed"* ]] && [ "$(last_key)" = "$BEFORE_KEY" ]; then
  report 0 "backup aborts when the schema_migrations existence probe fails"
else
  report 1 "backup aborts when the schema_migrations existence probe fails" "rc=$rc out=$out"
fi
out=$(FAKEPG_SCHEMA_FAIL=1 run_backup 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"schema_migrations query failed"* ]] && [ "$(last_key)" = "$BEFORE_KEY" ]; then
  report 0 "backup aborts when the schema_migrations query fails"
else
  report 1 "backup aborts when the schema_migrations query fails" "rc=$rc out=$out"
fi
out=$(FAKEPG_SCHEMA="54:true" run_backup 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"is not an exact clean <version>:false revision"* ]] \
  && [ "$(last_key)" = "$BEFORE_KEY" ]; then
  report 0 "backup aborts when the source schema revision is dirty (54:true, not an exact clean revision)"
else
  report 1 "backup aborts when the source schema revision is dirty (54:true, not an exact clean revision)" "rc=$rc out=$out"
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
run_restore_guard 'must differ from the production project "bodysense"' \
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
#     right network set, non-host dedicated drill network declaration, labels,
#     running state, exclusivity from the production container/endpoint and from
#     the production Compose project.  Network enumeration is fail-closed: an
#     inspection/parse failure is refused, never treated as an empty "isolated"
#     result.
write_inspect restore-shared-net true \
  '{"bodysense.restore-project":"drill","bodysense.disposable-restore":"yes","bodysense.restore-network":"pg1-net"}' \
  '{"pg1-net":{}}'
run_restore_guard "attached to the production postgres network" \
  --object-key "$OBJKEY" --target-db drill_net --target-project drill \
  --restore-pg container:restore-shared-net --confirm-target-isolated=yes
write_inspect restore-wrong-project true \
  '{"bodysense.restore-project":"staging","bodysense.disposable-restore":"yes","bodysense.restore-network":"restore-wrong-project-net"}' \
  '{"restore-wrong-project-net":{}}'
run_restore_guard "does not declare bodysense.restore-project=drill" \
  --object-key "$OBJKEY" --target-db drill_proj --target-project drill \
  --restore-pg container:restore-wrong-project --confirm-target-isolated=yes
write_inspect restore-not-disposable true \
  '{"bodysense.restore-project":"drill","bodysense.disposable-restore":"no","bodysense.restore-network":"restore-not-disposable-net"}' \
  '{"restore-not-disposable-net":{}}'
run_restore_guard "refusing a non-disposable target" \
  --object-key "$OBJKEY" --target-db drill_disp --target-project drill \
  --restore-pg container:restore-not-disposable --confirm-target-isolated=yes
write_inspect restore-stopped false \
  '{"bodysense.restore-project":"drill","bodysense.disposable-restore":"yes","bodysense.restore-network":"restore-stopped-net"}' \
  '{"restore-stopped-net":{}}'
run_restore_guard "is not running" \
  --object-key "$OBJKEY" --target-db drill_run --target-project drill \
  --restore-pg container:restore-stopped --confirm-target-isolated=yes
write_inspect pg1 true \
  '{"com.docker.compose.project":"docker"}' \
  '{"pg1-net":{}}'
write_inspect restore-prod-compose true \
  '{"com.docker.compose.project":"docker","bodysense.restore-project":"drill","bodysense.disposable-restore":"yes","bodysense.restore-network":"restore-prod-compose-net"}' \
  '{"restore-prod-compose-net":{}}'
run_restore_guard "belongs to the production compose project 'docker'" \
  --object-key "$OBJKEY" --target-db drill_compose --target-project drill \
  --restore-pg container:restore-prod-compose --confirm-target-isolated=yes
rm -f "$FAKEDOCKER_INSPECT_DIR/pg1.json"

# 5c. host networking and an undeclared/undetached drill network are refused:
#     a host-network target retains reachability to host-published production
#     endpoints even with zero common Docker networks, so it is not provably
#     isolated regardless of its labels.
write_inspect restore-host-net true \
  '{"bodysense.restore-project":"drill","bodysense.disposable-restore":"yes","bodysense.restore-network":"host"}' \
  '{"host":{}}' host
run_restore_guard "refusing a restore container using host networking" \
  --object-key "$OBJKEY" --target-db drill_host --target-project drill \
  --restore-pg container:restore-host-net --confirm-target-isolated=yes
write_inspect restore-none-mode true \
  '{"bodysense.restore-project":"drill","bodysense.disposable-restore":"yes","bodysense.restore-network":"none"}' \
  '{"none":{}}' none
run_restore_guard "with no networking (NetworkMode=none)" \
  --object-key "$OBJKEY" --target-db drill_none --target-project drill \
  --restore-pg container:restore-none-mode --confirm-target-isolated=yes
write_inspect restore-no-network-label true \
  '{"bodysense.restore-project":"drill","bodysense.disposable-restore":"yes"}' \
  '{"restore-no-network-label-net":{}}'
run_restore_guard "does not declare bodysense.restore-network" \
  --object-key "$OBJKEY" --target-db drill_nolabel --target-project drill \
  --restore-pg container:restore-no-network-label --confirm-target-isolated=yes
write_inspect restore-attach-mismatch true \
  '{"bodysense.restore-project":"drill","bodysense.disposable-restore":"yes","bodysense.restore-network":"declared-drill-net"}' \
  '{"actual-net":{}}'
run_restore_guard "is not attached to its declared bodysense.restore-network" \
  --object-key "$OBJKEY" --target-db drill_attach --target-project drill \
  --restore-pg container:restore-attach-mismatch --confirm-target-isolated=yes

# 5d. network enumeration failures are fail-closed on BOTH sides: an inspect
#     output whose Networks set cannot be read is refused instead of being
#     treated as an empty "shares no network with production" result.
write_inspect restore-net-null true \
  '{"bodysense.restore-project":"drill","bodysense.disposable-restore":"yes","bodysense.restore-network":"restore-net-null-net"}' \
  null restore-net-null-net
run_restore_guard "unable to inspect the restore postgres container network(s)" \
  --object-key "$OBJKEY" --target-db drill_netnull --target-project drill \
  --restore-pg container:restore-net-null --confirm-target-isolated=yes
write_inspect pg1 true '{}' null
run_restore_guard "unable to inspect the production postgres container network(s)" \
  --object-key "$OBJKEY" --target-db drill_prodnet --target-project drill \
  --restore-pg container:restore-pg --confirm-target-isolated=yes
rm -f "$FAKEDOCKER_INSPECT_DIR/pg1.json"

# 5d2. the declared dedicated drill network must be the container's ONLY
#      network: a target attached to the drill network plus an additional
#      ingress/application network is traffic-reachable from that second network
#      even when it shares nothing with the production postgres container, so it
#      is refused despite zero network overlap with production.
write_inspect restore-extra-net true \
  '{"bodysense.restore-project":"drill","bodysense.disposable-restore":"yes","bodysense.restore-network":"drill-only-net"}' \
  '{"drill-only-net":{},"ingress-net":{}}'
run_restore_guard "attached to networks beyond its declared drill network" \
  --object-key "$OBJKEY" --target-db drill_extra --target-project drill \
  --restore-pg container:restore-extra-net --confirm-target-isolated=yes

# 5d3. published host ports make the target reachable from the host/ingress even
#      on a dedicated Docker network, so a drill server that publishes any host
#      port is refused (never "isolated merely because it is not attached to a
#      production Docker network").
write_inspect restore-published-ports true \
  '{"bodysense.restore-project":"drill","bodysense.disposable-restore":"yes","bodysense.restore-network":"restore-published-ports-net"}' \
  '{"restore-published-ports-net":{}}' restore-published-ports-net \
  '{"5432/tcp":[{"HostIp":"0.0.0.0","HostPort":"5432"}]}'
run_restore_guard "publishes host ports" \
  --object-key "$OBJKEY" --target-db drill_ports --target-project drill \
  --restore-pg container:restore-published-ports --confirm-target-isolated=yes

# 5d4. THE DRILL NETWORK MUST CONTAIN EXACTLY THE DISPOSABLE RESTORE CONTAINER
#      (review finding): Docker bridge connectivity is bidirectional, so ANY
#      container joined to the drill network — production or not, labelled or
#      unlabelled — could reach the disposable restore database.  The restore
#      container's own labels/ports/network proofs do not cover OTHER containers
#      on the drill network, so the restore refuses the moment any member of the
#      declared drill network is anything other than the disposable restore
#      container itself: the production postgres container, a production-project
#      member, or an unrelated/compromised container is equally refused.  This is
#      what rejects running the validators on the production api container.
write_inspect pg1 true \
  '{"com.docker.compose.project":"docker"}' \
  '{"pg1-net":{}}'
write_inspect docker-api-1 true \
  '{"com.docker.compose.project":"docker"}' \
  '{"default":{}}' default '{}' '"docker-api-image:1.0"'
write_inspect drill-mem-empty true \
  '{"bodysense.restore-project":"drill","bodysense.disposable-restore":"yes","bodysense.restore-network":"empty-member-net"}' \
  '{"empty-member-net":{}}'
# A drill network whose membership does NOT include the disposable restore
# container itself (here the empty inferred/fake membership) is refused: the
# drill network must be the restore container's exclusive network.
printf '[{"Name":"empty-member-net","Containers":{}}]' \
  > "$FAKEDOCKER_NETWORK_DIR/empty-member-net.json"
run_restore_guard "does not contain the disposable restore container itself" \
  --object-key "$OBJKEY" --target-db drill_memempty --target-project drill \
  --restore-pg container:drill-mem-empty --confirm-target-isolated=yes
rm -f "$FAKEDOCKER_NETWORK_DIR/empty-member-net.json"
write_inspect drill-prod-member true \
  '{"bodysense.restore-project":"drill","bodysense.disposable-restore":"yes","bodysense.restore-network":"prod-member-net"}' \
  '{"prod-member-net":{}}'
printf '[{"Name":"prod-member-net","Containers":{"docker-api-1":{"Name":"docker-api-1"}}}]' \
  > "$FAKEDOCKER_NETWORK_DIR/prod-member-net.json"
run_restore_guard "a production-project member" \
  --object-key "$OBJKEY" --target-db drill_member --target-project drill \
  --restore-pg container:drill-prod-member --confirm-target-isolated=yes
rm -f "$FAKEDOCKER_NETWORK_DIR/prod-member-net.json"
write_inspect drill-prod-pg true \
  '{"bodysense.restore-project":"drill","bodysense.disposable-restore":"yes","bodysense.restore-network":"prod-pg-net"}' \
  '{"prod-pg-net":{}}'
printf '[{"Name":"prod-pg-net","Containers":{"pg1":{"Name":"pg2"}}}]' \
  > "$FAKEDOCKER_NETWORK_DIR/prod-pg-net.json"
run_restore_guard "the production postgres container" \
  --object-key "$OBJKEY" --target-db drill_prodpg --target-project drill \
  --restore-pg container:drill-prod-pg --confirm-target-isolated=yes
rm -f "$FAKEDOCKER_NETWORK_DIR/prod-pg-net.json"
# NEW REGRESSION (review finding): an UNRELATED, UNLABELLED rogue container on
# the drill network — with no production Compose project label and not the
# production postgres — must still be refused.  A compromised or mistakenly
# attached non-production container on the drill network can reach the
# disposable restore database just as a production one can, so the membership
# guard must fail on it, never accept it merely because it is not labelled
# "production".
write_inspect rogue-1 true \
  '{}' \
  '{"rogue-1-net":{}}'
write_inspect drill-rogue-member true \
  '{"bodysense.restore-project":"drill","bodysense.disposable-restore":"yes","bodysense.restore-network":"rogue-member-net"}' \
  '{"rogue-member-net":{}}'
printf '[{"Name":"rogue-member-net","Containers":{"rogue-1":{"Name":"rogue-1"}}}]' \
  > "$FAKEDOCKER_NETWORK_DIR/rogue-member-net.json"
run_restore_guard "an unrelated container" \
  --object-key "$OBJKEY" --target-db drill_rogue --target-project drill \
  --restore-pg container:drill-rogue-member --confirm-target-isolated=yes
rm -f "$FAKEDOCKER_NETWORK_DIR/rogue-member-net.json"
# A drill-network membership that cannot be enumerated (uninspectable) is a
# refusal too: never treated as an empty "no production member" proof.
write_inspect drill-mem-garbage true \
  '{"bodysense.restore-project":"drill","bodysense.disposable-restore":"yes","bodysense.restore-network":"garbage-net"}' \
  '{"garbage-net":{}}'
printf 'not-json' > "$FAKEDOCKER_NETWORK_DIR/garbage-net.json"
run_restore_guard "unable to inspect the drill network garbage-net container membership" \
  --object-key "$OBJKEY" --target-db drill_memgarbage --target-project drill \
  --restore-pg container:drill-mem-garbage --confirm-target-isolated=yes
rm -f "$FAKEDOCKER_NETWORK_DIR/garbage-net.json"
rm -f "$FAKEDOCKER_INSPECT_DIR/pg1.json" "$FAKEDOCKER_INSPECT_DIR/docker-api-1.json"

# 5e. an unverifiable schema revision in the backup metadata never passes the
#     restore gate: `unknown`/`uninitialized`/empty metadata is refused before
#     any archive download, never accepted and never skipped.
meta_get "$OBJKEY.meta.json" "$TMP/meta-ok.json"
cp "$TMP/meta-ok.json" "$TMP/meta-tampered.json"
python3 - "$TMP/meta-tampered.json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))
d["schema_revision"] = "unknown"
json.dump(d, open(sys.argv[1], "w"))
PY
s3put "$OBJKEY.meta.json" "$TMP/meta-tampered.json" >/dev/null
run_restore_guard "declares an unverifiable schema revision 'unknown'" \
  --object-key "$OBJKEY" --target-db drill_meta --target-project drill \
  --restore-pg container:restore-meta --confirm-target-isolated=yes
s3put "$OBJKEY.meta.json" "$TMP/meta-ok.json" >/dev/null
rm -f "$TMP/meta-ok.json" "$TMP/meta-tampered.json"

# 5e-bis. the restore-side schema gate is an EXACT clean `<version>:false` value
#     (review finding: a dirty or structurally malformed revision used to be
#     accepted).  Metadata declaring a dirty revision (54:true) is refused before
#     download, and a restore whose target database fails to return a proven
#     clean revision is refused at the certification gate.
meta_get "$OBJKEY.meta.json" "$TMP/meta-ok.json"
cp "$TMP/meta-ok.json" "$TMP/meta-dirty.json"
python3 - "$TMP/meta-dirty.json" <<'PY'
import json, sys
d = json.load(open(sys.argv[1]))
d["schema_revision"] = "54:true"
json.dump(d, open(sys.argv[1], "w"))
PY
s3put "$OBJKEY.meta.json" "$TMP/meta-dirty.json" >/dev/null
run_restore_guard "declares an unverifiable schema revision '54:true'" \
  --object-key "$OBJKEY" --target-db drill_meta_dirty --target-project drill \
  --restore-pg container:restore-meta-dirty --confirm-target-isolated=yes
s3put "$OBJKEY.meta.json" "$TMP/meta-ok.json" >/dev/null
rm -f "$TMP/meta-ok.json" "$TMP/meta-dirty.json"
out=$(FAKEPG_SCHEMA="49:true" BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" \
  OFFHOST_PGCONTAINER_ID=pg1 \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db drill_restored_dirty --target-project drill \
  --restore-pg container:restore-dirty --confirm-target-isolated=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"restored schema revision '49:true' is not an exact clean"* ]] \
  && [[ "$out" == *"refusing to certify a dirty or unverifiable restore"* ]]; then
  report 0 "restore refuses to certify a restored database whose revision is not proven clean (49:true)"
else
  report 1 "restore refuses to certify a restored database whose revision is not proven clean (49:true)" "rc=$rc out=$out"
fi
FAKEPG_SCHEMA="49:false"

# ==============================================================================
# 5f. recovery mode (--recovery-mode=yes / OFFHOST_RECOVERY_MODE=true) lets a
#     restore proceed when the production Postgres container cannot be inspected
#     because production is down (review finding).  The production-side proofs
#     (container-ID difference, shared-network intersection, discovered compose
#     project) are NOT claimed in recovery mode; the target's own declarations
#     plus the operator-declared production project name remain the isolation
#     proof.  Without --recovery-mode a missing/uninspectable production
#     container still refuses.
# ==============================================================================
rm -f "$FAKEDOCKER_INSPECT_DIR/pg1.json"
FAKEPG_DB_EXISTS="" FAKEPG_SCHEMA="49:false"
write_inspect restore-recovery true \
  '{"com.docker.compose.project":"recovery-drill","bodysense.restore-project":"recovery","bodysense.disposable-restore":"yes","bodysense.restore-network":"recovery-net"}' \
  '{"recovery-net":{}}'
out=$(BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" \
  OFFHOST_API_CONTAINER=fake-api-1 \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db recovery_db --target-project recovery \
  --restore-pg container:restore-recovery --confirm-target-isolated=yes \
  --recovery-mode=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
if [ $rc -eq 0 ] && [[ "$out" == *RESTORE_RESULT=PASS* ]] \
  && [[ "$out" == *"RECOVERY MODE: production postgres container inspection is skipped"* ]]; then
  report 0 "recovery mode restores and validates without inspecting a live production container"
else
  report 1 "recovery mode restores and validates without inspecting a live production container" "rc=$rc out=$out"
fi
out=$(OFFHOST_RECOVERY_MODE=true BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" \
  OFFHOST_API_CONTAINER=fake-api-1 \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db recovery_env_db --target-project recovery \
  --restore-pg container:restore-recovery --confirm-target-isolated=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
if [ $rc -eq 0 ] && [[ "$out" == *RESTORE_RESULT=PASS* ]] \
  && [[ "$out" == *"RECOVERY MODE"* ]]; then
  report 0 "OFFHOST_RECOVERY_MODE=true enables recovery mode through the environment"
else
  report 1 "OFFHOST_RECOVERY_MODE=true enables recovery mode through the environment" "rc=$rc out=$out"
fi
rm -f "$FAKEDOCKER_INSPECT_DIR/restore-recovery.json"
out=$(BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db recov_solo --target-project recovery \
  --restore-pg container:restore-pg --confirm-target-isolated=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"unable to identify the production postgres container"* ]]; then
  report 0 "without --recovery-mode a restore still refuses when no production container is resolvable"
else
  report 1 "without --recovery-mode a restore still refuses when no production container is resolvable" "rc=$rc out=$out"
fi
out=$(BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db recov_proj --target-project bodysense \
  --restore-pg container:restore-pg --confirm-target-isolated=yes \
  --recovery-mode=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *'must differ from the production project "bodysense"'* ]]; then
  report 0 "recovery mode refuses --target-project equal to the production project"
else
  report 1 "recovery mode refuses --target-project equal to the production project" "rc=$rc out=$out"
fi
write_inspect restore-prod-labeled true \
  '{"com.docker.compose.project":"bodysense","bodysense.restore-project":"drill","bodysense.disposable-restore":"yes","bodysense.restore-network":"restore-prod-labeled-net"}' \
  '{"restore-prod-labeled-net":{}}'
out=$(BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db recov_compose --target-project drill \
  --restore-pg container:restore-prod-labeled --confirm-target-isolated=yes \
  --recovery-mode=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"labeled with the production compose/project name 'bodysense'"* ]]; then
  report 0 "recovery mode refuses a target labeled with the production compose/project name"
else
  report 1 "recovery mode refuses a target labeled with the production compose/project name" "rc=$rc out=$out"
fi
rm -f "$FAKEDOCKER_INSPECT_DIR/restore-prod-labeled.json"
out=$(OFFHOST_RECOVERY_MODE=garbage BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db recov_badenv --target-project recovery \
  --restore-pg container:restore-pg --confirm-target-isolated=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"OFFHOST_RECOVERY_MODE must be true or false"* ]]; then
  report 0 "OFFHOST_RECOVERY_MODE rejects invalid values"
else
  report 1 "OFFHOST_RECOVERY_MODE rejects invalid values" "rc=$rc out=$out"
fi
out=$(BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db recov_badflag --target-project recovery \
  --restore-pg container:restore-pg --confirm-target-isolated=yes \
  --recovery-mode=maybe --validator-runner docker 2>&1) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [[ "$out" == *"--recovery-mode must be yes or no"* ]]; then
  report 0 "--recovery-mode rejects invalid values"
else
  report 1 "--recovery-mode rejects invalid values" "rc=$rc out=$out"
fi

# ==============================================================================
# 6. restore happy path with validator invocations
# ==============================================================================
FAKEPG_DB_EXISTS="" FAKEPG_SCHEMA="49:false"
# The validators run in SEPARATE disposable containers derived from the resolved
# api container's Config.Image (never inside the api container itself), so the
# api containers must be inspectable to the image that hosts the validators.
write_inspect pg1 true \
  '{"com.docker.compose.project":"docker"}' \
  '{"pg1-net":{}}'
write_inspect fake-api-1 true \
  '{"com.docker.compose.project":"docker"}' \
  '{"default":{}}' default '{}' '"fake-api-image:2.0"'
write_inspect docker-api-1 true \
  '{"com.docker.compose.project":"docker"}' \
  '{"default":{}}' default '{}' '"docker-api-image:1.0"'
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
# Review finding: the validators must run inside a disposable container derived
# from the api image and attached ONLY to the drill network — never inside the
# production api container (which the restore must not join to the drill
# network).  The api image is taken from the resolved api container (the
# OFFHOST_API_CONTAINER seam wins over any lookup), the container is created
# with --rm, is labeled disposable, and is attached to the declared drill
# network; a literal "api" / a host-side `docker exec` is never used.
if grep -q '^run --rm --network restore-pg-net -l bodysense.disposable-restore=yes --env-file [^ ]* --entrypoint [^ ]* fake-api-image:2.0 ' "$DOCKER_LOG" \
  && grep -q -- '--entrypoint /app/validators/migration-validator ' "$DOCKER_LOG" \
  && grep -q -- '--entrypoint /app/validators/domain-validator ' "$DOCKER_LOG" \
  && ! grep -q 'docker exec' "$DOCKER_LOG" \
  && ! grep -q '/app/validators/ api ' "$DOCKER_LOG"; then
  report 0 "validators run in disposable containers derived from the resolved api image (fake-api-image:2.0) on the drill network, never via the production api container"
else
  report 1 "validators run in disposable containers derived from the resolved api image (fake-api-image:2.0) on the drill network, never via the production api container" "docker_log=$(tr '\n' '|' < "$DOCKER_LOG")"
fi
# OFFHOST_VALIDATOR_IMAGE, when set, must be used verbatim as the validator
# image (a deployment may pin the drill validator image explicitly).
: > "$DOCKER_LOG"
: > "$DOCKER_ENVLOG"
out=$(BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" OFFHOST_PGCONTAINER_ID=pg1 \
  OFFHOST_API_CONTAINER=fake-api-1 OFFHOST_VALIDATOR_IMAGE=drill-validator:v9 \
  DOCKER_LOG="$DOCKER_LOG" DOCKER_ENVLOG="$DOCKER_ENVLOG" \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db drill_db_img --target-project drill \
  --restore-pg container:restore-pg --confirm-target-isolated=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
if [ $rc -eq 0 ] && grep -q '^run --rm --network restore-pg-net -l bodysense.disposable-restore=yes --env-file [^ ]* --entrypoint [^ ]* drill-validator:v9 ' "$DOCKER_LOG"; then
  report 0 "OFFHOST_VALIDATOR_IMAGE pins the disposable validator image (drill-validator:v9)"
else
  report 1 "OFFHOST_VALIDATOR_IMAGE pins the disposable validator image (drill-validator:v9)" "rc=$rc docker_log=$(tr '\n' '|' < "$DOCKER_LOG")"
fi
# The database password (fixed test secret 0123456789abcdef) must never appear
# on any recorded docker argv (host side) or psql/fake-pg argv; it may only be
# present in the --env-file the stub read on behalf of the daemon.  The DSN in
# -database-url must carry no password at all.
if ! grep -q '0123456789abcdef' "$DOCKER_LOG" \
  && grep -q 'postgres://bodysense@restore-pg:5432/drill_db_img?sslmode=disable' "$DOCKER_LOG" \
  && grep -q 'PGPASSWORD=0123456789abcdef' "$DOCKER_ENVLOG"; then
  report 0 "database password never appears in argv; delivered only via PGPASSWORD in an --env-file"
else
  report 1 "database password never appears in argv; delivered only via PGPASSWORD in an --env-file" \
    "docker_log=$(tr '\n' '|' < "$DOCKER_LOG") env_log=$(tr '\n' '|' < "$DOCKER_ENVLOG")"
fi
# Without an explicit OFFHOST_API_CONTAINER the restore path must fall back to
# Compose's default "<project>-api-1" naming (docker-api-1 for project "docker"),
# which is exactly what production runs — the original P1 blocker — and derive
# the disposable validator image from that container's Config.Image.
: > "$DOCKER_LOG"
: > "$DOCKER_ENVLOG"
out=$(BODYSENSE_DEPLOY_ROOT="$ROOT" OFFHOST_PG_PREFIX="$BIN/fake-pg" OFFHOST_PGCONTAINER_ID=pg1 \
  DOCKER_LOG="$DOCKER_LOG" DOCKER_ENVLOG="$DOCKER_ENVLOG" \
  bash "$RESTORE" --object-key "$OBJKEY" --target-db drill_db_b --target-project drill \
  --restore-pg container:restore-pg --confirm-target-isolated=yes --validator-runner docker 2>&1) && rc=0 || rc=$?
if [ $rc -eq 0 ] && grep -q '^run --rm --network restore-pg-net -l bodysense.disposable-restore=yes --env-file [^ ]* --entrypoint [^ ]* docker-api-image:1.0 ' "$DOCKER_LOG"; then
  report 0 "without OFFHOST_API_CONTAINER the restore uses Compose's '<project>-api-1' naming (docker-api-1) for the validator image"
else
  report 1 "without OFFHOST_API_CONTAINER the restore uses Compose's '<project>-api-1' naming (docker-api-1) for the validator image" \
    "rc=$rc docker_log=$(tr '\n' '|' < "$DOCKER_LOG")"
fi
rm -f "$FAKEDOCKER_INSPECT_DIR/fake-api-1.json" "$FAKEDOCKER_INSPECT_DIR/docker-api-1.json" "$FAKEDOCKER_INSPECT_DIR/pg1.json"

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
# 13. restore never hard-codes a literal 'api' container name, and never joins a
#     production api container to the drill network (validators run in separate
#     disposable containers derived from the api image; the restore script only
#     ever docker-runs / docker-execs against the disposable restore database and
#     the disposable validator containers)
# ==============================================================================
if grep -nE 'docker[[:space:]]+exec[[:space:]]+api([[:space:]]|$)' "$RESTORE" >/dev/null 2>&1 \
  || grep -nE 'docker[[:space:]]+network[[:space:]]+connect' "$RESTORE" >/dev/null 2>&1; then
  report 1 "restore runs validators in disposable containers, never on a production api container or a hard-coded 'api'" \
    "found a hard-coded 'docker exec api' or a docker network connect in scripts/restore-production-backup.sh"
else
  report 0 "restore runs validators in disposable containers, never on a production api container or a hard-coded 'api'"
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

# ==============================================================================
# 15. deploy-watch rollback restores the WHOLE managed runtime, not just the
#     stack files (review finding: a failed deployment used to roll back only
#     .env.production / compose / Caddyfile / litellm, leaving the new revision's
#     DR operator scripts and systemd units installed).  Backing up and rolling
#     back a deployment that replaces scripts/ and deploy/systemd/ must return
#     those artifacts to their exact previous state: restored scripts, REMOVED
#     scripts the old runtime did not have, restored/removed systemd units, and
#     host symlinks re-pointed + daemon-reload under BODYSENSE_SYSTEMD_DIR.
# ==============================================================================
DLW_ROOT="$TMP/dlw-root"
DLW_SYSD="$TMP/systemd-etc"
mkdir -p "$DLW_ROOT/docker/litellm" "$DLW_ROOT/scripts" "$DLW_ROOT/deploy/systemd"
cat > "$DLW_ROOT/.env.production" <<ENV
APP_DOMAIN=invalid.invalid
AUTO_DEPLOY_ENABLED=true
REGISTRY=registry.example.test
ACR_NAMESPACE=bodysense
WEB_TAG=prod-latest
API_TAG=prod-latest
AI_TAG=prod-latest
RUNTIME_TAG=prod-latest
DB_USER=bodysense
DB_NAME=bodysense
#OLD-ENV-MARKER
ENV
cat > "$DLW_ROOT/.env.production.local" <<ENV
DB_PASSWORD=0123456789abcdef
ENV
printf 'old-compose\n' > "$DLW_ROOT/docker/docker-compose.prod.yml"
printf 'old-caddyfile\n' > "$DLW_ROOT/docker/Caddyfile"
printf 'old-litellm-config\n' > "$DLW_ROOT/docker/litellm/config.yaml"
printf '#!/usr/bin/env bash\necho OLD-DEPLOY-WATCH\n' > "$DLW_ROOT/scripts/production-deploy-watch.sh"
printf 'old-offhost-s3\n' > "$DLW_ROOT/scripts/offhost-s3.py"
printf 'old-offhost-backup\n' > "$DLW_ROOT/scripts/production-offhost-backup.sh"
printf 'old-backup-unit\n' > "$DLW_ROOT/deploy/systemd/bodysense-offhost-backup.service"
printf 'old-backup-timer\n' > "$DLW_ROOT/deploy/systemd/bodysense-offhost-backup.timer"
cat > "$DLW_ROOT/.deploy-state" <<STATE
revision=r1
runtime_revision=r1
runtime_source=acr
deployed_at=2026-08-23T00:00:00Z
STATE
DLW_BUNDLE="$TMP/new-runtime-bundle"
mkdir -p "$DLW_BUNDLE/docker/litellm" "$DLW_BUNDLE/scripts" "$DLW_BUNDLE/deploy/systemd"
cat > "$DLW_BUNDLE/.env.production" <<ENV
APP_DOMAIN=invalid.invalid
AUTO_DEPLOY_ENABLED=true
REGISTRY=registry.example.test
ACR_NAMESPACE=bodysense
WEB_TAG=prod-latest
API_TAG=prod-latest
AI_TAG=prod-latest
RUNTIME_TAG=prod-latest
DB_USER=bodysense
DB_NAME=bodysense
#NEW-ENV-MARKER
ENV
printf 'new-compose\n' > "$DLW_BUNDLE/docker/docker-compose.prod.yml"
printf 'new-caddyfile\n' > "$DLW_BUNDLE/docker/Caddyfile"
printf 'new-litellm-config\n' > "$DLW_BUNDLE/docker/litellm/config.yaml"
cp "$PWD/scripts/production-deploy-watch.sh" "$DLW_BUNDLE/scripts/production-deploy-watch.sh"
chmod 0755 "$DLW_BUNDLE/scripts/production-deploy-watch.sh"
printf 'new-offhost-s3\n' > "$DLW_BUNDLE/scripts/offhost-s3.py"
printf 'new-offhost-backup\n' > "$DLW_BUNDLE/scripts/production-offhost-backup.sh"
printf 'new-offhost-restore\n' > "$DLW_BUNDLE/scripts/restore-production-backup.sh"
for u in bodysense-offhost-backup.service bodysense-offhost-backup.timer bodysense-offhost-freshness.service bodysense-offhost-freshness.timer; do
  printf 'new-%s\n' "$u" > "$DLW_BUNDLE/deploy/systemd/$u"
done

# Deploy-capable fake docker: compose ps/exec/pull/up/config, image inspect for
# revision labels, container inspect for image-id/health, docker cp of the
# ACR runtime bundle.  Replaces the restore-only stub; section 15 is the last
# section, so no earlier test depends on the previous stub afterwards.
cat > "$BIN/docker" <<'DLWDOCKSTUB'
#!/usr/bin/env bash
{
  printf '%s\n' "$*"
} >> "${FAKEDOCKER_LOG:-/dev/null}"
case "$1" in
  compose)
    case "$*" in
      *pg_dump*) printf 'pre-deploy postgres dump fixture\n'; exit 0 ;;
    esac
    case "$*" in
      *" ps -q "*) printf 'faectr-%s\n' "${!#}"; exit 0 ;;
    esac
    exit 0 ;;
  image) printf '%s\n' "${FAKEDOCKER_IMAGE_REV:-r2}"; exit 0 ;;
  inspect)
    case "$*" in
      *'{{.Image}}'*) printf 'img-%s\n' "${2:-unknown}"; exit 0 ;;
      *State.Health*) printf 'healthy\n'; exit 0 ;;
      *) printf 'running\n'; exit 0 ;;
    esac
    ;;
  create) printf 'faectr-runtime\n'; exit 0 ;;
  cp)
    if [[ "$2" == *":/runtime/scripts/production-deploy-watch.sh" ]] && [ -n "${FAKEDOCKER_RUNTIME_BUNDLE:-}" ]; then
      cp "$FAKEDOCKER_RUNTIME_BUNDLE/scripts/production-deploy-watch.sh" "$3"
      exit 0
    fi
    if [[ "$2" == *":/runtime/." ]] && [ -n "${FAKEDOCKER_RUNTIME_BUNDLE:-}" ]; then
      cp -R "$FAKEDOCKER_RUNTIME_BUNDLE/." "$3"
      exit 0
    fi
    exit 0 ;;
  *) exit 0 ;;
esac
exit 0
DLWDOCKSTUB
chmod +x "$BIN/docker"
cat > "$BIN/systemctl" <<'SYSCTLSTUB'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "${SYSTEMCTL_LOG:-/dev/null}"
exit 0
SYSCTLSTUB
chmod +x "$BIN/systemctl"
cat > "$BIN/curl" <<'CURLSTUB'
#!/usr/bin/env bash
printf 'curl %s\n' "$*" >> "${FAKE_CURL_LOG:-/dev/null}"
n=$(cat "${FAKE_CURL_COUNT:-/dev/null}" 2>/dev/null || printf 0)
n=$((n + 1))
printf '%s' "$n" > "${FAKE_CURL_COUNT:-/dev/null}"
# The FIRST external health probe fails (deploy fails -> rollback); the health
# probe inside the rollback (after the old runtime is restored) succeeds, so the
# rollback itself is certified restored.  Hermetic-only; production uses systemd,
# docker health and a reachable APP_DOMAIN.
if [ "$n" -eq 1 ]; then
  printf 'curl: (7) Failed to connect to invalid.invalid\n' >&2
  exit 7
fi
exit 0
CURLSTUB
chmod +x "$BIN/curl"

mkdir -p "$DLW_SYSD"
run_deploy_watch() {
  FAKEDOCKER_LOG="$TMP/deploy-docker.log" SYSTEMCTL_LOG="$TMP/systemctl.log" \
  FAKE_CURL_LOG="$TMP/curl.log" FAKE_CURL_COUNT="$TMP/curl-count" \
  FAKEDOCKER_RUNTIME_BUNDLE="$DLW_BUNDLE" FAKEDOCKER_IMAGE_REV=r2 \
  BODYSENSE_SYSTEMD_DIR="$DLW_SYSD" BODYSENSE_DEPLOY_ROOT="$DLW_ROOT" \
  bash "$PWD/scripts/production-deploy-watch.sh" --force 2>&1
}
out=$(run_deploy_watch) && rc=0 || rc=$?
if [ $rc -ne 0 ] && [ -f "$DLW_ROOT/.deploy-blocked" ]; then
  report 0 "deploy-watch failed deployment marks the revision blocked"
else
  report 1 "deploy-watch failed deployment marks the revision blocked" "rc=$rc blocked=$([ -f "$DLW_ROOT/.deploy-blocked" ] && echo yes || echo no)"
fi
if [ $rc -ne 0 ] && grep -q '^rollback=restored$' "$DLW_ROOT/.deploy-blocked" \
  && grep -q "^revision=r2$" "$DLW_ROOT/.deploy-blocked"; then
  report 0 "deploy-watch rollback restores the previous runtime and is certified restored"
else
  report 1 "deploy-watch rollback restores the previous runtime and is certified restored" "rc=$rc $(tr '\n' '|' < "$DLW_ROOT/.deploy-blocked" 2>/dev/null)"
fi
if [ $rc -ne 0 ] && grep -q 'OLD-ENV-MARKER' "$DLW_ROOT/.env.production" \
  && ! grep -q 'NEW-ENV-MARKER' "$DLW_ROOT/.env.production" \
  && grep -q '^old-compose$' "$DLW_ROOT/docker/docker-compose.prod.yml" \
  && grep -q '^old-caddyfile$' "$DLW_ROOT/docker/Caddyfile" \
  && grep -q '^old-litellm-config$' "$DLW_ROOT/docker/litellm/config.yaml"; then
  report 0 "rollback restores .env.production, compose, Caddyfile and litellm config from the pre-deploy archive"
else
  report 1 "rollback restores .env.production, compose, Caddyfile and litellm config from the pre-deploy archive" "rc=$rc"
fi
if [ $rc -ne 0 ] \
  && grep -q 'OLD-DEPLOY-WATCH' "$DLW_ROOT/scripts/production-deploy-watch.sh" \
  && grep -q '^old-offhost-s3$' "$DLW_ROOT/scripts/offhost-s3.py" \
  && grep -q '^old-offhost-backup$' "$DLW_ROOT/scripts/production-offhost-backup.sh" \
  && [ ! -e "$DLW_ROOT/scripts/restore-production-backup.sh" ]; then
  report 0 "rollback restores the DR scripts and REMOVES scripts the old runtime did not have"
else
  report 1 "rollback restores the DR scripts and REMOVES scripts the old runtime did not have" "rc=$rc scripts_dir=$(ls "$DLW_ROOT/scripts" 2>/dev/null | tr '\n' '|')"
fi
if [ $rc -ne 0 ] && grep -q '^old-backup-unit$' "$DLW_ROOT/deploy/systemd/bodysense-offhost-backup.service" \
  && grep -q '^old-backup-timer$' "$DLW_ROOT/deploy/systemd/bodysense-offhost-backup.timer" \
  && [ ! -e "$DLW_ROOT/deploy/systemd/bodysense-offhost-freshness.service" ] \
  && [ ! -e "$DLW_ROOT/deploy/systemd/bodysense-offhost-freshness.timer" ]; then
  report 0 "rollback restores the previous systemd units and removes units it did not have"
else
  report 1 "rollback restores the previous systemd units and removes units it did not have" "rc=$rc $(ls "$DLW_ROOT/deploy/systemd" 2>/dev/null | tr '\n' '|')"
fi
if [ $rc -ne 0 ] \
  && [ ! -e "$DLW_SYSD/bodysense-offhost-backup.service" ] \
  && [ ! -e "$DLW_SYSD/bodysense-offhost-backup.timer" ] \
  && [ ! -e "$DLW_SYSD/bodysense-offhost-freshness.service" ] \
  && [ ! -e "$DLW_SYSD/bodysense-offhost-freshness.timer" ] \
  && grep -q 'daemon-reload' "$TMP/systemctl.log" \
  && grep -q 'disable --now bodysense-offhost-backup.timer bodysense-offhost-freshness.timer' "$TMP/systemctl.log" \
  && grep -q 'reset-failed bodysense-offhost-backup.service bodysense-offhost-freshness.service bodysense-offhost-backup.timer bodysense-offhost-freshness.timer' "$TMP/systemctl.log"; then
  report 0 "rollback preserves legacy unit files for compatibility, clears stale failure state, and never resurrects their host scheduler"
else
  report 1 "rollback preserves legacy unit files for compatibility but never resurrects their host scheduler" "rc=$rc symds_dir=$(ls "$DLW_SYSD" 2>/dev/null | tr '\n' '|') sysctl=$(tr '\n' '|' < "$TMP/systemctl.log" 2>/dev/null)"
fi
if [ $rc -ne 0 ] && ls "$DLW_ROOT/runtime-backups"/r1-*/scripts/production-deploy-watch.sh >/dev/null 2>&1 \
  && [ "$(find "$DLW_ROOT/runtime-backups" -mindepth 1 -maxdepth 1 -type d | wc -l)" -ge 1 ]; then
  report 0 "the pre-deploy archive snapshots the DR scripts and systemd units for rollback"
else
  report 1 "the pre-deploy archive snapshots the DR scripts and systemd units for rollback" "rc=$rc archives=$(ls "$DLW_ROOT/runtime-backups" 2>/dev/null | tr '\n' '|')"
fi

# ==============================================================================
# 16. A runtime release that changes the deploy watcher must hand the still-
#     side-effect-free transaction to the TARGET watcher before creating a DB
#     backup or touching schema/services.  This prevents a self-updating old
#     watcher from certifying success while omitting a service or health gate
#     introduced by the target runtime.
# ==============================================================================
HDO_ROOT="$TMP/handoff-root"
HDO_SYSD="$TMP/handoff-systemd"
HDO_BUNDLE="$TMP/handoff-runtime-bundle"
HDO_MARKER="$TMP/handoff-target-executed"
mkdir -p "$HDO_ROOT/docker/litellm" "$HDO_ROOT/scripts" "$HDO_ROOT/deploy/systemd" \
  "$HDO_ROOT/backups" "$HDO_ROOT/runtime-backups" "$HDO_SYSD"
cat > "$HDO_ROOT/.env.production" <<ENV
APP_DOMAIN=invalid.invalid
AUTO_DEPLOY_ENABLED=true
REGISTRY=registry.example.test
ACR_NAMESPACE=bodysense
WEB_TAG=prod-latest
API_TAG=prod-latest
AI_TAG=prod-latest
RUNTIME_TAG=prod-latest
DB_USER=bodysense
DB_NAME=bodysense
ENV
cat > "$HDO_ROOT/.env.production.local" <<ENV
DB_PASSWORD=0123456789abcdef
ENV
printf 'old-compose\n' > "$HDO_ROOT/docker/docker-compose.prod.yml"
printf 'old-caddyfile\n' > "$HDO_ROOT/docker/Caddyfile"
printf 'old-litellm-config\n' > "$HDO_ROOT/docker/litellm/config.yaml"
printf '#!/usr/bin/env bash\necho OLD-INSTALLED-WATCHER\n' > "$HDO_ROOT/scripts/production-deploy-watch.sh"
printf 'old-offhost-s3\n' > "$HDO_ROOT/scripts/offhost-s3.py"
printf 'old-offhost-backup\n' > "$HDO_ROOT/scripts/production-offhost-backup.sh"
printf 'old-backup-unit\n' > "$HDO_ROOT/deploy/systemd/bodysense-offhost-backup.service"
printf 'old-backup-timer\n' > "$HDO_ROOT/deploy/systemd/bodysense-offhost-backup.timer"
cat > "$HDO_ROOT/.deploy-state" <<STATE
revision=r1
runtime_revision=r1
runtime_source=acr
deployed_at=2026-08-23T00:00:00Z
STATE

cp -R "$DLW_BUNDLE" "$HDO_BUNDLE"
python3 - "$PWD/scripts/production-deploy-watch.sh" "$HDO_BUNDLE/scripts/production-deploy-watch.sh" <<'PYMARK'
from pathlib import Path
import sys
source = Path(sys.argv[1]).read_text()
marker = '''set -Eeuo pipefail
if [ -n "${BODYSENSE_HANDOFF_TEST_MARKER:-}" ]; then
  printf 'target-watcher-executed\\n' >> "$BODYSENSE_HANDOFF_TEST_MARKER"
fi
'''
source = source.replace('set -Eeuo pipefail\n', marker, 1)
Path(sys.argv[2]).write_text(source)
PYMARK
chmod 0755 "$HDO_BUNDLE/scripts/production-deploy-watch.sh"
: > "$TMP/handoff-docker.log"
: > "$TMP/handoff-systemctl.log"
: > "$TMP/handoff-curl.log"
printf '1' > "$TMP/handoff-curl-count" # make the deployment health probe succeed

run_handoff_watch() {
  FAKEDOCKER_LOG="$TMP/handoff-docker.log" SYSTEMCTL_LOG="$TMP/handoff-systemctl.log" \
  FAKE_CURL_LOG="$TMP/handoff-curl.log" FAKE_CURL_COUNT="$TMP/handoff-curl-count" \
  FAKEDOCKER_RUNTIME_BUNDLE="$HDO_BUNDLE" FAKEDOCKER_IMAGE_REV=r2 \
  BODYSENSE_HANDOFF_TEST_MARKER="$HDO_MARKER" \
  BODYSENSE_SYSTEMD_DIR="$HDO_SYSD" BODYSENSE_DEPLOY_ROOT="$HDO_ROOT" \
  bash "$PWD/scripts/production-deploy-watch.sh" --force 2>&1
}
handoff_out=$(run_handoff_watch) && handoff_rc=0 || handoff_rc=$?
printf '%s\n' "$handoff_out"
handoff_backups=$(find "$HDO_ROOT/backups" -maxdepth 1 -type f -name 'bodysense-pre-*.dump' | wc -l | tr -d ' ')
handoff_temp=$(find "$HDO_ROOT/scripts" -maxdepth 1 -type f -name '.deploy-watch-handoff-*' | wc -l | tr -d ' ')
if [ "$handoff_rc" -eq 0 ] \
  && grep -q 'handing off before backup/schema/service changes' <<<"$handoff_out" \
  && [ "$(grep -c '^target-watcher-executed$' "$HDO_MARKER" 2>/dev/null || true)" -eq 1 ] \
  && grep -q '^revision=r2$' "$HDO_ROOT/.deploy-state" \
  && [ "$handoff_backups" -eq 1 ] \
  && [ "$handoff_temp" -eq 0 ] \
  && [ ! -e "$HDO_ROOT/.deploy-blocked" ]; then
  report 0 "deploy-watch hands a changed runtime contract to the target watcher before one transactional backup/deploy"
else
  report 1 "deploy-watch hands a changed runtime contract to the target watcher before one transactional backup/deploy" \
    "rc=$handoff_rc backups=$handoff_backups temp=$handoff_temp marker=$(cat "$HDO_MARKER" 2>/dev/null | tr '\n' '|') state=$(tr '\n' '|' < "$HDO_ROOT/.deploy-state" 2>/dev/null)"
fi

# A changed target watcher that drops the handoff protocol is not safe to run.
# The current watcher must reject it BEFORE backup/runtime/schema/service side effects.
rm -rf "$HDO_ROOT/backups"/* "$HDO_ROOT/runtime-backups"/* "$HDO_ROOT/.runtime-next" "$HDO_ROOT/.deploy-blocked"
cat > "$HDO_ROOT/.env.production" <<ENV
APP_DOMAIN=invalid.invalid
AUTO_DEPLOY_ENABLED=true
REGISTRY=registry.example.test
ACR_NAMESPACE=bodysense
WEB_TAG=prod-latest
API_TAG=prod-latest
AI_TAG=prod-latest
RUNTIME_TAG=prod-latest
DB_USER=bodysense
DB_NAME=bodysense
#HANDOFF-OLD-ENV
ENV
printf 'handoff-old-compose\n' > "$HDO_ROOT/docker/docker-compose.prod.yml"
printf 'handoff-old-caddy\n' > "$HDO_ROOT/docker/Caddyfile"
printf '#!/usr/bin/env bash\necho HANDOFF-OLD-WATCHER\n' > "$HDO_ROOT/scripts/production-deploy-watch.sh"
cat > "$HDO_ROOT/.deploy-state" <<STATE
revision=r1
runtime_revision=r1
runtime_source=acr
deployed_at=2026-08-23T00:00:00Z
STATE
HFI_BUNDLE="$TMP/handoff-incompatible-runtime-bundle"
cp -R "$DLW_BUNDLE" "$HFI_BUNDLE"
printf '#!/usr/bin/env bash\nset -Eeuo pipefail\necho incompatible-target\n' > "$HFI_BUNDLE/scripts/production-deploy-watch.sh"
chmod 0755 "$HFI_BUNDLE/scripts/production-deploy-watch.sh"
: > "$TMP/handoff-incompatible-docker.log"
set +e
hfi_out=$(FAKEDOCKER_LOG="$TMP/handoff-incompatible-docker.log" \
  FAKEDOCKER_RUNTIME_BUNDLE="$HFI_BUNDLE" FAKEDOCKER_IMAGE_REV=r2 \
  BODYSENSE_SYSTEMD_DIR="$HDO_SYSD" BODYSENSE_DEPLOY_ROOT="$HDO_ROOT" \
  bash "$PWD/scripts/production-deploy-watch.sh" --force 2>&1)
hfi_rc=$?
set -e
hfi_backups=$(find "$HDO_ROOT/backups" -maxdepth 1 -type f -name 'bodysense-pre-*.dump' | wc -l | tr -d ' ')
hfi_temp=$(find "$HDO_ROOT/scripts" -maxdepth 1 -type f -name '.deploy-watch-handoff-*' | wc -l | tr -d ' ')
if [ "$hfi_rc" -ne 0 ] \
  && grep -q 'changed without compatible handoff protocol 1' <<<"$hfi_out" \
  && [ "$hfi_backups" -eq 0 ] \
  && [ "$hfi_temp" -eq 0 ] \
  && grep -q '^revision=r1$' "$HDO_ROOT/.deploy-state" \
  && grep -q 'HANDOFF-OLD-ENV' "$HDO_ROOT/.env.production" \
  && grep -q '^handoff-old-compose$' "$HDO_ROOT/docker/docker-compose.prod.yml" \
  && [ ! -e "$HDO_ROOT/.deploy-blocked" ]; then
  report 0 "deploy-watch rejects an incompatible target watcher before any deployment side effect"
else
  report 1 "deploy-watch rejects an incompatible target watcher before any deployment side effect" \
    "rc=$hfi_rc backups=$hfi_backups temp=$hfi_temp state=$(tr '\n' '|' < "$HDO_ROOT/.deploy-state" 2>/dev/null) out=$(tr '\n' '|' <<<"$hfi_out")"
fi

echo
echo "offhost DR unit tests: PASS=$PASS FAIL=$FAIL"
[ "$FAIL" -eq 0 ]
