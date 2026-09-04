#!/usr/bin/env bash
set -euo pipefail

ROOT="${BODYSENSE_DEPLOY_ROOT:-/opt/bodysense}"
PUBLIC_ENV="$ROOT/.env.production"
SECRET_ENV="$ROOT/.env.production.local"
UNIT_SOURCE="$ROOT/deploy/systemd"
SYSTEMD_DIR="${BODYSENSE_SYSTEMD_DIR:-/etc/systemd/system}"

if [ "$EUID" -ne 0 ] && [ "${BODYSENSE_ALLOW_NON_ROOT_DR_INSTALL:-false}" != true ]; then
  echo 'install-production-dr.sh must run as root' >&2
  exit 1
fi

[ -s "$PUBLIC_ENV" ] || { echo "missing $PUBLIC_ENV" >&2; exit 1; }
[ -s "$SECRET_ENV" ] || { echo "missing $SECRET_ENV" >&2; exit 1; }
mkdir -p "$SYSTEMD_DIR"

read_env_value() {
  local key="$1" default="${2:-}" value
  value=$(sed -n "s/^${key}=//p" "$SECRET_ENV" | tail -1)
  if [ -z "$value" ]; then
    value=$(sed -n "s/^${key}=//p" "$PUBLIC_ENV" | tail -1)
  fi
  printf '%s' "${value:-$default}"
}

units=(
  bodysense-postgres-dr-backup.service
  bodysense-postgres-dr-backup.timer
  bodysense-postgres-dr-restore.service
  bodysense-postgres-dr-restore.timer
  bodysense-postgres-dr-status.service
  bodysense-postgres-dr-status.timer
)
timers=(
  bodysense-postgres-dr-backup.timer
  bodysense-postgres-dr-restore.timer
  bodysense-postgres-dr-status.timer
)
services=(
  bodysense-postgres-dr-backup.service
  bodysense-postgres-dr-restore.service
  bodysense-postgres-dr-status.service
)

for unit in "${units[@]}"; do
  [ -s "$UNIT_SOURCE/$unit" ] || { echo "missing $UNIT_SOURCE/$unit" >&2; exit 1; }
  install -m 0644 "$UNIT_SOURCE/$unit" "$SYSTEMD_DIR/$unit"
done

systemctl daemon-reload
DR_ENABLED=$(read_env_value DR_ENABLED false)
case "$DR_ENABLED" in
  true)
    systemctl enable --now "${timers[@]}"
    systemctl status "${timers[@]}" --no-pager || true
    echo 'PostgreSQL off-host DR timers enabled.'
    ;;
  false)
    # Unit files are deliberately installed even while the durability plan is
    # parked. This makes future activation a configuration change rather than a
    # bootstrap migration, while guaranteeing no backup/status/restore job runs
    # until DR_ENABLED=true is explicit on the production host.
    systemctl disable --now "${timers[@]}" >/dev/null 2>&1 || true
    systemctl stop "${services[@]}" >/dev/null 2>&1 || true
    systemctl reset-failed "${services[@]}" "${timers[@]}" >/dev/null 2>&1 || true
    echo 'PostgreSQL off-host DR is installed but disabled (DR_ENABLED=false).'
    ;;
  *)
    echo "DR_ENABLED must be true or false (got: $DR_ENABLED)" >&2
    exit 1
    ;;
esac
