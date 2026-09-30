# Diagnosis production provider qualification

Status: Active production-promotion gate.

## Purpose

This runbook qualifies the physical provider behind production `bodysense-diagnosis` without changing the production Diagnosis Champion and without sending public user traffic through an unqualified configuration.

MiMo is retired from the current non-vision LLM route. The target production contract is now:

```text
Diagnosis Agent configuration = diag-config-3f64de162dc937ee  # immutable v10
staging physical model        = openai/gemini-3.7-flash
production physical model     = openai/gemini-3.7-flash
primary credential boundary   = PRIMARY_LLM_BASE_URL / PRIMARY_LLM_API_KEY
```

The production configuration intentionally uses generic `PRIMARY_LLM_*` names so another provider migration does not require renaming the runtime contract.

## Safety boundary

- Do not change the production Diagnosis Champion during provider qualification.
- Do not use a mutable AI image. Use an immutable `repository@sha256:digest` containing the accepted v10 runtime and qualification corpus.
- The one-off qualification container connects only to the internal production LiteLLM gateway.
- It does not call the public API and does not persist Diagnosis analyses.
- Provider credentials stay on the production host. Reports contain only credential-presence booleans and non-sensitive route metadata.
- A successful fallback does **not** count as primary-provider acceptance.
- The physical route must be attested from LiteLLM response headers before and after the 20-sample run.

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
- `PRIMARY_LLM_API_KEY` is non-empty without exposing its value;
- fallback credential presence;
- configured primary/fallback model identities;
- a real `bodysense-diagnosis` gateway probe;
- `x-litellm-model-name=openai/gemini-3.7-flash`;
- `x-litellm-model-group=bodysense-diagnosis`;
- `x-litellm-attempted-fallbacks=0`.

Fail closed when the primary route is not ready:

```bash
python3 scripts/diagnosis-production-provider-preflight.py \
  --root /opt/bodysense \
  --require-ready
```

No 20-sample qualification may start unless this passes.

## 2. Isolated 20-sample qualification

Resolve the immutable accepted AI image digest:

```text
crpi-.../bodysense/bodysense-ai-service@sha256:<digest>
```

Then run:

```bash
cd /opt/bodysense
AI_IMAGE='<immutable repository@sha256:digest>' \
  scripts/run-diagnosis-production-provider-qualification.sh
```

The orchestrator:

1. runs production provider preflight with `--require-ready`;
2. creates a temporary mode-600 env file containing only the internal LiteLLM URL/key;
3. starts the accepted v10 AI image on the production Docker network;
4. runs 20 paced `structured_capture_only` Diagnosis executions;
5. requires 100% success;
6. requires zero provider errors, contract failures, governance rejections, and configuration mismatches;
7. reruns route preflight;
8. writes before/after route attestation into the acceptance artifact;
9. rejects the result if either attestation used a fallback or a different physical model.

Required artifact:

```text
apps/ai-service/data/evals/reports/diagnosis_v10_production_provider_acceptance.json
```

## 3. Recompute production readiness

```bash
cd apps/ai-service
uv run --extra dev python scripts/run_diagnosis_production_promotion_readiness.py
```

Promotion-ready requires:

```text
decision = PROMOTE
ready_for_production = true
```

The evaluator independently requires:

- v10 final acceptance;
- DGS-SAFE-090 identity/replay/rollback acceptance;
- production runtime preflight ready;
- a production-candidate report for the exact configured production model;
- before/after route attestation proving the primary physical model with zero fallback;
- at least 20 samples and 100% success.

## Migration from retired MiMo route

The production runtime observed before this migration still used the old tracked route:

```text
configured primary              openai/mimo-v2.5-pro
MIMO_API_KEY                     missing
configured fallback             openrouter/deepseek/deepseek-chat
fallback credential             present but expired
bodysense-diagnosis probe        HTTP 500
```

That observation is retained only as historical migration evidence.

The current target route removes MiMo entirely:

```text
bodysense-diagnosis
bodysense-consultation
bodysense-structured
bodysense-text
    -> openai/gemini-3.7-flash
    -> PRIMARY_LLM_BASE_URL / PRIMARY_LLM_API_KEY
```

The OpenRouter fallback remains logically separate and should also receive a valid credential, but fallback health does not substitute for primary qualification.

## Required operational order

1. Merge and publish the MiMo-retirement runtime configuration.
2. Configure `PRIMARY_LLM_API_KEY` on the production host; configure `PRIMARY_LLM_BASE_URL` only if it differs from the tracked default.
3. Refresh the OpenRouter fallback credential separately.
4. Reconcile/recreate only LiteLLM when applying provider env changes.
5. Run preflight until the logical probe proves `openai/gemini-3.7-flash` with zero fallback.
6. Run the isolated 20-sample production provider qualification.
7. Recompute production readiness.
8. Promote the production Diagnosis Champion only as a separate explicit release operation after readiness becomes PROMOTE.
