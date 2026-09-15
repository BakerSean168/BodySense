#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COLLECTION="$ROOT/postman/collections/BodySense Public API.postman_collection.json"

if ! command -v postman >/dev/null 2>&1; then
  echo "Postman CLI is required for native validation." >&2
  echo "Install it with: npm install -g postman-cli@1.56.2" >&2
  exit 1
fi

for environment in "$ROOT"/postman/environments/*.environment.json; do
  postman environment lint "$environment"
done

tmp_a="$(mktemp -d)"
tmp_b="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_a" "$tmp_b"
}
trap cleanup EXIT

postman collection migrate "$COLLECTION" -o "$tmp_a"
postman collection migrate "$COLLECTION" -o "$tmp_b"

diff -qr "$tmp_a" "$tmp_b" >/dev/null
postman collection lint "$tmp_a" --reporter cli --fail-severity error

echo "POSTMAN_NATIVE_VALIDATION=PASS"
