#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
spec="$root/spec/bodysense-spike.openapi.yaml"
base="$root/spec/bodysense-spike.bundle-ref.json"
out="$root/mutations/generated"
mkdir -p "$out"

pnpm --package=@redocly/cli@2.51.2 dlx redocly bundle "$spec" --output "$base" >/dev/null

jq '
  .components.schemas.HealthWorkspace.properties.generated_time = .components.schemas.HealthWorkspace.properties.generated_at |
  del(.components.schemas.HealthWorkspace.properties.generated_at) |
  .components.schemas.HealthWorkspace.required |= map(if . == "generated_at" then "generated_time" else . end)
' "$base" > "$out/M1-required-field-rename.json"

jq '.components.schemas.WorkspaceAction.properties.priority = {"type":"string"}' \
  "$base" > "$out/M2-field-type-change.json"

jq '.components.schemas.ReviewBodyStateFactRequest.properties.review_state.enum = ["confirmed"]' \
  "$base" > "$out/M3-enum-value-removal.json"

jq '.components.schemas.BodyStateFactInput.required += ["concern_key"] | .components.schemas.BodyStateFactInput.required |= unique' \
  "$base" > "$out/M4-request-optional-to-required.json"

jq '.components.schemas.HealthWorkspace.properties.conversation_id = {"type":"string","format":"uuid"}' \
  "$base" > "$out/M5-nullable-to-non-null.json"

jq '.components.schemas.HealthWorkspace.properties.contract_revision = {"type":"string"}' \
  "$base" > "$out/M6-add-optional-field.json"

jq '.unexpected_contract_field = "forward-compatible-control"' \
  "$root/fixtures/health-workspace.valid.json" > "$out/M7-health-workspace-unknown-field.json"

cp "$root/fixtures/health-workspace.invalid-nested.json" "$out/M8-health-workspace-malformed-nested.json"

echo "Generated OpenAPI/fixture mutations under $out"
