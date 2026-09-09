#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo="$(cd "$root/../../../.." && pwd)"
spec="$root/spec/bodysense-spike.openapi.yaml"
tools="$root/.tools/bin"
out="$root/metrics/oasdiff"
mkdir -p "$tools" "$out"

pnpm --package=@redocly/cli@2.51.2 dlx redocly lint "$spec" > "$root/metrics/redocly-lint.txt" 2>&1
"$root/scripts/make-openapi-mutations.sh"

if [[ ! -x "$tools/oasdiff" ]]; then
  (cd "$repo" && GOBIN="$tools" go install github.com/oasdiff/oasdiff@v1.31.0)
fi

base="$root/spec/bodysense-spike.bundle-ref.json"
printf 'mutation\texit_code\tbreaking_detected\tci_gate\n' > "$out/summary.tsv"

set +e
for mutation in "$root"/mutations/generated/M[1-6]-*.json; do
  id="$(basename "$mutation" .json)"
  "$tools/oasdiff" breaking "$base" "$mutation" --format json --fail-on ERR \
    > "$out/$id.json" 2> "$out/$id.err"
  code=$?
  text="$(jq -r '.. | .text? // empty' "$out/$id.json" 2>/dev/null | head -1)"
  if [[ -n "$text" ]]; then detected=yes; else detected=no; fi
  if [[ "$code" -eq 1 ]]; then gate=blocked; elif [[ "$code" -eq 0 ]]; then gate=passed; else gate=tool_error; fi
  printf '%s\t%s\t%s\t%s\n' "$id" "$code" "$detected" "$gate" >> "$out/summary.tsv"
done
set -e

cat "$out/summary.tsv"
