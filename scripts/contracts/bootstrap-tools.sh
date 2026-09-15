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

install_protoc() {
  local version="32.1"
  local arch asset sha256
  case "$(uname -m)" in
    x86_64)
      arch="x86_64"
      sha256="e9c129c176bb7df02546c4cd6185126ca53c89e7d2f09511e209319704b5dd7e"
      ;;
    aarch64|arm64)
      arch="aarch_64"
      sha256="4a802ed23d70f7bad7eb19e5a3e724b3aa967250d572cadfd537c1ba939aee6a"
      ;;
    *)
      echo "unsupported protoc bootstrap architecture: $(uname -m)" >&2
      return 1
      ;;
  esac

  local stamp="$BIN/.protoc-version"
  if [[ -x "$BIN/protoc" && -f "$stamp" && "$(cat "$stamp")" == "$version-$arch-$sha256" ]]; then
    return 0
  fi

  asset="protoc-${version}-linux-${arch}.zip"
  local tmp
  tmp="$(mktemp -d)"
  echo "installing protoc@$version ($arch)" >&2
  curl -fsSL \
    "https://github.com/protocolbuffers/protobuf/releases/download/v${version}/${asset}" \
    -o "$tmp/$asset"
  printf '%s  %s\n' "$sha256" "$tmp/$asset" | sha256sum -c - >/dev/null
  unzip -q "$tmp/$asset" -d "$tmp/protoc"
  install -m 0755 "$tmp/protoc/bin/protoc" "$BIN/protoc"
  rm -rf "$tmp"
  printf '%s\n' "$version-$arch-$sha256" > "$stamp"
}

install_go_tool oapi-codegen github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen v2.8.0
install_go_tool oasdiff github.com/oasdiff/oasdiff v1.31.0
install_go_tool buf github.com/bufbuild/buf/cmd/buf v1.72.0
install_go_tool protoc-gen-go google.golang.org/protobuf/cmd/protoc-gen-go v1.36.12
install_protoc
