#!/usr/bin/env bash
set -Eeuo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
ENV_FILE=${BODYSENSE_DEV_ENV_FILE:-$ROOT/.env.dev.local}

if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

# BodySense direct-dev owns the 20100-20199 project block. The local env file
# may override these values; otherwise the canonical project allocation wins.
export DEV_BIND_HOST=${DEV_BIND_HOST:-127.0.0.1}
export BODYSENSE_WEB_PORT=${BODYSENSE_WEB_PORT:-${WEB_PORT:-20100}}
export WEB_PORT=${WEB_PORT:-$BODYSENSE_WEB_PORT}
export API_HOST=${API_HOST:-127.0.0.1}
export API_PORT=${API_PORT:-20101}
export AI_SERVICE_PORT=${AI_SERVICE_PORT:-20102}
export DOCUMENT_SERVICE_PORT=${DOCUMENT_SERVICE_PORT:-20103}
export DB_HOST=${DB_HOST:-127.0.0.1}
export DB_PORT=${DB_PORT:-20110}
export REDIS_HOST=${REDIS_HOST:-127.0.0.1}
export REDIS_PORT=${REDIS_PORT:-20111}
export LITELLM_PORT=${LITELLM_PORT:-20112}

export DB_USER=${DB_USER:-bodysense}
export DB_PASSWORD=${DB_PASSWORD:-bodysense123}
export DB_NAME=${DB_NAME:-bodysense}
export DB_SSLMODE=${DB_SSLMODE:-disable}
export REDIS_PASSWORD=${REDIS_PASSWORD:-bodysense123}
export JWT_SECRET_KEY=${JWT_SECRET_KEY:-bodysense-dev-only-secret}
export JWT_ACCESS_TTL_HOURS=${JWT_ACCESS_TTL_HOURS:-168}
export JWT_REFRESH_TTL_HOURS=${JWT_REFRESH_TTL_HOURS:-720}
export TRUSTED_PROXIES=${TRUSTED_PROXIES:-127.0.0.1,::1}
export LITELLM_MASTER_KEY=${LITELLM_MASTER_KEY:-sk-bodysense-dev-gateway}
export LITELLM_BASE_URL=${LITELLM_BASE_URL:-http://127.0.0.1:${LITELLM_PORT}/v1}

# ── DEV-ONLY Oracle2 gateway opt-in ────────────────────────
# BODYSENSE_LITELLM_DEV_ORACLE2=1 routes the local dev LiteLLM gateway to the
# Oracle2 upstream through docker/docker-compose.dev-oracle2.yml. Default 0
# keeps docker/litellm/config.yaml, so production/staging and ordinary dev are
# unaffected. The Virtual Key is only ever read from the environment/local env
# file; it is never written into the repository.
export BODYSENSE_LITELLM_DEV_ORACLE2=${BODYSENSE_LITELLM_DEV_ORACLE2:-0}
export ORACLE2_BODYSENSE_BASE_URL=${ORACLE2_BODYSENSE_BASE_URL:-https://oracle.taile92a8e.ts.net:10446/v1}
export ORACLE2_BODYSENSE_HOSTNAME=${ORACLE2_BODYSENSE_HOSTNAME:-oracle.taile92a8e.ts.net}
export BODYSENSE_DEV_COMPOSE_OVERLAY=

# ── Authorized Oracle2 endpoint (the single allowed upstream) ──────────────
# Only one destination is authorized for the dev gateway. The Oracle2 Virtual Key
# and real BodySense request content travel to whatever ORACLE2_BODYSENSE_BASE_URL
# names, so any other target is refused at env-load time — before the overlay is
# exported and before anything can be sent off-host.
#
# Exact string match is deliberate. It rejects a different scheme (http), host,
# port, path, userinfo (@), query (?) and fragment (#) without parsing a URL, so
# there is no normalization or parsing gap to bypass.
ORACLE2_AUTHORIZED_HOST='oracle.taile92a8e.ts.net'
ORACLE2_AUTHORIZED_PORT='10446'
ORACLE2_AUTHORIZED_BASE_URL="https://${ORACLE2_AUTHORIZED_HOST}:${ORACLE2_AUTHORIZED_PORT}/v1"

oracle2_is_authorized_hostname() {
  [[ "$1" == "$ORACLE2_AUTHORIZED_HOST" ]]
}

oracle2_is_authorized_base_url() {
  local url=$1
  # Tolerate exactly one trailing slash; nothing else is normalized.
  [[ "$url" == */ ]] && url=${url%/}
  [[ "$url" == "$ORACLE2_AUTHORIZED_BASE_URL" ]]
}

# Resolve the Oracle2 MagicDNS name to its Tailscale CGNAT address.
#
# The retired GCP host previously had Docker MagicDNS resolve to an
# unreachable public address. That historical error does not prove WSL behavior.
# On wsl-local the approved Tailnet address is resolved by the host, validated
# as 100.64.0.0/10, and pinned through extra_hosts for the gateway container.
# A different DNS result must fail closed rather than use a public route.
# Nothing here changes ports or the Docker bridge network boundary.
#
# Tailscale allocates from 100.64.0.0/10. Validation applies to the host resolver
# result AND to an explicit ORACLE2_BODYSENSE_TAILSCALE_IP override, so a typo or
# a stale manual value fails the same way a bad DNS answer does.
oracle2_is_tailscale_ipv4() {
  local ip=$1 a b c d
  [[ "$ip" =~ ^([0-9]{1,3})\.([0-9]{1,3})\.([0-9]{1,3})\.([0-9]{1,3})$ ]] || return 1
  a=${BASH_REMATCH[1]}
  b=${BASH_REMATCH[2]}
  c=${BASH_REMATCH[3]}
  d=${BASH_REMATCH[4]}
  (( a <= 255 && b <= 255 && c <= 255 && d <= 255 )) || return 1
  (( a == 100 && b >= 64 && b <= 127 ))
}

oracle2_tailscale_ip() {
  local host=${ORACLE2_BODYSENSE_HOSTNAME}
  local ip
  ip=$(getent ahostsv4 "$host" 2>/dev/null | awk '$2 == "STREAM" {print $1; exit}')
  if [[ -z "$ip" ]]; then
    ip=$(getent hosts "$host" 2>/dev/null | awk '{print $1; exit}')
  fi
  [[ -n "$ip" ]] || return 1
  oracle2_is_tailscale_ipv4 "$ip" || return 1
  printf '%s\n' "$ip"
}

case "${BODYSENSE_LITELLM_DEV_ORACLE2,,}" in
  1 | true | yes | on)
    # Authorization first: never resolve DNS, never export the overlay, and never
    # leave a usable key in the environment for an unauthorized destination.
    if ! oracle2_is_authorized_base_url "${ORACLE2_BODYSENSE_BASE_URL:-}"; then
      echo "ORACLE2_BODYSENSE_BASE_URL is not the authorized Oracle2 endpoint." >&2
      echo "  given:      ${ORACLE2_BODYSENSE_BASE_URL:-<unset>}" >&2
      echo "  authorized: ${ORACLE2_AUTHORIZED_BASE_URL}" >&2
      echo "https only; no other host, port, path, userinfo, query or fragment is allowed." >&2
      unset ORACLE2_BODYSENSE_API_KEY
      return 1 2>/dev/null || exit 1
    fi
    if ! oracle2_is_authorized_hostname "${ORACLE2_BODYSENSE_HOSTNAME:-}"; then
      echo "ORACLE2_BODYSENSE_HOSTNAME is not the authorized Oracle2 MagicDNS name." >&2
      echo "  given:      ${ORACLE2_BODYSENSE_HOSTNAME:-<unset>}" >&2
      echo "  authorized: ${ORACLE2_AUTHORIZED_HOST}" >&2
      unset ORACLE2_BODYSENSE_API_KEY
      return 1 2>/dev/null || exit 1
    fi
    # Export the authorized values verbatim so Compose and the LiteLLM config
    # cannot receive a tolerated variant (for example a trailing slash).
    export ORACLE2_BODYSENSE_BASE_URL=$ORACLE2_AUTHORIZED_BASE_URL
    export ORACLE2_BODYSENSE_HOSTNAME=$ORACLE2_AUTHORIZED_HOST

    if [[ -z "${ORACLE2_BODYSENSE_API_KEY:-}" ]]; then
      echo "BODYSENSE_LITELLM_DEV_ORACLE2 is enabled but ORACLE2_BODYSENSE_API_KEY is empty." >&2
      echo "Set it in $ENV_FILE (git-ignored) or the environment; never commit the key." >&2
      return 1 2>/dev/null || exit 1
    fi
    if [[ -z "${ORACLE2_BODYSENSE_TAILSCALE_IP:-}" ]]; then
      if ! ORACLE2_BODYSENSE_TAILSCALE_IP=$(oracle2_tailscale_ip); then
        echo "Could not resolve ${ORACLE2_BODYSENSE_HOSTNAME} to a Tailscale 100.64.0.0/10 address." >&2
        echo "Confirm 'tailscale status' is up, or set ORACLE2_BODYSENSE_TAILSCALE_IP explicitly." >&2
        return 1 2>/dev/null || exit 1
      fi
    elif ! oracle2_is_tailscale_ipv4 "${ORACLE2_BODYSENSE_TAILSCALE_IP}"; then
      # An explicit override is validated too: a typo or a stale public address
      # must not silently become the dev upstream pin.
      echo "ORACLE2_BODYSENSE_TAILSCALE_IP=${ORACLE2_BODYSENSE_TAILSCALE_IP} is not a Tailscale 100.64.0.0/10 IPv4 address." >&2
      return 1 2>/dev/null || exit 1
    fi
    export ORACLE2_BODYSENSE_TAILSCALE_IP
    export BODYSENSE_DEV_COMPOSE_OVERLAY=$ROOT/docker/docker-compose.dev-oracle2.yml
    ;;
esac
export LITELLM_API_KEY=${LITELLM_API_KEY:-$LITELLM_MASTER_KEY}
export DATABASE_URL=${DATABASE_URL:-postgresql://${DB_USER}:${DB_PASSWORD}@127.0.0.1:${DB_PORT}/${DB_NAME}}
export AI_SERVICE_URL=${AI_SERVICE_URL:-http://127.0.0.1:${AI_SERVICE_PORT}}
export HEALTH_DOCUMENT_SERVICE_URL=${HEALTH_DOCUMENT_SERVICE_URL:-http://127.0.0.1:${DOCUMENT_SERVICE_PORT}}
export EMBEDDING_PROVIDER=${EMBEDDING_PROVIDER:-hashing}
export CORS_ORIGINS=${CORS_ORIGINS:-http://localhost:${WEB_PORT},http://127.0.0.1:${WEB_PORT}}
export VITE_DEV_API_TARGET=${VITE_DEV_API_TARGET:-http://127.0.0.1:${API_PORT}}
export VITE_WS_URL=${VITE_WS_URL:-ws://127.0.0.1:${API_PORT}/ws}
# On remote dev connections, RTT can make Vite's native-ESM module waterfall
# expensive. Bundled Dev preserves HMR while collapsing that request graph.
# Set false in
# .env.dev.local when debugging Vite/plugin behavior with classic dev serving.
export BODYSENSE_VITE_BUNDLED_DEV=${BODYSENSE_VITE_BUNDLED_DEV:-true}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  if [[ $# -eq 0 ]]; then
    printf 'BodySense direct-dev env: web=%s api=%s ai=%s document=%s postgres=%s redis=%s litellm=%s oracle2_dev_opt_in=%s oracle2_tailscale_ip=%s\n' \
      "$WEB_PORT" "$API_PORT" "$AI_SERVICE_PORT" "$DOCUMENT_SERVICE_PORT" "$DB_PORT" "$REDIS_PORT" "$LITELLM_PORT" "$BODYSENSE_LITELLM_DEV_ORACLE2" "${ORACLE2_BODYSENSE_TAILSCALE_IP:-}"
    exit 0
  fi
  cd "$ROOT"
  exec "$@"
fi
