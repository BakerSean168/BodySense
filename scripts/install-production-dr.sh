#!/usr/bin/env bash
set -euo pipefail

ROOT="${BODYSENSE_DEPLOY_ROOT:-/opt/bodysense}"
UNIT_SOURCE="$ROOT/deploy/systemd"

if [ "$EUID" -ne 0 ]; then
  echo 'install-production-dr.sh must run as root' >&2
  exit 1
fi

for unit in \
  bodysense-postgres-dr-backup.service \
  bodysense-postgres-dr-backup.timer \
  bodysense-postgres-dr-restore.service \
  bodysense-postgres-dr-restore.timer \
  bodysense-postgres-dr-status.service \
  bodysense-postgres-dr-status.timer; do
  [ -s "$UNIT_SOURCE/$unit" ] || { echo "missing $UNIT_SOURCE/$unit" >&2; exit 1; }
  install -m 0644 "$UNIT_SOURCE/$unit" "/etc/systemd/system/$unit"
done

systemctl daemon-reload
systemctl enable --now bodysense-postgres-dr-backup.timer bodysense-postgres-dr-restore.timer bodysense-postgres-dr-status.timer
systemctl status bodysense-postgres-dr-backup.timer bodysense-postgres-dr-restore.timer bodysense-postgres-dr-status.timer --no-pager || true
