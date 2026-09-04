#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

report="$(mktemp)"
cleanup() { rm -f "$report"; }
trap cleanup EXIT

set +e
pnpm audit --prod --audit-level=high --json >"$report"
audit_status=$?
set -e

node - "$report" <<'NODE'
const fs = require('fs');
const path = process.argv[2];
let payload;
try {
  payload = JSON.parse(fs.readFileSync(path, 'utf8'));
} catch (error) {
  console.error(`SUPPLY_CHAIN_AUDIT=FAIL invalid_audit_report: ${error.message}`);
  process.exit(2);
}
const vulnerabilities = payload.metadata?.vulnerabilities;
if (!vulnerabilities || typeof vulnerabilities !== 'object') {
  console.error('SUPPLY_CHAIN_AUDIT=FAIL missing_vulnerability_metadata');
  process.exit(2);
}
const high = Number(vulnerabilities.high ?? 0);
const critical = Number(vulnerabilities.critical ?? 0);
if (!Number.isFinite(high) || !Number.isFinite(critical)) {
  console.error('SUPPLY_CHAIN_AUDIT=FAIL invalid_vulnerability_metadata');
  process.exit(2);
}
console.log(`SUPPLY_CHAIN_RUNTIME_ADVISORIES high=${high} critical=${critical}`);
if (high > 0 || critical > 0) process.exit(1);
NODE
summary_status=$?

if [[ "$summary_status" -ne 0 ]]; then
  echo "SUPPLY_CHAIN_AUDIT=FAIL" >&2
  exit 1
fi

# pnpm may return non-zero when lower-severity advisories exist even though the
# policy threshold is --audit-level=high. Once a structurally valid report has
# proven high=0 and critical=0, the parsed severity summary is authoritative.
# Network/registry failures remain fail-closed because they do not contain the
# required metadata and therefore fail the parser above.
if [[ "$audit_status" -ne 0 ]]; then
  echo "SUPPLY_CHAIN_AUDIT_WARNING pnpm_exit=$audit_status below_high_threshold=true" >&2
fi

echo "SUPPLY_CHAIN_AUDIT=PASS"
