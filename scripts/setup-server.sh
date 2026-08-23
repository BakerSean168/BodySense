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

mkdir -p "$DEPLOY_DIR/docker/litellm" "$DEPLOY_DIR/scripts"

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
SECRET
  )
  echo "Created $DEPLOY_DIR/.env.production.local; configure provider keys before serving AI traffic."
  echo "On a shared :80/:443 host, also set BODYSENSE_EDGE_MODE=external in that file."
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
systemctl daemon-reload

# First deployment uses the exact same safety gates as subsequent polling deployments.
"$DEPLOY_DIR/scripts/production-deploy-watch.sh" --force
systemctl enable --now bodysense-deploy-watch.timer

echo "BodySense production bootstrap complete."
systemctl status bodysense-deploy-watch.timer --no-pager || true
"${compose[@]}" ps
