#!/usr/bin/env bash
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

python3 - <<'PY'
from pathlib import Path

caddy = Path('docker/Caddyfile').read_text()
watcher = Path('scripts/production-deploy-watch.sh').read_text()
origin = 'https://assets.bakersean.top'

csp_lines = [line.strip() for line in caddy.splitlines() if 'Content-Security-Policy ' in line]
if len(csp_lines) != 1:
    raise SystemExit(f'PRODUCTION_PROXY_CONTRACT=FAIL expected_one_csp_line got={len(csp_lines)}')
line = csp_lines[0]
required = {
    'default-src': "'self'",
    'script-src': origin,
    'style-src': origin,
    'font-src': origin,
    'img-src': origin,
    'media-src': origin,
    'connect-src': origin,
    'worker-src': origin,
}
parts = line.split('"', 2)
if len(parts) < 2:
    raise SystemExit('PRODUCTION_PROXY_CONTRACT=FAIL malformed_csp_header')
directives = {}
for item in parts[1].split(';'):
    tokens = item.strip().split()
    if tokens:
        directives[tokens[0]] = tokens[1:]
for directive, token in required.items():
    if token not in directives.get(directive, []):
        raise SystemExit(f'PRODUCTION_PROXY_CONTRACT=FAIL {directive}_missing={token}')
for directive in ('script-src', 'style-src', 'font-src', 'img-src', 'media-src', 'connect-src', 'worker-src'):
    if '*' in directives.get(directive, []):
        raise SystemExit(f'PRODUCTION_PROXY_CONTRACT=FAIL wildcard_not_allowed directive={directive}')

needle = 'compose up -d --no-deps --force-recreate caddy'
if watcher.count(needle) != 2:
    raise SystemExit(f'PRODUCTION_PROXY_CONTRACT=FAIL caddy_force_recreate_count={watcher.count(needle)}')
if 'compose up -d --no-deps caddy\n' in watcher:
    raise SystemExit('PRODUCTION_PROXY_CONTRACT=FAIL stale_non_recreating_caddy_deploy')

print('PRODUCTION_PROXY_CONTRACT=PASS')
PY
