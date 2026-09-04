#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
BIN="$TMP/bin"
mkdir -p "$BIN"
REAL_NODE=$(command -v node)
ln -s "$REAL_NODE" "$BIN/node"

write_fake_pnpm() {
  local payload="$1" status="$2"
  cat > "$BIN/pnpm" <<EOF
#!/usr/bin/env bash
cat <<'JSON'
$payload
JSON
exit $status
EOF
  chmod +x "$BIN/pnpm"
}

write_fake_pnpm '{"metadata":{"vulnerabilities":{"low":1,"moderate":2,"high":0,"critical":0}}}' 1
out=$(PATH="$BIN:/usr/bin:/bin" bash scripts/validate-supply-chain.sh 2>&1)
[[ "$out" == *'SUPPLY_CHAIN_AUDIT=PASS'* ]]
[[ "$out" == *'below_high_threshold=true'* ]]

write_fake_pnpm '{"metadata":{"vulnerabilities":{"low":0,"moderate":0,"high":1,"critical":0}}}' 1
if PATH="$BIN:/usr/bin:/bin" bash scripts/validate-supply-chain.sh >/dev/null 2>&1; then
  echo 'high advisory fixture unexpectedly passed' >&2
  exit 1
fi

write_fake_pnpm '{"error":{"code":"ERR_PNPM_META_FETCH_FAIL"}}' 1
if PATH="$BIN:/usr/bin:/bin" bash scripts/validate-supply-chain.sh >/dev/null 2>&1; then
  echo 'malformed/network audit fixture unexpectedly passed' >&2
  exit 1
fi

echo 'SUPPLY_CHAIN_VALIDATOR_TEST=PASS'
