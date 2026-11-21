#!/usr/bin/env bash
set -euo pipefail

# Run inside the selected Dev Container after its automatic postCreate command.
profile="${1:-lite}"
case "$profile" in
  lite) config=.devcontainer/devcontainer.json ;;
  full) config=.devcontainer/full/devcontainer.json ;;
  *) echo 'Usage: bash .devcontainer/verify.sh [lite|full]' >&2; exit 2 ;;
esac
cd "$(git rev-parse --show-toplevel)"
test "$(id -un)" = vscode
test "$(id -u)" -ne 0
expected_node=$(node -p 'JSON.parse(require("node:fs").readFileSync(process.argv[1], "utf8")).features["ghcr.io/devcontainers/features/node:2"].version' "$config")
expected_pnpm=$(node -p 'require("./package.json").packageManager.split("@").pop()')
test "$(node --version)" = "v$expected_node"
test "$(pnpm --version)" = "$expected_pnpm"
node --version
pnpm --version
gh --version

node <<'NODE'
const net = require('node:net');
const socket = net.connect({ host: '127.0.0.1', port: 2222 });
let banner = '';
let verified = false;
socket.setTimeout(5000);
socket.on('data', (data) => {
  banner += data.toString();
  if (banner.length > 1024) { socket.destroy(); return; }
  if (!banner.includes('\n')) return;
  verified = banner.split('\n')[0].startsWith('SSH-2.0-');
  if (verified) console.log('SSHD listener verified on 2222');
  socket.destroy();
});
socket.once('timeout', () => socket.destroy());
socket.once('error', (error) => { console.error(error.message); process.exitCode = 1; });
socket.once('close', () => { if (!verified) process.exitCode = 1; });
NODE

if [[ "$profile" == full ]]; then
  docker compose version
  ready=0
  for attempt in $(seq 1 30); do
    if docker info --format '{{.ServerVersion}} {{.DockerRootDir}}'; then ready=1; break; fi
    sleep 1
  done
  test "$ready" = 1
  docker build --network=none -t devcontainer-acceptance:local - <<'DOCKERFILE'
FROM scratch
LABEL io.baker.devcontainer-acceptance="smoke"
DOCKERFILE
  go version
  go version | grep -Eq 'go1\.26(\.|[[:space:]])'
  python3.13 -c 'import sys; assert sys.version_info[:2] == (3,13); print(sys.version)'
  uv --version
  uvx --version
fi
