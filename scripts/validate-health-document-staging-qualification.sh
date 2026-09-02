#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT=${STAGING_COMPOSE_PROJECT:-bodysense-staging}
WEB_BASE=${BODYSENSE_STAGING_WEB_BASE:-http://127.0.0.1:20150}
QUALIFICATION_ID=${HEALTH_DOCUMENT_QUALIFICATION_CONFIGURATION_ID:-hdex-config-f2495c95b6ed9de2}
CHAMPION_ID=${HEALTH_DOCUMENT_CHAMPION_CONFIGURATION_ID:-hdex-config-14af808ef184bf8b}
TIMEOUT_SECONDS=${HEALTH_DOCUMENT_QUALIFICATION_TIMEOUT_SECONDS:-180}

api_container=$(docker ps -q \
  --filter "label=com.docker.compose.project=$PROJECT" \
  --filter 'label=com.docker.compose.service=api' | head -n 1)
[[ -n "$api_container" ]] || { echo "staging API container not found for project $PROJECT" >&2; exit 2; }

compose_file=$(docker inspect "$api_container" --format '{{index .Config.Labels "com.docker.compose.project.config_files"}}')
env_file=$(docker inspect "$api_container" --format '{{index .Config.Labels "com.docker.compose.project.environment_file"}}')
api_image=$(docker inspect "$api_container" --format '{{.Config.Image}}')
[[ -f "$compose_file" ]] || { echo "staging compose file not found: $compose_file" >&2; exit 2; }
[[ -f "$env_file" ]] || { echo "staging env file not found: $env_file" >&2; exit 2; }

compose=(docker compose -p "$PROJECT" -f "$compose_file" --env-file "$env_file")
work=$(mktemp -d /tmp/bodysense-health-document-staging-qualification-XXXXXX)
report="$work/report.pdf"
register_json="$work/register.json"
upload_json="$work/upload.json"
email="healthdoc-qualification-$(date +%s)-$RANDOM@invalid.local"
password="Qualification-$(date +%s)-$RANDOM-Aa1!"
upload_id=""
user_id=""
stage_changed=0
container_report="/tmp/healthdoc-qualification-$$.pdf"

cleanup_test_data() {
  if [[ -n "$upload_id" && -n "${access_token:-}" ]]; then
    curl -fsS --max-time 10 -X DELETE \
      -H "Authorization: Bearer $access_token" \
      "$WEB_BASE/api/v1/uploads/$upload_id" >/dev/null 2>&1 || true
  fi
  local postgres_container
  postgres_container=$(docker ps -q \
    --filter "label=com.docker.compose.project=$PROJECT" \
    --filter 'label=com.docker.compose.service=postgres' | head -n 1)
  if [[ -n "$postgres_container" ]]; then
    docker exec "$postgres_container" sh -lc \
      "psql -v ON_ERROR_STOP=1 -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" -c \"DELETE FROM users WHERE email = '$email';\"" \
      >/dev/null 2>&1 || true
  fi
}

restore_champion() {
  (( stage_changed == 1 )) || return 0
  echo "restoring staging health-document stage=champion"
  HEALTH_DOCUMENT_STAGE=champion \
  HEALTH_DOCUMENT_CHAMPION_CONFIGURATION_ID="$CHAMPION_ID" \
  HEALTH_DOCUMENT_QUALIFICATION_CONFIGURATION_ID="$QUALIFICATION_ID" \
  STAGING_API_IMAGE="$api_image" \
    "${compose[@]}" up -d --no-deps --no-build --force-recreate api >/dev/null
  local deadline=$((SECONDS + 90))
  until docker inspect "$PROJECT-api-1" --format '{{.State.Health.Status}}' 2>/dev/null | grep -qx healthy; do
    (( SECONDS < deadline )) || { echo 'staging API did not become healthy after restoring champion' >&2; return 1; }
    sleep 2
  done
  stage_changed=0
}

cleanup() {
  local status=$?
  cleanup_test_data
  restore_champion || status=1
  if [[ -n "${document_container:-}" ]]; then
    docker exec "$document_container" rm -f "$container_report" >/dev/null 2>&1 || true
  fi
  rm -rf "$work"
  exit "$status"
}
trap cleanup EXIT INT TERM

current_revision=$(docker inspect "$api_container" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}')
[[ -n "$current_revision" ]] || { echo 'staging API image is missing revision label' >&2; exit 2; }

document_container=$(docker ps -q \
  --filter "label=com.docker.compose.project=$PROJECT" \
  --filter 'label=com.docker.compose.service=document-service' | head -n 1)
[[ -n "$document_container" ]] || { echo 'staging document-service container is not running' >&2; exit 2; }
document_revision=$(docker inspect "$document_container" --format '{{index .Config.Labels "org.opencontainers.image.revision"}}')
document_health=$(docker inspect "$document_container" --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}')
[[ "$document_revision" == "$current_revision" ]] || { echo "staging document-service revision mismatch: $document_revision != $current_revision" >&2; exit 2; }
[[ "$document_health" == "healthy" ]] || { echo "staging document-service is not healthy: $document_health" >&2; exit 2; }

current_stage=$(docker inspect "$api_container" --format '{{range .Config.Env}}{{println .}}{{end}}' | sed -n 's/^HEALTH_DOCUMENT_STAGE=//p' | tail -n1)
current_champion=$(docker inspect "$api_container" --format '{{range .Config.Env}}{{println .}}{{end}}' | sed -n 's/^HEALTH_DOCUMENT_CHAMPION_CONFIGURATION_ID=//p' | tail -n1)
current_qualification=$(docker inspect "$api_container" --format '{{range .Config.Env}}{{println .}}{{end}}' | sed -n 's/^HEALTH_DOCUMENT_QUALIFICATION_CONFIGURATION_ID=//p' | tail -n1)
[[ "${current_stage:-champion}" == "champion" ]] || { echo "refusing: staging is already in non-champion stage: $current_stage" >&2; exit 2; }
[[ "$current_champion" == "$CHAMPION_ID" ]] || { echo "unexpected staging champion: $current_champion" >&2; exit 2; }
[[ "$current_qualification" == "$QUALIFICATION_ID" ]] || { echo "unexpected staging qualification candidate: $current_qualification" >&2; exit 2; }

# Generate a tiny born-digital report inside the exact deployed document runtime.
# This is a mechanics fixture only; it is never persisted in Git or used as
# real-layout Champion-selection evidence.
docker exec -i "$document_container" python - "$container_report" <<'PY'
from pathlib import Path
import sys
import fitz
path = Path(sys.argv[1])
doc = fitz.open()
page = doc.new_page(width=595, height=842)
page.insert_font(fontname="china-s")
page.insert_text((72, 96), "BodySense staging qualification / 非真实患者数据", fontname="china-s", fontsize=12)
page.insert_text((72, 140), "血红蛋白 142 g/L 参考范围 130-175", fontname="china-s", fontsize=12)
doc.save(path, no_new_id=True)
doc.close()
PY
docker cp "$document_container:$container_report" "$report" >/dev/null
[[ -s "$report" ]] || { echo 'failed to create qualification report' >&2; exit 2; }

# Flip only the staging API control plane. The document-service image and all
# durable stores remain unchanged; finally/trap always restores Champion.
echo "switching staging health-document stage=qualification revision=$current_revision candidate=$QUALIFICATION_ID"
stage_changed=1
HEALTH_DOCUMENT_STAGE=qualification \
HEALTH_DOCUMENT_CHAMPION_CONFIGURATION_ID="$CHAMPION_ID" \
HEALTH_DOCUMENT_QUALIFICATION_CONFIGURATION_ID="$QUALIFICATION_ID" \
STAGING_API_IMAGE="$api_image" \
  "${compose[@]}" up -d --no-deps --no-build --force-recreate api >/dev/null

deadline=$((SECONDS + 90))
until docker inspect "$PROJECT-api-1" --format '{{.State.Health.Status}}' 2>/dev/null | grep -qx healthy; do
  (( SECONDS < deadline )) || { echo 'staging API did not become healthy in qualification stage' >&2; exit 1; }
  sleep 2
done

stage_after=$(docker inspect "$PROJECT-api-1" --format '{{range .Config.Env}}{{println .}}{{end}}' | sed -n 's/^HEALTH_DOCUMENT_STAGE=//p' | tail -n1)
[[ "$stage_after" == "qualification" ]] || { echo "qualification stage was not applied: $stage_after" >&2; exit 1; }

curl -fsS --max-time 15 -H 'Content-Type: application/json' \
  -d "{\"email\":\"$email\",\"password\":\"$password\"}" \
  "$WEB_BASE/api/v1/auth/register" >"$register_json"
access_token=$(jq -r '.access_token // empty' "$register_json")
[[ -n "$access_token" ]] || { echo 'registration did not return access_token' >&2; cat "$register_json" >&2; exit 1; }

# Resolve the user id through the authenticated API for deterministic cleanup/evidence.
me_json=$(curl -fsS --max-time 10 -H "Authorization: Bearer $access_token" "$WEB_BASE/api/v1/me")
user_id=$(jq -r '.id // empty' <<<"$me_json")
[[ -n "$user_id" ]] || { echo 'unable to resolve staging qualification user id' >&2; exit 1; }

curl -fsS --max-time 20 \
  -H "Authorization: Bearer $access_token" \
  -F 'file_type=report' \
  -F "file=@$report;type=application/pdf" \
  "$WEB_BASE/api/v1/uploads" >"$upload_json"
upload_id=$(jq -r '.id // empty' "$upload_json")
[[ -n "$upload_id" ]] || { echo 'upload response missing id' >&2; cat "$upload_json" >&2; exit 1; }

# Upload jobs are polled every 10 seconds by the durable Go worker.
deadline=$((SECONDS + TIMEOUT_SECONDS))
ocr_status=""
while (( SECONDS < deadline )); do
  upload_state=$(curl -fsS --max-time 10 -H "Authorization: Bearer $access_token" "$WEB_BASE/api/v1/uploads/$upload_id")
  ocr_status=$(jq -r '.ocr_status // empty' <<<"$upload_state")
  case "$ocr_status" in
    completed) break ;;
    failed) echo "staging qualification OCR failed: $upload_state" >&2; exit 1 ;;
  esac
  sleep 3
done
[[ "$ocr_status" == "completed" ]] || { echo "OCR did not complete before timeout; last status=$ocr_status" >&2; exit 1; }

postgres_container=$(docker ps -q \
  --filter "label=com.docker.compose.project=$PROJECT" \
  --filter 'label=com.docker.compose.service=postgres' | head -n 1)
[[ -n "$postgres_container" ]] || { echo 'staging postgres container not found' >&2; exit 2; }

run_row=$(docker exec "$postgres_container" sh -lc \
  "psql -At -F '|' -U \"\$POSTGRES_USER\" -d \"\$POSTGRES_DB\" -c \"SELECT configuration_id, mechanism_revision, mechanism_provenance->>'verification_revision', mechanism_provenance->>'admissibility_policy_revision', document_sha256, result_sha256, raw_text_sha256 FROM document_extraction_runs WHERE upload_id = '$upload_id'::uuid ORDER BY created_at DESC LIMIT 1;\"")
[[ -n "$run_row" ]] || { echo 'no durable document_extraction_run was created' >&2; exit 1; }
IFS='|' read -r configuration_id mechanism_revision verification_revision admissibility_revision document_sha result_sha raw_text_sha <<<"$run_row"

[[ "$configuration_id" == "$QUALIFICATION_ID" ]] || { echo "durable configuration mismatch: $configuration_id" >&2; exit 1; }
[[ "$mechanism_revision" == "health-document-extraction-v20" ]] || { echo "mechanism revision mismatch: $mechanism_revision" >&2; exit 1; }
[[ "$verification_revision" == "health-document-row-verification-v7-percent-unit-normalization" ]] || { echo "verification revision mismatch: $verification_revision" >&2; exit 1; }
[[ "$admissibility_revision" == "ocr-indicator-admissibility-v2" ]] || { echo "admissibility revision mismatch: $admissibility_revision" >&2; exit 1; }
for value in "$document_sha" "$result_sha" "$raw_text_sha"; do
  [[ "$value" =~ ^[0-9a-f]{64}$ ]] || { echo "invalid durable SHA256: $value" >&2; exit 1; }
done

echo "HEALTH_DOCUMENT_STAGING_QUALIFICATION=PASS revision=$current_revision configuration_id=$configuration_id upload_id=$upload_id"
