#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHECK="$ROOT/scripts/quality/check-validation-capacity.sh"

pass_output="$({
  BODYSENSE_VALIDATION_MIN_FREE_GIB=8 \
  BODYSENSE_VALIDATION_AVAILABLE_KIB=$((10 * 1024 * 1024)) \
    bash "$CHECK"
} 2>&1)"
grep -q 'VALIDATION_HOST_CAPACITY=PASS' <<<"$pass_output"

set +e
fail_output="$({
  BODYSENSE_VALIDATION_MIN_FREE_GIB=8 \
  BODYSENSE_VALIDATION_AVAILABLE_KIB=$((4 * 1024 * 1024)) \
    bash "$CHECK"
} 2>&1)"
fail_status=$?
set -e
if [ "$fail_status" -eq 0 ]; then
  echo "capacity fixture unexpectedly passed" >&2
  exit 1
fi
grep -q 'VALIDATION_HOST_CAPACITY=FAIL' <<<"$fail_output"
grep -q 'required_gib=8' <<<"$fail_output"

set +e
invalid_output="$(BODYSENSE_VALIDATION_MIN_FREE_GIB=bad bash "$CHECK" 2>&1)"
invalid_status=$?
set -e
if [ "$invalid_status" -eq 0 ]; then
  echo "invalid capacity policy unexpectedly passed" >&2
  exit 1
fi
grep -q 'reason=invalid-min-free-gib' <<<"$invalid_output"

echo "VALIDATION_HOST_CAPACITY_TEST=PASS"
