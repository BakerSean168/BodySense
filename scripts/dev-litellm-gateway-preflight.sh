#!/usr/bin/env bash
set -Eeuo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# shellcheck source=scripts/dev-env.sh
source "$ROOT/scripts/dev-env.sh"

# Until the BodySense-only scoped Virtual Key, supplier permissions, and
# zero-cost Posture route are independently verified, never allow the local
# direct-dev scripts to activate an external model gateway.
if [[ "${BODYSENSE_LITELLM_DEV_ORACLE2:-0}" != "0" ]]; then
  echo "BodySense external LiteLLM route BLOCKED: scoped client authorization and free-only route acceptance are pending" >&2
  exit 1
fi

local_url="http://127.0.0.1:${LITELLM_PORT}/v1"
if [[ "$LITELLM_BASE_URL" != "$local_url" ]]; then
  echo "BodySense external LiteLLM route BLOCKED: direct-dev currently permits only the local gateway" >&2
  exit 1
fi
if [[ -z "${LITELLM_API_KEY:-}" ]]; then
  echo "BodySense local LiteLLM API key is missing" >&2
  exit 1
fi
