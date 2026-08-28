#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

PG16_IMAGE="${POSTGRES_MAJOR_TEST_SOURCE_IMAGE:-pgvector/pgvector:pg16}"
PG18_IMAGE="${POSTGRES_MAJOR_TEST_TARGET_IMAGE:-pgvector/pgvector:pg18}"
BASE="/tmp/bodysense-pg-major-test-$$"
mkdir -p "$BASE"

projects=()
volumes=()
cleanup() {
  local project volume
  for project in "${projects[@]:-}"; do
    docker compose -p "$project" -f "$BASE/$project-source.yml" --env-file "$BASE/$project-source.env" down --remove-orphans >/dev/null 2>&1 || true
    docker compose -p "$project" -f "$BASE/$project/root/docker/docker-compose.prod.yml" --env-file "$BASE/$project/root/.env.production" --env-file "$BASE/$project/root/.env.production.local" down --remove-orphans >/dev/null 2>&1 || true
  done
  for volume in "${volumes[@]:-}"; do docker volume rm "$volume" >/dev/null 2>&1 || true; done
  rm -rf "$BASE"
}
trap cleanup EXIT

wait_healthy() {
  local project="$1" compose_file="$2" public_env="$3" secret_env="$4" start id health
  start=$(date +%s)
  while :; do
    id=$(docker compose -p "$project" -f "$compose_file" --env-file "$public_env" --env-file "$secret_env" ps -q postgres 2>/dev/null || true)
    if [ -n "$id" ]; then
      health=$(docker inspect "$id" --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' 2>/dev/null || true)
      [ "$health" = healthy ] && { printf '%s' "$id"; return 0; }
    fi
    [ $(( $(date +%s) - start )) -lt 90 ] || return 1
    sleep 1
  done
}

write_target_runtime() {
  local dir="$1" target_volume="$2"
  mkdir -p "$dir/root/docker" "$dir/root/backups"
  cat > "$dir/root/.env.production" <<ENV
DB_NAME=bodysense
DB_USER=bodysense
POSTGRES_MAJOR=18
POSTGRES_DATA_VOLUME=$target_volume
ENV
  cat > "$dir/root/.env.production.local" <<'ENV'
DB_PASSWORD=pg-major-test
ENV
  chmod 600 "$dir/root/.env.production.local"
  cat > "$dir/root/docker/docker-compose.prod.yml" <<YAML
services:
  postgres:
    image: $PG18_IMAGE
    environment:
      POSTGRES_DB: bodysense
      POSTGRES_USER: bodysense
      POSTGRES_PASSWORD: pg-major-test
    volumes:
      - postgres-pg18:/var/lib/postgresql
    healthcheck:
      test: ['CMD-SHELL', 'pg_isready -U bodysense -d bodysense']
      interval: 1s
      timeout: 3s
      retries: 30
volumes:
  postgres-pg18:
    name: \${POSTGRES_DATA_VOLUME}
YAML
}

write_source_runtime() {
  local project="$1" source_volume="$2"
  cat > "$BASE/$project-source.env" <<'ENV'
DB_PASSWORD=pg-major-test
ENV
  cat > "$BASE/$project-source.yml" <<YAML
services:
  postgres:
    image: $PG16_IMAGE
    environment:
      POSTGRES_DB: bodysense
      POSTGRES_USER: bodysense
      POSTGRES_PASSWORD: pg-major-test
    volumes:
      - postgres-source:/var/lib/postgresql/data
    healthcheck:
      test: ['CMD-SHELL', 'pg_isready -U bodysense -d bodysense']
      interval: 1s
      timeout: 3s
      retries: 30
volumes:
  postgres-source:
    name: $source_volume
YAML
}

seed_source() {
  local id="$1"
  docker exec -i "$id" psql -v ON_ERROR_STOP=1 -U bodysense -d bodysense >/dev/null <<'SQL'
CREATE EXTENSION IF NOT EXISTS vector;
CREATE TABLE schema_migrations (version bigint PRIMARY KEY, dirty boolean NOT NULL);
INSERT INTO schema_migrations(version, dirty) VALUES (59, false);
CREATE TABLE migration_probe (id bigint PRIMARY KEY, body text NOT NULL);
INSERT INTO migration_probe(id, body) VALUES (1, 'one'), (2, 'two'), (3, 'three');
SQL
}

run_case() {
  local mode="$1"
  local project="bspgmajor-${mode}-$$"
  local source_volume="bspgmajor-${mode}-pg16-$$"
  local target_volume="bspgmajor-${mode}-pg18-$$"
  local dir="$BASE/$project" source_id target_id state major count mount
  projects+=("$project")
  volumes+=("$source_volume" "$target_volume")
  write_source_runtime "$project" "$source_volume"
  write_target_runtime "$dir" "$target_volume"

  docker compose -p "$project" -f "$BASE/$project-source.yml" --env-file "$BASE/$project-source.env" up -d postgres >/dev/null
  source_id=$(wait_healthy "$project" "$BASE/$project-source.yml" "$BASE/$project-source.env" "$dir/root/.env.production.local")
  seed_source "$source_id"

  BODYSENSE_DEPLOY_ROOT="$dir/root" BODYSENSE_COMPOSE_PROJECT="$project" POSTGRES_MAJOR_RELEASE_REVISION="test-$mode" \
    scripts/production-postgres-major-upgrade.sh cutover >/dev/null
  state=$(sed -n 's/^status=//p' "$dir/root/.postgres-major-upgrade-state")
  [ "$state" = cutover_complete ]
  target_id=$(docker ps -q --filter "label=com.docker.compose.project=$project" --filter 'label=com.docker.compose.service=postgres' | head -1)
  major=$(docker exec "$target_id" psql -U bodysense -d bodysense -Atc "show server_version_num")
  [ $((major / 10000)) -eq 18 ]
  count=$(docker exec "$target_id" psql -U bodysense -d bodysense -Atc 'select count(*) from migration_probe')
  [ "$count" = 3 ]
  mount=$(docker inspect "$target_id" --format '{{range .Mounts}}{{if eq .Destination "/var/lib/postgresql"}}{{.Name}}{{end}}{{end}}')
  [ "$mount" = "$target_volume" ]
  docker volume inspect "$source_volume" >/dev/null

  if [ "$mode" = commit ]; then
    BODYSENSE_DEPLOY_ROOT="$dir/root" BODYSENSE_COMPOSE_PROJECT="$project" POSTGRES_MAJOR_RELEASE_REVISION="test-$mode" \
      scripts/production-postgres-major-upgrade.sh commit >/dev/null
    [ "$(sed -n 's/^status=//p' "$dir/root/.postgres-major-upgrade-state")" = committed ]
    docker volume inspect "$source_volume" >/dev/null
  else
    BODYSENSE_DEPLOY_ROOT="$dir/root" BODYSENSE_COMPOSE_PROJECT="$project" POSTGRES_MAJOR_RELEASE_REVISION="test-$mode" \
      POSTGRES_MAJOR_ROLLBACK_COMPOSE="$BASE/$project-source.yml" \
      POSTGRES_MAJOR_ROLLBACK_ENV="$BASE/$project-source.env" \
      POSTGRES_MAJOR_EXPECTED_SCHEMA='59:false' \
      scripts/production-postgres-major-upgrade.sh rollback >/dev/null
    [ "$(sed -n 's/^status=//p' "$dir/root/.postgres-major-upgrade-state")" = rolled_back ]
    source_id=$(docker ps -q --filter "label=com.docker.compose.project=$project" --filter 'label=com.docker.compose.service=postgres' | head -1)
    major=$(docker exec "$source_id" psql -U bodysense -d bodysense -Atc 'show server_version_num')
    [ $((major / 10000)) -eq 16 ]
    count=$(docker exec "$source_id" psql -U bodysense -d bodysense -Atc 'select count(*) from migration_probe')
    [ "$count" = 3 ]
    mount=$(docker inspect "$source_id" --format '{{range .Mounts}}{{if eq .Destination "/var/lib/postgresql/data"}}{{.Name}}{{end}}{{end}}')
    [ "$mount" = "$source_volume" ]
    if docker volume inspect "$target_volume" >/dev/null 2>&1; then
      echo "rolled-back target volume still exists: $target_volume" >&2
      exit 1
    fi
  fi
}

run_case commit
run_case rollback

echo POSTGRES_MAJOR_UPGRADE_TRANSACTION=PASS
