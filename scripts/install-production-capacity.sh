#!/usr/bin/env bash
set -Eeuo pipefail

# Linux swap metadata consumes one kernel page. A nominal swapfile created at
# the configured byte size can therefore be reported by /proc/meminfo as one
# page smaller than that nominal size. Accept exactly that formatting overhead,
# but keep rejecting materially undersized or unknown swap layouts.
capacity_swap_meets_target() {
  local current_kb="$1" target_kb="$2" page_bytes="${3:-}"
  if [ -z "$page_bytes" ]; then
    page_bytes=$(getconf PAGESIZE 2>/dev/null || true)
  fi
  case "$page_bytes" in
    ''|*[!0-9]*|0) page_bytes=4096 ;;
  esac
  local page_kb=$(( (page_bytes + 1023) / 1024 ))
  (( current_kb + page_kb >= target_kb ))
}

# Report whole GiB for observability without flooring a nominal swap target by
# the single metadata page that mkswap reserves. Values materially below the
# target keep their ordinary floor so monitoring still detects undersized swap.
capacity_swap_effective_gib() {
  local current_kb="$1" target_gib="$2" page_bytes="${3:-}"
  local whole_gib=$(( current_kb / 1024 / 1024 ))
  local target_kb=$(( target_gib * 1024 * 1024 ))
  if (( whole_gib < target_gib )) && capacity_swap_meets_target "$current_kb" "$target_kb" "$page_bytes"; then
    printf '%s' "$target_gib"
  else
    printf '%s' "$whole_gib"
  fi
}

# The validator sources this file to exercise the capacity comparison without
# mutating the host. Normal execution continues below unchanged.
if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then
  return 0
fi

ROOT="${BODYSENSE_DEPLOY_ROOT:-/opt/bodysense}"
PUBLIC_ENV="$ROOT/.env.production"
UNIT_SOURCE="$ROOT/deploy/systemd"
MODE="${1:-full}"

[ "$EUID" -eq 0 ] || { echo 'install-production-capacity.sh must run as root' >&2; exit 1; }
[ -s "$PUBLIC_ENV" ] || { echo "missing $PUBLIC_ENV" >&2; exit 1; }
read_env() { local key="$1" default="$2" value; value=$(sed -n "s/^${key}=//p" "$PUBLIC_ENV" | tail -1); printf '%s' "${value:-$default}"; }

swap_gib=$(read_env CAPACITY_SWAP_GIB 2)
swappiness=$(read_env CAPACITY_SWAPPINESS 10)
case "$swap_gib" in ''|*[!0-9]*) echo 'invalid CAPACITY_SWAP_GIB' >&2; exit 1 ;; esac
(( swap_gib >= 1 && swap_gib <= 4 )) || { echo 'CAPACITY_SWAP_GIB must be 1..4' >&2; exit 1; }
case "$swappiness" in ''|*[!0-9]*) echo 'invalid CAPACITY_SWAPPINESS' >&2; exit 1 ;; esac
(( swappiness >= 1 && swappiness <= 20 )) || { echo 'CAPACITY_SWAPPINESS must be 1..20' >&2; exit 1; }

target_kb=$(( swap_gib * 1024 * 1024 ))
current_kb=$(awk '/^SwapTotal:/ {print $2}' /proc/meminfo)
if (( current_kb == 0 )); then
  swapfile=/swapfile
  [ ! -e "$swapfile" ] || { echo "$swapfile exists but swap is inactive; refusing to overwrite" >&2; exit 1; }
  available_bytes=$(df -PB1 / | awk 'NR==2 {print $4}')
  target_bytes=$(( swap_gib * 1024 * 1024 * 1024 ))
  (( available_bytes > target_bytes * 2 )) || { echo 'insufficient disk headroom to create swap' >&2; exit 1; }
  if ! fallocate -l "${swap_gib}G" "$swapfile" 2>/dev/null; then
    dd if=/dev/zero of="$swapfile" bs=1M count=$((swap_gib*1024)) status=none
  fi
  chmod 600 "$swapfile"
  mkswap "$swapfile" >/dev/null
  swapon "$swapfile"
  grep -qE '^/swapfile[[:space:]]' /etc/fstab || printf '/swapfile none swap sw 0 0\n' >> /etc/fstab
elif ! capacity_swap_meets_target "$current_kb" "$target_kb"; then
  echo "existing swap is smaller than ${swap_gib} GiB after allowing one kernel page of swap metadata; refusing to mutate an unknown swap layout" >&2
  exit 1
fi

cat > /etc/sysctl.d/99-bodysense-capacity.conf <<SYSCTL
# BodySense: swap is emergency headroom, not normal working memory.
vm.swappiness=$swappiness
SYSCTL
sysctl -p /etc/sysctl.d/99-bodysense-capacity.conf >/dev/null

echo "CAPACITY_SWAP=PASS total_kb=$(awk '/^SwapTotal:/ {print $2}' /proc/meminfo) swappiness=$(sysctl -n vm.swappiness)"
[ "$MODE" = --swap-only ] && exit 0
[ "$MODE" = full ] || { echo 'usage: install-production-capacity.sh [--swap-only]' >&2; exit 2; }

for unit in \
  bodysense-capacity-status.service bodysense-capacity-status.timer \
  bodysense-capacity-cleanup.service bodysense-capacity-cleanup.timer; do
  [ -s "$UNIT_SOURCE/$unit" ] || { echo "missing $UNIT_SOURCE/$unit" >&2; exit 1; }
  install -m 0644 "$UNIT_SOURCE/$unit" "/etc/systemd/system/$unit"
done
systemctl daemon-reload
systemctl enable --now bodysense-capacity-status.timer bodysense-capacity-cleanup.timer
systemctl status bodysense-capacity-status.timer bodysense-capacity-cleanup.timer --no-pager || true
