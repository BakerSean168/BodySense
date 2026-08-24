#!/usr/bin/env bash
# BS-PROD-012 docker-backed disaster-recovery integration test.
#
# Re-proves the off-host backup + restore flow end-to-end against REAL
# PostgreSQL and the REAL pg_dump/pg_restore tooling and the REAL validator
# binaries, while keeping the object store fake (scripts/test_offhost_s3.py's
# signature-verified in-process server).  It exercises the same scripts that the
# systemd units run, including the `docker compose exec`-style postgres seam.
#
# Requires docker and outbound registry/module access (golang build).
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

PG_IMAGE="${OFFHOST_DR_PG_IMAGE:-pgvector/pgvector:pg18}"
GO_IMAGE="${OFFHOST_DR_GO_IMAGE:-golang:1.26-alpine}"
ALPINE_IMAGE="${OFFHOST_DR_ALPINE_IMAGE:-alpine:3.20}"
NET="bodysense-dr-net-$$"
PG_NAME="postgres"     # restore validators reach the DB at the hostname `postgres`
API_NAME="api"
BUILDER_NAME="bodysense-dr-builder-$$"
export DB_USER=bodysense DB_NAME=bodysense DB_PASSWORD=0123456789abcdef

TMP="$(mktemp -d)"
ROOT="$TMP/root"
PORT_FILE="$TMP/server.port"
CORRUPT_FILE="$TMP/corrupt.on"
export TMP ROOT CORRUPT_FILE

cleanup() {
  docker rm -f "$API_NAME" "$BUILDER_NAME" "$PG_NAME" >/dev/null 2>&1 || true
  docker network rm "$NET" >/dev/null 2>&1 || true
  if [ -n "${SERVER_PID:-}" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$TMP"
}
trap cleanup EXIT

command -v docker >/dev/null || { echo "docker is required for validate-offhost-dr.sh" >&2; exit 1; }

# --- object store (fake, signature-verified) -----------------------------------
export S3LIBS="$PWD/scripts/test_offhost_s3.py"
export S3CLI="$PWD/scripts/offhost-s3.py"
export ACCESS="AKID0123456789TESTKEY"
export SECRET="S3cr3t+S3cr3t/K7MDENG/bPxRfiCYEXAMPLESecret"
export PREFIX="bodysense/postgres"
export BUCKET="testbucket"
export REGION="cn-hangzhou"

python3 - "$PORT_FILE" <<'PY' &
import importlib.util, os, sys
spec = importlib.util.spec_from_file_location("test_offhost_s3", os.environ["S3LIBS"])
T = importlib.util.module_from_spec(spec)
sys.modules["test_offhost_s3"] = T
spec.loader.exec_module(T)
T.FakeS3Handler.store = {}
srv = T.FakeServer()
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
[ -s "$PORT_FILE" ] || { echo "fake S3 server did not start" >&2; exit 1; }
ENDPOINT=$(cat "$PORT_FILE")

# --- env files as production would have them -----------------------------------
mkdir -p "$ROOT"
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
DB_USER=$DB_USER
DB_NAME=$DB_NAME
ENV
umask 077
cat > "$ROOT/.env.production.local" <<ENV
OFFHOST_BACKUP_ACCESS_KEY=$ACCESS
OFFHOST_BACKUP_SECRET_KEY=$SECRET
DB_PASSWORD=$DB_PASSWORD
ENV
chmod 600 "$ROOT/.env.production.local"
chmod 644 "$ROOT/.env.production"
umask 022

# --- postgres peer through the same docker-exec seam production uses -----------
cat > "$TMP/fake-pg" <<PGSTUB
#!/usr/bin/env bash
exec docker exec -i $PG_NAME "\$@"
PGSTUB
chmod +x "$TMP/fake-pg"
export OFFHOST_PG_PREFIX="$TMP/fake-pg"

# --- real PostgreSQL ------------------------------------------------------------
docker network create "$NET" >/dev/null
docker run -d --name "$PG_NAME" --network "$NET" \
  -e "POSTGRES_USER=$DB_USER" -e "POSTGRES_PASSWORD=$DB_PASSWORD" -e "POSTGRES_DB=$DB_NAME" \
  "$PG_IMAGE" >/dev/null

ready=0
for _ in $(seq 1 60); do
  if docker exec "$PG_NAME" pg_isready -U "$DB_USER" -d "$DB_NAME" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
[ "$ready" = 1 ] || { echo "PostgreSQL did not become ready" >&2; exit 1; }

# --- build the real validator binaries with the repo toolchain -------------------
docker run -d --name "$BUILDER_NAME" "$GO_IMAGE" sleep 99999 >/dev/null
docker exec "$BUILDER_NAME" mkdir -p /build /out
docker cp apps/api/. "$BUILDER_NAME":/build/ >/dev/null
docker exec -w /build "$BUILDER_NAME" sh -c \
  'CGO_ENABLED=0 go build -o /out/domain-validator ./cmd/domain-validator && \
   CGO_ENABLED=0 go build -o /out/migration-validator ./cmd/migration-validator' \
  || { echo "validator build failed" >&2; exit 1; }

# --- api container: hosts the validator binaries + migrations (drill-only) -------
mkdir -p "$TMP/validators"
docker cp "$BUILDER_NAME":/out/. "$TMP/validators/" >/dev/null
docker run -d --name "$API_NAME" --network "$NET" \
  -v "$TMP/validators:/app/validators:ro" \
  -v "$PWD/apps/api/migrations:/app/migrations:ro" \
  "$ALPINE_IMAGE" tail -f /dev/null >/dev/null
export OFFHOST_PGCONTAINER_ID
OFFHOST_PGCONTAINER_ID=$(docker inspect -f '{{.Id}}' "$PG_NAME")

# --- bring the database to the latest published migrations -------------------------
docker exec "$API_NAME" /app/validators/migration-validator \
  -database-url "postgres://$DB_USER:$DB_PASSWORD@postgres:5432/$DB_NAME?sslmode=disable" \
  -migrations file://migrations

# --- prove real data travels with the backup --------------------------------------
docker exec "$PG_NAME" psql -U "$DB_USER" -d "$DB_NAME" -Atc \
  "INSERT INTO users(email, password_hash) VALUES ('dr-probe@example.com','x') RETURNING 1;" \
  | grep -qx 1

# --- real backup through the signed off-host client --------------------------------
BODYSENSE_DEPLOY_ROOT="$ROOT" bash scripts/production-offhost-backup.sh --backup > "$TMP/backup.out"
grep -q OFFHOST_BACKUP_OBJECT= "$TMP/backup.out" || { echo "backup did not report an object key" >&2; exit 1; }
object_key=$(sed -n 's/^OFFHOST_BACKUP_OBJECT=//p' "$TMP/backup.out" | tail -1)
n=$(python3 "$S3CLI" list --endpoint "$ENDPOINT" --bucket "$BUCKET" --region "$REGION" \
  --access-key "$ACCESS" --secret-key "$SECRET" --prefix "$PREFIX/" \
  | awk -F'\t' 'NF { n++ } END { print n+0 }')
[ "$n" -eq 3 ] || { echo "expected 3 objects, got $n" >&2; exit 1; }
echo "DR_INTEGRATION_BACKUP=PASS objects=$n object_key=$object_key"

# --- freshness check passes ---------------------------------------------------------
BODYSENSE_DEPLOY_ROOT="$ROOT" bash scripts/production-offhost-backup.sh --check-freshness \
  | grep -q OFFHOST_BACKUP_FRESH=OK \
  || { echo "freshness check failed" >&2; exit 1; }
echo "DR_INTEGRATION_FRESHNESS=PASS"

# --- full restore drill into a disposable database ------------------------------------
target_db="drill_restore_$$"
BODYSENSE_DEPLOY_ROOT="$ROOT" bash scripts/restore-production-backup.sh \
  --object-key "$object_key" --target-db "$target_db" --target-project drill \
  --confirm-target-isolated=yes --validator-runner docker \
  > "$TMP/restore.out" 2>&1
grep -q "RESTORE_RESULT=PASS" "$TMP/restore.out" \
  || { echo "restore did not reach PASS" >&2; cat "$TMP/restore.out" >&2; exit 1; }
restored_rev=$(docker exec "$PG_NAME" psql -U "$DB_USER" -d "$target_db" -Atc \
  "SELECT version::text || ':' || dirty::text FROM schema_migrations ORDER BY version DESC LIMIT 1;")
backup_rev=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["schema_revision"])' \
  "$ROOT/.offhost-state/last-success.json")
[ "$restored_rev" = "$backup_rev" ] \
  || { echo "schema revision mismatch: restored=$restored_rev metadata=$backup_rev" >&2; exit 1; }
probe_rows=$(docker exec "$PG_NAME" psql -U "$DB_USER" -d "$target_db" -Atc \
  "SELECT count(*) FROM users WHERE email='dr-probe@example.com';")
[ "$probe_rows" = 1 ] \
  || { echo "dr-probe row missing from restored database (count=$probe_rows)" >&2; exit 1; }
echo "DR_INTEGRATION_RESTORE=PASS database=$target_db schema=$restored_rev data_round_trip=verified"

echo "OFFHOST_DR_INTEGRATION=PASS"