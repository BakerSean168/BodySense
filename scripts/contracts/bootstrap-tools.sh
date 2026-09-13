#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BIN="$ROOT/.tools/contracts/bin"
mkdir -p "$BIN"

install_go_tool() {
  local name="$1"
  local module="$2"
  local version="$3"
  local stamp="$BIN/.$name-version"
  if [[ -x "$BIN/$name" && -f "$stamp" && "$(cat "$stamp")" == "$version" ]]; then
    return 0
  fi
  echo "installing $name@$version" >&2
  (cd "$ROOT" && GOTOOLCHAIN=auto GOBIN="$BIN" go install "$module@$version")
  printf '%s\n' "$version" > "$stamp"
}

install_go_tool oapi-codegen github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen v2.8.0
install_go_tool oasdiff github.com/oasdiff/oasdiff v1.31.0
install_go_tool buf github.com/bufbuild/buf/cmd/buf v1.72.0
