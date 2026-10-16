#!/usr/bin/env bash
# Source-only, bounded Go tool bootstrap retry helper.
# Expects ROOT and BIN from the caller. Does not change GOPROXY or GOSUMDB.

go_transport_failure_is_transient() {
  local log_file="$1"
  # Authentication, security, and module integrity errors are not retries.
  if grep -Eqi 'checksum mismatch|security error|invalid checksum|permission denied|access denied|401 Unauthorized|403 Forbidden|authentication required|invalid credentials' "$log_file"; then
    return 1
  fi
  grep -Eqi 'stream error: stream ID|INTERNAL_ERROR; received from peer|connection reset by peer|TLS handshake timeout|i/o timeout|502 Bad Gateway|503 Service Unavailable|unexpected EOF' "$log_file"
}

install_go_tool() {
  local name="$1" module="$2" version="$3"
  local stamp="$BIN/.$name-version" attempt=1 rc log_file
  if [[ -x "$BIN/$name" && -f "$stamp" && "$(cat "$stamp")" == "$version" ]]; then
    return 0
  fi
  echo "installing $name@$version" >&2
  log_file="$(mktemp)"
  while :; do
    if (cd "$ROOT" && GOTOOLCHAIN=auto GOBIN="$BIN" go install "$module@$version") >"$log_file" 2>&1; then
      cat "$log_file" >&2
      rm -f "$log_file"
      if [[ ! -x "$BIN/$name" ]]; then
        echo "Go tool installation reported success without executable: $name" >&2
        return 1
      fi
      printf '%s\n' "$version" > "$stamp"
      return 0
    else
      rc=$?
    fi
    cat "$log_file" >&2
    if [[ "$attempt" -ge 3 ]] || ! go_transport_failure_is_transient "$log_file"; then
      rm -f "$log_file"
      return "$rc"
    fi
    echo "transient Go transport failure installing $name; retry $((attempt + 1))/3" >&2
    sleep "$attempt"
    attempt=$((attempt + 1))
  done
}
