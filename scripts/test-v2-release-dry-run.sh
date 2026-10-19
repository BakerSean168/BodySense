#!/usr/bin/env bash
# Behavioural guard for scripts/v2-release-dry-run.sh: it must print the plan
# with every human approval point and must never invoke a remote-capable tool.
set -Eeuo pipefail
ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin"
calls="$work/calls.log"
: > "$calls"
for tool in ssh scp rsync docker gh curl wget psql systemctl kubectl aliyun gcloud; do
  cat > "$work/bin/$tool" <<STUB
#!/usr/bin/env bash
echo "$tool \$*" >> "$calls"
exit 97
STUB
  chmod +x "$work/bin/$tool"
done

run_dry() {
  local status=0
  V2_DRY_RUN_ALLOW_DIRTY=1 PATH="$work/bin:$PATH" bash scripts/v2-release-dry-run.sh "$@" > "$work/out.txt" 2>&1 || status=$?
  echo "$status"
}

status="$(run_dry)"
case "$status" in
  0|3) ;;
  *) cat "$work/out.txt"; echo "dry run exited $status (expected 0 or 3)" >&2; exit 1 ;;
esac
if [[ -s "$calls" ]]; then
  cat "$calls" >&2
  echo 'dry run invoked a remote-capable tool' >&2
  exit 1
fi
for needle in \
  'HUMAN APPROVAL A1: staging wipe' \
  'HUMAN APPROVAL A2: production wipe' \
  'HUMAN APPROVAL A3: canary promote' \
  'DRY_RUN_REMOTE_ACTIONS=0' \
  'bash scripts/schema/check-snapshot.sh' \
  'pnpm contracts:verify' \
  'bash scripts/validate-supply-chain.sh' \
  'Browser longitudinal health E2E child'; do
  grep -Fq "$needle" "$work/out.txt" || { cat "$work/out.txt"; echo "missing: $needle" >&2; exit 1; }
done

# Approval points must precede the destructive steps they guard.
line_of() { grep -nF -- "$1" "$work/out.txt" | head -1 | cut -d: -f1; }
a1="$(line_of 'HUMAN APPROVAL A1')"
vrm="$(line_of 'docker volume rm bodysense-staging_postgres-staging-vnext-data')"
a2="$(line_of 'HUMAN APPROVAL A2')"
a3="$(line_of 'HUMAN APPROVAL A3')"
deploy="$(line_of 'gh workflow run deploy-production.yml')"
[[ -n "$a1" && -n "$vrm" && -n "$a2" && -n "$a3" && -n "$deploy" ]] &&
  [[ "$a1" -lt "$vrm" && "$a2" -lt "$a3" && "$a3" -lt "$deploy" ]] || {
  echo "approval ordering broken: A1=$a1 staging-rm=$vrm A2=$a2 A3=$a3 deploy=$deploy" >&2
  exit 1
}

# Scoped targets and unsupported execute flags.
status="$(run_dry --target staging)"
[[ "$status" == 0 || "$status" == 3 ]] || { echo "staging target exited $status" >&2; exit 1; }
! grep -Fq 'HUMAN APPROVAL A2' "$work/out.txt" || { echo 'staging target printed production steps' >&2; exit 1; }
status="$(run_dry --execute)"
[[ "$status" == 2 ]] || { echo "--execute must be rejected (exit 2), got $status" >&2; exit 1; }
[[ ! -s "$calls" ]] || { cat "$calls" >&2; exit 1; }

echo 'V2_RELEASE_DRY_RUN_TEST=PASS'
