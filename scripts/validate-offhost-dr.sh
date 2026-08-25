#!/usr/bin/env bash
# BS-PROD-012 docker-backed disaster-recovery integration test.
#
# Re-proves the off-host backup + restore flow end-to-end against REAL
# PostgreSQL and the REAL pg_dump/pg_restore tooling and the REAL validator
# binaries, while keeping the object store fake (scripts/test_offhost_s3.py's
# signature-verified in-process server).  It exercises the same scripts that the
# systemd units run, including the `docker compose exec`-style postgres seam,
# and restores into a second, disposable PostgreSQL container (`restore-pg`)
# that the restore operator proves isolated from the production `postgres`
# container: dedicated drill network (never attached to the production network),
# disposable labels, running state and distinct container identity.
#
# The validators run in SEPARATE disposable validator containers derived from
# the api image and attached ONLY to the drill network (never inside an api
# container, never on a network shared with a production container): a
# validator-image Dockerfile bakes the real validator binaries + migrations at
# /app, the api container is created from that same image on the production
# network only, and the restore path derives its disposable validator image from
# the resolved api container's Config.Image (OFFHOST_VALIDATOR_IMAGE is NOT
# pinned so the derivation path is exercised).  As a regression for the review
# finding, the test first joins a production-Compose-project-labeled rogue
# container to the drill network and proves the restore REFUSES it (Docker
# bridge connectivity is bidirectional), then removes it and runs a PASSing
# drill.
#
# All container and network names are suffixed with the shell PID so the test
# can never collide with (or clean up) unrelated containers/networks: the EXIT
# cleanup only touches resources this run created.  The production postgres is
# additionally given the network alias `postgres` so a validator container can
# reach it by the same DNS name production uses (mirroring the Compose service
# name) while its actual container name stays unique to this run.
#
# Requires docker and outbound registry/module access (golang build).
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

PG_IMAGE="${OFFHOST_DR_PG_IMAGE:-pgvector/pgvector:pg18}"
GO_IMAGE="${OFFHOST_DR_GO_IMAGE:-golang:1.26-alpine}"
ALPINE_IMAGE="${OFFHOST_DR_ALPINE_IMAGE:-alpine:3.20}"
NET="bodysense-dr-net-$$"
DRILL_NET="bodysense-dr-drill-net-$$"
# Unique per-run names: a fixed name (e.g. "postgres", "restore-pg") could
# collide with an unrelated container on a shared CI host, and the EXIT cleanup
# must never delete a container this test did not create.
PG_NAME="bodysense-dr-pg-$$"             # production postgres (source), on NET only
RESTORE_PG_NAME="bodysense-dr-restore-pg-$$" # disposable, explicitly isolated restore postgres, DRILL_NET only
# The api validator container uses Compose's default naming ("<project>-api-1")
# — production does NOT set container_name, so the running container is
# "docker-api-1", never a literal "api". Naming this one "bodysense-dr-api-<pid>"
# (and pinning it via OFFHOST_API_CONTAINER) proves the restore path resolves
# the validator container instead of assuming "api".  The api container is
# attached to the production network only; its Config.Image is the disposable
# validator image the restore derives for the drill (OFFHOST_VALIDATOR_IMAGE is
# deliberately NOT pinned so the derivation path is exercised end-to-end).
API_NAME="bodysense-dr-api-$$"
ROGUE_NAME="bodysense-dr-rogue-api-$$"
BUILDER_NAME="bodysense-dr-builder-$$"
VALIDATOR_IMG="bodysense-dr-validator-img-$$"
# The production postgres declares a Compose project so the drill-network
# membership guard can recognize production-project members (and the rogue
# regression below joins one to the drill network to prove a refusal).
COMPOSE_PROJECT_LABEL="bodysense-dr-prod-$$"
export DB_USER=bodysense DB_NAME=bodysense DB_PASSWORD=0123456789abcdef

TMP="$(mktemp -d)"
ROOT="$TMP/root"
PORT_FILE="$TMP/server.port"
CORRUPT_FILE="$TMP/corrupt.on"
export TMP ROOT CORRUPT_FILE

cleanup() {
  docker rm -f "$ROGUE_NAME" "$API_NAME" "$BUILDER_NAME" "$RESTORE_PG_NAME" "$PG_NAME" >/dev/null 2>&1 || true
  docker rmi "$VALIDATOR_IMG" >/dev/null 2>&1 || true
  docker network rm "$DRILL_NET" >/dev/null 2>&1 || true
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
# OFFHOST_PG_RESTORE_CONTAINER is set by restore-production-backup.sh when it
# routes each psql/pg_restore call at the disposable restore container, so the
# seam forwards to the correct endpoint depending on phase (backup vs restore).
cat > "$TMP/fake-pg" <<PGSTUB
#!/usr/bin/env bash
exec docker exec -i "\${OFFHOST_PG_RESTORE_CONTAINER:-\$PG_NAME}" "\$@"
PGSTUB
chmod +x "$TMP/fake-pg"
export OFFHOST_PG_PREFIX="$TMP/fake-pg" PG_NAME

# --- real PostgreSQL ------------------------------------------------------------
docker network create "$NET" >/dev/null
docker network create "$DRILL_NET" >/dev/null
docker run -d --name "$PG_NAME" --network "$NET" --network-alias postgres \
  --label "com.docker.compose.project=$COMPOSE_PROJECT_LABEL" \
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

# Disposable restore PostgreSQL: explicitly isolated from the production server.
# It lives ONLY on its own dedicated drill network (it must never share a Docker
# network with the production `postgres` container — the restore operator
# refuses that), and it declares the labels that prove it is a disposable drill
# target for --target-project drill: a genuinely isolated, non-host drill
# network is declared and enforced (bodysense.restore-network), alongside the
# disposability and project-ownership labels.
docker run -d --name "$RESTORE_PG_NAME" --network "$DRILL_NET" \
  --label bodysense.restore-project=drill \
  --label bodysense.disposable-restore=yes \
  --label bodysense.restore-network="$DRILL_NET" \
  -e "POSTGRES_USER=$DB_USER" -e "POSTGRES_PASSWORD=$DB_PASSWORD" -e "POSTGRES_DB=postgres" \
  "$PG_IMAGE" >/dev/null
ready=0
for _ in $(seq 1 60); do
  if docker exec "$RESTORE_PG_NAME" pg_isready -U "$DB_USER" -d postgres >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
[ "$ready" = 1 ] || { echo "restore PostgreSQL did not become ready" >&2; exit 1; }

# --- build the real validator binaries with the repo toolchain -------------------
docker run -d --name "$BUILDER_NAME" "$GO_IMAGE" sleep 99999 >/dev/null
docker exec "$BUILDER_NAME" mkdir -p /build /out
docker cp apps/api/. "$BUILDER_NAME":/build/ >/dev/null
docker exec -w /build "$BUILDER_NAME" sh -c \
  'CGO_ENABLED=0 go build -o /out/domain-validator ./cmd/domain-validator && \
   CGO_ENABLED=0 go build -o /out/migration-validator ./cmd/migration-validator' \
  || { echo "validator build failed" >&2; exit 1; }

# --- validator image: bakes the real binaries + migrations at /app --------------
# The disposable validator containers the restore drill runs derive from the api
# container's Config.Image, so an image with /app/validators + /app/migrations
# (exactly what the api image carries post-build) is built from the same repo
# toolchain.  The api container is created from this SAME image so the restore's
# image-derivation path (OFFHOST_API_CONTAINER -> Config.Image) is exercised
# end-to-end; OFFHOST_VALIDATOR_IMAGE is deliberately NOT pinned.
mkdir -p "$TMP/validator-image/validators"
docker cp "$BUILDER_NAME":/out/. "$TMP/validator-image/validators/" >/dev/null
cp -R "$PWD/apps/api/migrations" "$TMP/validator-image/migrations"
printf 'FROM %s\nWORKDIR /app\nCOPY validators/ /app/validators/\nCOPY migrations/ /app/migrations/\n' \
  "$ALPINE_IMAGE" > "$TMP/validator-image/Dockerfile"
docker build -t "$VALIDATOR_IMG" "$TMP/validator-image" >/dev/null

# --- api container: the production api container stand-in, on the production
# --- network ONLY (never joined to the drill network) --------------------------
# The container name is deliberately not "api": restore-production-backup.sh must
# resolve it (here via OFFHOST_API_CONTAINER, mirroring production's default
# "<compose-project>-api-1" naming) and read its Config.Image for the disposable
# validator image.  It hangs off the production network only so it can run the
# migration baseline against production postgres; joining it to the drill network
# is precisely the unsafe topology the membership guard refuses.
docker run -d --name "$API_NAME" --network "$NET" \
  "$VALIDATOR_IMG" tail -f /dev/null >/dev/null
export OFFHOST_PGCONTAINER_ID OFFHOST_API_CONTAINER
OFFHOST_PGCONTAINER_ID=$(docker inspect -f '{{.Id}}' "$PG_NAME")
OFFHOST_API_CONTAINER="$API_NAME"

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
n=$(OFFHOST_BACKUP_ACCESS_KEY="$ACCESS" OFFHOST_BACKUP_SECRET_KEY="$SECRET" \
  python3 "$S3CLI" list --endpoint "$ENDPOINT" --bucket "$BUCKET" --region "$REGION" \
  --prefix "$PREFIX/" \
  | awk -F'\t' 'NF { n++ } END { print n+0 }')
[ "$n" -eq 3 ] || { echo "expected 3 objects, got $n" >&2; exit 1; }
echo "DR_INTEGRATION_BACKUP=PASS objects=$n object_key=$object_key"

# --- freshness check passes ---------------------------------------------------------
BODYSENSE_DEPLOY_ROOT="$ROOT" bash scripts/production-offhost-backup.sh --check-freshness \
  | grep -q OFFHOST_BACKUP_FRESH=OK \
  || { echo "freshness check failed" >&2; exit 1; }
echo "DR_INTEGRATION_FRESHNESS=PASS"

# --- full restore drill into a disposable database on the disposable server --------
target_db="drill_restore_$$"

# --- regression: a production-project member on the drill network is REFUSED ------
# Docker bridge connectivity is bidirectional, so the disposable restore database
# on the drill network must never share that network with a production container —
# this is the unsafe topology that used to be produced by running the validators
# on the production api container joined to the drill network.  Join a rogue
# container carrying the production Compose project to the drill network and
# prove the restore refuses it before touching anything.
docker run -d --name "$ROGUE_NAME" --network "$DRILL_NET" \
  --label "com.docker.compose.project=$COMPOSE_PROJECT_LABEL" \
  "$VALIDATOR_IMG" tail -f /dev/null >/dev/null
if BODYSENSE_DEPLOY_ROOT="$ROOT" bash scripts/restore-production-backup.sh \
  --object-key "$object_key" --target-db "$target_db" --target-project drill \
  --restore-pg "container:$RESTORE_PG_NAME" \
  --confirm-target-isolated=yes --validator-runner docker \
  > "$TMP/rogue.out" 2>&1; then
  echo "restore unexpectedly PASSED with a production-project member on the drill network" >&2
  cat "$TMP/rogue.out" >&2
  exit 1
fi
grep -q "production-project member" "$TMP/rogue.out" \
  || { echo "restore refused for the wrong reason" >&2; cat "$TMP/rogue.out" >&2; exit 1; }
echo "DR_INTEGRATION_ROGUE_MEMBER=REFUSED"
docker rm -f "$ROGUE_NAME" >/dev/null

BODYSENSE_DEPLOY_ROOT="$ROOT" bash scripts/restore-production-backup.sh \
  --object-key "$object_key" --target-db "$target_db" --target-project drill \
  --restore-pg "container:$RESTORE_PG_NAME" \
  --confirm-target-isolated=yes --validator-runner docker \
  > "$TMP/restore.out" 2>&1
grep -q "RESTORE_RESULT=PASS" "$TMP/restore.out" \
  || { echo "restore did not reach PASS" >&2; cat "$TMP/restore.out" >&2; exit 1; }
restored_rev=$(docker exec "$RESTORE_PG_NAME" psql -U "$DB_USER" -d "$target_db" -Atc \
  "SELECT version::text || ':' || dirty::text FROM schema_migrations ORDER BY version DESC LIMIT 1;")
backup_rev=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["schema_revision"])' \
  "$ROOT/.offhost-state/last-success.json")
[ "$restored_rev" = "$backup_rev" ] \
  || { echo "schema revision mismatch: restored=$restored_rev metadata=$backup_rev" >&2; exit 1; }
probe_rows=$(docker exec "$RESTORE_PG_NAME" psql -U "$DB_USER" -d "$target_db" -Atc \
  "SELECT count(*) FROM users WHERE email='dr-probe@example.com';")
[ "$probe_rows" = 1 ] \
  || { echo "dr-probe row missing from restored database (count=$probe_rows)" >&2; exit 1; }
echo "DR_INTEGRATION_RESTORE=PASS database=$target_db restore_pg=$RESTORE_PG_NAME schema=$restored_rev data_round_trip=verified"

echo "OFFHOST_DR_INTEGRATION=PASS"