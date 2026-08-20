# ADR 0005: Adopt a Standalone LiteLLM Model Gateway for Agent Model Execution

## Status

Accepted — implemented and legacy routing retired in Phase 10 (2026-08-20)

## Date

2026-08-19

## Context

Diagnosis currently reaches physical model providers through application-owned routing (`llm.json`, `models.yaml`, `src/ai/pydantic_model.py`, and PydanticAI `FallbackModel`). That mixes two different responsibilities:

- BodySense business/Agent behavior selection; and
- provider/model transport, retry, fallback, credentials, and usage telemetry.

The Diagnosis north-star architecture treats already-proven domain invariants as protected contracts, but does not treat the legacy provider-routing implementation as protected.

## Decision

Introduce a standalone `litellm-gateway` service as the only physical-provider boundary for the Diagnosis Agent platform.

The ownership split is:

```text
Go API
  durable domain truth / decision authority / deployment policy
    -> Python AI Service
       PydanticAI Agent / tools / evidence control / semantic governance
         -> LiteLLM Gateway
            logical model group / provider retry / fallback / rate / spend / telemetry
              -> physical providers/models
```

LiteLLM owns provider credentials and physical-provider routing. Python Agent code requests a logical model group and does not construct provider clients. Go remains the owner of business authority and durable health state.

The first logical model group was `bodysense-diagnosis`. Phase 10 generalized the same boundary to `bodysense-consultation`, `bodysense-structured`, `bodysense-text`, and `bodysense-posture`, allowing the application-owned physical router to be deleted repository-wide.

## Protected contracts

This decision must preserve:

1. BodyState is Go-owned durable health truth.
2. Diagnosis pins an exact BodyState revision.
3. Active durable safety state can block ordinary Diagnosis.
4. Go assigns and persists durable DiagnosisAnalysis and candidate identities.
5. Python owns Agent reasoning, not durable business authorization.
6. Web remains a projection consumer.
7. Diagnosis does not silently create Treatment.

## Explicitly retirable implementation

After cutover and parity verification, the following Diagnosis routing responsibilities must be deleted rather than carried indefinitely:

- `use_case="llm.json"` as Diagnosis business intent;
- `src/config/models.yaml` as Diagnosis provider-routing truth;
- `src/ai/pydantic_model.py` physical provider construction;
- PydanticAI `FallbackModel` as the BodySense provider fallback layer;
- provider credentials in the `ai-service` container.

## Consequences

### Positive

- Agent qualification can identify an immutable BodySense Agent configuration independently from physical provider placement.
- Provider retry/fallback and telemetry have one operational boundary.
- Provider secrets are isolated from Agent runtime code.
- Diagnosis can later change model placement without changing business policy identity.

### Cost

- Docker topology gains one service.
- Local and production deployment require internal gateway authentication and health checks.
- Migration must temporarily preserve old routing only long enough to prove cutover and rollback.

## Rollback

During migration, the previous routing path may coexist only behind an explicit compatibility seam. Rollback is permitted by reverting the service cutover; it must not mutate historical Diagnosis analyses or their BodyState revision pins.

## Implementation outcome

All follow-ups are complete. The repository-wide call-site migration allowed deletion of `models.yaml`, `ModelRouter`, physical provider clients, `pydantic_model.py`, PydanticAI `FallbackModel` construction, and the temporary Diagnosis backend switch. `AIService` remains only as a provider-neutral business transport facade over LiteLLM logical groups. See [Model Gateway Routing Architecture](../architecture/model-gateway-routing.md).
