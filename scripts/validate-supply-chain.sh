#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

TMP_DIR=$(mktemp -d)
cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

NPM_REPORT="$TMP_DIR/pnpm-audit.json"
OSV_INPUT="$TMP_DIR/production-osv.json"
OSV_REPORT="$TMP_DIR/osv-report.json"
OSV_STDERR="$TMP_DIR/osv-stderr.log"
OSV_IMAGE="ghcr.io/google/osv-scanner@sha256:5b8b38e45bb2c5c4976f0f1f07860551ea6e1f235f642cf215f74d266fec2c1b"

parse_npm_report() {
  node - "$1" <<'NODE'
const fs = require('fs');
let payload;
try {
  payload = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
} catch (error) {
  console.error(`SUPPLY_CHAIN_NPM_AUDIT=UNAVAILABLE invalid_json=${error.message}`);
  process.exit(2);
}
const vulnerabilities = payload.metadata?.vulnerabilities;
if (!vulnerabilities || typeof vulnerabilities !== 'object') {
  console.error('SUPPLY_CHAIN_NPM_AUDIT=UNAVAILABLE missing_vulnerability_metadata');
  process.exit(2);
}
const high = Number(vulnerabilities.high ?? 0);
const critical = Number(vulnerabilities.critical ?? 0);
if (!Number.isFinite(high) || !Number.isFinite(critical)) {
  console.error('SUPPLY_CHAIN_NPM_AUDIT=UNAVAILABLE invalid_vulnerability_metadata');
  process.exit(2);
}
console.log(`SUPPLY_CHAIN_RUNTIME_ADVISORIES source=npm high=${high} critical=${critical}`);
if (high > 0 || critical > 0) process.exit(1);
NODE
}

parse_osv_report() {
  node - "$1" <<'NODE'
const fs = require('fs');
let payload;
try {
  payload = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
} catch (error) {
  console.error(`SUPPLY_CHAIN_OSV=FAIL invalid_json=${error.message}`);
  process.exit(2);
}
if (!Array.isArray(payload.results)) {
  console.error('SUPPLY_CHAIN_OSV=FAIL missing_results');
  process.exit(2);
}
let high = 0;
let critical = 0;
let unknown = 0;
for (const result of payload.results) {
  for (const pkg of result.packages ?? []) {
    for (const group of pkg.groups ?? []) {
      const score = Number(group.max_severity);
      if (!Number.isFinite(score)) {
        unknown += 1;
      } else if (score >= 9) {
        critical += 1;
      } else if (score >= 7) {
        high += 1;
      }
    }
  }
}
console.log(`SUPPLY_CHAIN_RUNTIME_ADVISORIES source=osv high=${high} critical=${critical} unknown=${unknown}`);
// Unknown severity is fail-closed: the fallback may only certify that the
// production graph is below the high threshold when every finding is scored.
if (high > 0 || critical > 0 || unknown > 0) process.exit(1);
NODE
}

# pnpm's audit is still the primary source because it has native knowledge of
# the pnpm workspace. Bound its availability wait: npm's bulk advisory endpoint
# can time out for minutes, which must not deadlock all repository quality lanes.
set +e
timeout -k 5s 45s pnpm audit --prod --audit-level=high --json >"$NPM_REPORT" 2>"$TMP_DIR/pnpm-audit.err"
pnpm_status=$?
parse_npm_report "$NPM_REPORT"
npm_parse_status=$?
set -e

case "$npm_parse_status" in
  0)
    if [[ "$pnpm_status" -ne 0 ]]; then
      echo "SUPPLY_CHAIN_NPM_AUDIT_WARNING pnpm_exit=$pnpm_status below_high_threshold=true" >&2
    fi
    echo 'SUPPLY_CHAIN_AUDIT=PASS source=npm'
    exit 0
    ;;
  1)
    echo 'SUPPLY_CHAIN_AUDIT=FAIL source=npm reason=high-or-critical-advisory' >&2
    exit 1
    ;;
  *)
    echo "SUPPLY_CHAIN_NPM_AUDIT=UNAVAILABLE pnpm_exit=$pnpm_status; using pinned OSV fallback" >&2
    ;;
esac

node scripts/production-npm-osv-input.mjs pnpm-lock.yaml >"$OSV_INPUT"

if [[ -n "${SUPPLY_CHAIN_OSV_REPORT_FIXTURE:-}" ]]; then
  cp "$SUPPLY_CHAIN_OSV_REPORT_FIXTURE" "$OSV_REPORT"
else
  command -v docker >/dev/null 2>&1 || { echo 'SUPPLY_CHAIN_OSV=FAIL docker unavailable' >&2; exit 1; }
  docker image inspect "$OSV_IMAGE" >/dev/null 2>&1 || docker pull "$OSV_IMAGE" >/dev/null
  set +e
  docker run --rm -v "$OSV_INPUT:/prod.json:ro" "$OSV_IMAGE" \
    scan source --format=json -L osv-scanner:/prod.json >"$OSV_REPORT" 2>"$OSV_STDERR"
  osv_status=$?
  set -e
  # OSV Scanner exits 1 whenever it finds any vulnerability, including low or
  # moderate findings. The parsed CVSS threshold below is authoritative.
  if [[ "$osv_status" -gt 1 ]]; then
    cat "$OSV_STDERR" >&2 || true
  fi
fi

set +e
parse_osv_report "$OSV_REPORT"
osv_parse_status=$?
set -e
if [[ "$osv_parse_status" -ne 0 ]]; then
  echo 'SUPPLY_CHAIN_AUDIT=FAIL source=osv' >&2
  exit 1
fi

echo 'SUPPLY_CHAIN_AUDIT=PASS source=osv-fallback'
