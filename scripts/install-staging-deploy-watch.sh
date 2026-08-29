#!/usr/bin/env bash
set -Eeuo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
CONFIG_DIR=${BODYSENSE_STAGING_CONFIG_DIR:-$HOME/.config/bodysense}
CONFIG_FILE=${BODYSENSE_STAGING_CHANNEL_CONFIG:-$CONFIG_DIR/staging-channel.env}
BIN_DIR=${BODYSENSE_STAGING_BIN_DIR:-$HOME/.local/bin}
SYSTEMD_DIR=${BODYSENSE_STAGING_SYSTEMD_DIR:-$HOME/.config/systemd/user}
DEFAULT_SECRET_ENV=${BODYSENSE_STAGING_SECRET_ENV:-$ROOT/.env.staging.local}

mkdir -p "$CONFIG_DIR" "$BIN_DIR" "$SYSTEMD_DIR"

if [[ ! -e "$CONFIG_FILE" ]]; then
  umask 077
  cat > "$CONFIG_FILE" <<ENV
# Non-secret coordinates for the canonical BodySense staging artifact channel.
STAGING_REGISTRY=crpi-cv97phwhms6wy4as.cn-hangzhou.personal.cr.aliyuncs.com
STAGING_NAMESPACE=bodysense
STAGING_CHANNEL_TAG=staging-latest
STAGING_SECRET_ENV=$DEFAULT_SECRET_ENV
STAGING_COMPOSE_PROJECT=bodysense-staging
STAGING_BIND_HOST=127.0.0.1
STAGING_WEB_PORT=20150
ENV
  chmod 0600 "$CONFIG_FILE"
  echo "created staging channel config: $CONFIG_FILE"
else
  echo "preserving existing staging channel config: $CONFIG_FILE"
fi

[[ -s "$DEFAULT_SECRET_ENV" || -s "$(sed -n 's/^STAGING_SECRET_ENV=//p' "$CONFIG_FILE" | tail -1)" ]] \
  || { echo "missing staging secret env; expected $DEFAULT_SECRET_ENV or STAGING_SECRET_ENV in $CONFIG_FILE" >&2; exit 2; }

install -m 0755 "$ROOT/scripts/staging-deploy-watch.sh" "$BIN_DIR/bodysense-staging-deploy-watch"
install -m 0644 "$ROOT/deploy/systemd/bodysense-staging-deploy-watch.service" "$SYSTEMD_DIR/bodysense-staging-deploy-watch.service"
install -m 0644 "$ROOT/deploy/systemd/bodysense-staging-deploy-watch.timer" "$SYSTEMD_DIR/bodysense-staging-deploy-watch.timer"

systemctl --user daemon-reload

if [[ "${1:-}" == '--enable' ]]; then
  "$BIN_DIR/bodysense-staging-deploy-watch" --check-only
  systemctl --user enable --now bodysense-staging-deploy-watch.timer
  echo 'staging deploy watcher enabled after coherent channel preflight'
else
  echo 'staging deploy watcher installed but not enabled'
  echo "preflight: $BIN_DIR/bodysense-staging-deploy-watch --check-only"
  echo "enable:   $0 --enable"
fi
