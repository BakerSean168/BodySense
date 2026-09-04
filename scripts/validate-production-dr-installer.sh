#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
ROOT="$TMP/root"
SYSTEMD="$TMP/systemd"
BIN="$TMP/bin"
LOG="$TMP/systemctl.log"
mkdir -p "$ROOT/deploy/systemd" "$SYSTEMD" "$BIN"
cp deploy/systemd/bodysense-postgres-dr-* "$ROOT/deploy/systemd/"
printf 'DR_ENABLED=false\n' > "$ROOT/.env.production"
printf 'DB_PASSWORD=test-only\n' > "$ROOT/.env.production.local"

cat > "$BIN/systemctl" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "${SYSTEMCTL_LOG:?}"
exit 0
EOF
chmod +x "$BIN/systemctl"

run_installer() {
  PATH="$BIN:$PATH" SYSTEMCTL_LOG="$LOG" \
    BODYSENSE_DEPLOY_ROOT="$ROOT" BODYSENSE_SYSTEMD_DIR="$SYSTEMD" \
    BODYSENSE_ALLOW_NON_ROOT_DR_INSTALL=true \
    bash scripts/install-production-dr.sh
}

out=$(run_installer)
[[ "$out" == *'installed but disabled'* ]]
for unit in bodysense-postgres-dr-backup.service bodysense-postgres-dr-backup.timer \
  bodysense-postgres-dr-restore.service bodysense-postgres-dr-restore.timer \
  bodysense-postgres-dr-status.service bodysense-postgres-dr-status.timer; do
  test -s "$SYSTEMD/$unit"
done
grep -q '^disable --now bodysense-postgres-dr-backup.timer bodysense-postgres-dr-restore.timer bodysense-postgres-dr-status.timer$' "$LOG"
! grep -q '^enable --now bodysense-postgres-dr-backup.timer' "$LOG"

: > "$LOG"
printf 'DR_ENABLED=true\n' > "$ROOT/.env.production.local"
out=$(run_installer)
[[ "$out" == *'timers enabled'* ]]
grep -q '^enable --now bodysense-postgres-dr-backup.timer bodysense-postgres-dr-restore.timer bodysense-postgres-dr-status.timer$' "$LOG"

# Deployment must bootstrap the new installer directly instead of requiring the
# target timer to pre-exist, and it must never re-enable the retired legacy timers.
! grep -q 'list-unit-files bodysense-postgres-dr-backup.timer' scripts/production-deploy-watch.sh
! grep -q 'enable --now bodysense-offhost-backup.timer bodysense-offhost-freshness.timer' scripts/production-deploy-watch.sh
! grep -q 'enable --now bodysense-offhost-backup.timer bodysense-offhost-freshness.timer' scripts/setup-server.sh
grep -q 'if \[ -x "$ROOT/scripts/install-production-dr.sh" \]; then' scripts/production-deploy-watch.sh

printf 'PRODUCTION_DR_INSTALLER=PASS\n'
