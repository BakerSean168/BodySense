#!/usr/bin/env bash
set -euo pipefail

ROOT="${ROOT:-/opt/bodysense}"
AI_IMAGE="${AI_IMAGE:-}"
SAMPLES="${SAMPLES:-20}"
CONCURRENCY="${CONCURRENCY:-3}"
MIN_START_INTERVAL_SECONDS="${MIN_START_INTERVAL_SECONDS:-5}"
REPORT_DIR="${REPORT_DIR:-$ROOT/apps/ai-service/data/evals/reports}"
PREFLIGHT_SCRIPT="${PREFLIGHT_SCRIPT:-$ROOT/scripts/diagnosis-production-provider-preflight.py}"
FINAL_REPORT="$REPORT_DIR/diagnosis_v10_production_provider_acceptance.json"
PREFLIGHT_REPORT="$REPORT_DIR/diagnosis_production_provider_preflight.json"

if [[ -z "$AI_IMAGE" ]]; then
  echo "AI_IMAGE is required and must be an immutable @sha256 digest" >&2
  exit 64
fi
if [[ "$AI_IMAGE" != *@sha256:* ]]; then
  echo "AI_IMAGE must use an immutable repository@sha256:digest reference" >&2
  exit 64
fi

compose=(
  docker compose
  -f "$ROOT/docker/docker-compose.prod.yml"
  --env-file "$ROOT/.env.production"
  --env-file "$ROOT/.env.production.local"
)

mkdir -p "$REPORT_DIR"
work_dir="$(mktemp -d)"
secret_env="$(mktemp)"
cleanup() {
  rm -rf "$work_dir"
  rm -f "$secret_env"
}
trap cleanup EXIT
chmod 600 "$secret_env"

before="$work_dir/preflight-before.json"
after="$work_dir/preflight-after.json"
raw_report_dir="$work_dir/output"
mkdir -p "$raw_report_dir"
chmod 777 "$raw_report_dir"

python3 "$PREFLIGHT_SCRIPT" \
  --root "$ROOT" \
  --json-output "$before" \
  --require-ready

gateway_id="$("${compose[@]}" ps -q litellm-gateway)"
if [[ -z "$gateway_id" ]]; then
  echo "production LiteLLM gateway container not found" >&2
  exit 65
fi
gateway_name="$(docker inspect "$gateway_id" --format '{{.Name}}' | sed 's#^/##')"
network_name="$(
  docker inspect "$gateway_id" \
    --format '{{range $name, $network := .NetworkSettings.Networks}}{{println $name}}{{end}}' \
    | head -n 1
)"
if [[ -z "$network_name" ]]; then
  echo "production LiteLLM network not found" >&2
  exit 65
fi

{
  printf 'LITELLM_BASE_URL=http://%s:4000/v1\n' "$gateway_name"
  printf 'LITELLM_API_KEY='
  docker exec "$gateway_id" printenv LITELLM_MASTER_KEY
} >"$secret_env"

expected_model="$(
  python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["configured_primary_model"])' "$before"
)"

docker pull "$AI_IMAGE" >/dev/null

docker run --rm \
  --network "$network_name" \
  --env-file "$secret_env" \
  --volume "$raw_report_dir:/qualification" \
  "$AI_IMAGE" \
  /app/.venv/bin/python /app/scripts/run_diagnosis_provider_acceptance.py \
    --samples "$SAMPLES" \
    --concurrency "$CONCURRENCY" \
    --min-start-interval-seconds "$MIN_START_INTERVAL_SECONDS" \
    --environment production-candidate \
    --physical-model "$expected_model" \
    --json-output /qualification/provider-acceptance-raw.json

python3 "$PREFLIGHT_SCRIPT" \
  --root "$ROOT" \
  --json-output "$after" \
  --require-ready

python3 - "$before" "$after" "$raw_report_dir/provider-acceptance-raw.json" "$FINAL_REPORT" "$PREFLIGHT_REPORT" <<'PY'
import json
import pathlib
import sys

before_path, after_path, raw_path, final_path, preflight_path = map(pathlib.Path, sys.argv[1:])
before = json.loads(before_path.read_text())
after = json.loads(after_path.read_text())
report = json.loads(raw_path.read_text())

expected_model = before["configured_primary_model"]
for phase, evidence in (("before", before), ("after", after)):
    if evidence.get("ready_for_primary_qualification") is not True:
        raise SystemExit(f"{phase} provider preflight is not ready")
    probe = evidence.get("logical_probe") or {}
    if probe.get("http_status") != 200:
        raise SystemExit(f"{phase} route probe did not return HTTP 200")
    if probe.get("actual_model") != expected_model:
        raise SystemExit(f"{phase} route probe did not use {expected_model}")
    if int(probe.get("attempted_fallbacks") or 0) != 0:
        raise SystemExit(f"{phase} route probe used a fallback")

summary = report.get("summary") or {}
if int(summary.get("total") or 0) < 20:
    raise SystemExit("provider acceptance has fewer than 20 samples")
if int(summary.get("successes") or 0) != int(summary.get("total") or 0):
    raise SystemExit("provider acceptance is not 100% successful")
for key in ("errors", "contract_failures", "governance_rejections", "configuration_mismatches"):
    if int(summary.get(key) or 0) != 0:
        raise SystemExit(f"provider acceptance contains {key}")

report["route_attestation"] = {
    "before": before["logical_probe"],
    "after": after["logical_probe"],
}
report["runtime_preflight"] = {
    "gateway_healthy": before["gateway_healthy"],
    "primary_credential_present": before["primary_credential_present"],
    "fallback_credential_present": before["fallback_credential_present"],
    "configured_primary_model": expected_model,
    "configured_fallback_model": before.get("configured_fallback_model"),
}

final_path.parent.mkdir(parents=True, exist_ok=True)
final_path.write_text(json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True) + "\n")
preflight_path.write_text(json.dumps(after, ensure_ascii=False, indent=2, sort_keys=True) + "\n")
print(f"PRODUCTION_PROVIDER_ACCEPTANCE=PASS samples={summary['total']} model={expected_model}")
print(f"Wrote {final_path}")
PY
