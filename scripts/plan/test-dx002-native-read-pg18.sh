#!/usr/bin/env bash
# PG18 negative/owner projection tests against an isolated synthetic DB only.
# No staging/prod credentials, migrations or user data are accessed.
set -Eeuo pipefail
root="$(cd "$(dirname "$0")/../.." && pwd)"
image='pgvector/pgvector:pg18@sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a'
exec 9>/tmp/bodysense-heavy-validation.lock
flock -w 300 9
socket_dir="$(mktemp -d /tmp/bs-dx002-read-pg18-XXXXXX)"
chmod 777 "$socket_dir"
container="bs-dx002-read-$$"
cleanup() {
  docker exec -u 0 "$container" sh -c 'chmod 0777 /var/run/postgresql; chown "$1:$2" /var/run/postgresql' sh "$(id -u)" "$(id -g)" >/dev/null 2>&1 || true
  docker rm -f "$container" >/dev/null 2>&1 || true
  rm -rf "$socket_dir"
}
trap cleanup EXIT
docker image inspect "$image" >/dev/null
docker run --rm --pull never -d --name "$container" --network none --memory 512m --cpus 1 --pids-limit 128 \
  -v "$socket_dir:/var/run/postgresql" -e POSTGRES_PASSWORD=synthetic-dx002-read-only \
  -e POSTGRES_DB=synthetic_dx002_read "$image" >/dev/null
ready=0
for _ in $(seq 1 60); do
  if docker logs "$container" 2>&1 | grep -q 'PostgreSQL init process complete' && \
    docker exec "$container" pg_isready -U postgres -d synthetic_dx002_read >/dev/null 2>&1; then
    ready=1; break
  fi
  sleep 1
done
if [[ "$ready" != 1 ]]; then docker logs --tail 25 "$container"; exit 1; fi
export BODYSENSE_INTEGRATION_DATABASE_URL="host=$socket_dir user=postgres password=synthetic-dx002-read-only dbname=synthetic_dx002_read sslmode=disable"
cd "$root/apps/api"
GOMAXPROCS=2 go run ./cmd/migration-validator -database-url "postgres://postgres:synthetic-dx002-read-only@/synthetic_dx002_read?host=$socket_dir&sslmode=disable"
GOMAXPROCS=2 go test -p 1 -count=1 -v ./internal/repository -run '^TestNativeDiagnosis'
GOMAXPROCS=2 go test -p 1 -count=1 -v ./internal/service ./internal/transport/httpapi -run '^TestNativeDiagnosis'
