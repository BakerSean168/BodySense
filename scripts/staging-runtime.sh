#!/usr/bin/env bash
set -Eeuo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"
ENV_FILE=${BODYSENSE_STAGING_ENV_FILE:-$ROOT/.env.staging.local}
STATIC_ENV_FILE=${BODYSENSE_STAGING_STATIC_ENV_FILE:-$ROOT/.runtime/staging-static-assets.env}
export COMPOSE_PROJECT_NAME=${COMPOSE_PROJECT_NAME:-bodysense-staging}
export STAGING_BIND_HOST=${STAGING_BIND_HOST:-127.0.0.1}
export STAGING_WEB_PORT=${STAGING_WEB_PORT:-20150}
compose=(docker compose -f docker/docker-compose.staging.yml --env-file "$ENV_FILE")
if [[ -f "$STATIC_ENV_FILE" ]]; then
  compose+=(--env-file "$STATIC_ENV_FILE")
fi

bootstrap() {
  if [[ -e "$ENV_FILE" ]]; then
    echo "staging env already exists: $ENV_FILE"
    return
  fi
  umask 077
  cat >"$ENV_FILE" <<ENV
DB_USER=bodysense
DB_NAME=bodysense
DB_PASSWORD=$(openssl rand -hex 24)
REDIS_PASSWORD=$(openssl rand -hex 24)
JWT_SECRET_KEY=$(openssl rand -hex 32)
LITELLM_MASTER_KEY=sk-$(openssl rand -hex 24)
EMBEDDING_PROVIDER=hashing
# Real provider credentials for non-stub staging inference.
# GROQ_API_KEY is required because bodysense-structured uses Groq in staging.
GROQ_API_KEY=
MIMO_API_KEY=
OPENROUTER_API_KEY=
ENV
  echo "created gitignored staging env: $ENV_FILE"
}

require_env() {
  [[ -f "$ENV_FILE" ]] || { echo "missing $ENV_FILE; run: $0 bootstrap" >&2; exit 2; }
}

require_groq() {
  local value
  value=$(grep -E '^GROQ_API_KEY=' "$ENV_FILE" | tail -n 1 | cut -d= -f2- || true)
  [[ -n "$value" ]] || { echo "missing GROQ_API_KEY in $ENV_FILE; staging requires real structured inference" >&2; exit 2; }
}

require_static_publish_env() {
  local missing=()
  for name in STATIC_ASSET_CDN_BASE R2_ENDPOINT R2_ACCESS_KEY_ID R2_SECRET_ACCESS_KEY R2_BUCKET; do
    [[ -n "${!name:-}" ]] || missing+=("$name")
  done
  (( ${#missing[@]} == 0 )) || {
    echo "missing static publication environment: ${missing[*]}" >&2
    exit 2
  }
}

publish_static_assets() {
  require_static_publish_env
  [[ -z "$(git status --porcelain)" ]] || {
    echo 'refusing immutable CDN publication from a dirty worktree; commit/checkpoint first' >&2
    exit 2
  }
  local revision asset_root asset_base atlas_catalog atlas_root
  revision=$(git rev-parse HEAD)
  asset_root=${STATIC_ASSET_CDN_BASE%/}
  asset_base="${asset_root}/web/${revision}/"
  atlas_catalog="${asset_root}/anatomy/vanatome/1.4.0/releases/1.4.0/catalog.json"
  atlas_root=$(mktemp -d)
  trap "rm -rf '$atlas_root'" EXIT

  node scripts/anatomy/sync.mjs --version 1.4.0 --output "$atlas_root"
  node scripts/anatomy/verify.mjs --version 1.4.0 --root "$atlas_root"
  node scripts/static-assets/publish-atlas-r2.mjs \
    --version 1.4.0 \
    --root "$atlas_root" \
    --public-base "$asset_root"

  rm -rf apps/web/dist
  VITE_ASSET_BASE="$asset_base" \
  VITE_BODYSENSE_ANATOMY_CATALOG_URL="$atlas_catalog" \
    pnpm --dir apps/web exec vite build
  node scripts/static-assets/manifest.mjs \
    --dist apps/web/dist \
    --revision "$revision" \
    --base-url "$asset_base"
  node scripts/static-assets/publish-web-r2.mjs \
    --dist apps/web/dist \
    --public-base "$asset_root"

  mkdir -p "$(dirname "$STATIC_ENV_FILE")"
  cat >"$STATIC_ENV_FILE" <<ENV
VITE_ASSET_BASE=$asset_base
VITE_BODYSENSE_ANATOMY_CATALOG_URL=$atlas_catalog
ENV
  rm -rf "$atlas_root"
  trap - EXIT
  echo "published staging static assets for $revision"
  echo "wrote public build configuration: $STATIC_ENV_FILE"
  echo "run '$0 restart' to rebuild staging Web with the CDN base"
}

disable_static_assets() {
  rm -f "$STATIC_ENV_FILE"
  echo "removed staging static CDN build configuration: $STATIC_ENV_FILE"
  echo "run '$0 restart' to return to same-origin Vite assets"
}

verify_static_assets() {
  [[ -f "$STATIC_ENV_FILE" ]] || { echo "static CDN is not configured for staging" >&2; exit 2; }
  local asset_base web_container failures=0
  asset_base=$(grep -E '^VITE_ASSET_BASE=' "$STATIC_ENV_FILE" | tail -n 1 | cut -d= -f2-)
  [[ -n "$asset_base" ]] || { echo "VITE_ASSET_BASE missing from $STATIC_ENV_FILE" >&2; exit 2; }
  curl -fsS "http://127.0.0.1:${STAGING_WEB_PORT}/" | grep -Fq "$asset_base" || {
    echo "running staging index.html does not reference $asset_base" >&2
    exit 1
  }
  web_container=$("${compose[@]}" ps -q web)
  [[ -n "$web_container" ]] || { echo 'staging web container is not running' >&2; exit 2; }
  while IFS= read -r relative; do
    [[ -n "$relative" ]] || continue
    if ! curl --fail --silent --show-error --head --location "${asset_base}${relative}" >/dev/null; then
      echo "missing CDN asset: ${asset_base}${relative}" >&2
      failures=$((failures + 1))
    fi
  done < <(docker exec "$web_container" sh -c 'cd /usr/share/nginx/html && find assets -type f -print')
  (( failures == 0 )) || exit 1
  echo "staging Web image and CDN prefix are coherent: $asset_base"
}

case "${1:-status}" in
  bootstrap) bootstrap ;;
  up) require_env; require_groq; "${compose[@]}" up -d --build ;;
  down) require_env; "${compose[@]}" down ;;
  restart) require_env; require_groq; "${compose[@]}" up -d --build --force-recreate ;;
  status)
    require_env
    "${compose[@]}" ps
    curl -fsS "http://127.0.0.1:${STAGING_WEB_PORT}/api/health" || true
    echo
    ;;
  logs) require_env; "${compose[@]}" logs --tail "${2:-120}" ;;
  publish-static) require_env; publish_static_assets ;;
  verify-static) require_env; verify_static_assets ;;
  disable-static) disable_static_assets ;;
  *) echo "usage: $0 {bootstrap|up|down|restart|status|logs [lines]|publish-static|verify-static|disable-static}" >&2; exit 2 ;;
esac
