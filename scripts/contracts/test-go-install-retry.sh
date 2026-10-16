#!/usr/bin/env bash
# Offline unit tests: no module requests, no secrets and no actual Go installs.
set -Eeuo pipefail
repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/mock-bin" "$tmp/install-bin" "$tmp/repo"
export ROOT="$tmp/repo" BIN="$tmp/install-bin"
export MOCK_STATE="$tmp/go-attempts" MOCK_SLEEP="$tmp/sleeps"
export PATH="$tmp/mock-bin:$PATH"
cat > "$tmp/mock-bin/go" <<'GO'
#!/usr/bin/env bash
set -euo pipefail
attempt=0
if [[ -f "$MOCK_STATE" ]]; then attempt="$(cat "$MOCK_STATE")"; fi
attempt=$((attempt + 1))
printf '%s\n' "$attempt" > "$MOCK_STATE"
test "$1" = "install"
case "$MOCK_SCENARIO" in
  transient_once)
    if [[ "$attempt" = 1 ]]; then
      echo 'verifying module: reading https://sum.golang.org/tile/: stream error: stream ID 155; INTERNAL_ERROR; received from peer' >&2
      exit 45
    fi
    ;;
  transient_forever)
    echo 'stream error: stream ID 156; INTERNAL_ERROR; received from peer' >&2
    exit 45
    ;;
  auth_denied)
    echo '403 Forbidden: permission denied' >&2
    exit 47
    ;;
  checksum_invalid)
    echo 'checksum mismatch (while transport saw connection reset by peer)' >&2
    exit 48
    ;;
  compile_failure)
    echo 'undefined: missingFunction' >&2
    exit 49
    ;;
  no_executable)
    exit 0
    ;;
  success)
    ;;
  *)
    echo "unknown mock scenario: $MOCK_SCENARIO" >&2
    exit 1
    ;;
esac
mkdir -p "$GOBIN"
printf '#!/usr/bin/env sh\nexit 0\n' > "$GOBIN/mocktool"
chmod +x "$GOBIN/mocktool"
GO
cat > "$tmp/mock-bin/sleep" <<'SLEEP'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$1" >> "$MOCK_SLEEP"
SLEEP
chmod +x "$tmp/mock-bin/go" "$tmp/mock-bin/sleep"
# Only the helper is sourced; bootstrap-tools.sh itself is NEVER executed.
# This avoids attempting protoc downloads and global tool installations.
source "$repo_root/scripts/contracts/lib/go-install-retry.sh"

reset_mock() {
  rm -f "$MOCK_STATE" "$MOCK_SLEEP" "$BIN/mocktool" "$BIN/.mocktool-version"
}
assert_attempts() {
  test "$(cat "$MOCK_STATE")" = "$1" || {
    echo "expected $1 Go installs, got $(cat "$MOCK_STATE")" >&2
    exit 1
  }
}
reset_mock
export MOCK_SCENARIO=transient_once
install_go_tool mocktool example.invalid/mocktool v1.2.3 >/dev/null 2>"$tmp/transient.log"
assert_attempts 2
test "$(cat "$BIN/.mocktool-version")" = v1.2.3
test -x "$BIN/mocktool"
test "$(wc -l < "$MOCK_SLEEP")" -eq 1
grep -F 'retry 2/3' "$tmp/transient.log" >/dev/null
echo 'GO_INSTALL_TRANSIENT_RETRY=PASS'

# Cache hit must not call go again.
install_go_tool mocktool example.invalid/mocktool v1.2.3 >/dev/null 2>&1
assert_attempts 2
echo 'GO_INSTALL_PINNED_CACHE_HIT=PASS'

reset_mock
export MOCK_SCENARIO=auth_denied
if install_go_tool mocktool example.invalid/mocktool v1.2.3 >"$tmp/auth.log" 2>&1; then
  echo 'Go tool installer retried/bypassed authentication denial' >&2; exit 1
fi
assert_attempts 1
test ! -e "$BIN/.mocktool-version"
test ! -e "$MOCK_SLEEP"
echo 'GO_INSTALL_AUTH_FAIL_CLOSED=PASS'

reset_mock
export MOCK_SCENARIO=checksum_invalid
if install_go_tool mocktool example.invalid/mocktool v1.2.3 >"$tmp/checksum.log" 2>&1; then
  echo 'Go tool installer bypassed checksum mismatch' >&2; exit 1
fi
assert_attempts 1
test ! -e "$BIN/.mocktool-version"
echo 'GO_INSTALL_CHECKSUM_FAIL_CLOSED=PASS'

reset_mock
export MOCK_SCENARIO=compile_failure
if install_go_tool mocktool example.invalid/mocktool v1.2.3 >"$tmp/compile.log" 2>&1; then
  echo 'Go tool installer bypassed compilation error' >&2; exit 1
fi
assert_attempts 1
test ! -e "$MOCK_SLEEP"
echo 'GO_INSTALL_COMPILE_FAIL_CLOSED=PASS'

reset_mock
export MOCK_SCENARIO=transient_forever
if install_go_tool mocktool example.invalid/mocktool v1.2.3 >"$tmp/exhausted.log" 2>&1; then
  echo 'Go tool installer claimed success after exhausting retries' >&2; exit 1
fi
assert_attempts 3
test ! -e "$BIN/.mocktool-version"
test "$(wc -l < "$MOCK_SLEEP")" -eq 2
echo 'GO_INSTALL_MAX_THREE_ATTEMPTS=PASS'

reset_mock
export MOCK_SCENARIO=no_executable
if install_go_tool mocktool example.invalid/mocktool v1.2.3 >"$tmp/missing-binary.log" 2>&1; then
  echo 'Go tool installer claimed success without executable' >&2; exit 1
fi
assert_attempts 1
test ! -e "$BIN/.mocktool-version"
echo 'GO_INSTALL_MISSING_BINARY_FAIL_CLOSED=PASS'
echo 'GO_INSTALL_RETRY_OFFLINE_UNIT_TESTS=7/7_PASS'
