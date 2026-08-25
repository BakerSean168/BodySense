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
const vulnerabilities = payload.metadata?.vulnerabilities ?? {};
const high = Number(vulnerabilities.high ?? 0);
const critical = Number(vulnerabilities.critical ?? 0);
console.log(`SUPPLY_CHAIN_RUNTIME_ADVISORIES high=${high} critical=${critical}`);
if (high > 0 || critical > 0) process.exit(1);
NODE
summary_status=$?

if [[ "$audit_status" -ne 0 || "$summary_status" -ne 0 ]]; then
  echo "SUPPLY_CHAIN_AUDIT=FAIL" >&2
  exit 1
fi

echo "SUPPLY_CHAIN_AUDIT=PASS"
