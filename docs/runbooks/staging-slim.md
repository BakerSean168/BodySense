# Slim staging on the production host

Staging runs on the same small Alibaba ECS host as production (2 vCPU, 1.6 GiB RAM + 2 GiB
swap) under `/opt/bodysense-staging`, as compose project `bodysense-staging`. It consumes the
same coherent `staging-latest` channel and runtime artifact as the canonical user-level
watcher (`scripts/staging-deploy-watch.sh`), with three host-specific differences:

| Piece | Repository source | Installed as |
| --- | --- | --- |
| Slim compose override | `deploy/staging-slim/docker-compose.staging.slim.yml` | `/opt/bodysense-staging/docker-compose.staging.slim.yml` |
| Memory-guarded watcher | `scripts/staging-slim-watch.sh` | `/opt/bodysense-staging/bin/bodysense-staging-slim-watch` |
| System units (root) | `deploy/systemd/bodysense-staging-slim-watch.{service,timer}` | `/etc/systemd/system/` |

## What the slim setup does

- Runs only postgres, redis, api, ai-service and web, with tight memory limits; the staging
  `litellm-gateway` and `document-service` are disabled.
- ai-service is attached to the production Docker network by container name only and uses
  the production LiteLLM gateway. The `ai-service` alias is never added there, so production
  traffic cannot resolve to staging.
- Memory guard (MemAvailable + SwapFree): no deploy below 600 MB of headroom; a running
  staging stack is stopped below 300 MB. Override with `MIN_HEADROOM_DEPLOY_MB` /
  `MIN_HEADROOM_RUN_MB`.
- Every container has `restart: "no"`. After a host reboot nothing starts staging until the
  timer's first run (`OnBootSec=3min`), which redeploys only if the guard allows it. The
  same path restarts staging after a guard stop or an OOM kill once headroom recovers.
- No self-update: upgrade the watcher by re-running the installer from a fresh checkout.

## Install or update

```bash
cd /opt/bodysense-staging/repo && git pull --ff-only
scripts/install-staging-slim-watch.sh --enable   # as root
```

The installer needs `/opt/bodysense-staging/.env.staging.local` (mode 600, never committed).
It also sets `restart=no` on existing staging containers and replaces the first
hand-installed `bodysense-staging-deploy-watch` system units.

## Operate

- Status: `systemctl list-timers bodysense-staging-slim-watch.timer`,
  `journalctl -u bodysense-staging-slim-watch -n 50`.
- Preflight: `/opt/bodysense-staging/bin/bodysense-staging-slim-watch --check-only`.
- Force a redeploy: `/opt/bodysense-staging/bin/bodysense-staging-slim-watch --force`.
- Pause staging: `systemctl stop bodysense-staging-slim-watch.timer`, then
  `docker compose -p bodysense-staging stop`.
- Access (web binds to 127.0.0.1 only):
  `ssh -L 20150:127.0.0.1:20150 root@<host>`, then open <http://localhost:20150>.

Never touch the production compose project (`docker`), its compose file or its env from
here. Check that `https://body.bakersean.top/api/health` stays 200 after any staging change.
