#!/usr/bin/env bash
# BodySense Alibaba Cloud production bootstrap.
# Persistent production state lives under BODYSENSE_DEPLOY_ROOT. Repository
# checkout state is deliberately kept in a separate disposable source tree.
set -euo pipefail

DEPLOY_DIR="${BODYSENSE_DEPLOY_ROOT:-/opt/bodysense}"
SOURCE_DIR="${BODYSENSE_BOOTSTRAP_SOURCE:-/opt/bodysense-source}"
REPO_URL="${REPO_URL:-https://github.com/BakerSean168/BodySense.git}"
ACR_REGISTRY="${ACR_REGISTRY:-crpi-cv97phwhms6wy4as.cn-hangzhou.personal.cr.aliyuncs.com}"
ACR_USERNAME="${ACR_USERNAME:-}"
ACR_PASSWORD="${ACR_PASSWORD:-}"

if [ "$EUID" -ne 0 ]; then
  echo "Run as root" >&2
  exit 1
fi

if [ "$DEPLOY_DIR" = "$SOURCE_DIR" ]; then
  echo "BODYSENSE_DEPLOY_ROOT and BODYSENSE_BOOTSTRAP_SOURCE must be different directories" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  curl -fsSL https://get.docker.com | sh
  systemctl enable --now docker
fi
command -v git >/dev/null 2>&1 || { apt-get update && apt-get install -y git; }
command -v flock >/dev/null 2>&1 || { apt-get update && apt-get install -y util-linux; }
command -v openssl >/dev/null 2>&1 || { apt-get update && apt-get install -y openssl; }

if [ -z "$ACR_USERNAME" ] || [ -z "$ACR_PASSWORD" ]; then
  read -r -p "ACR username: " ACR_USERNAME
  read -r -s -p "ACR password: " ACR_PASSWORD
  echo
fi
printf '%s' "$ACR_PASSWORD" | docker login "$ACR_REGISTRY" --username "$ACR_USERNAME" --password-stdin

# SOURCE_DIR is disposable. DEPLOY_DIR is not: it contains secrets, backups and
# deployment state and must never be removed by bootstrap reconciliation.
if [ ! -d "$SOURCE_DIR/.git" ]; then
  rm -rf "$SOURCE_DIR"
  git clone "$REPO_URL" "$SOURCE_DIR"
else
  git -C "$SOURCE_DIR" fetch origin main
  git -C "$SOURCE_DIR" checkout main
  git -C "$SOURCE_DIR" reset --hard origin/main
fi

# setup-server is a bootstrap/reconciliation path, not a destructive database reset tool.
# On an existing production host, refuse before overwriting runtime files when
# the running PostgreSQL major differs from the tracked target. The coherent
# release watcher owns the one-time legacy-database discard and fresh PG18 cutover.
existing_postgres=$(docker ps -aq \
  --filter 'label=com.docker.compose.project=docker' \
  --filter 'label=com.docker.compose.service=postgres' | head -1)
if [ -n "$existing_postgres" ]; then
  current_pg_num=$(docker exec "$existing_postgres" psql -U bodysense -d bodysense -Atc 'show server_version_num' 2>/dev/null || true)
  target_pg_major=$(sed -n 's/^POSTGRES_MAJOR=//p' "$SOURCE_DIR/.env.production" | tail -1)
  if [[ "$current_pg_num" =~ ^[0-9]+$ ]] && [[ "$target_pg_major" =~ ^[0-9]+$ ]]; then
    current_pg_major=$(( current_pg_num / 10000 ))
    if [ "$current_pg_major" != "$target_pg_major" ]; then
      echo "Existing PostgreSQL major=$current_pg_major differs from target=$target_pg_major; use the release watcher PostgreSQL 18 reset transaction instead of setup-server." >&2
      exit 1
    fi
  fi
fi

mkdir -p "$DEPLOY_DIR/docker/litellm" "$DEPLOY_DIR/scripts" "$DEPLOY_DIR/deploy/systemd"

if [ ! -f "$DEPLOY_DIR/.env.production.local" ]; then
  DB_PASSWORD=$(openssl rand -hex 24)
  REDIS_PASSWORD=$(openssl rand -hex 24)
  JWT_SECRET_KEY=$(openssl rand -base64 48 | tr -d '\n')
  LITELLM_MASTER_KEY=$(openssl rand -hex 32)
  (
    umask 077
    cat > "$DEPLOY_DIR/.env.production.local" <<SECRET
DB_PASSWORD=$DB_PASSWORD
REDIS_PASSWORD=$REDIS_PASSWORD
JWT_SECRET_KEY=$JWT_SECRET_KEY

# LiteLLM is the production model-routing authority. Provider keys are consumed only by the gateway.
OPENROUTER_API_KEY=
EMBEDDING_API_KEY=
LITELLM_MASTER_KEY=$LITELLM_MASTER_KEY
MIMO_API_KEY=

# Private upload OSS cutover is explicit. Leave blank until the bucket and ECS
# RAM role have been provisioned and validated.
UPLOAD_OSS_BUCKET=
UPLOAD_OSS_ECS_RAM_ROLE=
UPLOAD_OSS_ENDPOINT=

# Off-host backup (BS-PROD-012): least-privilege object-store credentials for
# the operator-owned off-host PostgreSQL backups.  Generate an access key/secret
# that has PutObject/GetObject/DeleteObject/ListBucket on the backup bucket and
# ALWAYS GetBucketAcl (the privacy preflight reads the ACL).  GetBucketPolicyStatus
# is additionally required ONLY when the tracked .env.production sets
# OFFHOST_BACKUP_PRIVACY_PROOF=acl+policy (the default); stores such as Alibaba
# OSS do not implement policy status at all, so .env.production ships with
# OFFHOST_BACKUP_PRIVACY_PROOF=acl (ACL-only proof, no GetBucketPolicyStatus).
# Mirror the proof-mode permission contract in .env.production, or backups will
# abort at the fail-closed private-destination preflight (the client refuses "I
# could not prove private" as anything except a failure).
OFFHOST_BACKUP_ACCESS_KEY=
OFFHOST_BACKUP_SECRET_KEY=
SECRET
  )
  echo "Created $DEPLOY_DIR/.env.production.local; configure provider keys before serving AI traffic."
fi
chmod 600 "$DEPLOY_DIR/.env.production.local"

# Install only the tracked runtime bundle. Persistent state remains untouched.
install -m 0644 "$SOURCE_DIR/.env.production" "$DEPLOY_DIR/.env.production"
install -m 0644 "$SOURCE_DIR/docker/docker-compose.prod.yml" "$DEPLOY_DIR/docker/docker-compose.prod.yml"
install -m 0644 "$SOURCE_DIR/docker/Caddyfile" "$DEPLOY_DIR/docker/Caddyfile"
if [ -f "$SOURCE_DIR/docker/litellm/config.yaml" ]; then
  install -m 0644 "$SOURCE_DIR/docker/litellm/config.yaml" "$DEPLOY_DIR/docker/litellm/config.yaml"
fi
install -m 0755 "$SOURCE_DIR/scripts/production-deploy-watch.sh" "$DEPLOY_DIR/scripts/production-deploy-watch.sh"
install -m 0755 "$SOURCE_DIR/scripts/offhost-s3.py" "$DEPLOY_DIR/scripts/offhost-s3.py"
install -m 0755 "$SOURCE_DIR/scripts/production-offhost-backup.sh" "$DEPLOY_DIR/scripts/production-offhost-backup.sh"
install -m 0755 "$SOURCE_DIR/scripts/restore-production-backup.sh" "$DEPLOY_DIR/scripts/restore-production-backup.sh"
install -m 0755 "$SOURCE_DIR/scripts/production-postgres-dr.sh" "$DEPLOY_DIR/scripts/production-postgres-dr.sh"
install -m 0755 "$SOURCE_DIR/scripts/install-production-dr.sh" "$DEPLOY_DIR/scripts/install-production-dr.sh"
install -m 0755 "$SOURCE_DIR/scripts/production-postgres18-reset.sh" "$DEPLOY_DIR/scripts/production-postgres18-reset.sh"
install -m 0755 "$SOURCE_DIR/scripts/production-capacity-status.sh" "$DEPLOY_DIR/scripts/production-capacity-status.sh"
install -m 0755 "$SOURCE_DIR/scripts/install-production-capacity.sh" "$DEPLOY_DIR/scripts/install-production-capacity.sh"
for unit in bodysense-postgres-dr-backup.service bodysense-postgres-dr-backup.timer bodysense-postgres-dr-restore.service bodysense-postgres-dr-restore.timer bodysense-postgres-dr-status.service bodysense-postgres-dr-status.timer; do
  install -m 0644 "$SOURCE_DIR/deploy/systemd/$unit" "$DEPLOY_DIR/deploy/systemd/$unit"
done
for unit in bodysense-capacity-status.service bodysense-capacity-status.timer bodysense-capacity-cleanup.service bodysense-capacity-cleanup.timer; do
  install -m 0644 "$SOURCE_DIR/deploy/systemd/$unit" "$DEPLOY_DIR/deploy/systemd/$unit"
done

# Establish bounded host swap before starting/restarting memory-capped services.
"$DEPLOY_DIR/scripts/install-production-capacity.sh" --swap-only

# Retire any legacy Watchtower container from older installations.
docker rm -f docker-watchtower-1 >/dev/null 2>&1 || true

compose=(
  docker compose -p docker
  -f "$DEPLOY_DIR/docker/docker-compose.prod.yml"
  --env-file "$DEPLOY_DIR/.env.production"
  --env-file "$DEPLOY_DIR/.env.production.local"
)
"${compose[@]}" config -q
"${compose[@]}" pull postgres redis
"${compose[@]}" up -d postgres redis

install -m 0644 "$SOURCE_DIR/deploy/systemd/bodysense-deploy-watch.service" /etc/systemd/system/bodysense-deploy-watch.service
install -m 0644 "$SOURCE_DIR/deploy/systemd/bodysense-deploy-watch.timer" /etc/systemd/system/bodysense-deploy-watch.timer

# Off-host backup + freshness scheduling (BS-PROD-012).  The units are always
# installed; the deploy watcher enables the timers.  Until the operator supplies
# OFFHOST_BACKUP_* credentials in .env.production.local the freshness check
# alerts every hour instead of reporting "OK", so an unconfigured host cannot
# masquerade as being protected.
install -m 0644 "$SOURCE_DIR/deploy/systemd/bodysense-offhost-backup.service" /etc/systemd/system/bodysense-offhost-backup.service
install -m 0644 "$SOURCE_DIR/deploy/systemd/bodysense-offhost-backup.timer" /etc/systemd/system/bodysense-offhost-backup.timer
install -m 0644 "$SOURCE_DIR/deploy/systemd/bodysense-offhost-freshness.service" /etc/systemd/system/bodysense-offhost-freshness.service
install -m 0644 "$SOURCE_DIR/deploy/systemd/bodysense-offhost-freshness.timer" /etc/systemd/system/bodysense-offhost-freshness.timer
systemctl daemon-reload

# First deployment uses the exact same safety gates as subsequent polling deployments.
"$DEPLOY_DIR/scripts/production-deploy-watch.sh" --force
systemctl enable --now bodysense-deploy-watch.timer
"$DEPLOY_DIR/scripts/install-production-dr.sh"
"$DEPLOY_DIR/scripts/install-production-capacity.sh"

echo "BodySense production bootstrap complete."
systemctl status bodysense-deploy-watch.timer --no-pager || true
"${compose[@]}" ps
