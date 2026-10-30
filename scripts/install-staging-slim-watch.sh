#!/usr/bin/env bash
# Install the slim, memory-guarded staging watcher on a host that also runs production.
# Run as root from a checkout of this repository. Writes only to $ROOT and the two
# bodysense-staging-slim-watch systemd units; never touches the production stack.
#   scripts/install-staging-slim-watch.sh            # install, do not enable
#   scripts/install-staging-slim-watch.sh --enable   # preflight, then enable the timer
set -Eeuo pipefail

REPO=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
ROOT=${BODYSENSE_STAGING_SLIM_ROOT:-/opt/bodysense-staging}
SYSTEMD_DIR=${BODYSENSE_STAGING_SLIM_SYSTEMD_DIR:-/etc/systemd/system}
UNIT=bodysense-staging-slim-watch
# Unit name used by the first hand-installed copy; replaced by $UNIT.
LEGACY_UNIT=bodysense-staging-deploy-watch

[[ $(id -u) -eq 0 ]] || { echo 'run as root' >&2; exit 2; }
[[ -s "$ROOT/.env.staging.local" ]] || { echo "missing $ROOT/.env.staging.local" >&2; exit 2; }

install -d -m 0755 "$ROOT" "$ROOT/bin"
install -m 0755 "$REPO/scripts/staging-slim-watch.sh" "$ROOT/bin/$UNIT"
install -m 0644 "$REPO/deploy/staging-slim/docker-compose.staging.slim.yml" \
  "$ROOT/docker-compose.staging.slim.yml"
install -m 0644 "$REPO/deploy/systemd/$UNIT.service" "$SYSTEMD_DIR/$UNIT.service"
install -m 0644 "$REPO/deploy/systemd/$UNIT.timer" "$SYSTEMD_DIR/$UNIT.timer"

# Running staging containers keep their old restart policy until the next deploy
# recreates them; align them now so a reboot cannot start staging before the guard.
mapfile -t staging_ids < <(docker ps -aq --filter label=com.docker.compose.project=bodysense-staging)
((${#staging_ids[@]} == 0)) || docker update --restart=no "${staging_ids[@]}" >/dev/null

if [[ -e "$SYSTEMD_DIR/$LEGACY_UNIT.timer" && "$SYSTEMD_DIR" == /etc/systemd/system ]]; then
  grep -q "$ROOT/bin/" "$SYSTEMD_DIR/$LEGACY_UNIT.service" 2>/dev/null && {
    systemctl disable --now "$LEGACY_UNIT.timer" || true
    rm -f "$SYSTEMD_DIR/$LEGACY_UNIT.timer" "$SYSTEMD_DIR/$LEGACY_UNIT.service"
    echo "removed hand-installed $LEGACY_UNIT units"
  }
fi
systemctl daemon-reload

if [[ "${1:-}" == --enable ]]; then
  "$ROOT/bin/$UNIT" --check-only
  systemctl enable --now "$UNIT.timer"
  echo "$UNIT timer enabled after coherent channel preflight"
else
  echo "$UNIT installed but not enabled; preflight: $ROOT/bin/$UNIT --check-only"
fi
