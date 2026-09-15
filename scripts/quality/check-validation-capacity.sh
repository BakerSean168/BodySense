#!/usr/bin/env bash
set -euo pipefail

MIN_FREE_GIB="${BODYSENSE_VALIDATION_MIN_FREE_GIB:-8}"
DISK_PATH="${BODYSENSE_VALIDATION_DISK_PATH:-.}"

if ! [[ "$MIN_FREE_GIB" =~ ^[0-9]+$ ]] || [ "$MIN_FREE_GIB" -lt 1 ]; then
  echo "VALIDATION_HOST_CAPACITY=FAIL reason=invalid-min-free-gib value=$MIN_FREE_GIB" >&2
  exit 2
fi

if [ -n "${BODYSENSE_VALIDATION_AVAILABLE_KIB:-}" ]; then
  AVAILABLE_KIB="$BODYSENSE_VALIDATION_AVAILABLE_KIB"
else
  AVAILABLE_KIB="$(df -Pk "$DISK_PATH" | awk 'NR == 2 {print $4}')"
fi

if ! [[ "$AVAILABLE_KIB" =~ ^[0-9]+$ ]]; then
  echo "VALIDATION_HOST_CAPACITY=FAIL reason=invalid-available-kib value=$AVAILABLE_KIB" >&2
  exit 2
fi

REQUIRED_KIB=$((MIN_FREE_GIB * 1024 * 1024))
AVAILABLE_GIB=$((AVAILABLE_KIB / 1024 / 1024))

if [ "$AVAILABLE_KIB" -lt "$REQUIRED_KIB" ]; then
  echo "VALIDATION_HOST_CAPACITY=FAIL free_gib=$AVAILABLE_GIB required_gib=$MIN_FREE_GIB path=$DISK_PATH" >&2
  echo "Full validation includes disposable Docker/Go builds; reclaim build cache or unused validation images before retrying." >&2
  exit 1
fi

echo "VALIDATION_HOST_CAPACITY=PASS free_gib=$AVAILABLE_GIB required_gib=$MIN_FREE_GIB path=$DISK_PATH"
