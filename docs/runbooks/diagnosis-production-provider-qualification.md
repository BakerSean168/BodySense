# Diagnosis production provider qualification

Status: Active production-promotion gate.

## Purpose

This runbook qualifies the physical provider behind the production logical model `bodysense-diagnosis` without changing the production Champion or sending public user traffic through a new Diagnosis configuration.

Current contract:

```text
Diagnosis Agent configuration = diag-config-3f64de162dc937ee  # immutable v10
staging physical model        = openai/gemini-3.7-flash
production physical model     = openai/mimo-v2.5-pro
```

Staging provider acceptance cannot authorize a different production provider. Production requires its own runtime preflight, route attestation, and 20/20 v10 provider acceptance.

## Safety boundary

- Do not change the production Diagnosis Champion during provider qualification.
- Do not use `prod-latest` or another mutable image for qualification. Use an immutable `repository@sha256:digest` AI image that contains the accepted v10 runtime and qualification corpus.
- The one-off qualification container connects only to the existing internal LiteLLM gateway.
- The qualification script does not call the public API and does not write Diagnosis analyses to PostgreSQL.
- Provider credentials remain inside the production host/runtime. Reports contain only credential-presence booleans and non-sensitive route metadata.
- A successful fallback must not count as primary-provider acceptance.

## 1. Runtime preflight

On the production host:

```bash
cd /opt/bodysense
python3 scripts/diagnosis-production-provider-preflight.py \
  --root /opt/bodysense \
  --json-output apps/ai-service/data/evals/reports/diagnosis_production_provider_preflight.json
```

The report verifies:

- LiteLLM gateway health;
- `MIMO_API_KEY` is non-empty without exposing its value;
- fallback credential presence;
- configured primary/fallback model identities;
- a real `bodysense-diagnosis` gateway probe;
- `x-litellm-model-name` equals `openai/mimo-v2.5-pro`;
- `x-litellm-attempted-fallbacks=0`.

To fail closed when the primary route is not ready:

```bash
python3 scripts/diagnosis-production-provider-preflight.py \
  --root /opt/bodysense \
  --require-ready
```

No 20-sample qualification should start unless this passes.

## 2. Run the isolated 20-sample qualification

Resolve the immutable accepted AI image digest first. The value must look like:

```text
crpi-.../bodysense/bodysense-ai-service@sha256:<digest>
```

Then on the production host:

```bash
cd /opt/bodysense
AI_IMAGE='<immutable repository@sha256:digest>' \
  scripts/run-diagnosis-production-provider-qualification.sh
```

The orchestrator:

1. runs the production provider preflight with `--require-ready`;
2. creates a temporary mode-600 env file containing only the internal LiteLLM URL/key;
3. starts a one-off accepted v10 AI image on the existing production Docker network;
4. runs 20 paced `structured_capture_only` Diagnosis executions;
5. requires 100% success with zero provider errors, contract failures, governance rejections, and configuration mismatches;
6. reruns the route preflight after the sample;
7. writes before/after route attestation into the acceptance report;
8. rejects the report if either attestation used a fallback.

Required artifact:

```text
apps/ai-service/data/evals/reports/diagnosis_v10_production_provider_acceptance.json
```

## 3. Recompute production promotion readiness

After copying the production-candidate evidence back into the repository:

```bash
cd apps/ai-service
uv run --extra dev python scripts/run_diagnosis_production_promotion_readiness.py
```

A promotion-ready result requires:

```text
decision = PROMOTE
ready_for_production = true
```

The evaluator independently requires:

- v10 final acceptance;
- DGS-SAFE-090 historical/replay/rollback acceptance;
- production runtime preflight ready;
- production-candidate report for the exact configured production model;
- before/after route attestation proving the primary model and zero fallback;
- at least 20 samples and 100% success.

## Current observed production state

The production runtime preflight performed after DGS-SAFE-090 found:

```text
LiteLLM gateway                  healthy
configured primary              openai/mimo-v2.5-pro
MIMO_API_KEY                     missing
configured fallback             openrouter/deepseek/deepseek-chat
fallback credential             present but expired
bodysense-diagnosis probe        HTTP 500
ready_for_primary_qualification false
```

The gateway error path is:

```text
MiMo primary -> authentication unavailable
OpenRouter fallback -> 401 API key expired
```

Therefore production promotion remains HOLD. This is a provider-credential/runtime blocker, not a Diagnosis v10 correctness regression.

## Recovery order

1. Configure a valid `MIMO_API_KEY` in the production host secret environment.
2. Preferably refresh the expired OpenRouter fallback credential as a separate resilience repair.
3. Recreate only the LiteLLM gateway if needed for new environment values.
4. Run the runtime preflight until the logical probe proves MiMo with zero fallback.
5. Run the isolated 20-sample production provider qualification.
6. Recompute production promotion readiness.
7. Make the production Champion promotion as a separate explicit release decision only after readiness becomes PROMOTE.
