#!/usr/bin/env bash
set -euo pipefail

ROOT="${BODYSENSE_REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
HOST="${DB_HOST:-127.0.0.1}"
PORT="${DB_PORT:-5432}"
USER_NAME="${DB_USER:-bodysense}"
PASSWORD="${DB_PASSWORD:-bodysense123}"
ADMIN_DB="${DB_ADMIN_NAME:-postgres}"
GOOD_DB="bodysense_upload_storage_good_${RANDOM}_$$"
BAD_DB="bodysense_upload_storage_bad_${RANDOM}_$$"
BASELINE="$ROOT/apps/api/migrations/baselines/production-pg16-v29.sql"

for cmd in psql createdb dropdb go; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "required command not found: $cmd" >&2; exit 2; }
done

export PGPASSWORD="$PASSWORD"
cleanup() {
  dropdb --if-exists -h "$HOST" -p "$PORT" -U "$USER_NAME" "$GOOD_DB" >/dev/null 2>&1 || true
  dropdb --if-exists -h "$HOST" -p "$PORT" -U "$USER_NAME" "$BAD_DB" >/dev/null 2>&1 || true
}
trap cleanup EXIT

for db in "$GOOD_DB" "$BAD_DB"; do
  createdb -h "$HOST" -p "$PORT" -U "$USER_NAME" "$db"
  psql -v ON_ERROR_STOP=1 -h "$HOST" -p "$PORT" -U "$USER_NAME" -d "$db" -f "$BASELINE" >/dev/null
done

(
  cd "$ROOT/apps/api"
  go run ./cmd/upload-storage-migration-validator \
    -database-url "postgres://${USER_NAME}:${PASSWORD}@${HOST}:${PORT}/${GOOD_DB}?sslmode=disable" \
    -migrations file://migrations
  go run ./cmd/upload-storage-migration-validator \
    -database-url "postgres://${USER_NAME}:${PASSWORD}@${HOST}:${PORT}/${BAD_DB}?sslmode=disable" \
    -migrations file://migrations \
    -expect-reject
)

echo UPLOAD_STORAGE_MIGRATION_VALIDATION=PASS
