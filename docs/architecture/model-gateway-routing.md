# Model Gateway Routing Architecture

Status: current architecture after Diagnosis Agent platform Phase 10 (2026-08-20)

## One model-execution topology

BodySense now has one LLM provider-routing topology:

```text
Business role / utility
        |
        | selects only a BodySense logical route or immutable AgentConfiguration
        v
Python AI service
  - AIService (provider-neutral request/stream adapter)
  - PydanticAI typed Agents
        |
        | LITELLM_BASE_URL + LITELLM_API_KEY only
        v
Standalone LiteLLM gateway
  - physical provider credentials
  - provider/model selection
  - retry/fallback
        |
        v
MiMo / OpenRouter physical models
```

Application code does not construct physical provider candidates or fallback
chains. `AIService` remains as a business-facing transport facade because
Consultation/RAG/Title/Posture already depend on its stable request and streaming
contracts; its former provider-routing responsibility has been removed.

## Logical groups

The repository-owned logical route registry in `src/ai/gateway.py` maps business
intent to gateway model groups:

| Business route | LiteLLM logical group | Typical consumer |
| --- | --- | --- |
| `consultation.reply` | `bodysense-consultation` | Consultation runtime + tools |
| `assessment.generate` | `bodysense-structured` | typed Assessment Agent |
| `treatment.proposal` | `bodysense-structured` | typed Treatment Agent |
| `knowledge.curate` | `bodysense-structured` | knowledge refinement |
| `knowledge.split` | `bodysense-structured` | semantic video splitting |
| `conversation.title` | `bodysense-text` | conversation title generation |
| `posture.analyze` | `bodysense-posture` | multimodal posture analysis |

Typed Diagnosis and Treatment are stricter than generic utility routes: each immutable
`AgentConfiguration` carries its logical model and model-group revision. Go chooses
the exact configuration that serves the request; Python validates the manifest
revisions and creates one PydanticAI model against the internal gateway. Diagnosis
uses `bodysense-diagnosis`; Treatment v1 uses `bodysense-structured`.

## Ownership

### Application / Agent layer owns

- business role and logical route;
- immutable Diagnosis and Treatment AgentConfiguration;
- prompt/tool/evidence/governance revisions;
- generation parameters that are part of Agent behavior identity;
- Go DecisionAuthority, rollout selection, and business safety.

### LiteLLM owns

- physical provider/model mapping;
- physical LLM credentials;
- provider retry/fallback;
- gateway transport availability and provider telemetry.

LiteLLM is not allowed to become BodySense safety or promotion authority.

## Credential isolation

`ai-service` receives only:

```text
LITELLM_BASE_URL
LITELLM_API_KEY
```

for LLM traffic. `MIMO_API_KEY`, `MIMO_BASE_URL`, and `OPENROUTER_API_KEY` are
injected into `litellm-gateway`, not into `ai-service`.

Embedding and ASR credentials are separate non-LLM subsystems and are not covered
by this routing boundary (`EMBEDDING_*`, `ASR_*`).

## Retired implementation

Phase 10 physically deleted:

```text
src/ai/config.py
src/ai/router.py
src/ai/providers/
src/ai/pydantic_model.py
src/ai/diagnosis_model_boundary.py
src/config/models.yaml
DIAGNOSIS_MODEL_BACKEND
DIAGNOSIS_AGENT_CONFIGURATION_ID compatibility alias
llm.json / llm.text business routing semantics
```

PydanticAI `FallbackModel` is no longer constructed by BodySense. Provider
fallback is exercised through the real LiteLLM container smoke instead.

## Immutable older Agent configurations

Diagnosis v1/v2 manifests remain resolvable on purpose. v1 is still the repository
default Champion until the explicit Phase-9 observation gates justify advancing a
real environment; it is also the rollback/replay reference. Keeping an immutable
old Agent configuration is not keeping an old provider-routing stack: all of its
model execution now goes through the same LiteLLM gateway topology.

The v1 bare `search_evidence` tool policy therefore remains reachable only because
that configuration is a real rollout artifact. Newer v2/v3 configurations use the
typed EvidenceGap acquisition contract. It must not be deleted merely to make a
migration ledger look empty; it can be retired from serving after an actual
promotion, while historical replay can continue to resolve its immutable manifest.

## Verification

Structural tests fail if retired router artifacts reappear or if `ai-service`
Compose receives physical LLM provider credentials. Repository release validation
also executes a real LiteLLM container smoke that proves:

- authenticated gateway access;
- Diagnosis primary-to-fallback behavior;
- PydanticAI Diagnosis adapter through the gateway;
- AIService requests across consultation / structured / text / posture groups;
- streaming through the gateway.

This architecture is the single model-routing truth for current code.
