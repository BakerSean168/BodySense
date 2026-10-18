#!/usr/bin/env bash
# Isolated, bounded PG18 regression runner. Never connects to a live database.
set -Eeuo pipefail
repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
image='pgvector/pgvector:pg18@sha256:2ba9ca5f2e7daa0f0e7723cba1ee9167bab54efd3640516a44ac1a928dd67e7a'
# Cooperating local writers serialize heavyweight validation via this lock.
exec 9>/tmp/bodysense-heavy-validation.lock
flock -w 300 9
socket_dir="$(mktemp -d /tmp/bs-bsagent001-pg18-XXXXXX)"
chmod 777 "$socket_dir"
container="bs-bsagent001-pg18-$$"
cleanup() {
 docker exec -u 0 "$container" sh -c 'chmod 0777 /var/run/postgresql; chown "$1:$2" /var/run/postgresql' sh "$(id -u)" "$(id -g)" >/dev/null 2>&1 || true
 docker rm -f "$container" >/dev/null 2>&1 || true
 rm -rf "$socket_dir"
}
trap cleanup EXIT
docker image inspect "$image" >/dev/null
docker run --rm --pull never -d --name "$container" --network none \
 --memory 512m --cpus 1 --pids-limit 128 \
 -v "$socket_dir:/var/run/postgresql" \
 -e POSTGRES_PASSWORD=synthetic-bsagent001-only -e POSTGRES_DB=synthetic_bsagent001 "$image" >/dev/null
ready=0
for _ in $(seq 1 60); do
 if docker logs "$container" 2>&1 | rg -q 'PostgreSQL init process complete' && \
 docker exec "$container" pg_isready -U postgres -d synthetic_bsagent001 >/dev/null 2>&1; then ready=1; break; fi
 sleep 1
done
if [[ "$ready" != 1 ]]; then docker logs --tail 30 "$container"; exit 1; fi
export BODYSENSE_INTEGRATION_DATABASE_URL="host=$socket_dir user=postgres password=synthetic-bsagent001-only dbname=synthetic_bsagent001 sslmode=disable"
cd "$repo_root/apps/api"
GOMAXPROCS=2 go run ./cmd/migration-validator -database-url "postgres://postgres:synthetic-bsagent001-only@/synthetic_bsagent001?host=$socket_dir&sslmode=disable"
if [[ "${BSAGENT001_FULL_CHECKS:-0}" == 1 ]]; then
 GOMAXPROCS=2 go run ./cmd/domain-validator -database-url "$BODYSENSE_INTEGRATION_DATABASE_URL"
 GOMAXPROCS=2 go vet ./...
 GOMAXPROCS=2 go test -p 1 -race -count=1 ./...
else
 GOMAXPROCS=2 go test -p 1 -race -count=1 -v ./internal/consultation -run '^(TestSuggestion|TestInteractionResume)' "$@"

fi
