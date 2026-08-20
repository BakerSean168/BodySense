#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
IMAGE="${LITELLM_IMAGE:-docker.litellm.ai/berriai/litellm:v1.97.0}"
MASTER_KEY="sk-bodysense-gateway-smoke"
NAME="bodysense-litellm-smoke-$$"
CONFIG="$ROOT/docker/litellm/config.smoke.yaml"

cleanup() {
  docker rm -f "$NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker run -d \
  --name "$NAME" \
  -p 127.0.0.1::4000 \
  -e "LITELLM_MASTER_KEY=$MASTER_KEY" \
  -v "$CONFIG:/app/config.yaml:ro" \
  "$IMAGE" \
  --config /app/config.yaml >/dev/null

host_port=""
for _ in $(seq 1 60); do
  host_port="$(docker port "$NAME" 4000/tcp 2>/dev/null | awk -F: 'NR==1 {print $NF}')"
  if [[ -n "$host_port" ]] && curl -fsS "http://127.0.0.1:${host_port}/health/liveliness" >/dev/null 2>&1; then
    break
  fi
  if ! docker inspect -f '{{.State.Running}}' "$NAME" 2>/dev/null | grep -q true; then
    docker logs "$NAME" >&2 || true
    exit 1
  fi
  sleep 1
done

if [[ -z "$host_port" ]]; then
  echo "LiteLLM gateway did not publish a host port" >&2
  exit 1
fi
curl -fsS "http://127.0.0.1:${host_port}/health/liveliness" >/dev/null

unauth_status="$(curl -sS -o /dev/null -w '%{http_code}' \
  "http://127.0.0.1:${host_port}/v1/chat/completions" \
  -H 'Content-Type: application/json' \
  -d '{"model":"bodysense-diagnosis","messages":[{"role":"user","content":"ping"}]}')"
if [[ "$unauth_status" != "401" ]]; then
  echo "Expected unauthenticated gateway request to return 401, got $unauth_status" >&2
  exit 1
fi

response="$(curl -fsS \
  "http://127.0.0.1:${host_port}/v1/chat/completions" \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"bodysense-diagnosis","messages":[{"role":"user","content":"ping"}]}')"

python3 - "$response" <<'PY'
import json
import sys

body = json.loads(sys.argv[1])
assert body["object"] == "chat.completion", body
assert body["choices"][0]["message"]["content"] == "bodysense-gateway-fallback-ok", body
usage = body.get("usage") or {}
assert int(usage.get("total_tokens") or 0) > 0, body
print(
    "LITELLM_GATEWAY_SMOKE=PASS "
    f"model={body.get('model')} total_tokens={usage.get('total_tokens')} fallback=verified"
)
PY

(
  cd "$ROOT/apps/ai-service"
  export LITELLM_BASE_URL="http://127.0.0.1:${host_port}/v1"
  export LITELLM_API_KEY="$MASTER_KEY"
  uv run python - <<'PY'
from pydantic_ai import Agent

from src.ai.diagnosis_gateway_model import get_diagnosis_gateway_model

get_diagnosis_gateway_model.cache_clear()
agent = Agent(get_diagnosis_gateway_model(), output_type=str)
result = agent.run_sync("gateway adapter smoke")
assert result.output == "bodysense-gateway-fallback-ok", result.output
print("PYDANTICAI_LITELLM_ADAPTER_SMOKE=PASS logical_model=bodysense-diagnosis")
PY

  uv run python - <<'PY'
import asyncio

from src.ai import AIService, AiRequest
from src.ai.gateway import (
    ASSESSMENT_ROUTE,
    CONSULTATION_ROUTE,
    POSTURE_ROUTE,
    TITLE_ROUTE,
)
from src.ai.types import ChatMessage


async def main() -> None:
    service = AIService()
    cases = [
        (CONSULTATION_ROUTE, "bodysense-consultation-ok"),
        (ASSESSMENT_ROUTE, '{"status":"ok"}'),
        (TITLE_ROUTE, "bodysense-text-ok"),
        (POSTURE_ROUTE, '{"view":"front","findings":[]}'),
    ]
    for route, expected in cases:
        response = await service.generate(
            AiRequest(route, [ChatMessage(role="user", content="gateway route smoke")])
        )
        assert response.text == expected, (route, response.text)
        assert response.provider == "litellm-gateway", response.provider
    streamed = ""
    async for event in service.generate_stream(
        AiRequest(CONSULTATION_ROUTE, [ChatMessage(role="user", content="stream smoke")])
    ):
        if event.type == "text_delta":
            streamed += event.text or ""
    assert streamed == "bodysense-consultation-ok", streamed
    print("AI_SERVICE_LITELLM_GATEWAY_SMOKE=PASS logical_groups=4 streaming=verified")


asyncio.run(main())
PY
)
