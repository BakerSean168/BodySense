#!/usr/bin/env bash
# TP-D1 real Go owner-transaction concurrency against a disposable PG18.
# No published TCP port, Internet access, real accounts or shared database.
set -Eeuo pipefail
root="$(cd "$(dirname "$0")/../.." && pwd)"
image="pgvector/pgvector:pg18@sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a"
docker image inspect "$image" >/dev/null
socket_dir="$(mktemp -d /tmp/bs-tpd1-owner-XXXXXXXX)"
chmod 0777 "$socket_dir"
container="bs-tpd1-owner-lock-$$-$RANDOM"
cleanup() {
  if [[ -n "$container" ]]; then
    docker rm -f "$container" >/dev/null 2>&1 || true
  fi
  if [[ -d "$socket_dir" ]]; then
    # Only our own mktemp directory is ever touched.
    find "$socket_dir" -mindepth 1 -maxdepth 1 -type f -delete 2>/dev/null || true
    rmdir "$socket_dir" 2>/dev/null || true
  fi
}
trap cleanup EXIT
docker run --rm --pull never -d --name "$container" --network none \
  --memory 1g --cpus 2 --pids-limit 192 \
  --mount "type=bind,src=$socket_dir,dst=/tmp/pgsocket" \
  -e POSTGRES_PASSWORD=synthetic-only \
  -e POSTGRES_DB=synthetic_tpd1 \
  "$image" postgres -c unix_socket_directories=/tmp/pgsocket,/var/run/postgresql \
  -c unix_socket_permissions=0777 -c listen_addresses='' >/dev/null
ready=0
for _ in $(seq 1 55); do
  if docker logs "$container" 2>&1 | grep -q 'PostgreSQL init process complete' &&
    docker exec "$container" psql -X -q -t -A -v ON_ERROR_STOP=1 \
      -h /tmp/pgsocket -U postgres -d synthetic_tpd1 \
      -c 'SELECT 1' 2>/dev/null | grep -qx 1; then
    ready=1
    break
  fi
  sleep 1
done
if [[ "$ready" != 1 ]]; then
  docker logs "$container" 2>&1 | tail -30 >&2
  echo 'TP-D1 local Unix-socket PG18 fixture did not start' >&2
  exit 1
fi
count=0
while IFS= read -r sql_file; do
  docker exec -i "$container" psql -X -q -v ON_ERROR_STOP=1 \
    -h /tmp/pgsocket -U postgres -d synthetic_tpd1 < "$sql_file" >/dev/null
  count=$((count + 1))
done < <(find "$root/apps/api/migrations" -maxdepth 1 -type f -name '*.up.sql' | sort -V)
if [[ "$count" -lt 12 ]]; then
  echo "TP-D1 expected at least 12 canonical migrations; found $count" >&2
  exit 1
fi
docker exec -i "$container" psql -X -q -v ON_ERROR_STOP=1 \
  -h /tmp/pgsocket -U postgres -d synthetic_tpd1 \
  < "$root/scripts/plan/sql/tp-d1-multi-active-forward-preview.sql" >/dev/null
echo "TP_D1_NATIVE_PG18_CANONICAL_MIGRATIONS=$count"
(
  cd "$root/apps/api"
  BODYSENSE_TP_D1_SYNTHETIC_DATABASE_URL="host=$socket_dir user=postgres password=synthetic-only dbname=synthetic_tpd1 sslmode=disable" \
    GOPROXY=off GOTOOLCHAIN=local \
    go test -race -count=1 -timeout 4m ./internal/repository \
      -run '^TestTrainingOwnerLockConcurrentNativePG18$' -v
)
echo 'TP_D1_REAL_GO_OWNER_LOCK_NATIVE_PG18=PASS'
