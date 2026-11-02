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

# The direct-dev compose publishes PostgreSQL, Redis, and the gateway on
# DEV_BIND_HOST. A wildcard or LAN bind would expose the default dev gateway
# credential beyond this workstation, even when the client URL is loopback.
if [[ "$DEV_BIND_HOST" != "127.0.0.1" ]]; then
  echo "BodySense direct-dev bind BLOCKED: DEV_BIND_HOST must be 127.0.0.1" >&2
  exit 1
fi

# Validate the port before constructing a URL. Otherwise a value such as
# 20112@remote.invalid makes 127.0.0.1 URL userinfo, not the network host.
if [[ ! "$LITELLM_PORT" =~ ^[1-9][0-9]{0,4}$ ]] || (( 10#$LITELLM_PORT > 65535 )); then
  echo "BodySense local LiteLLM port is invalid: expected decimal TCP port 1-65535" >&2
  exit 1
fi

local_url="http://127.0.0.1:${LITELLM_PORT}/v1"
if [[ "$LITELLM_BASE_URL" != "$local_url" ]]; then
  echo "BodySense external LiteLLM route BLOCKED: direct-dev currently permits only the local gateway" >&2
  exit 1
fi
# An absent client credential is allowed only because the default dev gateway
# has no model routes. Actual inference fails closed until a scoped key exists.
# Never permit the infrastructure master credential to stand in for that key.
if [[ -n "${LITELLM_API_KEY:-}" && "$LITELLM_API_KEY" == "$LITELLM_MASTER_KEY" ]]; then
  echo "BodySense LiteLLM client credential BLOCKED: master-key alias is forbidden" >&2
  exit 1
fi
