#!/usr/bin/env bash
# BodySense Alibaba Cloud production bootstrap.
# The same coherent-release watcher is used for first deployment and later updates.
set -euo pipefail

DEPLOY_DIR="/opt/bodysense"
REPO_URL="${REPO_URL:-https://github.com/T1moooo/BodySense.git}"
ACR_REGISTRY="${ACR_REGISTRY:-crpi-cv97phwhms6wy4as.cn-hangzhou.personal.cr.aliyuncs.com}"
ACR_USERNAME="${ACR_USERNAME:-}"
ACR_PASSWORD="${ACR_PASSWORD:-}"

if [ "$EUID" -ne 0 ]; then
  echo "Run as root" >&2
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  curl -fsSL https://get.docker.com | sh
  systemctl enable --now docker
fi
command -v git >/dev/null 2>&1 || { apt-get update && apt-get install -y git; }
command -v flock >/dev/null 2>&1 || { apt-get update && apt-get install -y util-linux; }

if [ -z "$ACR_USERNAME" ] || [ -z "$ACR_PASSWORD" ]; then
  read -r -p "ACR username: " ACR_USERNAME
  read -r -s -p "ACR password: " ACR_PASSWORD
  echo
fi
printf '%s' "$ACR_PASSWORD" | docker login "$ACR_REGISTRY" --username "$ACR_USERNAME" --password-stdin

if [ ! -d "$DEPLOY_DIR/.git" ]; then
  rm -rf "$DEPLOY_DIR"
  git clone "$REPO_URL" "$DEPLOY_DIR"
else
  git -C "$DEPLOY_DIR" fetch origin main
  git -C "$DEPLOY_DIR" checkout main
  git -C "$DEPLOY_DIR" reset --hard origin/main
fi
cd "$DEPLOY_DIR"

if [ ! -f .env.production.local ]; then
  umask 077
  DB_PASSWORD=$(openssl rand -hex 24)
  REDIS_PASSWORD=$(openssl rand -hex 24)
  JWT_SECRET_KEY=$(openssl rand -base64 48 | tr -d '\n')
  cat > .env.production.local <<SECRET
DB_PASSWORD=$DB_PASSWORD
REDIS_PASSWORD=$REDIS_PASSWORD
JWT_SECRET_KEY=$JWT_SECRET_KEY

# LiteLLM is the production model-routing authority. Provider keys are consumed only by the gateway.
OPENROUTER_API_KEY=
EMBEDDING_API_KEY=
LITELLM_MASTER_KEY=$(openssl rand -hex 32)
MIMO_API_KEY=
SECRET
  chmod 600 .env.production.local
  echo "Created $DEPLOY_DIR/.env.production.local; configure provider keys before serving AI traffic."
fi

# Retire any legacy Watchtower container from older installations.
docker rm -f docker-watchtower-1 >/dev/null 2>&1 || true

compose=(docker compose -p docker -f docker/docker-compose.prod.yml --env-file .env.production --env-file .env.production.local)
"${compose[@]}" config -q
"${compose[@]}" pull postgres redis
"${compose[@]}" up -d postgres redis

install -m 0755 scripts/production-deploy-watch.sh "$DEPLOY_DIR/scripts/production-deploy-watch.sh"
install -m 0644 deploy/systemd/bodysense-deploy-watch.service /etc/systemd/system/bodysense-deploy-watch.service
install -m 0644 deploy/systemd/bodysense-deploy-watch.timer /etc/systemd/system/bodysense-deploy-watch.timer
systemctl daemon-reload

# First deployment uses the exact same safety gates as subsequent polling deployments.
"$DEPLOY_DIR/scripts/production-deploy-watch.sh" --force
systemctl enable --now bodysense-deploy-watch.timer

echo "BodySense production bootstrap complete."
systemctl status bodysense-deploy-watch.timer --no-pager || true
"${compose[@]}" ps
