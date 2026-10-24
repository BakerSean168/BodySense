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
| `docker/docker-compose.dev-oracle2.yml` | Opt-in overlay: config mount, key env, `extra_hosts` |
| `scripts/dev-env.sh` | Opt-in switch, key presence check, Tailscale IP resolution/validation |
| `scripts/dev-infra.sh`, `scripts/dev-runtime.sh` | Append the overlay only when the opt-in is on |
| `scripts/dev-oracle2-gateway-eval.py` | Per-channel fixed-model measurement |
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

Active serving edges: WindHub primary, Nanliang fallback. Kunyou is declared as
`bodysense-dev-oracle2-kunyou` for measurement only and is deliberately **not** in
`router_settings.fallbacks`. Do not add it until it answers HTTP 200.

`bodysense-posture` keeps its pre-existing OpenRouter vision configuration; Oracle2 has no
qualified vision channel.

## Enable

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
any URL-parsing gap.

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
inside `100.64.0.0/10`).

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

## Known limitation: auth errors trigger fallback

Reproduce with the **pinned LiteLLM v1.102.1 image** and a local synthetic HTTP
upstream. This requires Docker on WSL but uses `--network none`, no real provider
keys, no external model calls, and no production or shared Compose containers:

```bash
docker run --rm --network none --entrypoint python \
  -e LITELLM_LOCAL_MODEL_COST_MAP=True \
  -v "$PWD/scripts/dev-oracle2-fallback-negative-smoke.py:/tmp/fallback-smoke.py:ro" \
  docker.litellm.ai/berriai/litellm:v1.102.1 /tmp/fallback-smoke.py
```

The script exercises the LiteLLM Router SDK path with the same one-edge fallback
semantics as this dev config. It expects 400/401/403 to return their original
errors without invoking the backup; 408/429/500/503 may use a healthy backup.
The smoke test **deliberately exits 1 while the vulnerability exists**. It is
not a proxy authentication or end-to-end business acceptance test, and must not
be mistaken for one.

Observed on WSL 2026-10-24 (job `wsl-local::job_req_44829f0baf7bf9028f48e8e1a09ac16c`):
200 returned the primary response; 400, 401 and 403 each invoked the fallback
and returned a successful synthetic response (**FAIL**); 408, 429, 500 and 503
correctly invoked the fallback (**PASS**). Overall
`FALLBACK_NEGATIVE_SMOKE=FAIL`. This is independent evidence that the dev gateway
cannot yet be promoted. Do not disable or weaken the negative check to make CI
green.

Measured on LiteLLM v1.102.1 with a synthetic upstream: `num_retries: 0` disables
**retries only**. Ordinary `router_settings.fallbacks` also fires on **400, 401 and 403**, so
an authentication or business-validation failure on WindHub is answered by the Nanliang
fallback instead of surfacing as an error.

This is an accepted **dev-only** limitation and a **known blocker**, not a fail-closed auth
story. Do not describe this topology as fail-closed: an expired or unauthorized Oracle2 key
on WindHub is currently masked by a Nanliang fallback answer. The eval script sidesteps the
masking by probing fixed groups that have no fallback edge, so a capability result always
comes from the channel it names. Do not promote this topology to staging or production
without a fail-closed authentication design.

This was **not** addressed by configuration. LiteLLM v1.102.1 introspection shows
`Router.__init__` has no `fallback_on_unsupported_params` parameter, and no setting was
invented to emulate one. The measured behaviour above stands; do not read this runbook as
claiming auth errors are contained.

## Related

- [`docs/architecture/model-gateway-routing.md`](../architecture/model-gateway-routing.md)
- [`docs/runbooks/diagnosis-production-provider-qualification.md`](./diagnosis-production-provider-qualification.md)
