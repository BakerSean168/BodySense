#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
BIN="$TMP/bin"
mkdir -p "$BIN"
REAL_NODE=$(command -v node)
REAL_TIMEOUT=$(command -v timeout)
ln -s "$REAL_NODE" "$BIN/node"
ln -s "$REAL_TIMEOUT" "$BIN/timeout"

write_fake_pnpm() {
  local payload="$1" status="$2"
  cat > "$BIN/pnpm" <<PNPM
#!/usr/bin/env bash
cat <<'JSON'
$payload
JSON
exit $status
PNPM
  chmod +x "$BIN/pnpm"
}

clean_osv='{"results":[{"packages":[]}]}'
high_osv='{"results":[{"packages":[{"package":{"name":"bad","version":"1.0.0","ecosystem":"npm"},"groups":[{"ids":["OSV-TEST"],"max_severity":"8.1"}]}]}]}'
unknown_osv='{"results":[{"packages":[{"package":{"name":"unknown","version":"1.0.0","ecosystem":"npm"},"groups":[{"ids":["OSV-UNKNOWN"],"max_severity":"N/A"}]}]}]}'
printf '%s\n' "$clean_osv" > "$TMP/osv-clean.json"
printf '%s\n' "$high_osv" > "$TMP/osv-high.json"
printf '%s\n' "$unknown_osv" > "$TMP/osv-unknown.json"

write_fake_pnpm '{"metadata":{"vulnerabilities":{"low":1,"moderate":2,"high":0,"critical":0}}}' 1
out=$(PATH="$BIN:/usr/bin:/bin" bash scripts/validate-supply-chain.sh 2>&1)
[[ "$out" == *'SUPPLY_CHAIN_AUDIT=PASS source=npm'* ]]

write_fake_pnpm '{"metadata":{"vulnerabilities":{"low":0,"moderate":0,"high":1,"critical":0}}}' 1
if PATH="$BIN:/usr/bin:/bin" SUPPLY_CHAIN_OSV_REPORT_FIXTURE="$TMP/osv-clean.json" bash scripts/validate-supply-chain.sh >/dev/null 2>&1; then
  echo 'high npm advisory fixture unexpectedly passed or fell back' >&2
  exit 1
fi

write_fake_pnpm '{"error":{"code":"ERR_PNPM_META_FETCH_FAIL"}}' 1
out=$(PATH="$BIN:/usr/bin:/bin" SUPPLY_CHAIN_OSV_REPORT_FIXTURE="$TMP/osv-clean.json" bash scripts/validate-supply-chain.sh 2>&1)
[[ "$out" == *'SUPPLY_CHAIN_AUDIT=PASS source=osv-fallback'* ]]

if PATH="$BIN:/usr/bin:/bin" SUPPLY_CHAIN_OSV_REPORT_FIXTURE="$TMP/osv-high.json" bash scripts/validate-supply-chain.sh >/dev/null 2>&1; then
  echo 'high OSV fallback fixture unexpectedly passed' >&2
  exit 1
fi
if PATH="$BIN:/usr/bin:/bin" SUPPLY_CHAIN_OSV_REPORT_FIXTURE="$TMP/osv-unknown.json" bash scripts/validate-supply-chain.sh >/dev/null 2>&1; then
  echo 'unknown-severity OSV fallback fixture unexpectedly passed' >&2
  exit 1
fi

echo 'SUPPLY_CHAIN_VALIDATOR_TEST=PASS'
