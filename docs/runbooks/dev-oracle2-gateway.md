# Dev-Only Oracle2 Gateway (WSL)

Status: dev-only opt-in, **not qualified for staging or production**.

This runbook covers routing the `wsl-local` dev LiteLLM gateway (`127.0.0.1:20112`) to the
Oracle2 gateway at `https://oracle.taile92a8e.ts.net:10446/v1` for compatible non-vision
logical groups. Production and staging are unaffected: they keep
`docker/litellm/config.yaml` and `docker/litellm/config.staging.yaml`.

## Files

| File | Role |
| --- | --- |
| `docker/litellm/config.dev-oracle2.yaml` | Isolated dev gateway topology (Oracle2 upstream) |
| `docker/litellm/Dockerfile.dev-oracle2-gateway`, `patch-dev-oracle2-fallback.py` | WSL dev-only pinned Router fail-closed image |
| `docker/docker-compose.dev-oracle2.yml` | Opt-in overlay: config mount, key env, `extra_hosts` |
| `scripts/dev-env.sh` | Opt-in switch, key presence check, Tailscale IP resolution/validation |
| `scripts/dev-infra.sh`, `scripts/dev-runtime.sh` | Append the overlay only when the opt-in is on |
| `scripts/dev-oracle2-gateway-eval.py` | Per-channel fixed-model measurement (deferred) |
| `scripts/dev-oracle2-fallback-negative-smoke.py` | Local synthetic Router SDK fail-closed regression |
| `scripts/dev-oracle2-proxy-fallback-smoke.py` | Local synthetic proxy HTTP regression (no provider network) |
| `apps/ai-service/tests/unit/test_dev_oracle2_gateway_opt_in.py` | Structural/schema guard (no network) |

Nothing here changes the Docker network boundary: the gateway keeps the project bridge
network, its existing `127.0.0.1:20112:4000` publish, and its in-container bind.

## Provider order

Intended order for compatible non-vision groups: **WindHub → Kunyou → Nanliang**.

| Channel | Physical model | Measured status (2026-10-22) |
| --- | --- | --- |
| WindHub | `windhub-free/glm-5.3` | chat/JSON/tool HTTP 200; chat ~42 s, JSON ~28 s, tool ~27 s |
| Kunyou | `gpt-6-luna` | **PENDING** — HTTP 503 "no channel" |
| Nanliang | `deepseek-v4.1-flash-1017` | chat/JSON/tool/SSE HTTP 200 |

Active configured edges: WindHub primary, a DeepSeek fallback previously described
as Nanliang. Kunyou is declared as `bodysense-dev-oracle2-kunyou` for measurement
only and is deliberately **not** in `router_settings.fallbacks`. Do not add it
until it answers HTTP 200.

**New upstream identity blocker (Oracle2 read-only inventory, 2026-10-24):**
`deepseek-v4.1-flash-1017` is now a **shared, unqualified** LiteLLM model group
with at least two FREE supplier bindings: `llm-pm-deepseek-0731` and
`kunyou-free`. The WSL fallback currently names this unqualified group, so
its traffic is **not guaranteed to stay on the previously labeled Nanliang
channel** and may instead select Kunyou. A separate supplier-qualified route
`llm-pm-deepseek-0731/deepseek-v4.1-flash-1017` exists but has not yet been
independently authorized as a substitute for Nanliang. The FREE designation
alone is not a provider-identity or runtime-authentication acceptance.
**Keep the dev-only Oracle2 opt-in disabled until the intended fallback
supplier is reconciled and a dedicated scoped Virtual Key is available.**
No shared Oracle2 registry or runtime routing was changed during this audit.

`bodysense-posture` keeps its pre-existing OpenRouter vision configuration; Oracle2 has no
qualified vision channel.

## Enable

**Blocked pending supplier identity and scoped key:** the commands below are
retained for future acceptance, not an instruction to activate this configuration
while the unqualified fallback described above remains unresolved.

```bash
cd /home/baker/projects/_bodysense_oracle2_gateway  # existing feature worktree
printf '%s\n' 'BODYSENSE_LITELLM_DEV_ORACLE2=1' \
  'ORACLE2_BODYSENSE_API_KEY=<dev-only Virtual Key>' >> .env.dev.local   # git-ignored
chmod 600 .env.dev.local
scripts/dev-infra.sh up          # or: scripts/dev-runtime.sh up
```

`ORACLE2_BODYSENSE_API_KEY` has no default. With the opt-in enabled and the variable empty,
`scripts/dev-env.sh` exits non-zero and Compose also fails closed. Never commit the key; it
is never written into any repository file.

## Authorized upstream only

The dev gateway sends the Oracle2 Virtual Key **and real BodySense request content** to
whatever `ORACLE2_BODYSENSE_BASE_URL` names. Only one destination is authorized:

```text
https://oracle.taile92a8e.ts.net:10446/v1
```

When `BODYSENSE_LITELLM_DEV_ORACLE2=1`, `scripts/dev-env.sh` refuses to proceed unless the
base URL matches that endpoint exactly (one trailing slash tolerated, then normalized) and
`ORACLE2_BODYSENSE_HOSTNAME` is exactly `oracle.taile92a8e.ts.net`. The check is an exact
string comparison, so it rejects a different scheme (`http`), host, port, path, userinfo
(`@`), query (`?`), fragment (`#`), lookalike suffix, whitespace, and case variant without
any URL-parsing gap. Rejected user-supplied URLs and hostnames are not echoed to
stderr because they may contain userinfo or other sensitive text.

The check runs **before** the overlay is exported and before DNS resolution, so an
unauthorized target cannot mount the Oracle2 config, cannot resolve a host, and cannot carry
the key off-host. The authorized values are re-exported verbatim so Compose never receives a
tolerated variant.

With the opt-in off (the default) these variables are inert: no overlay is exported and
`docker/litellm/config.yaml` stays in use.

Disable with `BODYSENSE_LITELLM_DEV_ORACLE2=0` (the default), then use
`scripts/dev-infra.sh restart-gateway` after verifying the intended Compose project.
Avoid `scripts/dev-infra.sh restart`: it performs a full Compose down/up including DB/Redis.
The main and feature worktrees default to the same `COMPOSE_PROJECT_NAME`; do not run
either against existing containers without verifying ownership and bind mounts.

## Why `extra_hosts`

The retired GCP host previously observed Docker resolving Oracle2 MagicDNS to public
addresses (`103.84.155.153` / `.217`) that its container could not route. This historical
observation is **not** WSL acceptance evidence. On `wsl-local`, pin the authorized Oracle2
Tailnet address using Compose `extra_hosts` to retain bridge networking and TLS hostname
verification. WSL host-level HTTPS health was checked on 2026-10-24; it does not establish
model authentication or inference.

`scripts/dev-env.sh` resolves the name on the host at opt-in time and exports
`ORACLE2_BODYSENSE_TAILSCALE_IP`. Only `100.64.0.0/10` is accepted, for both the resolved
value and an explicit override; anything else aborts the opt-in.

Historical 2026-10-22 GCP evidence: a container on `bodysense-dev-infra_bodysense-network`
reached Oracle2 health with HTTP 200 and valid TLS. GCP was retired on 2026-10-23.
Independently, a WSL temporary Docker bridge reached Oracle2 with valid TLS on
2026-10-24. Neither result proves that the current gateway service is running or
authenticated, and they must not be combined as a single end-to-end test.

**What that proves — and what it does not.** The HTTPS 200 on `/health/liveliness` proves
only two things: the container can reach the endpoint (Tailscale routing works) and TLS
verification succeeds. It is **not** evidence of any of the following:

- that `ORACLE2_BODYSENSE_API_KEY` is present, valid, or authorized;
- that real inference works for any model;
- that `router_settings.fallbacks` behaves as configured;
- any business or capability acceptance.

Reachability is a prerequisite for the opt-in, not its acceptance condition. Real
acceptance requires the per-channel evaluation below to run with a real key.

**Residual risk:** if Oracle2's tailnet address changes, a pinned mapping goes stale. The
address is re-resolved per `scripts/dev-env.sh` invocation, so recreate the gateway after
an address change:

```bash
scripts/dev-infra.sh restart-gateway  # only litellm-gateway, not PostgreSQL/Redis
```

Override manually when needed: `ORACLE2_BODYSENSE_TAILSCALE_IP=100.x.y.z` (must still be
inside `100.64.0.0/10`). Invalid override values are rejected without echoing
untrusted input into stderr, even if it resembles a secret.

## Verify

```bash
scripts/dev-infra.sh status                       # shows oracle2_dev_opt_in=1
docker compose -f docker/docker-compose.yml -f docker/docker-compose.dev-infra.yml \
  -f docker/docker-compose.dev-oracle2.yml --profile dev config | grep -A3 extra_hosts

# Container -> Oracle2 reachability (proves connectivity + TLS only, not the key)
docker compose -f docker/docker-compose.yml -f docker/docker-compose.dev-infra.yml \
  -f docker/docker-compose.dev-oracle2.yml --profile dev exec -T litellm-gateway \
  python -c "import urllib.request;print(urllib.request.urlopen('https://oracle.taile92a8e.ts.net:10446/health/liveliness',timeout=20).status)"
```

## Per-channel evaluation

**Deferred:** do not execute live channel/model evaluation in this integration pass. This
helper remains documented for a separately authorized qualification run. Before any
future use, it now accepts only a literal loopback gateway URL
(`http://127.0.0.1:<port>/v1` or `http://[::1]:<port>/v1`), disables HTTP
proxies and refuses redirects so a gateway master key cannot be forwarded to an
untrusted endpoint. These protections were tested with synthetic local HTTP
servers and no upstream model calls.

```bash
source scripts/dev-env.sh
python3 scripts/dev-oracle2-gateway-eval.py --json /tmp/oracle2-eval.json
```

Each probe targets a fixed `bodysense-dev-oracle2-*` group that has **no** fallback edge, so
a result can never be produced by a different provider. Prompts are synthetic; no personal
health data is sent and no real key is needed by the script (the local gateway master key
authenticates it).

Each capability is judged on an **exact contract**, using the full parsed payload rather
than a truncated preview:

| Capability | Passes only when |
| --- | --- |
| `chat` | the answer is exactly `PONG` (surrounding whitespace allowed) and a `finish_reason` is present |
| `json` | the body parses to exactly `{"probe": "synthetic", "ok": true}`, with `ok` a real boolean |
| `tool` | one call to `record_synthetic_probe` with parsed arguments exactly `{"label": "dev-probe"}` |
| `stream` | the reassembled content lines are exactly `1`, `2`, `3` in order, then `[DONE]` |

So `NOT PONG`, a wrong JSON payload, a wrong tool label, or unrelated SSE text all FAIL even
though they are non-empty HTTP 200 responses. Reported per channel: HTTP status, latency,
usage, JSON validity, tool-call count, stream chunk/done/usage behaviour.

Exit codes:

- `0` — every expected-working capability met its exact contract and no pending channel
  started working (`ORACLE2_GATEWAY_EVAL=PASS`);
- `1` — an expected-working capability failed (`ORACLE2_GATEWAY_EVAL=FAIL`), **or** a
  pending channel unexpectedly passed (`ORACLE2_GATEWAY_EVAL=REQUALIFY`), which demands a
  deliberate re-qualification before anything is wired into a live path.

## Dev-only fail-closed fallback (WSL)

LiteLLM v1.102.1 **upstream** has a reproduced regression: ordinary Router
`fallbacks` send 400/401/403 to another model group. The baseline synthetic
smoke on WSL (2026-10-24, execution `job_req_44829f0baf7bf9028f48e8e1a09ac16c`)
recorded `FALLBACK_NEGATIVE_SMOKE=FAIL`. `num_retries: 0` does not prevent
this behavior.

The **opt-in WSL dev overlay only** builds
`bodysense/litellm-dev-oracle2:v1.102.1-safe-fallback` from the immutable
v1.102.1 upstream digest. The small build-time source patch requires an
exact SHA256 match on `router.py` and aborts the image build on an upstream
change. The patched Router permits cross-group fallback only for HTTP
408/429/500/502/503/504 and known transport failures. Every other status,
including HTTP 400/401/402/403/404/409/422, fails closed with no fallback.
No fallback ordering, logical model names or authentication environment
variables were changed.

Build **without accessing providers** and verify with synthetic local HTTP
responses; `--network none` does not disable container loopback:

```bash
docker build --network none -t bodysense/litellm-dev-oracle2:v1.102.1-safe-fallback \
  -f docker/litellm/Dockerfile.dev-oracle2-gateway docker/litellm
docker run --rm --network none --entrypoint python \
  -e LITELLM_LOCAL_MODEL_COST_MAP=True \
  -v "$PWD/scripts/dev-oracle2-fallback-negative-smoke.py:/tmp/fallback-smoke.py:ro" \
  bodysense/litellm-dev-oracle2:v1.102.1-safe-fallback /tmp/fallback-smoke.py

# Independently exercise the HTTP proxy, not only the Router SDK:
docker run --rm --network none --entrypoint python \
  -e LITELLM_LOCAL_MODEL_COST_MAP=True \
  -v "$PWD/scripts:/tmp/bodysense-scripts:ro" \
  bodysense/litellm-dev-oracle2:v1.102.1-safe-fallback \
  /tmp/bodysense-scripts/dev-oracle2-proxy-fallback-smoke.py
```

WSL evidence (2026-10-24): Router SDK 14/14 synthetic status cases passed,
`FALLBACK_NEGATIVE_SMOKE=PASS` (job `job_req_df3e24297e208aae23830cca6f5b6231`).
The local HTTP proxy initially passed 8/8 cases, `PROXY_FALLBACK_NEGATIVE_SMOKE=PASS`
(job `job_req_c1d159685bd4328f97026f79d3aa5f79`). Expanded WSL HTTP proxy
regression passed **15/15 synthetic checks** on 2026-10-24 (job
`job_req_40367ac0b084789cc20b3ad9fd8cce33`): all 14 SDK status cases plus
an invalid local gateway key rejected without any upstream attempt. LiteLLM
v1.102.1 returned HTTP 400 (not 401) for that synthetic invalid key; this
client-facing status detail remains a separate compatibility consideration.
The proxy smoke disables
cooldowns **only in its own synthetic config** so one test does not suppress a
subsequent upstream call. The real gateway settings remain unchanged. The
unpatched upstream v1.102.1 image should still **FAIL** the Router smoke.

Run `scripts/dev-infra.sh restart-gateway` (it rebuilds **only** the gateway
when the opt-in is active) after checking Compose project/container ownership.
Do not use the unpatched upstream image with `config.dev-oracle2.yaml`.
This dev-only patch does not establish real API authentication, Oracle2
end-to-end model inference, independent security review or staging/production
acceptance.

## Related

- [`docs/architecture/model-gateway-routing.md`](../architecture/model-gateway-routing.md)
- [`docs/runbooks/diagnosis-production-provider-qualification.md`](./diagnosis-production-provider-qualification.md)
