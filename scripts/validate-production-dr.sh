#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
api_dir="$repo_root/apps/api"
for tool in go python3 psql pg_dump pg_restore createdb dropdb; do
  command -v "$tool" >/dev/null 2>&1 || { echo "PRODUCTION_DR_TOOLCHAIN=FAIL missing=$tool" >&2; exit 1; }
done
echo PRODUCTION_DR_TOOLCHAIN=PASS
store=$(mktemp -d)
validator=$(mktemp)
trap 'rm -rf "$store" "$validator" /tmp/bodysense-dr-ci-*.json' EXIT

DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-bodysense_migration_validator}"
DB_USER="${DB_USER:-bodysense}"
DB_PASSWORD="${DB_PASSWORD:-bodysense123}"
DB_SSLMODE="${DB_SSLMODE:-disable}"
revision="${DR_RELEASE_REVISION:-${GITHUB_SHA:-dr-integration-revision}}"

(
  cd "$api_dir"
  go build -o "$validator" ./cmd/domain-validator
)

run_dr() {
  local command="$1" output="$2"
  (
    cd "$api_dir"
    env \
      APP_ENV=test \
      DR_OBJECT_STORE_DRIVER=filesystem \
      DR_FILESYSTEM_ROOT="$store" \
      DR_OSS_PREFIX=bodysense/production/postgres \
      DR_MAX_BACKUP_AGE_HOURS=30 \
      DR_DOMAIN_VALIDATOR_PATH="$validator" \
      DR_RELEASE_REVISION="$revision" \
      DB_HOST="$DB_HOST" DB_PORT="$DB_PORT" DB_NAME="$DB_NAME" DB_USER="$DB_USER" DB_PASSWORD="$DB_PASSWORD" DB_SSLMODE="$DB_SSLMODE" \
      go run ./cmd/production-dr-manager "$command" > "$output"
  )
}

run_dr backup /tmp/bodysense-dr-ci-backup.json
run_dr status /tmp/bodysense-dr-ci-status.json
run_dr restore-drill /tmp/bodysense-dr-ci-restore.json

python3 - <<'PY'
import json
backup=json.load(open('/tmp/bodysense-dr-ci-backup.json'))
status=json.load(open('/tmp/bodysense-dr-ci-status.json'))
restore=json.load(open('/tmp/bodysense-dr-ci-restore.json'))
assert backup['remote_verified'] is True and backup['dump_size_bytes'] > 0
assert status['remote_verified'] is True and status['backup_id'] == backup['backup_id']
assert restore['domain_semantics'] == 'PASS' and restore['dropped'] is True
assert restore['migration_state'] == backup['migration_state']
print('PRODUCTION_DR_ARTIFACTS=PASS')
PY

PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -Atc \
  "SELECT count(*) FROM pg_database WHERE datname LIKE 'bodysense_dr_%';" | grep -qx 0

echo PRODUCTION_DR_INTEGRATION=PASS
