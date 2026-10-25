# Dev-Only Oracle2 Gateway (WSL)

Status: dev-only opt-in, **blocked for activation; not qualified for staging or production**.

**WSL independent checkpoint (2026-10-25):** `wsl-local` recovered and the
existing clean feature worktree at `/home/baker/projects/_bodysense_oracle2_gateway`
was verified at `df6e10142f2ea0e377a3f3ee23833e8f8c14c29f`.
There was no BodySense T3 thread or running BodySense Writer, and the normal
`/home/baker/projects/bodysense` worktree remained on `main`.
The feature worktree's `.env.dev.local`, `.env.local`, `.env`, and `.env.dev`
were absent; no dedicated Virtual Key or OpenRouter/Groq API credential was
available through those files. No keys were read or created.
The preceding PR #432 commit `7a6ac6817d7523603b4e5de300aace304732db98`
completed CI successfully with **12/12 jobs** (run `40157739444`); this is
static/CI evidence, not model-inference acceptance. Newer commits require
independent exact-SHA CI verification.
The WSL host independently reached Oracle2 `/health/liveliness` with HTTPS 200,
verified TLS (`ssl_verify_result=0`), and Tailnet IP `100.68.237.116`;
anonymous `/v1/models` returned 401 with verified TLS. This is **network and
authentication-boundary evidence only**, not inference acceptance.
`python3 scripts/dev-oracle2-route-preflight.py docker/litellm/config.dev-oracle2.yaml`
correctly returned exit 1 for the unqualified DeepSeek routes. The isolated
structural test module passed **134/134**, with one pytest configuration warning
(`asyncio_mode` unknown in the installed test environment). No opt-in container
was started; no real model calls were made. GitHub PR #432 remains Draft.
A fresh 2026-10-25 GitHub check independently confirmed that PR #432 HEAD
`ff8e31fc34288f6a6002942b8bc250f3340e8dd6` passed all **12/12 CI jobs**
(run `40165182364`); the WSL feature worktree matched this exact SHA and
was clean. This is still code/CI evidence, not live upstream authorization.

**Pending independent acceptance:** reconcile the supplier identity without
substituting another unapproved channel; obtain an authorized, restricted
BodySense Virtual Key; verify OpenRouter/Groq **free-only** authentication and
synthetic business requests from WSL; and separately verify Posture Vision only
if its exact route has been confirmed authorized at no unapproved cost.
The existing Posture OpenRouter model IDs are **not evidence of free pricing**.
A WSL read-only query to OpenRouter's public `/api/v1/models` catalog on
2026-10-25 (467 listed models) found that the exact configured primary
`qwen/qwen2.5-vl-72b-instruct` has **nonzero prompt and completion prices**
(`0.0000008` and `0.000001` in the catalog's respective price fields).
The configured fallback `google/gemini-2.0-flash-001` had **no exact match**
in that public catalog. These findings do not establish account-specific
availability or billing, but neither route meets the present free-only
acceptance requirement. **Do not make Posture Vision requests** without
separate explicit pricing and credential authorization. This was a public
metadata lookup only: no key was used, and no inference was sent.
Prior GCP results are historical and never count as WSL execution evidence.
Model evaluation/ranking and active-model optimization are deferred.

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
| `scripts/dev-oracle2-route-preflight.py` | Fail-closed complete dev topology check before Compose mutation |
| `scripts/dev-oracle2-fallback-negative-smoke.py` | Local synthetic Router SDK fail-closed regression |
| `scripts/dev-oracle2-proxy-fallback-smoke.py` | Local synthetic proxy HTTP regression (no provider network) |
| `apps/ai-service/tests/unit/test_dev_oracle2_gateway_opt_in.py` | Structural/schema guard (no network) |

Nothing here changes the Docker network boundary: the gateway keeps the project bridge
network, its existing `127.0.0.1:20112:4000` publish, and its in-container bind.

## Provider order

Intended order for compatible non-vision groups: **WindHub → Kunyou → Nanliang**.

| Historically named channel | Physical model | Historical GCP observations (2026-10-22; **not WSL acceptance**) |
| --- | --- | --- |
| WindHub | `windhub-free/glm-5.3` | chat/JSON/tool HTTP 200; chat ~42 s, JSON ~28 s, tool ~27 s |
| Kunyou | `gpt-6-luna` | **PENDING** — HTTP 503 "no channel" |
| Nanliang (original identity not independently established) | `deepseek-v4.1-flash-1017` | chat/JSON/tool/SSE HTTP 200 |

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
The qualified `llm-pm-deepseek-0731` binding advertises the upstream
`openai-responses` protocol (credential metadata is chat-compatible); Oracle2's
ProviderAdmin explicitly supports Chat Completions to Responses bridging.
A 2026-10-24 read-only `--inspect-resource` reconfirmed that the qualified
`llm-pm-deepseek-0731/deepseek-v4.1-flash-1017` binding is unblocked and
marked FREE, while `kunyou-free` also has an unblocked binding to the shared
unqualified DeepSeek group. Neither fact identifies `llm-pm-deepseek-0731`
as the originally intended Nanliang account. Exact resource IDs
`openrouter-free` and `groq-free` returned zero bindings; this is **not**
an exhaustive search of differently named resources, and is not an upstream
OpenRouter/Groq authorization result.
These facts resolve a superficial protocol-label mismatch, **not** supplier
identity, client authorization, or actual end-to-end wire compatibility.

**Supplier-label provenance found (Oracle2 read-only, 2026-10-25):** The clean,
versioned `personal-infrastructure` tree at `e7db5c2` explicitly maps the
resource prefix `llm-pm-deepseek-0731` to T3 provider `llm-pm` in
`etc/server/oracle2/t3code/project_models.py`, and displays `llm-pm` as
**南梁 API** in both that file and `etc/server/oracle2/t3code/pi-t3`.
`project_dsh_gateway.py` also binds that provider alias to the exact qualified
route `llm-pm-deepseek-0731/deepseek-v4.1-flash-1017`. `git blame` dates the
current display-name mapping to commit `68c75324` (2026-10-12), predating
this integration PR. The current ProviderAdmin read-only `--inspect` returns
a FREE `llm-pm-deepseek-0731` credential and an unblocked qualified
`deepseek-v4.1-flash-1017` binding. **This narrows the naming ambiguity:** the
existing infrastructure intentionally labels this *resource alias* as 南梁 API.
However, a T3 display label is not independent evidence of the underlying
account/endpoint identity or of a BodySense-scoped client authorization. Do
not set `APPROVED_DEEPSEEK_RESOURCE_ID`, replace the checked-in DeepSeek route,
or enable the opt-in on this evidence alone; upstream ownership, dedicated
client policy and exact-wire acceptance still require authorized verification.
No key material was inspected, and no provider or production policy was changed.

**WSL fail-closed start guard (2026-10-24):** `scripts/dev-infra.sh up`,
`restart`, and `restart-gateway` now run
`python3 scripts/dev-oracle2-route-preflight.py docker/litellm/config.dev-oracle2.yaml`
before any Compose changes. `scripts/dev-runtime.sh restart` also checks
before stopping services. Until both serving DeepSeek fallback groups and the
fixed-channel probe use supplier-qualified Oracle2 routes, this returns nonzero.
The guard also rejects duplicate LiteLLM deployments under the same
`model_name`, mismatched supplier pins across serving fallback and probe,
non-DeepSeek model substitutions, and unexpected credentials/base URLs.
It also enforces the declared five-edge fallback graph with retries disabled,
rejecting extra/duplicate edges, unexpected targets (including pending Kunyou),
and alternate fallback maps. The guard validates **all** eleven declared
deployments against the existing static model/provider contracts, requiring the
original WindHub primary and probe routes, pending Kunyou probe, and unchanged
OpenRouter Posture routes. It rejects new groups, unreviewed model parameters,
credential/base-URL substitutions, unexpected root/router settings, and invalid
timeouts; an apparently safe DeepSeek pin cannot hide a different primary,
extra model, or diverted vision request. A supplier pin alone cannot authorize
a new edge. These checks prevent a second deployment or fallback path from
hiding behind a matching entry.
The guard additionally requires a **reviewed supplier-identity pin** in
`scripts/dev-oracle2-route-preflight.py` (`APPROVED_DEEPSEEK_RESOURCE_ID`).
It is intentionally unset until the original Nanliang resource and dedicated
client scope are independently verified. Merely replacing all three DeepSeek
model strings with any syntactically supplier-qualified route will still block
activation; no environment variable can override the approval. A future
reviewed source change must explicitly approve the verified resource.

This remains a **local configuration/approval check**, not proof that a Virtual
Key permits the route or that its wire protocol/inference works. Ordinary dev
and staging/production are unchanged.

WSL 2026-10-24 verification: the dedicated structural/security test module
passed **134 tests**, including 13 new negative cases for non-fallback route
substitutions. Ruff lint and format, Python syntax, and Git diff checks passed.
The checked-in config still fails the preflight intentionally because the
DeepSeek route remains supplier-ambiguous; no Oracle2 opt-in was started.
This verification is synthetic and does not cover real provider authentication.

`bodysense-posture` keeps its pre-existing OpenRouter vision configuration; Oracle2 has no
qualified vision channel. Its pre-existing model IDs are not established as
`:free` routes; do not send vision requests under a free-only authorization
until pricing, exact model route, and credentials are independently verified.

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

Reachability is a prerequisite for the opt-in, not its acceptance condition.
Once the reviewed supplier and scoped free-only credentials are available,
real acceptance requires an independent, minimal **synthetic business-flow
smoke** from WSL through the local dev gateway to the authorized Oracle2
route. The separate per-channel evaluation below is deferred and is **not**
a prerequisite for this integration acceptance.

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
